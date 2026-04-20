package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"

	mcpserver "yunque-agent/internal/mcp/server"
	"yunque-agent/internal/mcp/server/adapters"
)

// registerMCPDispatchRoutes mounts the MCP dispatch server endpoint
// and worker management API.
func (g *Gateway) registerMCPDispatchRoutes() {
	g.initMCPDispatch()

	// MCP Streamable HTTP endpoint — external workers (Cursor, Claude Code, etc.)
	// connect here as MCP clients.
	g.mux.HandleFunc("/mcp/v1", mcpserver.HTTPHandler(g.mcpDispatchServer))

	// REST API for frontend worker management
	g.mux.HandleFunc("/v1/workers", g.requireAuth(g.handleWorkerList))
	g.mux.HandleFunc("/v1/workers/detail", g.requireAuth(g.handleWorkerDetail))
	g.mux.HandleFunc("/v1/workers/remove", g.requireAuth(g.handleWorkerRemove))
	g.mux.HandleFunc("/v1/dispatch/queue", g.requireAuth(g.handleDispatchQueue))
	g.mux.HandleFunc("/v1/dispatch/enqueue", g.requireAuth(g.handleDispatchEnqueue))
	g.mux.HandleFunc("/v1/workers/config", g.requireAuth(g.handleWorkerConfig))
}

func (g *Gateway) initMCPDispatch() {
	g.workerRegistry = mcpserver.NewWorkerRegistry(60_000_000_000) // 60s heartbeat timeout
	g.mcpDispatchServer = mcpserver.New("yunque-dispatch", "1.0.0")

	// NOTE: this runs during gateway.New → routes() before SetTaskStore is
	// invoked in cmd/agent/init_tasks.go, so g.taskStore is typically nil
	// here. The dispatch context is retained on Gateway so SetTaskStore can
	// late-bind the store; dispatch tools still keep a nil-check guard for
	// the window before SetTaskStore runs (and for deployments that never
	// configure a task store).
	g.mcpDispatchCtx = &mcpserver.DispatchContext{
		Workers:   g.workerRegistry,
		TaskStore: g.taskStore,
	}
	mcpserver.RegisterDispatchTools(g.mcpDispatchServer, g.mcpDispatchCtx)
}

// ──────────────────────────────────────────────
// REST handlers for frontend
// ──────────────────────────────────────────────

func (g *Gateway) handleWorkerList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	workers := g.workerRegistry.List()
	writeJSONStatus(w, http.StatusOK, map[string]any{
		"workers": workers,
		"count":   len(workers),
	})
}

func (g *Gateway) handleWorkerDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}
	worker, ok := g.workerRegistry.Get(id)
	if !ok {
		writeJSONStatus(w, http.StatusNotFound, map[string]string{"error": "worker not found"})
		return
	}
	writeJSONStatus(w, http.StatusOK, worker)
}

func (g *Gateway) handleWorkerRemove(w http.ResponseWriter, r *http.Request) {
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
	if !g.workerRegistry.Unregister(body.ID) {
		writeJSONStatus(w, http.StatusNotFound, map[string]string{"error": "worker not found"})
		return
	}
	writeJSONStatus(w, http.StatusOK, map[string]string{"status": "removed"})
}

func (g *Gateway) handleDispatchQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSONStatus(w, http.StatusOK, map[string]any{
		"message": "dispatch queue (use task system for now)",
	})
}

func (g *Gateway) handleDispatchEnqueue(w http.ResponseWriter, r *http.Request) {
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
	writeJSONStatus(w, http.StatusOK, map[string]any{
		"task_id": body.TaskID,
		"status":  "enqueued",
	})
}

func (g *Gateway) handleWorkerConfig(w http.ResponseWriter, r *http.Request) {
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
