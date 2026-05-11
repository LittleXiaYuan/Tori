package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/plugin"
)

//  from handlers_plugin_api.go 
// PluginAPIConfig holds configuration for the plugin API bridge.
type PluginAPIConfig struct {
	LLMClient    *llm.Client
	LLMBreaker   func(ctx context.Context, system, user string) (string, error)
	MemManager   *memory.Manager
	Orchestrator *memory.Orchestrator
	MemoryMgr    *plugin.PluginMemoryManager
	SearchFunc   func(ctx context.Context, query string, limit int) ([]SearchResult, error)
	SendFunc     func(channelType, target, content, format string) error
	CronAdd      func(name, expr, message string) (string, error)
	CronRemove   func(id string) error
	CronList     func(pluginName string) []map[string]any
	KnSearch     func(query string, limit int) []map[string]any
	KnIngest     func(content, source, filename string) error
	ExtRegistry  *plugin.ExtensionRegistry // system extension registration hub
}

// SearchResult for plugin API.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// PluginTokenManager manages per-plugin API tokens and their permissions.
type PluginTokenManager struct {
	mu     sync.RWMutex
	tokens map[string]*PluginTokenEntry // token 鈫?entry
}

// PluginTokenEntry holds a plugin's token metadata and granted permissions.
type PluginTokenEntry struct {
	PluginName  string
	Permissions map[string]bool // permission name 鈫?granted
	CreatedAt   time.Time
}

// NewPluginTokenManager creates a token manager.
func NewPluginTokenManager() *PluginTokenManager {
	return &PluginTokenManager{tokens: make(map[string]*PluginTokenEntry)}
}

// Issue creates a new token for a plugin with the specified permissions.
func (tm *PluginTokenManager) Issue(pluginName string, permissions []string) string {
	token := fmt.Sprintf("plg_%s_%d", pluginName, time.Now().UnixNano())
	perms := make(map[string]bool, len(permissions))
	for _, p := range permissions {
		perms[p] = true
	}
	tm.mu.Lock()
	tm.tokens[token] = &PluginTokenEntry{
		PluginName:  pluginName,
		Permissions: perms,
		CreatedAt:   time.Now(),
	}
	tm.mu.Unlock()
	return token
}

// Validate checks a token and returns the plugin entry if valid.
func (tm *PluginTokenManager) Validate(token string) (*PluginTokenEntry, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	entry, ok := tm.tokens[token]
	return entry, ok
}

// Revoke removes a plugin's token.
func (tm *PluginTokenManager) Revoke(pluginName string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	for token, entry := range tm.tokens {
		if entry.PluginName == pluginName {
			delete(tm.tokens, token)
		}
	}
}

// PluginAPIHandler handles all /v1/plugin-api/* requests.
type PluginAPIHandler struct {
	cfg      PluginAPIConfig
	tokenMgr *PluginTokenManager
}

// NewPluginAPIHandler creates the handler.
func NewPluginAPIHandler(cfg PluginAPIConfig, tokenMgr *PluginTokenManager) *PluginAPIHandler {
	return &PluginAPIHandler{cfg: cfg, tokenMgr: tokenMgr}
}

// RegisterRoutes mounts all plugin API routes on the given mux.
func (h *PluginAPIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/plugin-api/llm", h.withAuth("llm", h.handleLLM))
	mux.HandleFunc("/v1/plugin-api/search", h.withAuth("search", h.handleSearch))
	mux.HandleFunc("/v1/plugin-api/send", h.withAuth("channel.send", h.handleSend))
	mux.HandleFunc("/v1/plugin-api/memory/get", h.withAuth("memory", h.handleMemoryGet))
	mux.HandleFunc("/v1/plugin-api/memory/set", h.withAuth("memory", h.handleMemorySet))
	mux.HandleFunc("/v1/plugin-api/memory/delete", h.withAuth("memory", h.handleMemoryDelete))
	mux.HandleFunc("/v1/plugin-api/memory/list", h.withAuth("memory", h.handleMemoryList))
	mux.HandleFunc("/v1/plugin-api/memory/search", h.withAuth("memory", h.handleMemorySearch))
	mux.HandleFunc("/v1/plugin-api/agent-memory/search", h.withAuth("memory.read", h.handleAgentMemSearch))
	mux.HandleFunc("/v1/plugin-api/agent-memory/add", h.withAuth("memory.write", h.handleAgentMemAdd))
	mux.HandleFunc("/v1/plugin-api/knowledge/search", h.withAuth("knowledge", h.handleKnowledgeSearch))
	mux.HandleFunc("/v1/plugin-api/knowledge/ingest", h.withAuth("knowledge.write", h.handleKnowledgeIngest))
	mux.HandleFunc("/v1/plugin-api/cron/add", h.withAuth("cron", h.handleCronAdd))
	mux.HandleFunc("/v1/plugin-api/cron/remove", h.withAuth("cron", h.handleCronRemove))
	mux.HandleFunc("/v1/plugin-api/cron/list", h.withAuth("cron", h.handleCronList))

	// 鈹€鈹€ System Extension Registration 鈹€鈹€
	mux.HandleFunc("/v1/plugin-api/register/provider", h.withAuth("system.provider", h.handleRegisterProvider))
	mux.HandleFunc("/v1/plugin-api/register/channel", h.withAuth("system.channel", h.handleRegisterChannel))
	mux.HandleFunc("/v1/plugin-api/register/search", h.withAuth("system.search", h.handleRegisterSearch))
	mux.HandleFunc("/v1/plugin-api/register/guardrail", h.withAuth("system.guardrail", h.handleRegisterGuardrail))
	mux.HandleFunc("/v1/plugin-api/register/embedding", h.withAuth("system.embedding", h.handleRegisterEmbedding))
	mux.HandleFunc("/v1/plugin-api/register/speech", h.withAuth("system.speech", h.handleRegisterSpeech))
	mux.HandleFunc("/v1/plugin-api/extensions", h.withAuth("system.provider", h.handleListExtensions))
}

