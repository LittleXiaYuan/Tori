// Package rbacpack mounts role-based access control management as a native
// capability pack.
package rbacpack

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"

	"yunque-agent/internal/agentcore/rbac"
	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.rbac"

type Gateway interface {
	RBACEnforcer() *rbac.Enforcer
	RequireAuth(http.HandlerFunc) http.HandlerFunc
	RequireAdmin(http.HandlerFunc) http.HandlerFunc
	TenantOf(context.Context) string
}

type Handler struct {
	enforcerOf func() *rbac.Enforcer
	authOf     func(http.HandlerFunc) http.HandlerFunc
	adminOf    func(http.HandlerFunc) http.HandlerFunc
	tenantOf   func(context.Context) string
	host       packruntime.Host
	started    atomic.Bool
}

func New(gateway Gateway) *Handler {
	if gateway == nil {
		return NewProvider(nil, nil, nil, nil)
	}
	return NewProvider(gateway.RBACEnforcer, gateway.RequireAuth, gateway.RequireAdmin, gateway.TenantOf)
}

func NewProvider(
	enforcerOf func() *rbac.Enforcer,
	authOf func(http.HandlerFunc) http.HandlerFunc,
	adminOf func(http.HandlerFunc) http.HandlerFunc,
	tenantOf func(context.Context) string,
) *Handler {
	return &Handler{
		enforcerOf: enforcerOf,
		authOf:     authOf,
		adminOf:    adminOf,
		tenantOf:   tenantOf,
	}
}

var _ packruntime.Module = (*Handler)(nil)

func (h *Handler) PackID() string { return PackID }

func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("rbac pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{
			Methods: []string{http.MethodGet, http.MethodPost, http.MethodDelete},
			Path:    "/v1/rbac/roles",
			Handler: h.admin(h.handleRolesSwitch),
			Auth:    packruntime.BackendRouteAuthPassthrough,
		},
		{Method: http.MethodPost, Path: "/v1/rbac/assign", Handler: h.admin(h.handleAssign), Auth: packruntime.BackendRouteAuthPassthrough},
		{Method: http.MethodPost, Path: "/v1/rbac/revoke", Handler: h.admin(h.handleRevoke), Auth: packruntime.BackendRouteAuthPassthrough},
		{Method: http.MethodPost, Path: "/v1/rbac/check", Handler: h.auth(h.handleCheck), Auth: packruntime.BackendRouteAuthPassthrough},
		{Method: http.MethodGet, Path: "/v1/rbac/my-roles", Handler: h.auth(h.handleMyRoles), Auth: packruntime.BackendRouteAuthPassthrough},
	}
}

func RouteSpecs() []packruntime.BackendRouteSpec {
	return []packruntime.BackendRouteSpec{
		{Method: http.MethodGet, Path: "/v1/rbac/roles", Description: "List RBAC roles."},
		{Method: http.MethodPost, Path: "/v1/rbac/roles", Description: "Create a custom RBAC role."},
		{Method: http.MethodDelete, Path: "/v1/rbac/roles", Description: "Delete a custom RBAC role."},
		{Method: http.MethodPost, Path: "/v1/rbac/assign", Description: "Assign a role to a subject."},
		{Method: http.MethodPost, Path: "/v1/rbac/revoke", Description: "Revoke a role from a subject."},
		{Method: http.MethodPost, Path: "/v1/rbac/check", Description: "Check whether a subject can perform an action."},
		{Method: http.MethodGet, Path: "/v1/rbac/my-roles", Description: "List roles assigned to the current subject."},
	}
}

func Paths() []string {
	return []string{
		"/v1/rbac/roles",
		"/v1/rbac/assign",
		"/v1/rbac/revoke",
		"/v1/rbac/check",
		"/v1/rbac/my-roles",
	}
}

func (h *Handler) enforcer() *rbac.Enforcer {
	if h.enforcerOf == nil {
		return nil
	}
	return h.enforcerOf()
}

func (h *Handler) auth(next http.HandlerFunc) http.HandlerFunc {
	if h.authOf == nil {
		return next
	}
	return h.authOf(next)
}

func (h *Handler) admin(next http.HandlerFunc) http.HandlerFunc {
	if h.authOf == nil && h.adminOf == nil {
		return next
	}
	if h.adminOf == nil {
		return h.auth(next)
	}
	return h.auth(h.adminOf(next))
}

func (h *Handler) tenant(ctx context.Context) string {
	if h.tenantOf == nil {
		return ""
	}
	return h.tenantOf(ctx)
}

func (h *Handler) handleRolesSwitch(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodPost:
		h.handleRoles(w, r)
	case http.MethodDelete:
		h.handleRoleDelete(w, r)
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET/POST/DELETE only")
	}
}

func (h *Handler) handleRoles(w http.ResponseWriter, r *http.Request) {
	enforcer := h.enforcer()
	if enforcer == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "RBAC not available")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		roles := enforcer.ListRoles()
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": roles, "total": len(roles)})
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
		enforcer.AddRole(role)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(role)
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET/POST only")
	}
}

func (h *Handler) handleRoleDelete(w http.ResponseWriter, r *http.Request) {
	enforcer := h.enforcer()
	if enforcer == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "RBAC not available")
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id query param required")
		return
	}
	if err := enforcer.RemoveRole(id); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"deleted": id})
}

func (h *Handler) handleAssign(w http.ResponseWriter, r *http.Request) {
	enforcer := h.enforcer()
	if enforcer == nil {
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
	if err := enforcer.AssignRole(req.SubjectID, req.RoleID, req.TenantID); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":     "assigned",
		"subject_id": req.SubjectID,
		"role_id":    req.RoleID,
	})
}

func (h *Handler) handleRevoke(w http.ResponseWriter, r *http.Request) {
	enforcer := h.enforcer()
	if enforcer == nil {
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
	enforcer.RevokeRole(req.SubjectID, req.RoleID, req.TenantID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":     "revoked",
		"subject_id": req.SubjectID,
		"role_id":    req.RoleID,
	})
}

func (h *Handler) handleCheck(w http.ResponseWriter, r *http.Request) {
	enforcer := h.enforcer()
	if enforcer == nil {
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
		req.SubjectID = h.tenant(r.Context())
	}
	if req.TenantID == "" {
		req.TenantID = h.tenant(r.Context())
	}
	allowed := enforcer.Check(req.SubjectID, req.TenantID, req.Resource, req.Action)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"allowed":    allowed,
		"subject_id": req.SubjectID,
		"resource":   req.Resource,
		"action":     req.Action,
	})
}

func (h *Handler) handleMyRoles(w http.ResponseWriter, r *http.Request) {
	enforcer := h.enforcer()
	if enforcer == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "RBAC not available")
		return
	}
	subjectID := h.tenant(r.Context())
	tenantID := subjectID
	roles := enforcer.SubjectRoles(subjectID, tenantID)
	if roles == nil {
		roles = []rbac.Role{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"subject_id": subjectID,
		"roles":      roles,
		"total":      len(roles),
	})
}
