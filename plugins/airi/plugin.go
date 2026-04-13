package airi

import (
	"encoding/json"
	"net/http"

	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/pkg/plugin"
	"yunque-agent/pkg/skills"
)

// Plugin exposes Yunque as an Airi-native module via the plugin protocol,
// plus an OpenAI-compatible fallback API for direct Airi connections.
type Plugin struct {
	app    *agentrt.App
	bridge *Bridge
}

// New creates a new Airi Plugin.
func New(app *agentrt.App) *Plugin {
	return &Plugin{
		app: app,
	}
}

func (p *Plugin) Name() string        { return "airi" }
func (p *Plugin) Description() string { return "Airi 桌宠桥接插件 — 连接 Airi 作为原生模块，双向同步对话与表情动作" }

// SystemPrompt returns empty for the global plugin registry so that the
// ACT tag instructions don't leak into non-Airi channels (QQ, WebUI, etc.).
// The Airi-specific prompt is injected only through airiSystemPrompt() in the
// Airi completions handler.
func (p *Plugin) SystemPrompt() string { return "" }

// airiSystemPrompt returns the Airi-specific system prompt with ACT tag instructions.
// Only used by the Airi completions handler and bridge.
func (p *Plugin) airiSystemPrompt() string {
	return `你现在正在通过 Airi 桌宠客户端与用户面对面交流，你的文字会被转化为语音朗读出来，同时你拥有一个可爱的 Live2D/VRM 虚拟形象。
你可以在回复中插入特殊标签来控制自己的表情动作：<|ACT {"emotion":{"name":"表情名","intensity":1}}|>。
可用的表情有：happy, sad, angry, think, surprised, awkward, curious, neutral。
请在合适的时机自然地使用它们，比如开心时用 happy，思考问题时用 think，被夸奖时用 happy 等。
回复风格要简短、自然、可爱，像一个活泼的桌面伙伴，不要写长篇大论。`
}

func (p *Plugin) Skills() []skills.Skill {
	return nil // No extra skills exposed to Yunque
}

// UITabs implements plugin.UIPlugin.
func (p *Plugin) UITabs() []plugin.UITab {
	return []plugin.UITab{
		{
			Key:         "airi",
			Label:       "Airi 桥接",
			LabelEn:     "Airi Bridge",
			Icon:        "Bot",
			Description: "管理 Airi 桌宠连接、查看同步状态",
		},
	}
}

// HTTPHandlers exposes OpenAI compatible endpoints + bridge status API.
// Mounted at /v1/ext/airi/...
func (p *Plugin) HTTPHandlers() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"/models":           p.handleModels,
		"/chat/completions": p.handleChatCompletions,
		"/status":           p.handleStatus,
	}
}

// StartBridge initializes and connects the Airi bridge.
// Call this after the plugin is registered and the gateway is available.
func (p *Plugin) StartBridge() {
	p.bridge = NewBridge(p.app)
	p.app.Set("airi_plugin", p)
	go p.bridge.Run()
}

// Bridge returns the underlying Airi bridge (may be nil if not started).
func (p *Plugin) Bridge() *Bridge { return p.bridge }

// PushToAiri sends text to the Airi desktop client via the bridge.
func (p *Plugin) PushToAiri(text string) {
	if p.bridge != nil {
		p.bridge.PushTextToAiri(text)
	}
}

// StopBridge disconnects the Airi bridge.
func (p *Plugin) StopBridge() {
	if p.bridge != nil {
		p.bridge.Stop()
	}
}

// ── HTTP Handlers ──

// handleModels mocks the `/v1/models` endpoint.
func (p *Plugin) handleModels(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"object": "list",
		"data": []map[string]any{
			{
				"id":       "yunque-airi",
				"object":   "model",
				"created":  1677610602,
				"owned_by": "yunque",
			},
		},
	})
}

// handleStatus returns the bridge connection status.
func (p *Plugin) handleStatus(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	status := map[string]any{
		"plugin":    "airi",
		"connected": false,
	}

	if p.bridge != nil {
		status["connected"] = p.bridge.Connected()
		status["url"] = p.bridge.URL()
		status["module_name"] = p.bridge.ModuleName()
		status["messages_sent"] = p.bridge.MessagesSent()
		status["messages_received"] = p.bridge.MessagesReceived()
	}

	writeJSON(w, http.StatusOK, status)
}

// setCORS sets permissive cross-origin headers for Airi
func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

// writeJSON is a helper to write JSON responses.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
