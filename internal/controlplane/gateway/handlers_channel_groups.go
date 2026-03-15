package gateway

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/execution/channel"
)

// handleChannelGroups handles GET /v1/channels/groups?type=telegram
// Returns all groups the bot is currently in, optionally filtered by channel type.
func (g *Gateway) handleChannelGroups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	if g.channelReg == nil {
		http.Error(w, `{"error":"channel registry not configured"}`, http.StatusServiceUnavailable)
		return
	}

	typ := r.URL.Query().Get("type") // optional filter

	groups, err := g.channelReg.ListGroups(r.Context(), typ)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	if groups == nil {
		groups = make([]channel.GroupInfo, 0) // ensure JSON array, not null
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"groups": groups,
		"count":  len(groups),
	})
}
