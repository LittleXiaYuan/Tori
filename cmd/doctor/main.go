package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const banner = `
╔══════════════════════════════════════════════════════╗
║           Tori 环境自检 (Doctor)                    ║
╚══════════════════════════════════════════════════════╝
`

type check struct {
	name string
	run  func() (string, bool)
}

func main() {
	_ = godotenv.Load()
	fmt.Print(banner)

	checks := []check{
		{"配置文件 .env", checkEnvFile},
		{"LLM API Key", checkAPIKey},
		{"LLM Base URL", checkBaseURL},
		{"LLM 模型名称", checkModel},
		{"API 连通性", checkAPIConnect},
		{"数据目录", checkDataDirs},
		{"人设文件", checkPersona},
		{"数据库配置", checkDatabase},
		{"JWT 密钥", checkJWT},
		{"Telegram (可选)", checkTelegram},
		{"飞书 (可选)", checkFeishu},
	}

	passed := 0
	warned := 0
	failed := 0

	for i, c := range checks {
		msg, ok := c.run()
		status := " OK "
		if !ok {
			// Distinguish between required failures and optional warnings
			if strings.Contains(msg, "可选") || strings.Contains(msg, "推荐") {
				status = "WARN"
				warned++
			} else {
				status = "FAIL"
				failed++
			}
		} else {
			passed++
		}
		fmt.Printf("  %s [%d/%d] %s: %s\n", status, i+1, len(checks), c.name, msg)
	}

	fmt.Println()
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("  结果: %d 通过, %d 警告, %d 失败\n", passed, warned, failed)
	fmt.Println(strings.Repeat("─", 50))

	if failed > 0 {
		fmt.Println("\n  有必填项未通过，请运行 setup 向导修复：")
		fmt.Println("   go run ./cmd/setup")
		os.Exit(1)
	} else if warned > 0 {
		fmt.Println("\n  核心配置正常，部分可选项未配置。")
		fmt.Println("   可以直接启动: go run ./cmd/agent")
	} else {
		fmt.Println("\n  All checks passed. Ready to start:")
		fmt.Println("   go run ./cmd/agent")
	}
}

func checkEnvFile() (string, bool) {
	if _, err := os.Stat(".env"); os.IsNotExist(err) {
		return "未找到 .env 文件，请运行 go run ./cmd/setup", false
	}
	return "已找到", true
}

func checkAPIKey() (string, bool) {
	key := os.Getenv("LLM_API_KEY")
	if key == "" {
		return "未设置 LLM_API_KEY", false
	}
	if len(key) < 8 {
		return "API Key 太短，请检查", false
	}
	return fmt.Sprintf("已设置 (%s...)", key[:4]), true
}

func checkBaseURL() (string, bool) {
	url := os.Getenv("LLM_BASE_URL")
	if url == "" {
		return "未设置，将使用默认值", true
	}
	if !strings.HasPrefix(url, "http") {
		return "URL 格式不正确，需要 http:// 或 https:// 开头", false
	}
	return url, true
}

func checkModel() (string, bool) {
	model := os.Getenv("LLM_MODEL")
	if model == "" {
		return "未设置，将使用默认模型", true
	}
	return model, true
}

func checkAPIConnect() (string, bool) {
	baseURL := os.Getenv("LLM_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api-ai.gitcode.com/v1"
	}
	apiKey := os.Getenv("LLM_API_KEY")

	client := &http.Client{Timeout: 10 * time.Second}

	// Try /models first, fall back to /chat/completions OPTIONS
	endpoints := []string{"/models", "/chat/completions"}
	for _, ep := range endpoints {
		req, err := http.NewRequest("GET", strings.TrimRight(baseURL, "/")+ep, nil)
		if err != nil {
			continue
		}
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Sprintf("无法连接: %v", err), false
		}
		resp.Body.Close()

		if resp.StatusCode == 401 {
			return "API Key 无效 (401)", false
		}
		// 404 on /models is normal for some providers, try next endpoint
		if resp.StatusCode == 404 {
			continue
		}
		return fmt.Sprintf("连接正常 (HTTP %d)", resp.StatusCode), true
	}

	// If all endpoints returned 404 but server was reachable, still OK
	req, err := http.NewRequest("GET", strings.TrimRight(baseURL, "/"), nil)
	if err != nil {
		return "请求构建失败", false
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("无法连接: %v", err), false
	}
	resp.Body.Close()
	if resp.StatusCode == 401 {
		return "API Key 无效 (401)", false
	}
	return fmt.Sprintf("连接正常 (服务可达, 状态码: %d)", resp.StatusCode), true
}

func checkDataDirs() (string, bool) {
	dirs := []string{"data", "data/plugins", "data/persona"}
	missing := []string{}
	for _, d := range dirs {
		if _, err := os.Stat(d); os.IsNotExist(err) {
			missing = append(missing, d)
		}
	}
	if len(missing) > 0 {
		return fmt.Sprintf("推荐创建目录: %s", strings.Join(missing, ", ")), false
	}
	return "所有目录就绪", true
}

func checkPersona() (string, bool) {
	dir := os.Getenv("PERSONA_DIR")
	if dir == "" {
		dir = "data/persona"
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "推荐运行 setup 创建默认人设文件", false
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && (strings.HasSuffix(e.Name(), ".md") || strings.HasSuffix(e.Name(), ".txt")) {
			count++
		}
	}
	if count == 0 {
		return "推荐添加人设文件到 " + dir, false
	}
	return fmt.Sprintf("找到 %d 个人设文件", count), true
}

func checkDatabase() (string, bool) {
	dbPath := os.Getenv("LEDGER_DB_PATH")
	if dbPath == "" {
		dbPath = "data/ledger/ledger.db"
	}
	return "Ledger (SQLite): " + dbPath, true
}

func checkJWT() (string, bool) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "可选：未设置，API 将无认证保护", false
	}
	if len(secret) < 16 {
		return "推荐使用更长的密钥（至少 16 字符）", false
	}
	return "已设置", true
}

func checkTelegram() (string, bool) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return "可选：未配置 Telegram 渠道", false
	}
	return "已配置", true
}

func checkFeishu() (string, bool) {
	appID := os.Getenv("FEISHU_APP_ID")
	secret := os.Getenv("FEISHU_APP_SECRET")
	if appID == "" && secret == "" {
		return "可选：未配置飞书渠道", false
	}
	if appID == "" || secret == "" {
		return "飞书配置不完整（需要同时设置 APP_ID 和 APP_SECRET）", false
	}
	return "已配置", true
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
