package gateway

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/orchestrator"
)

func (g *Gateway) registerOrchestratorRoutes() {
	g.mux.HandleFunc("/v1/orchestrator/status", g.requireAuth(g.handleOrchestratorStatus))
	g.mux.HandleFunc("/v1/orchestrator/toggle", g.requireAuth(g.handleOrchestratorToggle))
	g.mux.HandleFunc("/v1/orchestrator/sessions", g.requireAuth(g.handleOrchestratorSessions))
	g.mux.HandleFunc("/v1/orchestrator/detect", g.requireAuth(g.handleDetectIDEs))
	g.mux.HandleFunc("/v1/orchestrator/adapters/add", g.requireAuth(g.handleAddCustomAdapter))
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

	writeJSON(w, map[string]any{
		"running":        running,
		"adapters":       adapters,
		"active_sessions": sessionCount,
	})
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
