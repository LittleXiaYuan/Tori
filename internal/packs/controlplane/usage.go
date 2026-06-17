package controlplanepack

import (
	"encoding/json"
	"net/http"
)

func (h *Handler) handleUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, h.gateway.UsageSnapshot(r.Context()))
}

func (h *Handler) handleQuota(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		TenantID string `json:"tenant_id"`
		Quota    struct {
			MaxChatCalls    int64 `json:"max_chat_calls"`
			MaxTokensPerDay int64 `json:"max_tokens_per_day"`
		} `json:"quota"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	h.gateway.SetUsageQuota(r.Context(), req.TenantID, req.Quota.MaxChatCalls, req.Quota.MaxTokensPerDay)
	writeJSON(w, map[string]string{"status": "ok"})
}
