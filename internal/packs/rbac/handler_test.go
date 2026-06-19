package rbacpack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/rbac"
	"yunque-agent/pkg/packruntime"
)

func TestRBACPackV2AndRouteSpecs(t *testing.T) {
	var _ packruntime.Module = (*Handler)(nil)

	h := NewProvider(nil, nil, nil, nil)
	if h.PackID() != PackID {
		t.Fatalf("PackID=%q, want %q", h.PackID(), PackID)
	}
	if got := len(h.Routes()); got != 5 {
		t.Fatalf("Routes len=%d, want 5", got)
	}
	if got := len(RouteSpecs()); got != 7 {
		t.Fatalf("RouteSpecs len=%d, want 7", got)
	}
	paths := map[string]bool{}
	for _, route := range h.Routes() {
		paths[route.Path] = true
		if route.Auth != packruntime.BackendRouteAuthPassthrough {
			t.Fatalf("route %s must keep auth passthrough for custom RBAC auth composition", route.Path)
		}
	}
	for _, spec := range RouteSpecs() {
		if !paths[spec.Path] {
			t.Fatalf("route spec path %s has no mounted route", spec.Path)
		}
	}
	if err := h.Init(nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := h.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestRBACAdminRoutesApplyAuthAndAdminWrappers(t *testing.T) {
	enforcer := rbac.NewEnforcer()
	var calls []string
	auth := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			calls = append(calls, "auth")
			next(w, r)
		}
	}
	admin := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			calls = append(calls, "admin")
			next(w, r)
		}
	}
	h := NewProvider(func() *rbac.Enforcer { return enforcer }, auth, admin, func(context.Context) string { return "tenant-a" })
	route := findRoute(t, h, "/v1/rbac/roles")

	rec := httptest.NewRecorder()
	route.Handler(rec, httptest.NewRequest(http.MethodGet, "/v1/rbac/roles", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Join(calls, ",") != "auth,admin" {
		t.Fatalf("expected auth then admin wrappers, got %v", calls)
	}
}

func TestRBACRoleCRUDAndPermissionCheck(t *testing.T) {
	enforcer := rbac.NewEnforcer()
	h := NewProvider(func() *rbac.Enforcer { return enforcer }, nil, nil, func(context.Context) string { return "tenant-a" })

	create := httptest.NewRecorder()
	h.handleRoles(create, httptest.NewRequest(http.MethodPost, "/v1/rbac/roles", strings.NewReader(`{
		"id":"writer",
		"name":"Writer",
		"permissions":[{"resource":"tasks","action":"write"}]
	}`)))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}

	assign := httptest.NewRecorder()
	h.handleAssign(assign, httptest.NewRequest(http.MethodPost, "/v1/rbac/assign", strings.NewReader(`{"subject_id":"alice","role_id":"writer","tenant_id":"tenant-a"}`)))
	if assign.Code != http.StatusOK {
		t.Fatalf("assign status=%d body=%s", assign.Code, assign.Body.String())
	}

	check := httptest.NewRecorder()
	h.handleCheck(check, httptest.NewRequest(http.MethodPost, "/v1/rbac/check", strings.NewReader(`{"subject_id":"alice","tenant_id":"tenant-a","resource":"tasks","action":"write"}`)))
	var checkBody struct {
		Allowed bool `json:"allowed"`
	}
	if err := json.Unmarshal(check.Body.Bytes(), &checkBody); err != nil {
		t.Fatalf("decode check: %v", err)
	}
	if !checkBody.Allowed {
		t.Fatalf("expected alice to be allowed after role assignment, body=%s", check.Body.String())
	}

	revoke := httptest.NewRecorder()
	h.handleRevoke(revoke, httptest.NewRequest(http.MethodPost, "/v1/rbac/revoke", strings.NewReader(`{"subject_id":"alice","role_id":"writer","tenant_id":"tenant-a"}`)))
	if revoke.Code != http.StatusOK {
		t.Fatalf("revoke status=%d body=%s", revoke.Code, revoke.Body.String())
	}

	check = httptest.NewRecorder()
	h.handleCheck(check, httptest.NewRequest(http.MethodPost, "/v1/rbac/check", strings.NewReader(`{"subject_id":"alice","tenant_id":"tenant-a","resource":"tasks","action":"write"}`)))
	checkBody.Allowed = true
	if err := json.Unmarshal(check.Body.Bytes(), &checkBody); err != nil {
		t.Fatalf("decode check after revoke: %v", err)
	}
	if checkBody.Allowed {
		t.Fatalf("expected alice to be denied after revoke, body=%s", check.Body.String())
	}

	del := httptest.NewRecorder()
	h.handleRoleDelete(del, httptest.NewRequest(http.MethodDelete, "/v1/rbac/roles?id=writer", nil))
	if del.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", del.Code, del.Body.String())
	}
}

func TestRBACMyRolesUsesTenantContext(t *testing.T) {
	enforcer := rbac.NewEnforcer()
	if err := enforcer.AssignRole("tenant-a", "viewer", "tenant-a"); err != nil {
		t.Fatalf("assign default viewer: %v", err)
	}
	h := NewProvider(func() *rbac.Enforcer { return enforcer }, nil, nil, func(context.Context) string { return "tenant-a" })

	rec := httptest.NewRecorder()
	h.handleMyRoles(rec, httptest.NewRequest(http.MethodGet, "/v1/rbac/my-roles", nil))
	var body struct {
		SubjectID string      `json:"subject_id"`
		Roles     []rbac.Role `json:"roles"`
		Total     int         `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode my roles: %v", err)
	}
	if body.SubjectID != "tenant-a" || body.Total != 1 || body.Roles[0].ID != "viewer" {
		t.Fatalf("unexpected my roles: %#v", body)
	}
}

func findRoute(t *testing.T, h *Handler, path string) packruntime.BackendRoute {
	t.Helper()
	for _, route := range h.Routes() {
		if route.Path == path {
			return route
		}
	}
	t.Fatalf("route %s not found", path)
	return packruntime.BackendRoute{}
}
