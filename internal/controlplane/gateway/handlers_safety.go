package gateway

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"yunque-agent/internal/agentcore/approval"
	"yunque-agent/internal/agentcore/audit"
	"yunque-agent/internal/agentcore/rbac"
	"yunque-agent/internal/apperror"
)

//  from handlers_approval.go 
// handleApprovalRouteSwitch dispatches /v1/approvals by method.
func (g *Gateway) handleApprovalRouteSwitch(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		g.handleApprovalList(w, r)
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
	}
}

// handleApprovalList returns approval requests.
// GET /v1/approvals                        → all pending
// GET /v1/approvals?status=pending         → pending
// GET /v1/approvals?status=approved        → history (approved)
// GET /v1/approvals?status=denied          → history (denied)
// GET /v1/approvals?status=               → all (pending + history)
// GET /v1/approvals?history=true           → (legacy) resolved history
//
// Response: { "approvals": [...], "total": N }
func (g *Gateway) handleApprovalList(w http.ResponseWriter, r *http.Request) {
	if g.approvalMgr == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "approval system not available")
		return
	}

	tenantID := tenantFromCtx(r.Context())
	w.Header().Set("Content-Type", "application/json")

	status := r.URL.Query().Get("status")
	isHistory := r.URL.Query().Get("history") == "true"

	var all []*approval.Request

	switch {
	case status == "pending":
		all = g.approvalMgr.Pending(tenantID)
	case status == "approved" || status == "denied" || isHistory:
		history := g.approvalMgr.History(tenantID, 100)
		if status != "" {
			filtered := make([]*approval.Request, 0)
			for _, h := range history {
				if h.Status == approval.Status(status) {
					filtered = append(filtered, h)
				}
			}
			all = filtered
		} else {
			all = history
		}
	case status == "":
		pending := g.approvalMgr.Pending(tenantID)
		if !isHistory && r.URL.Query().Get("status") == "" && r.URL.Query().Get("history") == "" {
			all = pending
		} else {
			history := g.approvalMgr.History(tenantID, 100)
			all = append(pending, history...)
		}
	default:
		all = g.approvalMgr.Pending(tenantID)
	}

	if all == nil {
		all = []*approval.Request{}
	}
	json.NewEncoder(w).Encode(map[string]any{"approvals": all, "total": len(all)})
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
// POST /v1/approvals/deny { "id": "xxx", "reason": "涓嶅畨鍏? }
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

//  from handlers_approval_rules.go 
// 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
// Approval Rules API Handlers
//
// Endpoints:
//   GET    /v1/approval/rules     鈥?list all rules
//   POST   /v1/approval/rules     鈥?add a rule
//   DELETE /v1/approval/rules     鈥?remove a rule (by id param)
//   POST   /v1/approval/decide    鈥?approve with decision (allow_once / allow_always / deny_always)
// 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€

// handleApprovalRules handles CRUD for approval rules.
func (g *Gateway) handleApprovalRules(w http.ResponseWriter, r *http.Request) {
	tenantID := tenantFromCtx(r.Context())

	switch r.Method {
	case http.MethodGet:
		if g.approvalMgr == nil {
			writeJSON(w, map[string]any{"rules": []any{}, "total": 0})
			return
		}
		rules := g.approvalMgr.Rules().List(tenantID)
		if rules == nil {
			rules = []approval.Rule{}
		}
		writeJSON(w, map[string]any{"rules": rules, "total": len(rules)})

	case http.MethodPost:
		if g.approvalMgr == nil {
			http.Error(w, `{"error":"approval not configured"}`, http.StatusServiceUnavailable)
			return
		}
		var rule approval.Rule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
			return
		}
		rule.TenantID = tenantID
		g.approvalMgr.Rules().Add(rule)
		writeJSON(w, map[string]string{"status": "ok", "id": rule.ID})

	case http.MethodDelete:
		if g.approvalMgr == nil {
			http.Error(w, `{"error":"approval not configured"}`, http.StatusServiceUnavailable)
			return
		}
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
			return
		}
		ok := g.approvalMgr.Rules().Remove(id)
		writeJSON(w, map[string]any{"deleted": ok})

	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// handleApprovalDecide allows approve/deny with persistent rule creation.
