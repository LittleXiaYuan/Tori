package controlplanepack

import (
	"encoding/json"
	"net/http"
)

func (h *Handler) handleTrustScores(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	tracker := h.gateway.TrustTracker()
	if tracker == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"scores": map[string]any{}, "count": 0})
		return
	}
	scores := tracker.All()
	_ = json.NewEncoder(w).Encode(map[string]any{"scores": scores, "count": len(scores)})
}

func (h *Handler) handleTrustReset(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	tracker := h.gateway.TrustTracker()
	if tracker == nil {
		http.Error(w, "trust tracker not configured", http.StatusInternalServerError)
		return
	}
	var req struct {
		Slug string `json:"slug"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Slug == "" {
		http.Error(w, "slug is required", http.StatusBadRequest)
		return
	}
	tracker.Reset(req.Slug)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "reset", "slug": req.Slug})
}

func (h *Handler) handleTrustGrant(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	tracker := h.gateway.TrustTracker()
	if tracker == nil {
		http.Error(w, "trust tracker not configured", http.StatusInternalServerError)
		return
	}
	var req struct {
		Slug string `json:"slug"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Slug == "" {
		http.Error(w, "slug is required", http.StatusBadRequest)
		return
	}
	callerID := h.gateway.TenantOf(r.Context())
	callerRole := h.gateway.RoleOf(r.Context())
	if req.Slug == "*" {
		count, err := tracker.GrantFullAll(callerID, callerRole)
		if err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "granted_all", "upgraded": count})
		return
	}
	if err := tracker.GrantFull(req.Slug, callerID, callerRole); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "granted", "slug": req.Slug, "level": "shell"})
}
