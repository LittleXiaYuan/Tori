package controlplanepack

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/agentcore/approval"
	"yunque-agent/internal/apperror"
)

func (h *Handler) approvalManager() *approval.Manager {
	if h == nil || h.gateway == nil {
		return nil
	}
	return h.gateway.ApprovalManager()
}

func (h *Handler) tenantOf(r *http.Request) string {
	if h == nil || h.gateway == nil || r == nil {
		return ""
	}
	return h.gateway.TenantOf(r.Context())
}

func (h *Handler) handleApprovalRouteSwitch(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleApprovalList(w, r)
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
	}
}

func (h *Handler) handleApprovalList(w http.ResponseWriter, r *http.Request) {
	manager := h.approvalManager()
	if manager == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "approval system not available")
		return
	}

	tenantID := h.tenantOf(r)
	w.Header().Set("Content-Type", "application/json")

	status := r.URL.Query().Get("status")
	isHistory := r.URL.Query().Get("history") == "true"

	var all []*approval.Request
	switch {
	case status == "pending":
		all = manager.Pending(tenantID)
	case status == "approved" || status == "denied" || isHistory:
		history := manager.History(tenantID, 100)
		if status != "" {
			filtered := make([]*approval.Request, 0)
			for _, item := range history {
				if item.Status == approval.Status(status) {
					filtered = append(filtered, item)
				}
			}
			all = filtered
		} else {
			all = history
		}
	case status == "":
		pending := manager.Pending(tenantID)
		if !isHistory && r.URL.Query().Get("status") == "" && r.URL.Query().Get("history") == "" {
			all = pending
		} else {
			history := manager.History(tenantID, 100)
			all = append(pending, history...)
		}
	default:
		all = manager.Pending(tenantID)
	}

	if all == nil {
		all = []*approval.Request{}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"approvals": all, "total": len(all)})
}

func (h *Handler) handleApprovalApprove(w http.ResponseWriter, r *http.Request) {
	manager := h.approvalManager()
	if manager == nil {
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

	if err := manager.Approve(req.ID, h.tenantOf(r)); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "approved", "id": req.ID})
}

func (h *Handler) handleApprovalDeny(w http.ResponseWriter, r *http.Request) {
	manager := h.approvalManager()
	if manager == nil {
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

	if err := manager.Deny(req.ID, h.tenantOf(r), req.Reason); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "denied", "id": req.ID})
}

func (h *Handler) handleApprovalRules(w http.ResponseWriter, r *http.Request) {
	manager := h.approvalManager()
	tenantID := h.tenantOf(r)

	switch r.Method {
	case http.MethodGet:
		if manager == nil {
			writeJSON(w, map[string]any{"rules": []any{}, "total": 0})
			return
		}
		rules := manager.Rules().List(tenantID)
		if rules == nil {
			rules = []approval.Rule{}
		}
		writeJSON(w, map[string]any{"rules": rules, "total": len(rules)})
	case http.MethodPost:
		if manager == nil {
			http.Error(w, `{"error":"approval not configured"}`, http.StatusServiceUnavailable)
			return
		}
		var rule approval.Rule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
			return
		}
		rule.TenantID = tenantID
		manager.Rules().Add(rule)
		writeJSON(w, map[string]string{"status": "ok", "id": rule.ID})
	case http.MethodDelete:
		if manager == nil {
			http.Error(w, `{"error":"approval not configured"}`, http.StatusServiceUnavailable)
			return
		}
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"deleted": manager.Rules().Remove(id)})
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleApprovalDecide(w http.ResponseWriter, r *http.Request) {
	manager := h.approvalManager()
	if manager == nil {
		http.Error(w, `{"error":"approval not configured"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		ID       string            `json:"id"`
		Decision approval.Decision `json:"decision"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}

	if err := manager.ApproveWithDecision(req.ID, h.tenantOf(r), req.Decision); err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeBadRequest, "approval decision failed", err))
		return
	}

	writeJSON(w, map[string]string{"status": "ok", "decision": string(req.Decision)})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
