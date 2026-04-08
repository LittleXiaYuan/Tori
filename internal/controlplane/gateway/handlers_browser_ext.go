package gateway

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"yunque-agent/internal/agentcore/browserskill"
)

func (g *Gateway) handleBrowserExtStatus(w http.ResponseWriter, r *http.Request) {
	hub := g.browserHub
	tid := tenantFromCtx(r.Context())
	if hub == nil {
		writeJSON(w, map[string]any{"connected": false, "error": "browser hub not initialized"})
		return
	}

	hub.mu.Lock()
	connected := hub.connected && hub.tenantID == tid
	status := map[string]any{
		"connected": connected,
	}
	if connected {
		status["version"] = hub.version
		status["pending"] = len(hub.pending)
	}
	hub.mu.Unlock()

	writeJSON(w, status)
}

func (g *Gateway) handleBrowserExtAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	hub := g.browserHub
	tid := tenantFromCtx(r.Context())
	if hub == nil || !hub.ConnectedForTenant(tid) {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]any{
			"ok": false, "error": "browser extension not connected for current tenant",
		})
		return
	}

	var action BrowserAction
	if err := json.NewDecoder(r.Body).Decode(&action); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{
			"ok": false, "error": "invalid action: " + err.Error(),
		})
		return
	}

	result, err := hub.SendAction(r.Context(), action)
	if err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]any{
			"ok": false, "error": err.Error(),
		})
		return
	}

	writeJSON(w, result)
}

// handleBrowserScenarios returns available preset browser automation scenarios.
func (g *Gateway) handleBrowserScenarios(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"scenarios": browserskill.PresetScenarios()})
}

// handleBrowserRunScenario runs a preset scenario step by step via the extension.
func (g *Gateway) handleBrowserRunScenario(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	hub := g.browserHub
	tid := tenantFromCtx(r.Context())
	if hub == nil || !hub.ConnectedForTenant(tid) {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]any{
			"ok": false, "error": "browser extension not connected for current tenant",
		})
		return
	}

	var req struct {
		ScenarioID string `json:"scenario_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ScenarioID == "" {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{
			"ok": false, "error": "scenario_id is required",
		})
		return
	}

	var scenario *browserskill.Scenario
	for _, s := range browserskill.PresetScenarios() {
		if s.ID == req.ScenarioID {
			scenario = &s
			break
		}
	}
	if scenario == nil {
		writeJSONStatus(w, http.StatusNotFound, map[string]any{
			"ok": false, "error": "scenario not found: " + req.ScenarioID,
		})
		return
	}

	slog.Info("running browser scenario", "id", scenario.ID, "name", scenario.Name, "steps", len(scenario.Steps))

	var results []map[string]any
	for i, step := range scenario.Steps {
		data, _ := json.Marshal(step)
		var action BrowserAction
		_ = json.Unmarshal(data, &action)

		result, err := hub.SendAction(r.Context(), action)
		entry := map[string]any{"step": i, "action": step["type"]}
		if err != nil {
			entry["ok"] = false
			entry["error"] = err.Error()
			results = append(results, entry)
			break
		}
		entry["ok"] = result.OK
		if result.Error != "" {
			entry["error"] = result.Error
		}
		if result.Screenshot != "" {
			entry["has_screenshot"] = true
		}
		results = append(results, entry)
		time.Sleep(500 * time.Millisecond)
	}

	writeJSON(w, map[string]any{
		"ok":       true,
		"scenario": scenario.ID,
		"results":  results,
	})
}
