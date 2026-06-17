package gateway

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/apperror"
	"yunque-agent/internal/execution/channel"
)

// handleChannelGroups handles GET /v1/channels/groups?type=telegram
func (g *Gateway) handleChannelGroups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	if g.channelReg == nil {
		http.Error(w, `{"error":"channel registry not configured"}`, http.StatusServiceUnavailable)
		return
	}

	typ := r.URL.Query().Get("type")

	groups, err := g.channelReg.ListGroups(r.Context(), typ)
	if err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "list groups failed", err))
		return
	}
	if groups == nil {
		groups = make([]channel.GroupInfo, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"groups": groups,
		"count":  len(groups),
	})
}
