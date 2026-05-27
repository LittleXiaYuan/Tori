package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"yunque-agent/internal/agentcore/planner"
)

// MissionParseResult is the structured intent returned from NL mission parsing.
type MissionParseResult = planner.MissionParseResult

func (g *Gateway) handleMissionParse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Description == "" {
		http.Error(w, "description is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	result, err := g.planner.ParseMissionIntent(ctx, req.Description)
	if err != nil {
		slog.Error("mission parse: failed", "err", err)
		http.Error(w, "failed to parse mission intent", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
