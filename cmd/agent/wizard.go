package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"yunque-agent/internal/appdir"
)

// wizardField describes a single config field in the setup wizard.
type wizardField struct {
	key      string // env var name
	label    string // display label
	defVal   string // default value (shown in [brackets])
	required bool   // must be non-empty
	secret   bool   // mask input hint (we still show it since no raw-mode on Windows)
}

// wizardGroup is a logical group of fields.
type wizardGroup struct {
	title  string
	fields []wizardField
}

// isTerminal returns true if stdin is attached to a console/terminal.
func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// runSetupWizard runs an interactive CLI wizard and returns true if config was saved.
func runSetupWizard() bool {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════╗")
	fmt.Println("?云雀 Agent ?首次配置向导                          。")
	fmt.Println("╚══════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("  检测到尚未配置，请按提示完成初始设置。")
	fmt.Println("  ?Enter 使用 [默认值]，直接回车跳过可选项。")
	fmt.Println()

	groups := []wizardGroup{
		{
			title: "基础配置",
			fields: []wizardField{
				{"LLM_API_KEY", "LLM API Key", "", true, true},
				{"LLM_BASE_URL", "LLM API 地址", "https://api-ai.gitcode.com/v1", false, false},
				{"LLM_MODEL", "模型名称", "zai-org/GLM-5", false, false},
				{"AGENT_ADDR", "监听地址", ":9090", false, false},
			},
		},
		{
			title: "多模型路由（可选，回车跳过。",
			fields: []wizardField{
				{"LLM_FAST_MODEL", "快速模型名。", "", false, false},
				{"LLM_FAST_URL", "快速模型地址", "", false, false},
				{"LLM_FAST_KEY", "快速模型Key（留空同主Key。", "", false, true},
				{"LLM_EXPERT_MODEL", "专家模型名称", "", false, false},
				{"LLM_EXPERT_URL", "专家模型地址", "", false, false},
				{"LLM_EXPERT_KEY", "专家模型 Key（留空同主Key。", "", false, true},
			},
		},
		{
			title: "存储（可选）",
			fields: []wizardField{
				{"STORAGE_MODE", "存储模式（ledger=持久?memory=内存。", "ledger", false, false},
				{"LEDGER_DB_PATH", "Ledger 数据库路。", filepath.Join(appdir.DataDir(), "ledger", "ledger.db"), false, false},
			},
		},
		{
			title: "渠道集成（可选，回车跳过。",
			fields: []wizardField{
				{"TELEGRAM_BOT_TOKEN", "Telegram Bot Token", "", false, true},
				{"FEISHU_APP_ID", "飞书 App ID", "", false, false},
				{"FEISHU_APP_SECRET", "飞书 App Secret", "", false, true},
				{"DISCORD_BOT_TOKEN", "Discord Bot Token", "", false, true},
				{"SLACK_BOT_TOKEN", "Slack Bot Token", "", false, true},
			},
		},
	}

	values := make(map[string]string)

	for _, g := range groups {
		fmt.Printf("  ── %s ─────────────────────────────\n", g.title)
		for _, f := range g.fields {
			for {
				prompt := "  " + f.label
				if f.defVal != "" {
					prompt += fmt.Sprintf(" [%s]", f.defVal)
				} else if f.required {
					prompt += "（必填）"
				}
				prompt += ": "
				fmt.Print(prompt)

				if !scanner.Scan() {
					fmt.Println("\n  已取消。")
					return false
				}
				input := strings.TrimSpace(scanner.Text())

				if input == "" && f.required {
					fmt.Println("    ⚠️ 此项为必填，请输入内容。")
					continue
				}
				if input == "" {
					input = f.defVal
				}
				if input != "" {
					values[f.key] = input
				}
				break
			}
		}
		fmt.Println()
	}

	// Test API connectivity
	baseURL := values["LLM_BASE_URL"]
	apiKey := values["LLM_API_KEY"]
	if baseURL != "" && apiKey != "" {
		fmt.Print("  正在测试 API 连接... ")
		if err := wizardTestAPI(baseURL, apiKey); err != nil {
			fmt.Printf("?%v\n", err)
			fmt.Println("  （可稍后?Settings 页面重新配置。")
		} else {
			fmt.Println("?连接成功")
		}
		fmt.Println()
	}

	// Auto-generate JWT secret
	if values["JWT_SECRET"] == "" {
		t := time.Now().UnixNano()
		chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		secret := make([]byte, 32)
		for i := range secret {
			secret[i] = chars[(t+int64(i)*31)%int64(len(chars))]
		}
		values["JWT_SECRET"] = string(secret)
	}

	// Save .env file
	if err := wizardSaveEnv(values); err != nil {
		fmt.Printf("  ?保存 .env 失败: %v\n", err)
		return false
	}
	fmt.Println("  ?配置已保存到 .env")

	// Create data directories
	subdirs := []string{
		"plugins", "persona", "memory/daily",
		"sessions", "knowledge", "cron", "i18n",
		"iterate", "audit", "skills", "clawhub_cache",
	}
	for _, d := range subdirs {
		appdir.Sub(d)
	}
	fmt.Printf("  ?数据目录已创? %s\n", appdir.DataDir())

	// Create default persona if missing
	personaFile := appdir.File("persona", "default.txt")
	if _, err := os.Stat(personaFile); os.IsNotExist(err) {
		os.WriteFile(personaFile, []byte("你是云雀助手，一个智能、友好、乐于助人的 AI Agent。\n请用中文回答用户的问题。"), 0644)
	}

	// Set env vars so config.Load() picks them up immediately
	for k, v := range values {
		os.Setenv(k, v)
	}

	fmt.Println()
	fmt.Println("  配置完成！正在启?Agent...")
	fmt.Println()
	return true
}

// wizardTestAPI sends a lightweight /models request to verify connectivity.
func wizardTestAPI(baseURL, apiKey string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	url := strings.TrimRight(baseURL, "/") + "/models"
	req, err := http.NewRequest("GET", url, nil)
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

// wizardSaveEnv writes a well-commented .env file from the collected values.
func wizardSaveEnv(values map[string]string) error {
	order := []struct {
		key     string
		comment string
	}{
		{"LLM_API_KEY", "# 大模型API 密钥（必填）"},
		{"LLM_BASE_URL", "# 大模型API 地址"},
		{"LLM_MODEL", "# 使用的模型名称"},
		{"AGENT_ADDR", "# Agent 监听地址"},
		{"JWT_SECRET", "# API 认证密钥（自动生成）"},
		{"DATABASE_URL", "# 数据库连接串（留空使用内置存储）"},
		{"LLM_FAST_URL", "# 快速模型API 地址（可选）"},
		{"LLM_FAST_KEY", "# 快速模型API Key（留空使用主 Key）"},
		{"LLM_FAST_MODEL", "# 快速模型名称（可选）"},
		{"LLM_EXPERT_URL", "# 专家模型 API 地址（可选）"},
		{"LLM_EXPERT_KEY", "# 专家模型 API Key（留空使用主 Key）"},
		{"LLM_EXPERT_MODEL", "# 专家模型名称（可选）"},
		{"HEARTBEAT_ENABLED", "# 心跳自检（true/false）"},
		{"HEARTBEAT_INTERVAL", "# 心跳间隔（分钟）"},
		{"SELF_ITERATE_ENABLED", "# 受控自我迭代（true/false）"},
		{"TELEGRAM_BOT_TOKEN", "# Telegram Bot Token（可选）"},
		{"FEISHU_APP_ID", "# 飞书 App ID（可选）"},
		{"FEISHU_APP_SECRET", "# 飞书 App Secret（可选）"},
		{"DISCORD_BOT_TOKEN", "# Discord Bot Token（可选）"},
		{"SLACK_BOT_TOKEN", "# Slack Bot Token（可选）"},
		{"SLACK_SIGNING_SECRET", "# Slack Signing Secret（可选）"},
	}

	var lines []string
	lines = append(lines, "# ╔════════════════════════════════════╗")
	lines = append(lines, "# ?云雀 Agent 配置文件               。")
	lines = append(lines, "# 📝 由配置向导自动生?               📝")
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

	lines = append(lines, "# ── 高级设置（按需取消注释?──")
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
