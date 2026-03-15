package gateway

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/apperror"
)

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