func (g *Gateway) handleApprovalDecide(w http.ResponseWriter, r *http.Request) {
	if g.approvalMgr == nil {
		http.Error(w, `{"error":"approval not configured"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		ID       string            `json:"id"`       // approval request ID
		Decision approval.Decision `json:"decision"` // allow_once | allow_always | deny_always
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}

	tenantID := tenantFromCtx(r.Context())
	if err := g.approvalMgr.ApproveWithDecision(req.ID, tenantID, req.Decision); err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeBadRequest, "approval decision failed", err))
		return
	}

	writeJSON(w, map[string]string{"status": "ok", "decision": string(req.Decision)})
}

//  from handlers_rbac.go 
// handleRBACRolesSwitch dispatches /v1/rbac/roles by method.
func (g *Gateway) handleRBACRolesSwitch(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		g.handleRBACRoles(w, r)
	case http.MethodPost:
		g.handleRBACRoles(w, r)
	case http.MethodDelete:
		g.handleRBACRoleDelete(w, r)
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET/POST/DELETE only")
	}
}

// handleRBACRoles lists all roles or creates a custom role.
// GET  /v1/rbac/roles 鈫?list all roles
// POST /v1/rbac/roles 鈫?create custom role
func (g *Gateway) handleRBACRoles(w http.ResponseWriter, r *http.Request) {
	if g.rbacEnforcer == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "RBAC not available")
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		roles := g.rbacEnforcer.ListRoles()
		json.NewEncoder(w).Encode(map[string]any{"roles": roles, "total": len(roles)})

	case http.MethodPost:
		var role rbac.Role
		if err := json.NewDecoder(r.Body).Decode(&role); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
			return
		}
		if role.ID == "" || role.Name == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "id and name are required")
			return
		}
		g.rbacEnforcer.AddRole(role)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(role)

	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET/POST only")
	}
}

// handleRBACRoleDelete deletes a custom role.
// DELETE /v1/rbac/roles?id=xxx
func (g *Gateway) handleRBACRoleDelete(w http.ResponseWriter, r *http.Request) {
	if g.rbacEnforcer == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "RBAC not available")
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id query param required")
		return
	}
	if err := g.rbacEnforcer.RemoveRole(id); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"deleted": id})
}

// handleRBACAssign assigns a role to a subject.
// POST /v1/rbac/assign { "subject_id": "xxx", "role_id": "admin", "tenant_id": "" }
func (g *Gateway) handleRBACAssign(w http.ResponseWriter, r *http.Request) {
	if g.rbacEnforcer == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "RBAC not available")
		return
	}

	var req struct {
		SubjectID string `json:"subject_id"`
		RoleID    string `json:"role_id"`
		TenantID  string `json:"tenant_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
		return
	}
	if req.SubjectID == "" || req.RoleID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "subject_id and role_id are required")
		return
	}

	if err := g.rbacEnforcer.AssignRole(req.SubjectID, req.RoleID, req.TenantID); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":     "assigned",
		"subject_id": req.SubjectID,
		"role_id":    req.RoleID,
	})
}

// handleRBACRevoke revokes a role from a subject.
// POST /v1/rbac/revoke { "subject_id": "xxx", "role_id": "admin", "tenant_id": "" }
func (g *Gateway) handleRBACRevoke(w http.ResponseWriter, r *http.Request) {
	if g.rbacEnforcer == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "RBAC not available")
		return
	}

	var req struct {
		SubjectID string `json:"subject_id"`
		RoleID    string `json:"role_id"`
		TenantID  string `json:"tenant_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
		return
	}
	if req.SubjectID == "" || req.RoleID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "subject_id and role_id are required")
		return
	}

	g.rbacEnforcer.RevokeRole(req.SubjectID, req.RoleID, req.TenantID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":     "revoked",
		"subject_id": req.SubjectID,
		"role_id":    req.RoleID,
	})
}

// handleRBACCheck checks if a subject has a specific permission.
// POST /v1/rbac/check { "subject_id": "xxx", "resource": "tasks", "action": "write" }
func (g *Gateway) handleRBACCheck(w http.ResponseWriter, r *http.Request) {
	if g.rbacEnforcer == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "RBAC not available")
		return
	}

	var req struct {
		SubjectID string        `json:"subject_id"`
		TenantID  string        `json:"tenant_id"`
		Resource  rbac.Resource `json:"resource"`
		Action    rbac.Action   `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
		return
	}
	if req.SubjectID == "" {
		req.SubjectID = tenantFromCtx(r.Context())
	}
	if req.TenantID == "" {
		req.TenantID = tenantFromCtx(r.Context())
	}

	allowed := g.rbacEnforcer.Check(req.SubjectID, req.TenantID, req.Resource, req.Action)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"allowed":    allowed,
		"subject_id": req.SubjectID,
		"resource":   req.Resource,
		"action":     req.Action,
	})
}

