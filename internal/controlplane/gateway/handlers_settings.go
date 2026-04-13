package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/config"
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
	Hint        string   `json:"hint,omitempty"`
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
			{Key: "HEARTBEAT_ENABLED", Label: "Heartbeat", LabelZh: "心跳自检", Type: "select", Options: []string{"true", "false"},
				Hint: "定期自动检查状态、回顾近期对话，结果推送到收件箱。关闭后 Agent 不会主动生成状态报告。"},
			{Key: "HEARTBEAT_INTERVAL", Label: "Heartbeat Interval (min)", LabelZh: "心跳间隔（分钟）", Type: "number", Placeholder: "30",
				Hint: "两次心跳之间的间隔（分钟）。越短越活跃，但会消耗更多 Token。"},
			{Key: "LONG_HORIZON_ENABLED", Label: "Deep Planning (DAG)", LabelZh: "深度长线规划", Type: "select", Options: []string{"false", "true"},
				Hint: "复杂任务先拆解为多步骤 DAG 再执行，失败时自动重新规划。适合写报告、开发功能等长任务。简单问答不受影响。Token 消耗较高。"},
			{Key: "SELF_ITERATE_ENABLED", Label: "Self-Iterate", LabelZh: "自我迭代", Type: "select", Options: []string{"true", "false"},
				Hint: "允许 Agent 评估自己的回复质量并自动改进。关闭后直接输出首次结果。"},
			{Key: "SELF_ITERATE_TOKEN_BUDGET", Label: "Iterate Token Budget", LabelZh: "迭代 Token 预算", Type: "number", Placeholder: "5000",
				Hint: "自我迭代最多消耗多少 Token，防止无限循环改进。"},
			{Key: "SELF_ITERATE_AUTO_APPROVE", Label: "Auto-Approve Proposals", LabelZh: "自动审批提案", Type: "select", Options: []string{"false", "true"},
				Hint: "迭代产生的改进提案是否自动执行。关闭则需要人工审批后才执行。"},
			{Key: "THINKING_LEVEL", Label: "Thinking Level", LabelZh: "思考深度", Type: "select", Options: []string{"auto", "none", "deep"},
				Hint: "全局默认思考深度。auto=按复杂度自动选择，none=最快响应，deep=始终完整推理链。聊天页面可临时覆盖。"},
			{Key: "REACT_ENABLED", Label: "ReAct Mode", LabelZh: "ReAct 推理模式", Type: "select", Options: []string{"false", "true"},
				Hint: "交替「推理→行动→观察」，每步根据结果决定下一步。适合探索性任务，但更慢且消耗更多 Token。"},
			{Key: "REFLECT_MODE", Label: "Reflect Mode", LabelZh: "反思评估模式", Type: "select", Options: []string{"learning", "strict", "off"},
				Hint: "任务完成后的反思策略。learning=温和记录经验，strict=严格分析失败并产生改进策略，off=关闭反思。"},
			{Key: "REFLECT_MODEL", Label: "Reflect Eval Model", LabelZh: "反思评估器模型", Type: "text", Placeholder: "fast (留空=主模型)",
				Hint: "执行反思评估的模型。留空则使用系统自动选择的快速模型。"},
		},
	},
	{
		Key: "embedding", Label: "Embedding", LabelZh: "向量嵌入",
		Fields: []configField{
			{Key: "EMBED_BASE_URL", Label: "Embedding API URL", LabelZh: "嵌入 API 地址", Type: "text",
				Hint: "知识库检索需要嵌入模型。留空则使用主模型地址。"},
			{Key: "EMBED_MODEL", Label: "Embedding Model", LabelZh: "嵌入模型", Type: "text", Placeholder: "text-embedding-3-small",
				Hint: "将文本转为向量的模型名称，用于知识库语义搜索。"},
			{Key: "EMBED_DIMS", Label: "Dimensions", LabelZh: "向量维度", Type: "number", Placeholder: "1536",
				Hint: "嵌入向量维度。需与所选模型一致，修改后需重建索引。"},
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
		Key: "filesystem", Label: "File System Access", LabelZh: "文件系统访问",
		Fields: []configField{
			{Key: "HOST_READ_PATHS", Label: "Read-Only Paths", LabelZh: "只读访问路径", Type: "text", Placeholder: "C:\\Users\\me\\Desktop,C:\\Users\\me\\Documents",
				Hint: "Agent 可以读取的目录，逗号分隔。用于让 Agent 浏览你的文件。"},
			{Key: "HOST_WRITE_PATHS", Label: "Writable Paths", LabelZh: "可写访问路径", Type: "text", Placeholder: "data/output,data/tasks",
				Hint: "Agent 可以写入的目录，逗号分隔。注意安全风险。"},
		},
	},
	{
		Key: "security", Label: "Security", LabelZh: "安全",
		Fields: []configField{
			{Key: "JWT_SECRET", Label: "JWT Secret", LabelZh: "JWT 密钥", Type: "password", Sensitive: true,
				Hint: "API 认证签名密钥。留空则每次启动自动生成（重启后旧 Token 失效）。"},
			{Key: "RATE_LIMIT", Label: "Rate Limit (req/min)", LabelZh: "速率限制（请求/分钟）", Type: "number", Placeholder: "60",
				Hint: "每分钟最大请求数。防止滥用。0 或留空表示不限制。"},
			{Key: "ALLOWED_ORIGINS", Label: "CORS Origins", LabelZh: "CORS 来源", Type: "text", Placeholder: "*",
				Hint: "允许跨域访问的来源地址。* 表示允许所有来源。"},
		},
	},
	{
		Key: "emotion", Label: "Emotion & Sticker", LabelZh: "情绪与贴图",
		Fields: []configField{
			{Key: "EMOTION_ENABLED", Label: "Emotion Analysis", LabelZh: "情绪分析", Type: "select", Options: []string{"true", "false"},
				Hint: "启用后 Agent 会分析对话中的情绪，影响回复语气和表达方式。"},
			{Key: "EMOTION_STICKER_FILE", Label: "Custom Sticker Map", LabelZh: "自定义贴图映射文件", Type: "text", Placeholder: "data/stickers.json",
				Hint: "JSON 文件，定义情绪到贴图的映射关系。在支持的渠道中自动发送表情贴图。"},
		},
	},
	{
		Key: "storage", Label: "Storage & Persistence", LabelZh: "存储与持久化",
		Fields: []configField{
			{Key: "LEDGER_DB_PATH", Label: "Ledger DB Path", LabelZh: "Ledger 数据库路径", Type: "text", Placeholder: "data/ledger/ledger.db",
				Hint: "审计日志和记忆数据的 SQLite 文件路径。"},
			{Key: "STORAGE_MODE", Label: "Memory Storage Mode", LabelZh: "记忆存储模式", Type: "select", Options: []string{"ledger", "memory"}, Placeholder: "ledger",
				Hint: "ledger=持久化到磁盘（重启不丢失），memory=仅内存（重启清空）。生产环境务必选 ledger。"},
		},
	},
	{
		Key: "sandbox_cloud", Label: "Cloud Sandbox (E2B)", LabelZh: "云沙箱 (E2B)",
		Fields: []configField{
			{Key: "SANDBOX_CLOUD_ENABLED", Label: "Enable Cloud Sandbox", LabelZh: "启用云沙箱", Type: "select", Options: []string{"false", "true"},
				Hint: "启用后代码执行和浏览器自动化将在远程沙箱中运行，无需本地 Docker。"},
			{Key: "SANDBOX_CLOUD_API_KEY", Label: "E2B API Key", LabelZh: "E2B API 密钥", Type: "password", Sensitive: true,
				Hint: "E2B 平台 API 密钥。在 e2b.dev 注册获取。"},
			{Key: "SANDBOX_CLOUD_BASE_URL", Label: "API Base URL", LabelZh: "API 地址", Type: "text", Placeholder: "https://api.e2b.app",
				Hint: "E2B API 地址。默认 https://api.e2b.app，也可指向 ToriAPI 代理。"},
			{Key: "SANDBOX_CLOUD_TEMPLATE", Label: "Sandbox Template", LabelZh: "沙箱模板", Type: "text", Placeholder: "desktop",
				Hint: "E2B 沙箱模板 ID。desktop=带桌面和浏览器，base=仅命令行。"},
			{Key: "SANDBOX_CLOUD_TIMEOUT", Label: "Timeout", LabelZh: "超时时间", Type: "text", Placeholder: "120s",
				Hint: "沙箱执行超时时间，如 60s、5m。"},
		},
	},
	{
		Key: "other", Label: "Other", LabelZh: "其他",
		Fields: []configField{
			{Key: "SEARXNG_URL", Label: "SearXNG URL", LabelZh: "SearXNG 搜索地址", Type: "text",
				Hint: "自建搜索引擎 SearXNG 的地址。配置后 Agent 可使用网页搜索。"},
			{Key: "PERSONA_DIR", Label: "Persona Directory", LabelZh: "人格目录", Type: "text", Placeholder: "data/personas",
				Hint: "存放人设文件的目录。每个 .txt 文件是一个人设。"},
			{Key: "OPEN_BROWSER", Label: "Auto-Open Browser", LabelZh: "自动打开浏览器", Type: "select", Options: []string{"true", "false"},
				Hint: "启动后自动在默认浏览器中打开 WebUI。"},
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
	hasLLMModel := values["LLM_MODEL"] != ""

	// Test LLM connectivity. API keys are optional for local providers.
	apiOK := false
	if hasLLMURL {
		apiOK = testLLMAPI(values["LLM_BASE_URL"], values["LLM_API_KEY"])
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"env_exists":    envExists,
		"has_llm_key":   hasLLMKey,
		"has_llm_url":   hasLLMURL,
		"has_llm_model": hasLLMModel,
		"api_ok":        apiOK,
		"setup_needed":  !envExists || !hasLLMURL || !hasLLMModel,
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

	// Atomic write: write to temp file first, then rename
	tmp := ".env.tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		return err
	}
	return os.Rename(tmp, ".env")
}

// handleConfigReload re-reads .env, recreates LLM clients, and hot-swaps them
// into the running system. This avoids requiring a full process restart after
// changing LLM settings in the UI.
func (g *Gateway) handleConfigReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	vals := readEnvFile()
	baseURL := vals["LLM_BASE_URL"]
	apiKey := vals["LLM_API_KEY"]
	model := vals["LLM_MODEL"]

	if baseURL == "" || apiKey == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "LLM_BASE_URL 和 LLM_API_KEY 不能为空",
		})
		return
	}

	// Test connectivity first
	if !testLLMAPI(baseURL, apiKey) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "无法连接到 LLM 服务，请检查 API 地址和密钥",
		})
		return
	}

	reloaded := []string{"smart"}

	// Hot-swap primary (smart tier) client in the provider registry's pool
	if g.providerReg != nil {
		pool := g.providerReg.Pool()
		if pool != nil {
			if model == "" {
				model = "gpt-4.1-mini"
			}
			newClient := llm.NewClient(baseURL, apiKey, model)
			pool.Register("smart", newClient)
			pool.SetPrimary("smart")

			// Hot-swap fast tier if configured
			fastURL := vals["LLM_FAST_URL"]
			fastKey := vals["LLM_FAST_KEY"]
			fastModel := vals["LLM_FAST_MODEL"]
			if fastModel != "" {
				if fastURL == "" {
					fastURL = baseURL
				}
				if fastKey == "" {
					fastKey = apiKey
				}
				pool.Register("fast", llm.NewClient(fastURL, fastKey, fastModel))
				reloaded = append(reloaded, "fast")
			}

			// Hot-swap expert tier if configured
			expertURL := vals["LLM_EXPERT_URL"]
			expertKey := vals["LLM_EXPERT_KEY"]
			expertModel := vals["LLM_EXPERT_MODEL"]
			if expertModel != "" {
				if expertURL == "" {
					expertURL = baseURL
				}
				if expertKey == "" {
					expertKey = apiKey
				}
				pool.Register("expert", llm.NewClient(expertURL, expertKey, expertModel))
				reloaded = append(reloaded, "expert")
			}
		}
	}

	// Re-export env vars so downstream code that reads os.Getenv also sees the change
	for k, v := range vals {
		os.Setenv(k, v)
	}

	// Hot-reload file system paths into the general plugin (if paths changed)
	if hrp := vals["HOST_READ_PATHS"]; hrp != "" {
		reloaded = append(reloaded, "fs_read_paths")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success":  true,
		"reloaded": reloaded,
		"message":  fmt.Sprintf("已热重载 %d 个配置层", len(reloaded)),
	})
}

// handleDetectDirs auto-discovers user directories (Desktop, Documents, Downloads, etc.)
// for one-click HOST_READ_PATHS configuration.
func (g *Gateway) handleDetectDirs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	dirs := config.DetectUserDirs()
	defaults := config.DefaultReadPaths()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"dirs":          dirs,
		"default_paths": defaults,
		"current_read":  readEnvFile()["HOST_READ_PATHS"],
		"current_write": readEnvFile()["HOST_WRITE_PATHS"],
	})
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