// withAuth wraps a handler with token validation and permission checking.
func (h *PluginAPIHandler) withAuth(requiredPerm string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)
		if token == "" {
			http.Error(w, `{"error":"missing plugin token"}`, http.StatusUnauthorized)
			return
		}
		entry, ok := h.tokenMgr.Validate(token)
		if !ok {
			http.Error(w, `{"error":"invalid plugin token"}`, http.StatusUnauthorized)
			return
		}
		if !entry.Permissions[requiredPerm] {
			slog.Warn("plugin API: permission denied",
				"plugin", entry.PluginName, "required", requiredPerm)
			apperror.WriteCode(w, apperror.CodeForbidden, fmt.Sprintf("permission %q not granted", requiredPerm))
			return
		}
		// Inject plugin name into context
		ctx := context.WithValue(r.Context(), pluginNameKey{}, entry.PluginName)
		next(w, r.WithContext(ctx))
	}
}

type pluginNameKey struct{}

func pluginNameFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(pluginNameKey{}).(string); ok {
		return v
	}
	return ""
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func readBody(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// 鈹€鈹€ Handlers 鈹€鈹€

func (h *PluginAPIHandler) handleLLM(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Messages    []llm.Message `json:"messages"`
		Temperature float64       `json:"temperature"`
		Model       string        `json:"model"`
	}
	if err := readBody(r, &req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if len(req.Messages) == 0 {
		http.Error(w, `{"error":"messages required"}`, http.StatusBadRequest)
		return
	}
	temp := req.Temperature
	if temp <= 0 {
		temp = 0.7
	}
	reply, err := h.cfg.LLMClient.Chat(r.Context(), req.Messages, temp)
	if err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeLLMError, "LLM call failed", err))
		return
	}
	writeJSON(w, map[string]string{"reply": reply})
}

func (h *PluginAPIHandler) handleSearch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := readBody(r, &req); err != nil || req.Query == "" {
		http.Error(w, `{"error":"query required"}`, http.StatusBadRequest)
		return
	}
	if req.Limit <= 0 {
		req.Limit = 5
	}
	if h.cfg.SearchFunc == nil {
		writeJSON(w, map[string]any{"results": []any{}, "error": "no search provider configured"})
		return
	}
	results, err := h.cfg.SearchFunc(r.Context(), req.Query, req.Limit)
	if err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "search failed", err))
		return
	}
	writeJSON(w, map[string]any{"results": results})
}

func (h *PluginAPIHandler) handleSend(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Channel string `json:"channel"`
		Target  string `json:"target"`
		Content string `json:"content"`
		Format  string `json:"format"`
	}
	if err := readBody(r, &req); err != nil || req.Channel == "" || req.Target == "" {
		http.Error(w, `{"error":"channel and target required"}`, http.StatusBadRequest)
		return
	}
	if h.cfg.SendFunc == nil {
		http.Error(w, `{"error":"channel send not configured"}`, http.StatusInternalServerError)
		return
	}
	err := h.cfg.SendFunc(req.Channel, req.Target, req.Content, req.Format)
	writeJSON(w, map[string]any{"ok": err == nil})
}

// 鈹€鈹€ Plugin Memory 鈹€鈹€

