package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

//go:embed static
var staticFS embed.FS

func main() {
	fmt.Println()
	fmt.Println("  云雀 Agent 安装向导 (GUI)")
	fmt.Println("  正在启动图形界面...")

	mux := http.NewServeMux()

	// Serve embedded static files
	sub, _ := fs.Sub(staticFS, "static")
	mux.Handle("/", http.FileServer(http.FS(sub)))

	// API: save config + run checks
	mux.HandleFunc("/api/save", handleSave)

	// API: shutdown setup server
	mux.HandleFunc("/api/shutdown", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true}`))
		go func() {
			time.Sleep(500 * time.Millisecond)
			os.Exit(0)
		}()
	})

	// Find available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Printf("  无法监听端口: %v\n", err)
		os.Exit(1)
	}
	addr := listener.Addr().String()
	url := "http://" + addr

	fmt.Printf("  向导已启动: %s\n", url)
	fmt.Println("  浏览器应自动打开，如未打开请手动访问上述地址")
	fmt.Println("  按 Ctrl+C 退出")

	// Auto-open browser
	go openBrowser(url)

	srv := &http.Server{Handler: mux}
	srv.Serve(listener)
}

type saveRequest map[string]string

type checkResult struct {
	OK   bool   `json:"ok"`
	Warn bool   `json:"warn,omitempty"`
	Msg  string `json:"msg"`
}

type saveResponse struct {
	Success bool          `json:"success"`
	Checks  []checkResult `json:"checks"`
}

func handleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	var cfg saveRequest
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	var checks []checkResult
	allOK := true

	// 1. Save .env
	err := writeEnvFile(cfg)
	if err != nil {
		checks = append(checks, checkResult{OK: false, Msg: "写入失败: " + err.Error()})
		allOK = false
	} else {
		checks = append(checks, checkResult{OK: true, Msg: ".env 已保存"})
	}

	// 2. Create data directories
	dirs := []string{"data", "data/plugins", "data/persona", "data/memory/daily", "data/sessions", "data/knowledge", "data/cron", "data/i18n", "data/iterate", "data/audit", "data/skills", "data/clawhub_cache"}
	dirOK := true
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			dirOK = false
		}
	}
	if dirOK {
		checks = append(checks, checkResult{OK: true, Msg: "目录已就绪"})
	} else {
		checks = append(checks, checkResult{OK: false, Warn: true, Msg: "部分目录创建失败"})
	}

	// 3. Create default persona
	personaFile := filepath.Join("data", "persona", "default.txt")
	if _, err := os.Stat(personaFile); os.IsNotExist(err) {
		content := "你是云雀助手，一个智能、友好、乐于助人的 AI Agent。\n请用中文回答用户的问题。"
		os.WriteFile(personaFile, []byte(content), 0644)
	}
	checks = append(checks, checkResult{OK: true, Msg: "默认人设已就绪"})

	// 4. Test API connectivity
	apiErr := testAPI(cfg["LLM_BASE_URL"], cfg["LLM_API_KEY"])
	if apiErr != nil {
		checks = append(checks, checkResult{OK: false, Warn: true, Msg: apiErr.Error()})
	} else {
		checks = append(checks, checkResult{OK: true, Msg: "API 连接正常"})
	}

	json.NewEncoder(w).Encode(saveResponse{Success: allOK, Checks: checks})
}

func writeEnvFile(values saveRequest) error {
	order := []struct {
		key     string
		comment string
	}{
		// Core LLM
		{"LLM_API_KEY", "# 大模型 API 密钥（必填）"},
		{"LLM_BASE_URL", "# 大模型 API 地址"},
		{"LLM_MODEL", "# 使用的模型名称"},
		{"AGENT_ADDR", "# Agent 监听地址（默认 :9090）"},
		{"JWT_SECRET", "# API 认证密钥（自动生成）"},
		{"DATABASE_URL", "# 数据库连接串（留空使用内置存储）"},
		// Multi-model (Phase F)
		{"LLM_FAST_URL", "# 快速模型 API 地址（可选）"},
		{"LLM_FAST_KEY", "# 快速模型 API Key（留空使用主 Key）"},
		{"LLM_FAST_MODEL", "# 快速模型名称（可选）"},
		{"LLM_EXPERT_URL", "# 专家模型 API 地址（可选）"},
		{"LLM_EXPERT_KEY", "# 专家模型 API Key（留空使用主 Key）"},
		{"LLM_EXPERT_MODEL", "# 专家模型名称（可选）"},
		// Advanced features
		{"HEARTBEAT_ENABLED", "# 心跳自检（true/false）"},
		{"HEARTBEAT_INTERVAL", "# 心跳间隔（分钟）"},
		{"SELF_ITERATE_ENABLED", "# 受控自我迭代（true/false）"},
		// Channels
		{"TELEGRAM_BOT_TOKEN", "# Telegram Bot Token（可选）"},
		{"FEISHU_APP_ID", "# 飞书 App ID（可选）"},
		{"FEISHU_APP_SECRET", "# 飞书 App Secret（可选）"},
		{"DISCORD_BOT_TOKEN", "# Discord Bot Token（可选）"},
		{"SLACK_BOT_TOKEN", "# Slack Bot Token（可选）"},
		{"SLACK_SIGNING_SECRET", "# Slack Signing Secret（可选）"},
	}

	// Auto-generate JWT secret if not provided
	if values["JWT_SECRET"] == "" {
		t := time.Now().UnixNano()
		chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		secret := make([]byte, 32)
		for i := range secret {
			secret[i] = chars[(t+int64(i)*31)%int64(len(chars))]
		}
		values["JWT_SECRET"] = string(secret)
	}

	var lines []string
	lines = append(lines, "# ╔════════════════════════════════════╗")
	lines = append(lines, "# ║  云雀 Agent 配置文件               ║")
	lines = append(lines, "# ║  由 setup 向导自动生成              ║")
	lines = append(lines, "# ╚════════════════════════════════════╝")
	lines = append(lines, "")

	for _, item := range order {
		v := values[item.key]
		lines = append(lines, item.comment)
		if v == "" {
			lines = append(lines, fmt.Sprintf("# %s=", item.key))
		} else {
			lines = append(lines, fmt.Sprintf("%s=%s", item.key, v))
		}
		lines = append(lines, "")
	}

	lines = append(lines, "# ── 高级设置（按需取消注释） ──")
	lines = append(lines, "")
	lines = append(lines, "# EMBED_BASE_URL=https://api-ai.gitcode.com/v1")
	lines = append(lines, "# EMBED_MODEL=text-embedding-3-small")
	lines = append(lines, "# EMBED_DIMS=1536")
	lines = append(lines, "# RATE_LIMIT=60")
	lines = append(lines, "# ALLOWED_ORIGINS=http://localhost:3000")
	lines = append(lines, "# PERSONA_DIR=data/personas")
	lines = append(lines, "# SEARXNG_URL=http://localhost:8888")

	return os.WriteFile(".env", []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

func testAPI(baseURL, apiKey string) error {
	if baseURL == "" {
		return fmt.Errorf("未设置 API 地址")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", strings.TrimRight(baseURL, "/")+"/models", nil)
	if err != nil {
		return err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("无法连接: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return fmt.Errorf("API Key 无效 (401)")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("API 返回错误: %d", resp.StatusCode)
	}
	return nil
}

func openBrowser(url string) {
	time.Sleep(300 * time.Millisecond)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}
