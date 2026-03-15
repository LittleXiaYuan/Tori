package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

// configGroup defines a group of env vars shown in the Settings UI.
type configGroup struct {
	Key     string        `json:"key"`
	Label   string        `json:"label"`
	LabelZh string        `json:"label_zh"`
	Fields  []configField `json:"fields"`
}

type configField struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	LabelZh     string   `json:"label_zh"`
	Type        string   `json:"type"` // text, password, select, number
	Placeholder string   `json:"placeholder,omitempty"`
	Options     []string `json:"options,omitempty"` // for select type
	Sensitive   bool     `json:"sensitive,omitempty"`
	Required    bool     `json:"required,omitempty"`
}

// configSchema defines the UI structure for the settings page.
var configSchema = []configGroup{
	{
		Key: "core", Label: "Core LLM", LabelZh: "核心大模型",
		Fields: []configField{
			{Key: "LLM_BASE_URL", Label: "API Base URL", LabelZh: "API 地址", Type: "text", Placeholder: "https://api.openai.com/v1", Required: true},
			{Key: "LLM_API_KEY", Label: "API Key", LabelZh: "API 密钥", Type: "password", Placeholder: "sk-...", Sensitive: true, Required: true},
			{Key: "LLM_MODEL", Label: "Model", LabelZh: "模型名称", Type: "text", Placeholder: "gpt-4.1-mini"},
			{Key: "AGENT_ADDR", Label: "Listen Address", LabelZh: "监听地址", Type: "text", Placeholder: ":9090"},
		},
	},
	{
		Key: "multimodel", Label: "Multi-Model Pool", LabelZh: "多模型池",
		Fields: []configField{
			{Key: "LLM_FAST_URL", Label: "Fast Model URL", LabelZh: "快速模型地址", Type: "text"},
			{Key: "LLM_FAST_KEY", Label: "Fast Model Key", LabelZh: "快速模型密钥", Type: "password", Sensitive: true},
			{Key: "LLM_FAST_MODEL", Label: "Fast Model", LabelZh: "快速模型名称", Type: "text", Placeholder: "gpt-4.1-nano"},
			{Key: "LLM_EXPERT_URL", Label: "Expert Model URL", LabelZh: "专家模型地址", Type: "text"},
			{Key: "LLM_EXPERT_KEY", Label: "Expert Model Key", LabelZh: "专家模型密钥", Type: "password", Sensitive: true},
			{Key: "LLM_EXPERT_MODEL", Label: "Expert Model", LabelZh: "专家模型名称", Type: "text", Placeholder: "gpt-4.1"},
		},
	},
	{
		Key: "advanced", Label: "Advanced Features", LabelZh: "高级功能",
		Fields: []configField{
			{Key: "HEARTBEAT_ENABLED", Label: "Heartbeat", LabelZh: "心跳自检", Type: "select", Options: []string{"true", "false"}},
			{Key: "HEARTBEAT_INTERVAL", Label: "Heartbeat Interval (min)", LabelZh: "心跳间隔（分钟）", Type: "number", Placeholder: "30"},
			{Key: "SELF_ITERATE_ENABLED", Label: "Self-Iterate", LabelZh: "自我迭代", Type: "select", Options: []string{"true", "false"}},
			{Key: "SELF_ITERATE_TOKEN_BUDGET", Label: "Iterate Token Budget", LabelZh: "迭代 Token 预算", Type: "number", Placeholder: "5000"},
			{Key: "SELF_ITERATE_AUTO_APPROVE", Label: "Auto-Approve Proposals", LabelZh: "自动审批提案", Type: "select", Options: []string{"false", "true"}},
		},
	},
	{
		Key: "embedding", Label: "Embedding", LabelZh: "向量嵌入",
		Fields: []configField{
			{Key: "EMBED_BASE_URL", Label: "Embedding API URL", LabelZh: "嵌入 API 地址", Type: "text"},
			{Key: "EMBED_MODEL", Label: "Embedding Model", LabelZh: "嵌入模型", Type: "text", Placeholder: "text-embedding-3-small"},
			{Key: "EMBED_DIMS", Label: "Dimensions", LabelZh: "向量维度", Type: "number", Placeholder: "1536"},
		},
	},
	{
		Key: "channels", Label: "Channel Integration", LabelZh: "频道集成",
		Fields: []configField{
			{Key: "TELEGRAM_BOT_TOKEN", Label: "Telegram Bot Token", LabelZh: "Telegram Bot Token", Type: "password", Sensitive: true},
			{Key: "FEISHU_APP_ID", Label: "Feishu App ID", LabelZh: "飞书 App ID", Type: "text"},
			{Key: "FEISHU_APP_SECRET", Label: "Feishu App Secret", LabelZh: "飞书 App Secret", Type: "password", Sensitive: true},
			{Key: "DISCORD_BOT_TOKEN", Label: "Discord Bot Token", LabelZh: "Discord Bot Token", Type: "password", Sensitive: true},
			{Key: "SLACK_BOT_TOKEN", Label: "Slack Bot Token", LabelZh: "Slack Bot Token", Type: "password", Sensitive: true},
			{Key: "SLACK_SIGNING_SECRET", Label: "Slack Signing Secret", LabelZh: "Slack Signing Secret", Type: "password", Sensitive: true},
			{Key: "QQ_APP_ID", Label: "QQ Bot App ID", LabelZh: "QQ 机器人 AppID", Type: "text"},
			{Key: "QQ_APP_SECRET", Label: "QQ Bot App Secret", LabelZh: "QQ 机器人 AppSecret", Type: "password", Sensitive: true},
			{Key: "QQ_SANDBOX", Label: "QQ Sandbox Mode", LabelZh: "QQ 沙箱模式", Type: "select", Options: []string{"false", "true"}},
		},
	},
	{
		Key: "security", Label: "Security", LabelZh: "安全",
		Fields: []configField{
			{Key: "JWT_SECRET", Label: "JWT Secret", LabelZh: "JWT 密钥", Type: "password", Sensitive: true},
			{Key: "RATE_LIMIT", Label: "Rate Limit (req/min)", LabelZh: "速率限制（请求/分钟）", Type: "number", Placeholder: "60"},
			{Key: "ALLOWED_ORIGINS", Label: "CORS Origins", LabelZh: "CORS 来源", Type: "text", Placeholder: "*"},
		},
	},
	{
		Key: "emotion", Label: "Emotion & Sticker", LabelZh: "情绪与贴图",
		Fields: []configField{
			{Key: "EMOTION_ENABLED", Label: "Emotion Analysis", LabelZh: "情绪分析", Type: "select", Options: []string{"true", "false"}},
			{Key: "EMOTION_STICKER_FILE", Label: "Custom Sticker Map", LabelZh: "自定义贴图映射文件", Type: "text", Placeholder: "data/stickers.json"},
		},
	},
	{
		Key: "other", Label: "Other", LabelZh: "其他",
		Fields: []configField{
			{Key: "DATABASE_URL", Label: "Database URL", LabelZh: "数据库连接串", Type: "text"},
			{Key: "SEARXNG_URL", Label: "SearXNG URL", LabelZh: "SearXNG 搜索地址", Type: "text"},
			{Key: "PERSONA_DIR", Label: "Persona Directory", LabelZh: "人格目录", Type: "text", Placeholder: "data/personas"},
			{Key: "OPEN_BROWSER", Label: "Auto-Open Browser", LabelZh: "自动打开浏览器", Type: "select", Options: []string{"true", "false"}},
		},
	},
}

