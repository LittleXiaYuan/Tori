package gateway

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"yunque-agent/internal/agentcore/emotion"
)

// handleEmotionHistory returns emotion history entries (GET) with optional query params.
func (g *Gateway) handleEmotionHistory(w http.ResponseWriter, r *http.Request) {
	if g.emotionHistory == nil {
		http.Error(w, `{"error":"emotion history not configured"}`, http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.URL.Query().Get("session_id")
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
		}
	}

	var from, to time.Time
	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			from = t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			to = t
		}
	}

	entries := g.emotionHistory.Query(sessionID, from, to, limit)
	summary := emotion.Summary(entries)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"entries": entries,
		"summary": summary,
		"total":   len(entries),
	})
}
