package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/internal/controlplane/tenant"
)

func TestRequireBrowserSessionAuthAcceptsExtensionGrant(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer ybext_test" {
			t.Fatalf("unexpected authorization header: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"grant_id":"grant-1","name":"Browser","scope":"browser:connect","client_id":"yunque-browser-extension","user":{"id":1,"username":"tester","email":"tester@example.com"}}}`))
	}))
	defer ts.Close()

	t.Setenv("TORI_API_BASE_URL", ts.URL)
	t.Setenv("DEFAULT_TENANT_ID", "default")

	tm := tenant.NewManager()
	tm.RegisterWithID("default", "default", "ya_test")
	gw := New(nil, tm, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	called := false
	h := gw.requireBrowserSessionAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if got := tenantFromCtx(r.Context()); got != "default" {
			t.Fatalf("expected default tenant, got %s", got)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/browser/ext/session", nil)
	req.Header.Set("Authorization", "Bearer ybext_test")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if !called {
		t.Fatal("expected wrapped handler to be called")
	}
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rr.Code)
	}
}

func TestResolveBrowserExtensionTenantRequiresBaseURL(t *testing.T) {
	t.Setenv("TORI_API_BASE_URL", "")
	t.Setenv("DEFAULT_TENANT_ID", "default")
	tm := tenant.NewManager()
	tm.RegisterWithID("default", "default", "ya_test")
	gw := New(nil, tm, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if _, _, err := gw.resolveBrowserExtensionTenant(context.Background(), "ybext_test"); err == nil {
		t.Fatal("expected missing base url to fail")
	}
}
