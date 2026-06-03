package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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
	// Tier controls visibility in the layered Settings UI:
	//   "common"   — shown by default (everyday behavior toggles, file access)
	//   "advanced" — folded under the Advanced level (the default when unset)
	//   "expert"   — folded under the Expert level (deployment/security internals)
	// Leaving it empty is intentionally treated as "advanced" by the UI, so a
	// new field is never accidentally exposed as common or buried as expert.
	Tier string `json:"tier,omitempty"`
}

// configSchema defines the UI structure for the settings page.
var configSchema = []configGroup{
	{
		Key: "core", Label: "Core LLM", LabelZh: "核心大模型",
		Fields: []configField{
			{Key: "LLM_BASE_URL", Label: "API Base URL", LabelZh: "API 地址", Type: "text", Placeholder: "https://api.openai.com/v1", Required: true},
			{Key: "LLM_API_KEY", Label: "API Key", LabelZh: "API 密钥", Type: "password", Placeholder: "sk-...", Sensitive: true, Required: true},
			{Key: "LLM_MODEL", Label: "Model", LabelZh: "模型名称", Type: "text", Placeholder: "gpt-4.1-mini"},
			{Key: "AGENT_ADDR", Label: "Listen Address", LabelZh: "监听地址", Type: "text", Placeholder: ":9090", Tier: "expert"},
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
			{Key: "THINKING_LEVEL", Label: "Thinking Level", LabelZh: "思考深度", Type: "select", Options: []string{"auto", "none", "deep"}, Tier: "common",
				Hint: "全局默认思考深度。auto=按复杂度自动选择，none=最快响应，deep=始终完整推理链。聊天页面可临时覆盖。"},
			{Key: "HEARTBEAT_ENABLED", Label: "Heartbeat", LabelZh: "心跳自检", Type: "select", Options: []string{"true", "false"}, Tier: "common",
				Hint: "定期自动检查状态、回顾近期对话，结果推送到收件箱。"},
			{Key: "HEARTBEAT_INTERVAL", Label: "Heartbeat Interval (min)", LabelZh: "心跳间隔（分钟）", Type: "number", Placeholder: "30",
				Hint: "两次心跳之间的间隔（分钟）。"},
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
			{Key: "HOST_READ_PATHS", Label: "Read-Only Paths", LabelZh: "只读访问路径", Type: "text", Placeholder: "C:\\Users\\me\\Desktop,C:\\Users\\me\\Documents", Tier: "common",
				Hint: "Agent 可以读取的目录，逗号分隔。用于让 Agent 浏览你的文件。"},
			{Key: "HOST_WRITE_PATHS", Label: "Writable Paths", LabelZh: "可写访问路径", Type: "text", Placeholder: "data/output,data/tasks", Tier: "common",
				Hint: "Agent 可以写入的目录，逗号分隔。注意安全风险。"},
		},
	},
	{
		Key: "security", Label: "Security", LabelZh: "安全",
		Fields: []configField{
			{Key: "JWT_SECRET", Label: "JWT Secret", LabelZh: "JWT 密钥", Type: "password", Sensitive: true, Tier: "expert",
				Hint: "API 认证签名密钥。留空则每次启动自动生成（重启后旧 Token 失效）。"},
			{Key: "RATE_LIMIT", Label: "Rate Limit (req/min)", LabelZh: "速率限制（请求/分钟）", Type: "number", Placeholder: "60", Tier: "expert",
				Hint: "每分钟最大请求数。防止滥用。0 或留空表示不限制。"},
			{Key: "ALLOWED_ORIGINS", Label: "CORS Origins", LabelZh: "CORS 来源", Type: "text", Placeholder: "*", Tier: "expert",
				Hint: "允许跨域访问的来源地址。* 表示允许所有来源。"},
		},
	},
	{
		Key: "storage", Label: "Storage & Persistence", LabelZh: "存储与持久化",
		Fields: []configField{
			{Key: "LEDGER_DB_PATH", Label: "Ledger DB Path", LabelZh: "Ledger 数据库路径", Type: "text", Placeholder: "data/ledger/ledger.db", Tier: "expert",
				Hint: "审计日志和记忆数据的 SQLite 文件路径。"},
			{Key: "STORAGE_MODE", Label: "Memory Storage Mode", LabelZh: "记忆存储模式", Type: "select", Options: []string{"ledger", "memory"}, Placeholder: "ledger", Tier: "expert",
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
			{Key: "OPEN_BROWSER", Label: "Auto-Open Browser", LabelZh: "自动打开浏览器", Type: "select", Options: []string{"true", "false"}, Tier: "common",
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

	envExists := config.ResolveEnvFilePath() != ""

	values := readEnvFile()
	hasLLMKey := values["LLM_API_KEY"] != ""
	hasLLMURL := values["LLM_BASE_URL"] != ""
	hasLLMModel := values["LLM_MODEL"] != ""

	// Test LLM connectivity. API keys are optional for local providers.
	apiOK := false
	if hasLLMURL {
		apiOK = testLLMAPI(values["LLM_BASE_URL"], values["LLM_API_KEY"])
	}

	// 设计演化兼容：早期版本通过 .env 写 LLM_* 字段，现在通过 UI/Ledger KV
	// 注册 Provider。只要任何一条路径配出可用模型，就视为已完成 setup，避免
	// 用户在 UI 加了 Provider 后仍被首次配置 toast/卡片骚扰。
	hasEnabledProvider := false
	if g.providerReg != nil {
		for _, p := range g.providerReg.List() {
			if p.Enabled {
				hasEnabledProvider = true
				break
			}
		}
	}
	setupNeeded := (!envExists || !hasLLMURL || !hasLLMModel) && !hasEnabledProvider

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"env_exists":    envExists,
		"has_llm_key":   hasLLMKey,
		"has_llm_url":   hasLLMURL,
		"has_llm_model": hasLLMModel,
		"api_ok":        apiOK,
		"setup_needed":  setupNeeded,
	})
}

// --- helpers ---

func readEnvFile() map[string]string {
	return config.ReadEnvFile()
}

func writeEnvFile(values map[string]string) error {
	return config.WriteEnvFile(values)
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

	// Hot-reload file-system paths: push the new read/write allowlists into every
	// path-configurable plugin, then rebuild the skill registry so the file skills
	// (file_search / doc_parse / zip / xlsx_split …) pick them up WITHOUT a restart.
	if g.pluginReg != nil {
		parse := func(s string) []string {
			var out []string
			for _, p := range strings.Split(s, ",") {
				if p = strings.TrimSpace(p); p != "" {
					out = append(out, p)
				}
			}
			return out
		}
		rp := parse(vals["HOST_READ_PATHS"])
		wp := parse(vals["HOST_WRITE_PATHS"])
		applied := false
		for _, pl := range g.pluginReg.All() {
			if pc, ok := pl.(interface {
				SetHostReadPaths([]string)
				SetHostWritePaths([]string)
			}); ok {
				pc.SetHostReadPaths(rp)
				if len(wp) > 0 {
					pc.SetHostWritePaths(wp)
				}
				applied = true
			}
		}
		if applied {
			g.rebuildSkillsFromPlugins()
			reloaded = append(reloaded, "fs_paths")
		}
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
