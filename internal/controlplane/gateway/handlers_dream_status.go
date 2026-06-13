package gateway

import (
	"encoding/json"
	"net/http"
)

// handleDreamStatus returns a read-only view of the offline dream loop
// (小羽 / RWKV-7) and recent self-evolution experiments, so the background
// self-improvement loop is visible to operators.
//
// GET /v1/reverie/dream/status
func (g *Gateway) handleDreamStatus(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{}
	if g.cogniKernel != nil {
		out["configured"] = true
		out["dream"] = g.cogniKernel.DreamStatus()
	} else {
		out["configured"] = false
		out["message"] = "offline dream engine not configured (set OFFLINE_LLM_BASE_URL)"
	}
	if g.cogniEvolution != nil {
		out["evolution"] = g.cogniEvolution.AllExperiments()
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
