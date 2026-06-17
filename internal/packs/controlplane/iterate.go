package controlplanepack

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/apperror"
)

func (h *Handler) handleIterateProposals(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	engine := h.gateway.IterateEngine()
	if engine == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"proposals": []any{}, "count": 0})
		return
	}
	if r.URL.Query().Get("status") == "pending" {
		proposals := engine.Proposals()
		_ = json.NewEncoder(w).Encode(map[string]any{"proposals": proposals, "count": len(proposals)})
		return
	}
	proposals := engine.AllProposals()
	_ = json.NewEncoder(w).Encode(map[string]any{"proposals": proposals, "count": len(proposals)})
}

func (h *Handler) handleIterateApprove(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST required")
		return
	}
	engine := h.gateway.IterateEngine()
	if engine == nil {
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
	if engine.ApproveProposal(req.ID) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "approved", "id": req.ID})
		return
	}
	apperror.WriteCode(w, apperror.CodeBadRequest, "proposal not found or not pending")
}

func (h *Handler) handleIterateReject(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST required")
		return
	}
	engine := h.gateway.IterateEngine()
	if engine == nil {
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
	if engine.RejectProposal(req.ID) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "rejected", "id": req.ID})
		return
	}
	apperror.WriteCode(w, apperror.CodeBadRequest, "proposal not found or not pending")
}

func (h *Handler) handleIterateTrigger(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST required")
		return
	}
	engine := h.gateway.IterateEngine()
	if engine == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "iterate engine not configured")
		return
	}
	log, err := engine.RunCycle(r.Context())
	if err != nil {
		resp := map[string]any{"error": err.Error()}
		if log != nil {
			resp["cycle"] = log
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(resp)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "cycle": log})
}

func (h *Handler) handleIterateStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	engine := h.gateway.IterateEngine()
	if engine == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"enabled": false})
		return
	}
	pending := engine.Proposals()
	_ = json.NewEncoder(w).Encode(map[string]any{
		"enabled":           engine.Enabled(),
		"pending_proposals": len(pending),
	})
}
