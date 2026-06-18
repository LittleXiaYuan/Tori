package mcpdispatchpack

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"

	"yunque-agent/internal/agentcore/task"
	mcpserver "yunque-agent/internal/mcp/server"
	"yunque-agent/internal/mcp/server/adapters"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.mcp-dispatch"

type Gateway interface {
	RequireAuth(http.HandlerFunc) http.HandlerFunc
	TaskStore() task.Store
}

type Handler struct {
	authOf    func(http.HandlerFunc) http.HandlerFunc
	taskStore func() task.Store
	server    *mcpserver.Server
	workers   *mcpserver.WorkerRegistry
	dispatch  *mcpserver.DispatchContext
	host      packruntime.Host
	started   atomic.Bool
}

func New(gateway Gateway) *Handler {
	if gateway == nil {
		return NewProvider(nil, nil)
	}
	return NewProvider(gateway.RequireAuth, gateway.TaskStore)
}

func NewProvider(auth func(http.HandlerFunc) http.HandlerFunc, taskStore func() task.Store) *Handler {
	h := &Handler{
		authOf:    auth,
		taskStore: taskStore,
		workers:   mcpserver.NewWorkerRegistry(60_000_000_000),
		server:    mcpserver.New("yunque-dispatch", "1.0.0"),
	}
	h.dispatch = &mcpserver.DispatchContext{
		Workers:   h.workers,
		TaskStore: h.currentTaskStore(),
	}
	mcpserver.RegisterDispatchTools(h.server, h.dispatch)
	return h
}

func (h *Handler) PackID() string { return PackID }

var _ packruntime.Module = (*Handler)(nil)

func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("mcp-dispatch pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	if h.workers != nil {
		h.workers.Stop()
	}
	return nil
}

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{
			Methods: []string{http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPost},
			Path:    "/mcp/v1",
			Handler: h.mcpHTTP,
			Auth:    packruntime.BackendRouteAuthPassthrough,
		},
		{Method: http.MethodGet, Path: "/v1/workers", Handler: h.workerList},
		{Method: http.MethodGet, Path: "/v1/workers/detail", Handler: h.workerDetail},
		{Method: http.MethodPost, Path: "/v1/workers/remove", Handler: h.workerRemove},
		{Method: http.MethodGet, Path: "/v1/dispatch/queue", Handler: h.dispatchQueue},
		{Method: http.MethodPost, Path: "/v1/dispatch/enqueue", Handler: h.dispatchEnqueue},
		{Method: http.MethodGet, Path: "/v1/workers/config", Handler: h.workerConfig},
	}
}

func RouteSpecs() []packruntime.BackendRouteSpec {
	return []packruntime.BackendRouteSpec{
		{Method: http.MethodGet, Path: "/mcp/v1", Description: "Probe the MCP dispatch endpoint."},
		{Method: http.MethodHead, Path: "/mcp/v1", Description: "Probe the MCP dispatch endpoint without a body."},
		{Method: http.MethodOptions, Path: "/mcp/v1", Description: "Probe MCP dispatch endpoint options."},
		{Method: http.MethodPost, Path: "/mcp/v1", Description: "Send authenticated JSON-RPC MCP dispatch requests."},
		{Method: http.MethodGet, Path: "/v1/workers", Description: "List registered external workers."},
		{Method: http.MethodGet, Path: "/v1/workers/detail", Description: "Read one external worker."},
		{Method: http.MethodPost, Path: "/v1/workers/remove", Description: "Remove one external worker."},
		{Method: http.MethodGet, Path: "/v1/dispatch/queue", Description: "Read dispatch queue status."},
		{Method: http.MethodPost, Path: "/v1/dispatch/enqueue", Description: "Enqueue a task for external worker dispatch."},
		{Method: http.MethodGet, Path: "/v1/workers/config", Description: "Generate worker MCP client configuration."},
	}
}