func (h *PluginAPIHandler) handleMemoryGet(w http.ResponseWriter, r *http.Request) {
	pluginName := pluginNameFromCtx(r.Context())
	var req struct {
		Key string `json:"key"`
	}
	readBody(r, &req)
	store := h.cfg.MemoryMgr.ForPlugin(pluginName)
	value, _ := store.Get(req.Key)
	writeJSON(w, map[string]string{"value": value})
}

func (h *PluginAPIHandler) handleMemorySet(w http.ResponseWriter, r *http.Request) {
	pluginName := pluginNameFromCtx(r.Context())
	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	readBody(r, &req)
	store := h.cfg.MemoryMgr.ForPlugin(pluginName)
	err := store.Set(req.Key, req.Value)
	writeJSON(w, map[string]any{"ok": err == nil})
}

func (h *PluginAPIHandler) handleMemoryDelete(w http.ResponseWriter, r *http.Request) {
	pluginName := pluginNameFromCtx(r.Context())
	var req struct {
		Key string `json:"key"`
	}
	readBody(r, &req)
	store := h.cfg.MemoryMgr.ForPlugin(pluginName)
	err := store.Delete(req.Key)
	writeJSON(w, map[string]any{"ok": err == nil})
}

func (h *PluginAPIHandler) handleMemoryList(w http.ResponseWriter, r *http.Request) {
	pluginName := pluginNameFromCtx(r.Context())
	var req struct {
		Prefix string `json:"prefix"`
	}
	readBody(r, &req)
	store := h.cfg.MemoryMgr.ForPlugin(pluginName)
	entries := store.List(req.Prefix)
	writeJSON(w, map[string]any{"entries": entries})
}

func (h *PluginAPIHandler) handleMemorySearch(w http.ResponseWriter, r *http.Request) {
	pluginName := pluginNameFromCtx(r.Context())
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	readBody(r, &req)
	if req.Limit <= 0 {
		req.Limit = 10
	}
	store := h.cfg.MemoryMgr.ForPlugin(pluginName)
	results := store.Search(req.Query, req.Limit)
	writeJSON(w, map[string]any{"results": results})
}

// 鈹€鈹€ Agent Memory 鈹€鈹€

func (h *PluginAPIHandler) handleAgentMemSearch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
		TopK  int    `json:"top_k"`
	}
	readBody(r, &req)
	if req.TopK <= 0 {
		req.TopK = 5
	}
	ctx := ""
	if h.cfg.Orchestrator != nil {
		ctx = h.cfg.Orchestrator.CompileContext(r.Context(), "system", req.Query)
	}
	writeJSON(w, map[string]string{"context": ctx})
}

func (h *PluginAPIHandler) handleAgentMemAdd(w http.ResponseWriter, r *http.Request) {
	pluginName := pluginNameFromCtx(r.Context())
	var req struct {
		Fact   string `json:"fact"`
		Source string `json:"source"`
	}
	readBody(r, &req)
	source := req.Source
	if source == "" {
		source = "plugin:" + pluginName
	}
	err := h.cfg.MemManager.AddMid(r.Context(), "system", memory.Item{
		Key: "", Value: req.Fact, Source: source,
	})
	writeJSON(w, map[string]any{"ok": err == nil})
}

// 鈹€鈹€ Knowledge 鈹€鈹€

func (h *PluginAPIHandler) handleKnowledgeSearch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	readBody(r, &req)
	if h.cfg.KnSearch == nil {
		writeJSON(w, map[string]any{"results": []any{}})
		return
	}
	results := h.cfg.KnSearch(req.Query, req.Limit)
	writeJSON(w, map[string]any{"results": results})
}

func (h *PluginAPIHandler) handleKnowledgeIngest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content  string `json:"content"`
		Source   string `json:"source"`
		Filename string `json:"filename"`
	}
	readBody(r, &req)
	if h.cfg.KnIngest == nil {
		http.Error(w, `{"error":"knowledge ingest not configured"}`, http.StatusInternalServerError)
		return
	}
	err := h.cfg.KnIngest(req.Content, req.Source, req.Filename)
	writeJSON(w, map[string]any{"ok": err == nil})
}

// 鈹€鈹€ Cron 鈹€鈹€

func (h *PluginAPIHandler) handleCronAdd(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Expression string `json:"expression"`
		Name       string `json:"name"`
		Message    string `json:"message"`
	}
	readBody(r, &req)
	if h.cfg.CronAdd == nil {
		http.Error(w, `{"error":"cron not configured"}`, http.StatusInternalServerError)
		return
	}
	id, err := h.cfg.CronAdd(req.Name, req.Expression, req.Message)
	if err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "cron add failed", err))
		return
	}
	writeJSON(w, map[string]any{"id": id, "status": "created"})
}

