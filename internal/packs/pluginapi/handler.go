// Package pluginapipack mounts the plugin SDK bridge as a native capability
// pack. It owns /v1/plugin-api/* so plugin-to-agent communication is controlled
// by Pack Runtime enablement instead of direct gateway mux registrations.
package pluginapipack

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/packruntime"
	"yunque-agent/pkg/plugin"
)

const PackID = "yunque.pack.plugin-api"

// Config holds configuration for the plugin API bridge.
type Config struct {
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

// Handler handles all /v1/plugin-api/* requests.
type Handler struct {
	cfg      Config
	tokenMgr *PluginTokenManager
	host     packruntime.Host
	started  atomic.Bool
}

// New creates the handler.
func New(cfg Config, tokenMgr *PluginTokenManager) *Handler {
	if tokenMgr == nil {
		tokenMgr = NewPluginTokenManager()
	}
	return &Handler{cfg: cfg, tokenMgr: tokenMgr}
}

var _ packruntime.Module = (*Handler)(nil)

func (h *Handler) PackID() string { return PackID }

func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("plugin-api pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

// Routes mounts all plugin API routes through Pack Runtime. Auth stays
// passthrough because the plugin token and permission checks are protocol
// specific and happen inside each route handler.
func (h *Handler) Routes() []packruntime.BackendRoute {
	post := []string{http.MethodPost}
	get := []string{http.MethodGet}
	return []packruntime.BackendRoute{
		{Methods: post, Path: "/v1/plugin-api/llm", Handler: h.withAuth("llm", h.handleLLM), Auth: packruntime.BackendRouteAuthPassthrough},
		{Methods: post, Path: "/v1/plugin-api/search", Handler: h.withAuth("search", h.handleSearch), Auth: packruntime.BackendRouteAuthPassthrough},
		{Methods: post, Path: "/v1/plugin-api/send", Handler: h.withAuth("channel.send", h.handleSend), Auth: packruntime.BackendRouteAuthPassthrough},
		{Methods: post, Path: "/v1/plugin-api/memory/get", Handler: h.withAuth("memory", h.handleMemoryGet), Auth: packruntime.BackendRouteAuthPassthrough},
		{Methods: post, Path: "/v1/plugin-api/memory/set", Handler: h.withAuth("memory", h.handleMemorySet), Auth: packruntime.BackendRouteAuthPassthrough},
		{Methods: post, Path: "/v1/plugin-api/memory/delete", Handler: h.withAuth("memory", h.handleMemoryDelete), Auth: packruntime.BackendRouteAuthPassthrough},
		{Methods: post, Path: "/v1/plugin-api/memory/list", Handler: h.withAuth("memory", h.handleMemoryList), Auth: packruntime.BackendRouteAuthPassthrough},
		{Methods: post, Path: "/v1/plugin-api/memory/search", Handler: h.withAuth("memory", h.handleMemorySearch), Auth: packruntime.BackendRouteAuthPassthrough},
		{Methods: post, Path: "/v1/plugin-api/agent-memory/search", Handler: h.withAuth("memory.read", h.handleAgentMemSearch), Auth: packruntime.BackendRouteAuthPassthrough},
		{Methods: post, Path: "/v1/plugin-api/agent-memory/add", Handler: h.withAuth("memory.write", h.handleAgentMemAdd), Auth: packruntime.BackendRouteAuthPassthrough},
		{Methods: post, Path: "/v1/plugin-api/knowledge/search", Handler: h.withAuth("knowledge", h.handleKnowledgeSearch), Auth: packruntime.BackendRouteAuthPassthrough},
		{Methods: post, Path: "/v1/plugin-api/knowledge/ingest", Handler: h.withAuth("knowledge.write", h.handleKnowledgeIngest), Auth: packruntime.BackendRouteAuthPassthrough},
		{Methods: post, Path: "/v1/plugin-api/cron/add", Handler: h.withAuth("cron", h.handleCronAdd), Auth: packruntime.BackendRouteAuthPassthrough},
		{Methods: post, Path: "/v1/plugin-api/cron/remove", Handler: h.withAuth("cron", h.handleCronRemove), Auth: packruntime.BackendRouteAuthPassthrough},
		{Methods: get, Path: "/v1/plugin-api/cron/list", Handler: h.withAuth("cron", h.handleCronList), Auth: packruntime.BackendRouteAuthPassthrough},
		{Methods: post, Path: "/v1/plugin-api/register/provider", Handler: h.withAuth("system.provider", h.handleRegisterProvider), Auth: packruntime.BackendRouteAuthPassthrough},
		{Methods: post, Path: "/v1/plugin-api/register/channel", Handler: h.withAuth("system.channel", h.handleRegisterChannel), Auth: packruntime.BackendRouteAuthPassthrough},
		{Methods: post, Path: "/v1/plugin-api/register/search", Handler: h.withAuth("system.search", h.handleRegisterSearch), Auth: packruntime.BackendRouteAuthPassthrough},
		{Methods: post, Path: "/v1/plugin-api/register/guardrail", Handler: h.withAuth("system.guardrail", h.handleRegisterGuardrail), Auth: packruntime.BackendRouteAuthPassthrough},
		{Methods: post, Path: "/v1/plugin-api/register/embedding", Handler: h.withAuth("system.embedding", h.handleRegisterEmbedding), Auth: packruntime.BackendRouteAuthPassthrough},
		{Methods: post, Path: "/v1/plugin-api/register/speech", Handler: h.withAuth("system.speech", h.handleRegisterSpeech), Auth: packruntime.BackendRouteAuthPassthrough},
		{Methods: get, Path: "/v1/plugin-api/extensions", Handler: h.withAuth("system.provider", h.handleListExtensions), Auth: packruntime.BackendRouteAuthPassthrough},
	}
}

// RouteSpecs exposes the plugin SDK bridge surface without loading the handler.
// The OpenAPI generator and pack migration gates scan these literal paths, so
// keep this in sync with Routes().
func RouteSpecs() []packruntime.BackendRouteSpec {
	return []packruntime.BackendRouteSpec{
		{Method: http.MethodPost, Path: "/v1/plugin-api/llm", Description: "Call the host LLM from a plugin with a plugin token."},
		{Method: http.MethodPost, Path: "/v1/plugin-api/search", Description: "Search through host-configured search providers."},
		{Method: http.MethodPost, Path: "/v1/plugin-api/send", Description: "Send a message through a host channel."},
		{Method: http.MethodPost, Path: "/v1/plugin-api/memory/get", Description: "Read plugin-private key/value memory."},
		{Method: http.MethodPost, Path: "/v1/plugin-api/memory/set", Description: "Write plugin-private key/value memory."},
		{Method: http.MethodPost, Path: "/v1/plugin-api/memory/delete", Description: "Delete plugin-private key/value memory."},
		{Method: http.MethodPost, Path: "/v1/plugin-api/memory/list", Description: "List plugin-private key/value memory."},
		{Method: http.MethodPost, Path: "/v1/plugin-api/memory/search", Description: "Search plugin-private memory values."},
		{Method: http.MethodPost, Path: "/v1/plugin-api/agent-memory/search", Description: "Read compiled host memory context for a plugin query."},
		{Method: http.MethodPost, Path: "/v1/plugin-api/agent-memory/add", Description: "Add a host memory fact attributed to the plugin."},
		{Method: http.MethodPost, Path: "/v1/plugin-api/knowledge/search", Description: "Search host knowledge sources from a plugin."},
		{Method: http.MethodPost, Path: "/v1/plugin-api/knowledge/ingest", Description: "Ingest plugin-provided content into host knowledge."},
		{Method: http.MethodPost, Path: "/v1/plugin-api/cron/add", Description: "Create a plugin-owned scheduled job."},
		{Method: http.MethodPost, Path: "/v1/plugin-api/cron/remove", Description: "Remove a plugin-owned scheduled job."},
		{Method: http.MethodGet, Path: "/v1/plugin-api/cron/list", Description: "List plugin-owned scheduled jobs."},
		{Method: http.MethodPost, Path: "/v1/plugin-api/register/provider", Description: "Register a provider extension with the host."},
		{Method: http.MethodPost, Path: "/v1/plugin-api/register/channel", Description: "Register a channel extension with the host."},
		{Method: http.MethodPost, Path: "/v1/plugin-api/register/search", Description: "Register a search extension with the host."},
		{Method: http.MethodPost, Path: "/v1/plugin-api/register/guardrail", Description: "Register a guardrail extension with the host."},
		{Method: http.MethodPost, Path: "/v1/plugin-api/register/embedding", Description: "Register an embedding extension with the host."},
		{Method: http.MethodPost, Path: "/v1/plugin-api/register/speech", Description: "Register a speech extension with the host."},
		{Method: http.MethodGet, Path: "/v1/plugin-api/extensions", Description: "List plugin-registered host extensions."},
	}
}

func Paths() []string {
	specs := RouteSpecs()
	seen := make(map[string]bool, len(specs))
	paths := make([]string, 0, len(specs))
	for _, spec := range specs {
		if seen[spec.Path] {
			continue
		}
		seen[spec.Path] = true
		paths = append(paths, spec.Path)
	}
	return paths
}

// withAuth wraps a handler with token validation and permission checking.
func (h *Handler) withAuth(requiredPerm string, next http.HandlerFunc) http.HandlerFunc {
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

func (h *Handler) handleLLM(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) handleSend(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) handleMemoryGet(w http.ResponseWriter, r *http.Request) {
	pluginName := pluginNameFromCtx(r.Context())
	var req struct {
		Key string `json:"key"`
	}
	readBody(r, &req)
	store := h.cfg.MemoryMgr.ForPlugin(pluginName)
	value, _ := store.Get(req.Key)
	writeJSON(w, map[string]string{"value": value})
}

func (h *Handler) handleMemorySet(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) handleMemoryDelete(w http.ResponseWriter, r *http.Request) {
	pluginName := pluginNameFromCtx(r.Context())
	var req struct {
		Key string `json:"key"`
	}
	readBody(r, &req)
	store := h.cfg.MemoryMgr.ForPlugin(pluginName)
	err := store.Delete(req.Key)
	writeJSON(w, map[string]any{"ok": err == nil})
}

func (h *Handler) handleMemoryList(w http.ResponseWriter, r *http.Request) {
	pluginName := pluginNameFromCtx(r.Context())
	var req struct {
		Prefix string `json:"prefix"`
	}
	readBody(r, &req)
	store := h.cfg.MemoryMgr.ForPlugin(pluginName)
	entries := store.List(req.Prefix)
	writeJSON(w, map[string]any{"entries": entries})
}

func (h *Handler) handleMemorySearch(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) handleAgentMemSearch(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) handleAgentMemAdd(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) handleKnowledgeSearch(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) handleKnowledgeIngest(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) handleCronAdd(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) handleCronRemove(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) handleCronList(w http.ResponseWriter, r *http.Request) {
	pluginName := r.URL.Query().Get("plugin")
	if h.cfg.CronList == nil {
		writeJSON(w, map[string]any{"jobs": []any{}})
		return
	}
	jobs := h.cfg.CronList(pluginName)
	writeJSON(w, map[string]any{"jobs": jobs})
}

// 鈹€鈹€ System Extension Registration Handlers 鈹€鈹€

func (h *Handler) handleRegisterProvider(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) handleRegisterChannel(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) handleRegisterSearch(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) handleRegisterGuardrail(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) handleRegisterEmbedding(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) handleRegisterSpeech(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) handleListExtensions(w http.ResponseWriter, r *http.Request) {
	if h.cfg.ExtRegistry == nil {
		writeJSON(w, map[string]any{"extensions": []any{}})
		return
	}
	writeJSON(w, map[string]any{"extensions": h.cfg.ExtRegistry.Extensions()})
}
