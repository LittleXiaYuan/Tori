package gateway

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"yunque-agent/internal/apperror"
)

//  from handlers_heartbeat.go 
func (g *Gateway) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if g.heartbeat == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "heartbeat not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"running": g.heartbeat.IsRunning(),
		})
	case http.MethodPut:
		var req struct {
			Enabled  *bool `json:"enabled"`
			Interval *int  `json:"interval_minutes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON body")
			return
		}
		if req.Enabled != nil {
			g.heartbeat.SetEnabled(r.Context(), *req.Enabled)
		}
		if req.Interval != nil && *req.Interval > 0 {
			g.heartbeat.SetInterval(r.Context(), time.Duration(*req.Interval)*time.Minute)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or PUT")
	}
}

func (g *Gateway) handleHeartbeatTrigger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	if g.heartbeat == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "heartbeat not configured")
		return
	}
	entry := g.heartbeat.Trigger(r.Context())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

func (g *Gateway) handleHeartbeatLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	if g.heartbeat == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "heartbeat not configured")
		return
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	logs := g.heartbeat.Logs(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

//  from handlers_federation.go 
func (g *Gateway) handleFedPeers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.fedHub == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "federation not configured"})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{
		"local_id": string(g.fedHub.LocalID()),
		"peers":    g.fedHub.ListPeers(),
	})
}

func (g *Gateway) handleFedStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.fedHub == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "federation not configured"})
		return
	}
	json.NewEncoder(w).Encode(g.fedHub.Stats())
}

//  from handlers_iterate.go 
// handleIterateProposals returns all proposals or just pending ones.
func (g *Gateway) handleIterateProposals(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.iterateEngine == nil {
		json.NewEncoder(w).Encode(map[string]any{"proposals": []any{}, "count": 0})
		return
	}

	status := r.URL.Query().Get("status")
	var proposals any
	if status == "pending" {
		p := g.iterateEngine.Proposals()
		proposals = p
		json.NewEncoder(w).Encode(map[string]any{"proposals": p, "count": len(p)})
	} else {
		p := g.iterateEngine.AllProposals()
		proposals = p
		_ = proposals
		json.NewEncoder(w).Encode(map[string]any{"proposals": p, "count": len(p)})
	}
}

// handleIterateApprove approves a pending proposal.
func (g *Gateway) handleIterateApprove(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST required")
		return
	}
	if g.iterateEngine == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "iterate engine not configured")
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
		return
	}

	if g.iterateEngine.ApproveProposal(req.ID) {
		json.NewEncoder(w).Encode(map[string]string{"status": "approved", "id": req.ID})
	} else {
		apperror.WriteCode(w, apperror.CodeBadRequest, "proposal not found or not pending")
	}
}

// handleIterateReject rejects a pending proposal.
func (g *Gateway) handleIterateReject(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST required")
		return
	}
	if g.iterateEngine == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "iterate engine not configured")
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
		return
	}

	if g.iterateEngine.RejectProposal(req.ID) {
		json.NewEncoder(w).Encode(map[string]string{"status": "rejected", "id": req.ID})
	} else {
		apperror.WriteCode(w, apperror.CodeBadRequest, "proposal not found or not pending")
	}
}

// handleIterateTrigger manually triggers one iteration cycle.
func (g *Gateway) handleIterateTrigger(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST required")
		return
	}
	if g.iterateEngine == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "iterate engine not configured")
		return
	}

	log, err := g.iterateEngine.RunCycle(r.Context())
	if err != nil {
		resp := map[string]any{"error": err.Error()}
		if log != nil {
			resp["cycle"] = log
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(resp)
		return
	}

	json.NewEncoder(w).Encode(map[string]any{"status": "ok", "cycle": log})
}

// handleIterateStatus returns the iterate engine status.
func (g *Gateway) handleIterateStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.iterateEngine == nil {
		json.NewEncoder(w).Encode(map[string]any{"enabled": false})
		return
	}

	pending := g.iterateEngine.Proposals()
	json.NewEncoder(w).Encode(map[string]any{
		"enabled":          g.iterateEngine.Enabled(),
		"pending_proposals": len(pending),
	})
}

//  from handlers_innovation.go 
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

// handleTrustGrant grants full trust (score=100) to a skill or all skills.
// POST {"slug": "skill_name"} 鈥?grant full trust to one skill.
// POST {"slug": "*"} 鈥?grant full trust to ALL skills (one-click delegation).
func (g *Gateway) handleTrustGrant(w http.ResponseWriter, r *http.Request) {
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
	callerID := tenantFromCtx(r.Context())
	callerRole := roleFromCtx(r.Context())

	if req.Slug == "*" {
		count, err := g.trustTracker.GrantFullAll(callerID, callerRole)
		if err != nil {
			apperror.WriteCode(w, apperror.CodeForbidden, err.Error())
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"status": "granted_all", "upgraded": count})
		return
	}
	if err := g.trustTracker.GrantFull(req.Slug, callerID, callerRole); err != nil {
		apperror.WriteCode(w, apperror.CodeForbidden, err.Error())
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "granted", "slug": req.Slug, "level": "shell"})
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
		"enabled":         g.reviewGate != nil,
		"trust_enabled":   g.trustTracker != nil,
		"distill_enabled": g.distiller != nil,
	})
}

// handleModules returns the list of registered modules and their status.
func (g *Gateway) handleModules(w http.ResponseWriter, r *http.Request) {
	if g.modules == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"modules": []any{}, "profile": ""})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"modules": g.modules.List(),
		"profile": g.profile,
	})
}