func (h *PluginAPIHandler) handleCronRemove(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	readBody(r, &req)
	if h.cfg.CronRemove == nil {
		http.Error(w, `{"error":"cron not configured"}`, http.StatusInternalServerError)
		return
	}
	err := h.cfg.CronRemove(req.ID)
	writeJSON(w, map[string]any{"ok": err == nil})
}

func (h *PluginAPIHandler) handleCronList(w http.ResponseWriter, r *http.Request) {
	pluginName := r.URL.Query().Get("plugin")
	if h.cfg.CronList == nil {
		writeJSON(w, map[string]any{"jobs": []any{}})
		return
	}
	jobs := h.cfg.CronList(pluginName)
	writeJSON(w, map[string]any{"jobs": jobs})
}

// 鈹€鈹€ System Extension Registration Handlers 鈹€鈹€

func (h *PluginAPIHandler) handleRegisterProvider(w http.ResponseWriter, r *http.Request) {
	if h.cfg.ExtRegistry == nil {
		http.Error(w, `{"error":"extension registry not configured"}`, http.StatusInternalServerError)
		return
	}
	pluginName := pluginNameFromCtx(r.Context())
	var cfg plugin.ProviderRegistration
	if err := readBody(r, &cfg); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if err := h.cfg.ExtRegistry.RegisterProvider(r.Context(), pluginName, cfg); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "provider_id": cfg.ID})
}

func (h *PluginAPIHandler) handleRegisterChannel(w http.ResponseWriter, r *http.Request) {
	if h.cfg.ExtRegistry == nil {
		http.Error(w, `{"error":"extension registry not configured"}`, http.StatusInternalServerError)
		return
	}
	pluginName := pluginNameFromCtx(r.Context())
	var cfg plugin.ChannelRegistration
	if err := readBody(r, &cfg); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if err := h.cfg.ExtRegistry.RegisterChannel(r.Context(), pluginName, cfg); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "channel": cfg.Name})
}

func (h *PluginAPIHandler) handleRegisterSearch(w http.ResponseWriter, r *http.Request) {
	if h.cfg.ExtRegistry == nil {
		http.Error(w, `{"error":"extension registry not configured"}`, http.StatusInternalServerError)
		return
	}
	pluginName := pluginNameFromCtx(r.Context())
	var cfg plugin.SearchRegistration
	if err := readBody(r, &cfg); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if err := h.cfg.ExtRegistry.RegisterSearch(r.Context(), pluginName, cfg); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "search": cfg.Name})
}

func (h *PluginAPIHandler) handleRegisterGuardrail(w http.ResponseWriter, r *http.Request) {
	if h.cfg.ExtRegistry == nil {
		http.Error(w, `{"error":"extension registry not configured"}`, http.StatusInternalServerError)
		return
	}
	pluginName := pluginNameFromCtx(r.Context())
	var cfg plugin.GuardrailRegistration
	if err := readBody(r, &cfg); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if err := h.cfg.ExtRegistry.RegisterGuardrail(r.Context(), pluginName, cfg); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "guardrail": cfg.Name})
}

func (h *PluginAPIHandler) handleRegisterEmbedding(w http.ResponseWriter, r *http.Request) {
	if h.cfg.ExtRegistry == nil {
		http.Error(w, `{"error":"extension registry not configured"}`, http.StatusInternalServerError)
		return
	}
	pluginName := pluginNameFromCtx(r.Context())
	var cfg plugin.EmbeddingRegistration
	if err := readBody(r, &cfg); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if err := h.cfg.ExtRegistry.RegisterEmbedding(r.Context(), pluginName, cfg); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "embedding": cfg.Name})
}

func (h *PluginAPIHandler) handleRegisterSpeech(w http.ResponseWriter, r *http.Request) {
	if h.cfg.ExtRegistry == nil {
		http.Error(w, `{"error":"extension registry not configured"}`, http.StatusInternalServerError)
		return
	}
	pluginName := pluginNameFromCtx(r.Context())
	var cfg plugin.SpeechRegistration
	if err := readBody(r, &cfg); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if err := h.cfg.ExtRegistry.RegisterSpeech(r.Context(), pluginName, cfg); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "speech": cfg.Name})
}

func (h *PluginAPIHandler) handleListExtensions(w http.ResponseWriter, r *http.Request) {
	if h.cfg.ExtRegistry == nil {
		writeJSON(w, map[string]any{"extensions": []any{}})
		return
	}
	writeJSON(w, map[string]any{"extensions": h.cfg.ExtRegistry.Extensions()})
}

