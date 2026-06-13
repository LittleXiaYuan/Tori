package gateway

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/agentcore/planner"
)

// handleCognitiveLayer reads (GET) or hot-toggles (POST) the master cognitive
// layer at runtime — no restart, WASM-pack style. When off, the agent is a
// clean "planner + tools" shell (no memory/reflection/dreaming/cogni/etc.);
// persona, trust and guardrails stay in the base.
//
//	GET  /v1/cognitive-layer            → {"enabled": true|false}
//	POST /v1/cognitive-layer {"enabled": false} → flips live, returns new state
func (g *Gateway) handleCognitiveLayer(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeCognitiveLayerState(w)
	case http.MethodPost:
		var body struct {
			Enabled *bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Enabled == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": "body must be {\"enabled\": true|false}",
			})
			return
		}
		planner.SetCognitiveLayerEnabled(*body.Enabled)
		writeCognitiveLayerState(w)
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "GET or POST only"})
	}
}

func writeCognitiveLayerState(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"enabled": planner.CognitiveLayerEnabled(),
	})
}
