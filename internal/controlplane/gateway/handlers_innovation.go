package gateway

import (
	"encoding/json"
	"net/http"
	"time"
)

// handleTrustScores returns all trust scores.
func (g *Gateway) handleTrustScores(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.trustTracker == nil {
		json.NewEncoder(w).Encode(map[string]any{"scores": map[string]any{}, "count": 0})
		return
	}
	scores := g.trustTracker.All()
	json.NewEncoder(w).Encode(map[string]any{"scores": scores, "count": len(scores)})
}

// handleTrustReset resets a skill's trust score.
func (g *Gateway) handleTrustReset(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	if g.trustTracker == nil {
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
	g.trustTracker.Reset(req.Slug)
	json.NewEncoder(w).Encode(map[string]string{"status": "reset", "slug": req.Slug})
}

// handleAuditTrail returns audit trail entries for a date.
func (g *Gateway) handleAuditTrail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.auditTrail == nil {
		json.NewEncoder(w).Encode(map[string]any{"entries": []any{}, "count": 0})
		return
	}

	dateStr := r.URL.Query().Get("date")
	opFilter := r.URL.Query().Get("type")

	date := time.Now()
	if dateStr != "" {
		if t, err := time.Parse("2006-01-02", dateStr); err == nil {
			date = t
		}
	}

	entries := g.auditTrail.Query(date, opFilter)
	json.NewEncoder(w).Encode(map[string]any{"entries": entries, "count": len(entries)})
}

// handleSkillGrowPatterns returns detected skill growth patterns.
func (g *Gateway) handleSkillGrowPatterns(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.skillGrow == nil {
		json.NewEncoder(w).Encode(map[string]any{"patterns": []any{}, "count": 0})
		return
	}
	patterns := g.skillGrow.Patterns()
	json.NewEncoder(w).Encode(map[string]any{"patterns": patterns, "count": len(patterns)})
}

// handleReviewStatus returns the review gate configuration.
func (g *Gateway) handleReviewStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"enabled":     g.reviewGate != nil,
		"trust_enabled": g.trustTracker != nil,
		"distill_enabled": g.distiller != nil,
	})
}
