package browserintent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeBrowserGateway struct {
	packCalls    int
	sessionCalls int
}

func (g *fakeBrowserGateway) HandleBrowserIntentPack(w http.ResponseWriter, _ *http.Request) {
	g.packCalls++
	w.WriteHeader(http.StatusNoContent)
}

func (g *fakeBrowserGateway) HandleBrowserIntentSession(w http.ResponseWriter, _ *http.Request) {
	g.sessionCalls++
	w.WriteHeader(http.StatusAccepted)
}

func TestBrowserIntentHandlerRoutesExposeSurface(t *testing.T) {
	gateway := &fakeBrowserGateway{}
	handler := NewHandler(gateway)

	if handler.PackID() != PackID {
		t.Fatalf("PackID = %q, want %q", handler.PackID(), PackID)
	}

	routes := handler.Routes()
	if len(routes) != 14 {
		t.Fatalf("expected 14 Browser Intent routes, got %d", len(routes))
	}

	byPath := map[string]string{}
	for _, route := range routes {
		if route.Path == "" || route.Handler == nil {
			t.Fatalf("route must declare path and handler: %#v", route)
		}
		if route.Method == "" {
			t.Fatalf("route must declare method: %#v", route)
		}
		byPath[route.Path] = route.Method
	}

	expected := map[string]string{
		"/v1/browser/status":             http.MethodGet,
		"/v1/browser/config":             http.MethodGet,
		"/v1/browser/intent/plan":        http.MethodPost,
		"/v1/browser/navigate":           http.MethodPost,
		"/v1/browser/screenshot":         http.MethodGet,
		"/v1/browser/ocr":                http.MethodPost,
		"/v1/browser/screenshot/latest":  http.MethodGet,
		"/v1/browser/opp/pending":        http.MethodGet,
		"/v1/browser/opp/decide":         http.MethodPost,
		"/api/browser/ext/status":        http.MethodGet,
		"/api/browser/ext/session":       http.MethodPost,
		"/api/browser/ext/action":        http.MethodPost,
		"/api/browser/ext/scenarios":     http.MethodGet,
		"/api/browser/ext/scenarios/run": http.MethodPost,
	}
	for path, method := range expected {
		if got := byPath[path]; got != method {
			t.Fatalf("expected %s to expose %s, got %q", path, method, got)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/browser/status", nil)
	w := httptest.NewRecorder()
	routes[0].Handler(w, req)
	if w.Code != http.StatusNoContent || gateway.packCalls != 1 {
		t.Fatalf("expected pack route to delegate to gateway, status=%d calls=%d", w.Code, gateway.packCalls)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/browser/ext/session", nil)
	w = httptest.NewRecorder()
	for _, route := range routes {
		if route.Path == "/api/browser/ext/session" {
			route.Handler(w, req)
			break
		}
	}
	if w.Code != http.StatusAccepted || gateway.sessionCalls != 1 {
		t.Fatalf("expected session route to use session bridge, status=%d calls=%d", w.Code, gateway.sessionCalls)
	}
}

func TestBrowserActPlanIsNonDestructiveGate(t *testing.T) {
	handler := NewHandler(&fakeBrowserGateway{})
	body := `{"intent":"open_url","target_url":"https://example.com/report","selector":"button[data-export]","requested_by":"unit","reason":"policy review","dry_run":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/browser/intent/plan", strings.NewReader(body))
	w := httptest.NewRecorder()

	handler.BrowserActPlan(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("browser_act plan status=%d body=%s", w.Code, w.Body.String())
	}
	var res struct {
		Plan BrowserActPlanReport `json:"plan"`
	}
	if err := json.NewDecoder(w.Body).Decode(&res); err != nil {
		t.Fatalf("decode browser_act plan: %v", err)
	}
	if !res.Plan.BrowserActPlanReady || res.Plan.BrowserActReady || !res.Plan.PermissionGateReady || !res.Plan.RuntimeSkillGateReady || !res.Plan.OPPGateReady {
		t.Fatalf("unexpected browser_act readiness: %#v", res.Plan)
	}
	if res.Plan.ConsumesBrowserSession || res.Plan.ExecutesBrowserActions || res.Plan.WritesBrowserState || res.Plan.WritesFiles || res.Plan.NetworkAccess {
		t.Fatalf("browser_act plan must remain non-destructive: %#v", res.Plan)
	}
	if res.Plan.ActionCount != 1 || len(res.Plan.PlannedActions) != 1 {
		t.Fatalf("expected one planned action: %#v", res.Plan.PlannedActions)
	}
	action := res.Plan.PlannedActions[0]
	if action.Intent != "open_url" || action.ExecutorAction != "browser.navigate" || action.RequiresPermission != "browser:write" {
		t.Fatalf("unexpected planned browser action: %#v", action)
	}
	if !res.Plan.RequiresHumanApproval || !res.Plan.PermissionGate.RequiresHumanApproval || !res.Plan.OPPGate.RequiresHumanApproval {
		t.Fatalf("browser_act plan should require human approval: %#v", res.Plan)
	}
	for _, artifact := range []string{"browser-act-plan.json", "browser-permission-gate.json", "runtime-skill-gate.json", "opp-gate-plan.json"} {
		if !containsString(res.Plan.Artifacts, artifact) {
			t.Fatalf("browser_act plan missing artifact %s: %#v", artifact, res.Plan.Artifacts)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
