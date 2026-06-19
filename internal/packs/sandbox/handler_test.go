package sandboxpack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/internal/tori"
	"yunque-agent/pkg/packruntime"
)

type fakeGateway struct{}

func (fakeGateway) TenantOf(context.Context) string  { return "tenant-test" }
func (fakeGateway) ToriTokenStore() *tori.TokenStore { return nil }
func (fakeGateway) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Auth") != "ok" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}
func (fakeGateway) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Admin") != "ok" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func TestRoutesDeclareSandboxSurface(t *testing.T) {
	h := New(fakeGateway{})
	routes := h.Routes()
	if len(routes) != 5 {
		t.Fatalf("Routes length = %d, want 5", len(routes))
	}
	if len(Paths()) != 5 {
		t.Fatalf("Paths length = %d, want 5", len(Paths()))
	}

	exec := findRoute(t, routes, "/v1/sandbox/exec")
	if exec.Auth != packruntime.BackendRouteAuthPassthrough || exec.Method != http.MethodPost {
		t.Fatalf("exec route = method %q auth %q", exec.Method, exec.Auth)
	}
	status := findRoute(t, routes, "/v1/sandbox/desktop/status")
	if status.Auth != packruntime.BackendRouteAuthDefault || status.Method != http.MethodGet {
		t.Fatalf("status route = method %q auth %q", status.Method, status.Auth)
	}
	destroy := findRoute(t, routes, "/v1/sandbox/desktop/destroy")
	if len(destroy.Methods) != 2 || destroy.Methods[0] != http.MethodDelete || destroy.Methods[1] != http.MethodPost {
		t.Fatalf("destroy methods = %v", destroy.Methods)
	}
}

func TestAdminRoutesComposeHostAuth(t *testing.T) {
	t.Setenv("TORI_API_BASE_URL", "")
	t.Setenv("SANDBOX_CLOUD_BASE_URL", "")
	t.Setenv("SANDBOX_CLOUD_API_KEY", "")
	t.Setenv("LLM_API_KEY", "")

	h := New(fakeGateway{})
	route := findRoute(t, h.Routes(), "/v1/sandbox/probe")

	req := httptest.NewRequest(http.MethodGet, "/v1/sandbox/probe", nil)
	w := httptest.NewRecorder()
	route.Handler(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("missing auth status = %d, want 401", w.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/sandbox/probe", nil)
	req.Header.Set("X-Auth", "ok")
	w = httptest.NewRecorder()
	route.Handler(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("missing admin status = %d, want 403", w.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/sandbox/probe", nil)
	req.Header.Set("X-Auth", "ok")
	req.Header.Set("X-Admin", "ok")
	w = httptest.NewRecorder()
	route.Handler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin probe status = %d body=%s", w.Code, w.Body.String())
	}
}

func TestStatusMapDefaultsToNotRunning(t *testing.T) {
	h := New(nil)
	status := h.StatusMap(context.Background())
	if ok, _ := status["ok"].(bool); !ok {
		t.Fatalf("status ok = %v", status["ok"])
	}
	if running, _ := status["running"].(bool); running {
		t.Fatalf("expected not running, got %#v", status)
	}
}

func findRoute(t *testing.T, routes []packruntime.BackendRoute, path string) packruntime.BackendRoute {
	t.Helper()
	for _, route := range routes {
		if route.Path == path {
			return route
		}
	}
	t.Fatalf("missing route %s", path)
	return packruntime.BackendRoute{}
}
