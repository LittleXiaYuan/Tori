package gateway

import (
	"encoding/json"
	"net/http"
	"strconv"

	"yunque-agent/internal/orchestrator"
)

func (g *Gateway) registerOrchestratorRoutes() {
	g.mux.HandleFunc("/v1/orchestrator/status", g.requireAuth(g.handleOrchestratorStatus))
	g.mux.HandleFunc("/v1/orchestrator/toggle", g.requireAuth(g.handleOrchestratorToggle))
	g.mux.HandleFunc("/v1/orchestrator/sessions", g.requireAuth(g.handleOrchestratorSessions))
	g.mux.HandleFunc("/v1/orchestrator/detect", g.requireAuth(g.handleDetectIDEs))
	g.mux.HandleFunc("/v1/orchestrator/adapters/add", g.requireAuth(g.handleAddCustomAdapter))
	g.mux.HandleFunc("/v1/orchestrator/events", g.requireAuth(g.handleOrchestratorEvents))
	g.mux.HandleFunc("/v1/orchestrator/events/task", g.requireAuth(g.handleOrchestratorTaskTimeline))
	g.mux.HandleFunc("/v1/orchestrator/policy", g.requireAuth(g.handleOrchestratorPolicy))
}

func (g *Gateway) handleOrchestratorStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}

	running := false
	if g.orchDaemon != nil {
		running = g.orchDaemon.IsRunning()
	}

	var adapters []string
	if g.orchLauncher != nil {
		adapters = g.orchLauncher.AvailableAdapters()
	}

	var sessionCount int
	if g.orchLauncher != nil {
		sessionCount = len(g.orchLauncher.ActiveSessions())
	}

	resp := map[string]any{
		"running":         running,
		"adapters":        adapters,
		"active_sessions": sessionCount,
	}
	if g.orchDaemon != nil {
		resp["policy"] = g.orchDaemon.Policy()
		if g.orchDaemon.Events() != nil {
			resp["event_count"] = g.orchDaemon.Events().Count()
		}
	}
	writeJSON(w, resp)
}

func (g *Gateway) handleOrchestratorToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if g.orchDaemon == nil {
		writeJSONStatus(w, 503, map[string]string{"error": "orchestrator not configured"})
		return
	}

	var body struct {
		Action string `json:"action"` // "start" | "stop"
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONStatus(w, 400, map[string]string{"error": err.Error()})
		return
	}

	switch body.Action {
	case "start":
		g.orchDaemon.Start(r.Context())
		writeJSON(w, map[string]string{"status": "started"})
	case "stop":
		g.orchDaemon.Stop()
		writeJSON(w, map[string]string{"status": "stopped"})
	default:
		writeJSONStatus(w, 400, map[string]string{"error": "action must be start or stop"})
	}
}

func (g *Gateway) handleOrchestratorSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	if g.orchLauncher == nil {
		writeJSON(w, map[string]any{"sessions": []any{}})
		return
	}

	sessions := g.orchLauncher.ActiveSessions()
	out := make([]map[string]any, len(sessions))
	for i, s := range sessions {
		out[i] = map[string]any{
			"session_id":   s.SessionID,
			"adapter":      s.AdapterName,
			"task_id":      s.TaskID,
			"started_at":   s.StartedAt,
		}
	}
	writeJSON(w, map[string]any{"sessions": out})
}

func (g *Gateway) handleDetectIDEs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	ides := orchestrator.DetectIDEs()
	writeJSON(w, map[string]any{"ides": ides})
}

func (g *Gateway) handleOrchestratorEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	if g.orchDaemon == nil || g.orchDaemon.Events() == nil {
		writeJSON(w, map[string]any{"events": []any{}})
		return
	}

	n := 50
	if q := r.URL.Query().Get("limit"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 && v <= 500 {
			n = v
		}
	}

	events := g.orchDaemon.Events().Recent(n)
	writeJSON(w, map[string]any{"events": events, "total": g.orchDaemon.Events().Count()})
}

func (g *Gateway) handleOrchestratorTaskTimeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	taskID := r.URL.Query().Get("task_id")
	if taskID == "" {
		writeJSONStatus(w, 400, map[string]string{"error": "task_id required"})
		return
	}
	if g.orchDaemon == nil || g.orchDaemon.Events() == nil {
		writeJSON(w, map[string]any{"events": []any{}})
		return
	}

	events := g.orchDaemon.Events().ForTask(taskID)
	writeJSON(w, map[string]any{"task_id": taskID, "events": events})
}

func (g *Gateway) handleOrchestratorPolicy(w http.ResponseWriter, r *http.Request) {
	if g.orchDaemon == nil {
		writeJSONStatus(w, 503, map[string]string{"error": "orchestrator not configured"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, g.orchDaemon.Policy())
	case http.MethodPut:
		var p orchestrator.DaemonPolicy
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeJSONStatus(w, 400, map[string]string{"error": err.Error()})
			return
		}
		g.orchDaemon.SetPolicy(p)
		writeJSON(w, map[string]any{"status": "updated", "policy": p})
	default:
		http.Error(w, "method not allowed", 405)
	}
}

func (g *Gateway) handleAddCustomAdapter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if g.orchLauncher == nil {
		writeJSONStatus(w, 503, map[string]string{"error": "orchestrator not configured"})
		return
	}

	var cfg orchestrator.GenericAdapterConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeJSONStatus(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if cfg.AdapterName == "" || cfg.Binary == "" || cfg.MCPConfigPath == "" {
		writeJSONStatus(w, 400, map[string]string{"error": "adapter_name, binary, and mcp_config_path are required"})
		return
	}

	adapter := orchestrator.NewGenericAdapter(cfg)
	g.orchLauncher.RegisterAdapter(adapter)
	writeJSONStatus(w, 201, map[string]any{
		"status":    "registered",
		"name":      cfg.AdapterName,
		"available": adapter.Available(),
	})
}