// handleRBACMyRoles returns roles assigned to the current subject.
// GET /v1/rbac/my-roles
func (g *Gateway) handleRBACMyRoles(w http.ResponseWriter, r *http.Request) {
	if g.rbacEnforcer == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "RBAC not available")
		return
	}

	subjectID := tenantFromCtx(r.Context())
	tenantID := subjectID
	roles := g.rbacEnforcer.SubjectRoles(subjectID, tenantID)
	if roles == nil {
		roles = []rbac.Role{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"subject_id": subjectID,
		"roles":      roles,
		"total":      len(roles),
	})
}

//  from handlers_audit.go 
func (g *Gateway) handleAuditTail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.auditChain == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "audit not configured"})
		return
	}
	n := 20
	if q := r.URL.Query().Get("n"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			n = v
		}
	}
	if n > 200 {
		n = 200
	}

	// Optional filters
	typ := audit.EventType(r.URL.Query().Get("type"))
	actor := r.URL.Query().Get("actor")

	var records []audit.Record
	if typ != "" || actor != "" {
		records = g.auditChain.Search(typ, actor, n)
	} else {
		records = g.auditChain.Tail(n)
	}
	json.NewEncoder(w).Encode(map[string]any{"records": records, "count": len(records)})
}

func (g *Gateway) handleAuditVerify(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.auditChain == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "audit not configured"})
		return
	}
	idx := g.auditChain.Verify()
	result := map[string]any{
		"valid":        idx == -1,
		"checked":      g.auditChain.Len(),
		"chain_length": g.auditChain.Len(),
	}
	if idx != -1 {
		result["broken_at"] = idx
		result["tampered_at"] = idx
	}
	json.NewEncoder(w).Encode(result)
}

func (g *Gateway) handleAuditStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.auditChain == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "audit not configured"})
		return
	}
	json.NewEncoder(w).Encode(g.auditChain.Stats())
}

//  from handlers_trace.go 
// 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
// Trace API 鈥?query execution traces for audit/replay
//
// GET /v1/trace/{trace_id}     鈫?events for a specific trace
// GET /v1/trace/recent?limit=N 鈫?most recent events
// GET /v1/trace/task/{task_id} 鈫?events for a specific task
// 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€

func (g *Gateway) handleTraceByID(w http.ResponseWriter, r *http.Request) {
	if g.eventTrail == nil {
		http.Error(w, "audit trail not available", http.StatusServiceUnavailable)
		return
	}

	// Extract trace_id from path: /v1/trace/{trace_id}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/trace/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "trace_id required", http.StatusBadRequest)
		return
	}
	traceID := parts[0]

	events := g.eventTrail.QueryByTraceID(traceID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"trace_id": traceID,
		"count":    len(events),
		"events":   events,
	})
}

func (g *Gateway) handleTraceRecent(w http.ResponseWriter, r *http.Request) {
	if g.eventTrail == nil {
		http.Error(w, "audit trail not available", http.StatusServiceUnavailable)
		return
	}

	limit := 50
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 500 {
		limit = 500
	}

	events := g.eventTrail.Recent(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"count":  len(events),
		"events": events,
	})
}

func (g *Gateway) handleTraceByTask(w http.ResponseWriter, r *http.Request) {
	if g.eventTrail == nil {
		http.Error(w, "audit trail not available", http.StatusServiceUnavailable)
		return
	}

	// Extract task_id from path: /v1/trace/task/{task_id}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/trace/task/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "task_id required", http.StatusBadRequest)
		return
	}
	taskID := parts[0]

	events := g.eventTrail.QueryByTaskID(taskID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"task_id": taskID,
		"count":   len(events),
		"events":  events,
	})
}
