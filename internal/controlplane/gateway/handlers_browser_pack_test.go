package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/controlplane/tenant"
	browserintentpack "yunque-agent/internal/packs/browserintent"
	"yunque-agent/pkg/packruntime"
)

func TestBrowserIntentPackGateReturnsNotFoundWhenDisabled(t *testing.T) {
	gw, tm := newTestGatewayWithBrowserIntentPack(t, packruntime.PackStatusDisabled)
	tenant := tm.Register("browser-intent-disabled")

	req := httptest.NewRequest(http.MethodGet, "/v1/browser/status", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("disabled Browser Intent pack should gate /v1/browser/status, status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestBrowserIntentPackRoutesStatusWhenEnabled(t *testing.T) {
	gw, tm := newTestGatewayWithBrowserIntentPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("browser-intent-enabled")

	req := httptest.NewRequest(http.MethodGet, "/v1/browser/status", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("enabled Browser Intent pack should expose status, status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestBrowserIntentPackRoutesBrowserActPlanWhenEnabled(t *testing.T) {
	gw, tm := newTestGatewayWithBrowserIntentPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("browser-intent-plan-enabled")

	req := httptest.NewRequest(http.MethodPost, "/v1/browser/intent/plan", strings.NewReader(`{"intent":"click","selector":"button.export","requested_by":"unit"}`))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("enabled Browser Intent pack should expose browser_act plan, status = %d, body = %s", w.Code, w.Body.String())
	}
	var res struct {
		Plan browserintentpack.BrowserActPlanReport `json:"plan"`
	}
	if err := json.NewDecoder(w.Body).Decode(&res); err != nil {
		t.Fatalf("decode browser_act plan: %v", err)
	}
	if !res.Plan.BrowserActPlanReady || res.Plan.BrowserActReady || !res.Plan.PermissionGateReady || !res.Plan.RuntimeSkillGateReady || !res.Plan.OPPGateReady {
		t.Fatalf("unexpected browser_act plan readiness: %#v", res.Plan)
	}
	if res.Plan.ConsumesBrowserSession || res.Plan.ExecutesBrowserActions || res.Plan.WritesBrowserState || res.Plan.WritesFiles || res.Plan.NetworkAccess {
		t.Fatalf("browser_act plan must not execute browser side effects: %#v", res.Plan)
	}
	if res.Plan.PlannedActions[0].Intent != "click" || res.Plan.PlannedActions[0].ExecutorAction != "browser.click" {
		t.Fatalf("unexpected planned browser_act action: %#v", res.Plan.PlannedActions)
	}
}

func TestBrowserIntentPackRouteSpecsGateByMethod(t *testing.T) {
	gw, tm := newTestGatewayWithBrowserIntentPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("browser-intent-method-gate")

	req := httptest.NewRequest(http.MethodGet, "/v1/browser/ocr", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /v1/browser/ocr should be blocked by pack method gate, status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestBrowserIntentSessionRouteKeepsExtensionGrantAuth(t *testing.T) {
	tori := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer ybext_pack" {
			t.Fatalf("unexpected extension authorization header: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"grant_id":"grant-pack","name":"Browser","scope":"browser:connect","client_id":"yunque-browser-extension","user":{"id":1,"username":"pack-user","email":"pack@example.com"}}}`))
	}))
	defer tori.Close()

	t.Setenv("TORI_API_BASE_URL", tori.URL)
	t.Setenv("DEFAULT_TENANT_ID", "default")

	gw, tm := newTestGatewayWithBrowserIntentPack(t, packruntime.PackStatusEnabled)
	tm.RegisterWithID("default", "default", "ya_default")

	req := httptest.NewRequest(http.MethodPost, "/api/browser/ext/session", nil)
	req.Header.Set("Authorization", "Bearer ybext_pack")
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("extension grant should reach Browser Intent session bridge, status = %d, body = %s", w.Code, w.Body.String())
	}
	if w.Body.String() == "" || !containsAll(w.Body.String(), []string{"ticket", "ws_url", "ttl_sec"}) {
		t.Fatalf("session response should include browser session ticket fields, body = %s", w.Body.String())
	}
}

func TestBrowserIntentSessionRouteStillGatedWhenPackDisabled(t *testing.T) {
	gw, _ := newTestGatewayWithBrowserIntentPack(t, packruntime.PackStatusDisabled)

	req := httptest.NewRequest(http.MethodPost, "/api/browser/ext/session", nil)
	req.Header.Set("Authorization", "Bearer ybext_disabled")
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("disabled Browser Intent pack should gate extension session route before bridge auth, status = %d, body = %s", w.Code, w.Body.String())
	}
}

func containsAll(text string, tokens []string) bool {
	for _, token := range tokens {
		if !strings.Contains(text, token) {
			return false
		}
	}
	return true
}

func newTestGatewayWithBrowserIntentPack(t *testing.T, status packruntime.PackStatus) (*Gateway, *tenant.Manager) {
	t.Helper()
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           browserintentpack.PackID,
		Name:         "Browser Intent Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend: packruntime.BackendManifest{
			Routes: []string{
				"/v1/browser/status",
				"/v1/browser/config",
				"/v1/browser/intent/plan",
				"/v1/browser/navigate",
				"/v1/browser/screenshot",
				"/v1/browser/ocr",
				"/v1/browser/screenshot/latest",
				"/v1/browser/opp/pending",
				"/v1/browser/opp/decide",
				"/api/browser/ext/status",
				"/api/browser/ext/session",
				"/api/browser/ext/action",
				"/api/browser/ext/scenarios",
				"/api/browser/ext/scenarios/run",
			},
			RouteSpecs: []packruntime.BackendRouteSpec{
				{Method: http.MethodGet, Path: "/v1/browser/status"},
				{Method: http.MethodGet, Path: "/v1/browser/config"},
				{Method: http.MethodPost, Path: "/v1/browser/intent/plan"},
				{Method: http.MethodPost, Path: "/v1/browser/navigate"},
				{Method: http.MethodGet, Path: "/v1/browser/screenshot"},
				{Method: http.MethodPost, Path: "/v1/browser/ocr"},
				{Method: http.MethodGet, Path: "/v1/browser/screenshot/latest"},
				{Method: http.MethodGet, Path: "/v1/browser/opp/pending"},
				{Method: http.MethodPost, Path: "/v1/browser/opp/decide"},
				{Method: http.MethodGet, Path: "/api/browser/ext/status"},
				{Method: http.MethodPost, Path: "/api/browser/ext/session"},
				{Method: http.MethodPost, Path: "/api/browser/ext/action"},
				{Method: http.MethodGet, Path: "/api/browser/ext/scenarios"},
				{Method: http.MethodPost, Path: "/api/browser/ext/scenarios/run"},
			},
		},
		Frontend: packruntime.FrontendManifest{Menus: []packruntime.FrontendMenu{{Key: "browser-intent", Label: "浏览器意图", Path: "/packs/browser"}}},
		SDK:      packruntime.SDKManifest{TypeScript: "yunque-client/browser"},
		Update:   packruntime.UpdateManifest{Rollback: true},
	}, "test")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if status == packruntime.PackStatusDisabled {
		if _, err := registry.Disable(browserintentpack.PackID); err != nil {
			t.Fatalf("Disable: %v", err)
		}
	}
	gw, tm := newTestGatewayWithConfig(GatewayConfig{Packs: registry})
	gw.RegisterBackendPack(browserintentpack.NewHandler(gw))
	return gw, tm
}