// handleSettingsSchema returns the config schema (groups + fields) for the UI.
func (g *Gateway) handleSettingsSchema(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"groups": configSchema})
}

// handleSettingsConfig reads or writes the .env file.
func (g *Gateway) handleSettingsConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		g.handleGetConfig(w, r)
	case http.MethodPut:
		g.handlePutConfig(w, r)
	default:
		http.Error(w, "GET or PUT only", http.StatusMethodNotAllowed)
	}
}

func (g *Gateway) handleGetConfig(w http.ResponseWriter, _ *http.Request) {
	values := readEnvFile()
	// Mask sensitive fields
	for _, group := range configSchema {
		for _, f := range group.Fields {
			if f.Sensitive {
				if v, ok := values[f.Key]; ok && v != "" {
					if len(v) <= 8 {
						values[f.Key] = "****"
					} else {
						values[f.Key] = v[:4] + "****" + v[len(v)-4:]
					}
				}
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"values": values})
}

func (g *Gateway) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Values map[string]string `json:"values"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Merge with existing values (don't overwrite masked sensitive fields)
	existing := readEnvFile()
	sensitiveKeys := map[string]bool{}
	for _, group := range configSchema {
		for _, f := range group.Fields {
			if f.Sensitive {
				sensitiveKeys[f.Key] = true
			}
		}
	}

	for k, v := range req.Values {
		if sensitiveKeys[k] && (v == "" || strings.Contains(v, "****")) {
			// Don't overwrite with masked value — keep existing
			continue
		}
		existing[k] = v
	}

	if err := writeEnvFile(existing); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success":          true,
		"restart_required": true,
		"message":          "配置已保存。部分配置需要重启生效。",
	})
}

// handleSettingsCheck checks if the system is set up (for first-run detection).
func (g *Gateway) handleSettingsCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}

	envExists := false
	if _, err := os.Stat(".env"); err == nil {
		envExists = true
	}

	values := readEnvFile()
	hasLLMKey := values["LLM_API_KEY"] != ""
	hasLLMURL := values["LLM_BASE_URL"] != ""

	// Test LLM connectivity
	apiOK := false
	if hasLLMKey && hasLLMURL {
		apiOK = testLLMAPI(values["LLM_BASE_URL"], values["LLM_API_KEY"])
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"env_exists":   envExists,
		"has_llm_key":  hasLLMKey,
		"has_llm_url":  hasLLMURL,
		"api_ok":       apiOK,
		"setup_needed": !envExists || !hasLLMKey,
	})
}

// --- helpers ---

func readEnvFile() map[string]string {
	values := make(map[string]string)
	data, err := os.ReadFile(".env")
	if err != nil {
		return values
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			values[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return values
}

func writeEnvFile(values map[string]string) error {
	var lines []string
	lines = append(lines, "# ╔════════════════════════════════════╗")
	lines = append(lines, "# ║  云雀 Agent 配置文件               ║")
	lines = append(lines, "# ║  由 Settings 页面管理              ║")
	lines = append(lines, "# ╚════════════════════════════════════╝")
	lines = append(lines, "")

	// Write groups in schema order, then any extra keys
	written := map[string]bool{}
	for _, group := range configSchema {
		lines = append(lines, fmt.Sprintf("# ── %s ──", group.Label))
		for _, f := range group.Fields {
			v := values[f.Key]
			if v == "" {
				lines = append(lines, fmt.Sprintf("# %s=", f.Key))
			} else {
				lines = append(lines, fmt.Sprintf("%s=%s", f.Key, v))
			}
			written[f.Key] = true
		}
		lines = append(lines, "")
	}

	// Write any extra keys not in the schema
	var extras []string
	for k := range values {
		if !written[k] && values[k] != "" {
			extras = append(extras, k)
		}
	}
	if len(extras) > 0 {
		sort.Strings(extras)
		lines = append(lines, "# ── Extra ──")
		for _, k := range extras {
			lines = append(lines, fmt.Sprintf("%s=%s", k, values[k]))
		}
		lines = append(lines, "")
	}

	return os.WriteFile(".env", []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

func testLLMAPI(baseURL, apiKey string) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", strings.TrimRight(baseURL, "/")+"/models", nil)
	if err != nil {
		return false
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 400
}
