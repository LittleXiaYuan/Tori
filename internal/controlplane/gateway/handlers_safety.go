package gateway

import (
	"encoding/json"
	"net/http"
	"strconv"

	"yunque-agent/internal/agentcore/audit"
	"yunque-agent/internal/agentcore/rbac"
	"yunque-agent/internal/apperror"
)

//	from handlers_rbac.go
//
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

// from handlers_audit.go
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

// Execution trace APIs were de-shelled into the trace pack
// (internal/packs/trace). The gateway only exposes EventTrail().
