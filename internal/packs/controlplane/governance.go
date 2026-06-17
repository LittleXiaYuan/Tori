package controlplanepack

import (
	"encoding/json"
	"net/http"
)

func (h *Handler) handleReviewStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"enabled":         h.gateway.ReviewGate() != nil,
		"trust_enabled":   h.gateway.TrustTracker() != nil,
		"distill_enabled": h.gateway.Distiller() != nil,
	})
}

func (h *Handler) handleSkillGrowPatterns(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	detector := h.gateway.SkillGrowDetector()
	if detector == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"patterns": []any{}, "count": 0})
		return
	}
	patterns := detector.Patterns()
	_ = json.NewEncoder(w).Encode(map[string]any{"patterns": patterns, "count": len(patterns)})
}
