package gateway

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/agentcore/approval"
	"yunque-agent/internal/apperror"
)

// handleApprovalRouteSwitch dispatches /v1/approvals by method.
func (g *Gateway) handleApprovalRouteSwitch(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		g.handleApprovalList(w, r)
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
	}
}

// handleApprovalList returns pending approval requests.
// GET /v1/approvals          → pending
// GET /v1/approvals?history=true&limit=20 → resolved history
func (g *Gateway) handleApprovalList(w http.ResponseWriter, r *http.Request) {
	if g.approvalMgr == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "approval system not available")
		return
	}

	tenantID := tenantFromCtx(r.Context())
	w.Header().Set("Content-Type", "application/json")

	if r.URL.Query().Get("history") == "true" {
		limit := 50
		history := g.approvalMgr.History(tenantID, limit)
		if history == nil {
			history = []*approval.Request{}
		}
		json.NewEncoder(w).Encode(map[string]any{"history": history, "total": len(history)})
		return
	}

	pending := g.approvalMgr.Pending(tenantID)
	if pending == nil {
		pending = []*approval.Request{}
	}
	json.NewEncoder(w).Encode(map[string]any{"pending": pending, "total": len(pending)})
}

// handleApprovalApprove approves a pending request.
// POST /v1/approvals/approve { "id": "xxx" }
func (g *Gateway) handleApprovalApprove(w http.ResponseWriter, r *http.Request) {
	if g.approvalMgr == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "approval system not available")
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
		return
	}
	if req.ID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
		return
	}

	approver := tenantFromCtx(r.Context())
	if err := g.approvalMgr.Approve(req.ID, approver); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "approved",
		"id":     req.ID,
	})
}

// handleApprovalDeny denies a pending request.
// POST /v1/approvals/deny { "id": "xxx", "reason": "不安全" }
func (g *Gateway) handleApprovalDeny(w http.ResponseWriter, r *http.Request) {
	if g.approvalMgr == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "approval system not available")
		return
	}

	var req struct {
		ID     string `json:"id"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
		return
	}
	if req.ID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
		return
	}

	approver := tenantFromCtx(r.Context())
	if err := g.approvalMgr.Deny(req.ID, approver, req.Reason); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "denied",
		"id":     req.ID,
	})
}
