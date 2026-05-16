package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/controlplane/tenant"
	rpareplaypack "yunque-agent/internal/packs/rpareplay"
	"yunque-agent/pkg/packruntime"
)

func TestRPAReplayPackGateReturnsNotFoundWhenDisabled(t *testing.T) {
	gw, tm := newTestGatewayWithRPAReplayPack(t, packruntime.PackStatusDisabled)
	tenant := tm.Register("rpa-replay-disabled")

	req := httptest.NewRequest(http.MethodGet, "/v1/rpa-replay/status", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("disabled RPA Replay pack should gate status, status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestRPAReplayPackRoutesStatusWhenEnabled(t *testing.T) {
	gw, tm := newTestGatewayWithRPAReplayPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("rpa-replay-enabled")

	req := httptest.NewRequest(http.MethodGet, "/v1/rpa-replay/status", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "yunque.pack.rpa-replay") {
		t.Fatalf("enabled RPA Replay pack should expose status, status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestRPAReplayPackRouteSpecsGateByMethod(t *testing.T) {
	gw, tm := newTestGatewayWithRPAReplayPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("rpa-replay-method-gate")

	req := httptest.NewRequest(http.MethodGet, "/v1/rpa-replay/replay", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /v1/rpa-replay/replay should be blocked by pack method gate, status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestRPAReplayPackCanCreateTraceAndDryRunReplay(t *testing.T) {
	gw, tm := newTestGatewayWithRPAReplayPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("rpa-replay-flow")

	body := `{"slug":"export-report","name":"Export report","steps":[{"action":"navigate","value":"https://erp.example.com/reports?month={{month}}"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/rpa-replay/traces", strings.NewReader(body))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create trace status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/rpa-replay/replay", strings.NewReader(`{"slug":"export-report","params":{"month":"2026-05"},"dry_run":true}`))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "month=2026-05") {
		t.Fatalf("dry-run replay status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/rpa-replay/executor/plan", strings.NewReader(`{"slug":"export-report","params":{"month":"2026-05"},"dry_run":true}`))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"executor_plan_ready":true`) || !strings.Contains(w.Body.String(), `"executor_ready":false`) || !strings.Contains(w.Body.String(), `"browser_intent_gate_plan_ready":true`) || !strings.Contains(w.Body.String(), `"executes_browser_actions":false`) || !strings.Contains(w.Body.String(), `"writes_browser_state":false`) || !strings.Contains(w.Body.String(), `"network_access":false`) {
		t.Fatalf("executor plan status=%d body=%s", w.Code, w.Body.String())
	}
}

func newTestGatewayWithRPAReplayPack(t *testing.T, status packruntime.PackStatus) (*Gateway, *tenant.Manager) {
	t.Helper()
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           rpareplaypack.PackID,
		Name:         "RPA Replay Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "disabled",
		Backend: packruntime.BackendManifest{
			Routes: []string{
				"/v1/rpa-replay/status",
				"/v1/rpa-replay/traces",
				"/v1/rpa-replay/traces/",
				"/v1/rpa-replay/recordings/start",
				"/v1/rpa-replay/recordings/stop",
				"/v1/rpa-replay/replay",
				"/v1/rpa-replay/executor/plan",
				"/v1/rpa-replay/evidence/",
			},
			RouteSpecs: []packruntime.BackendRouteSpec{
				{Method: http.MethodGet, Path: "/v1/rpa-replay/status"},
				{Method: http.MethodGet, Path: "/v1/rpa-replay/traces"},
				{Method: http.MethodPost, Path: "/v1/rpa-replay/traces"},
				{Method: http.MethodGet, Path: "/v1/rpa-replay/traces/"},
				{Method: http.MethodPost, Path: "/v1/rpa-replay/recordings/start"},
				{Method: http.MethodPost, Path: "/v1/rpa-replay/recordings/stop"},
				{Method: http.MethodPost, Path: "/v1/rpa-replay/replay"},
				{Method: http.MethodPost, Path: "/v1/rpa-replay/executor/plan"},
				{Method: http.MethodGet, Path: "/v1/rpa-replay/evidence/"},
			},
		},
		Frontend: packruntime.FrontendManifest{Menus: []packruntime.FrontendMenu{{Key: "rpa-replay", Label: "RPA 回放", Path: "/packs/rpa-replay"}}},
		SDK:      packruntime.SDKManifest{TypeScript: "yunque-client/rpa-replay"},
		Update:   packruntime.UpdateManifest{Rollback: true},
	}, "test")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if status == packruntime.PackStatusEnabled {
		if _, err := registry.Enable(rpareplaypack.PackID); err != nil {
			t.Fatalf("Enable: %v", err)
		}
	}
	gw, tm := newTestGatewayWithConfig(GatewayConfig{Packs: registry})
	gw.RegisterBackendPack(rpareplaypack.New(rpareplaypack.Config{DataDir: t.TempDir()}))
	return gw, tm
}