func Paths() []string {
	seen := map[string]bool{}
	paths := []string{}
	for _, spec := range RouteSpecs() {
		if seen[spec.Path] {
			continue
		}
		seen[spec.Path] = true
		paths = append(paths, spec.Path)
	}
	return paths
}

func (h *Handler) currentTaskStore() task.Store {
	if h.taskStore == nil {
		return nil
	}
	return h.taskStore()
}

func (h *Handler) refreshDispatchDeps() {
	if h.dispatch != nil {
		h.dispatch.TaskStore = h.currentTaskStore()
	}
}

func (h *Handler) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	if h.authOf == nil {
		return next
	}
	return h.authOf(next)
}

func (h *Handler) mcpHTTP(w http.ResponseWriter, r *http.Request) {
	h.refreshDispatchDeps()
	raw := mcpserver.HTTPHandler(h.server)
	switch r.Method {
	case http.MethodGet, http.MethodOptions, http.MethodHead:
		raw(w, r)
	default:
		h.requireAuth(raw)(w, r)
	}
}

func (h *Handler) workerList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	workers := h.workers.List()
	writeJSONStatus(w, http.StatusOK, map[string]any{"workers": workers, "count": len(workers)})
}

func (h *Handler) workerDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}
	worker, ok := h.workers.Get(id)
	if !ok {
		writeJSONStatus(w, http.StatusNotFound, map[string]string{"error": "worker not found"})
		return
	}
	writeJSONStatus(w, http.StatusOK, worker)
}

func (h *Handler) workerRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}
	if !h.workers.Unregister(body.ID) {
		writeJSONStatus(w, http.StatusNotFound, map[string]string{"error": "worker not found"})
		return
	}
	writeJSONStatus(w, http.StatusOK, map[string]string{"status": "removed"})
}

func (h *Handler) dispatchQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSONStatus(w, http.StatusOK, map[string]any{"message": "dispatch queue (use task system for now)"})
}

func (h *Handler) dispatchEnqueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		TaskID   string   `json:"task_id"`
		Caps     []string `json:"capabilities"`
		Priority int      `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if body.TaskID == "" {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "task_id is required"})
		return
	}
	writeJSONStatus(w, http.StatusOK, map[string]any{"task_id": body.TaskID, "status": "enqueued"})
}

func (h *Handler) workerConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	workerType := r.URL.Query().Get("type")
	if workerType == "" {
		workerType = "cursor"
	}

	host := r.Host
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	serverURL := fmt.Sprintf("%s://%s/mcp/v1", scheme, host)

	var configJSON string
	var err error
	var instructions string

	switch workerType {
	case "cursor":
		cfg := &adapters.CursorConfig{ServerURL: serverURL}
		configJSON, err = cfg.GenerateMCPJSON()
		instructions = adapters.WorkerInstructions("My Cursor", "cursor", []string{"coding", "testing", "review"})
	case "claude_code":
		cfg := &adapters.ClaudeCodeConfig{ServerURL: serverURL}
		configJSON, err = cfg.GenerateMCPJSON()
		instructions = adapters.WorkerInstructions("My Claude Code", "claude_code", []string{"coding", "testing", "docs"})
	case "windsurf":
		cfg := &adapters.WindsurfConfig{ServerURL: serverURL}
		configJSON, err = cfg.GenerateMCPJSON()
		instructions = adapters.WorkerInstructions("My Windsurf", "windsurf", []string{"coding", "testing"})
	default:
		cfg := &adapters.CursorConfig{ServerURL: serverURL}
		configJSON, err = cfg.GenerateMCPJSON()
		instructions = adapters.WorkerInstructions("Custom Worker", "custom", []string{"coding"})
	}

	if err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSONStatus(w, http.StatusOK, map[string]any{
		"type":         workerType,
		"mcp_config":   configJSON,
		"instructions": instructions,
		"server_url":   serverURL,
	})
}

func writeJSONStatus(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
