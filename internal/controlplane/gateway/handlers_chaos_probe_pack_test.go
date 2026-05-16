package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/controlplane/tenant"
	chaosprobepack "yunque-agent/internal/packs/chaosprobe"
	"yunque-agent/pkg/packruntime"
)

func TestChaosProbePackGateReturnsNotFoundWhenDisabled(t *testing.T) {
	gw, tm := newTestGatewayWithChaosProbePack(t, packruntime.PackStatusDisabled)
	tenant := tm.Register("chaos-probe-disabled")

	req := httptest.NewRequest(http.MethodGet, "/v1/chaos-probe/status", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("disabled Chaos Probe pack should gate status, status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestChaosProbePackRoutesStatusWhenEnabled(t *testing.T) {
	gw, tm := newTestGatewayWithChaosProbePack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("chaos-probe-enabled")

	req := httptest.NewRequest(http.MethodGet, "/v1/chaos-probe/status", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "yunque.pack.chaos-probe") {
		t.Fatalf("enabled Chaos Probe pack should expose status, status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestChaosProbePackRouteSpecsGateByMethod(t *testing.T) {
	gw, tm := newTestGatewayWithChaosProbePack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("chaos-probe-method-gate")

	req := httptest.NewRequest(http.MethodGet, "/v1/chaos-probe/run", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /v1/chaos-probe/run should be blocked by pack method gate, status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestChaosProbePackCanSaveDefinitionsAndRunProbe(t *testing.T) {
	gw, tm := newTestGatewayWithChaosProbePack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("chaos-probe-flow")

	definitionBody := `{"probes":[{"id":"runtime-healthz-probe","name":"Runtime healthz probe","category":"network","description":"local health","safe":true,"enabled":true,"interval_seconds":30,"weight":1}],"replace":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chaos-probe/probes", strings.NewReader(definitionBody))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), "saved") {
		t.Fatalf("save probes status=%d body=%s", w.Code, w.Body.String())
	}

	runBody := `{"probe_ids":["runtime-healthz-probe"],"persist":false}`
	req = httptest.NewRequest(http.MethodPost, "/v1/chaos-probe/run", strings.NewReader(runBody))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "health_score") || !strings.Contains(w.Body.String(), "degrade_level") {
		t.Fatalf("run chaos probe status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestChaosProbePackCanPlanScheduler(t *testing.T) {
	gw, tm := newTestGatewayWithChaosProbePack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("chaos-probe-scheduler-plan")

	req := httptest.NewRequest(http.MethodPost, "/v1/chaos-probe/scheduler/plan", strings.NewReader(`{"interval":"5m","requested_by":"gateway-test"}`))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"scheduler_plan_ready":true`) || !strings.Contains(w.Body.String(), `"scheduler_ready":false`) {
		t.Fatalf("scheduler plan status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestChaosProbePackCanWritePackLocalDegradeState(t *testing.T) {
	gw, tm := newTestGatewayWithChaosProbePack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("chaos-probe-degrade-state")

	definitionBody := `{"probes":[{"id":"custom-degraded-probe","name":"Custom degraded probe","category":"storage","description":"stored but no runner","safe":true,"enabled":true,"interval_seconds":30,"weight":1}],"replace":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chaos-probe/probes", strings.NewReader(definitionBody))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("save probe definition status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/chaos-probe/run", strings.NewReader(`{"probe_ids":["custom-degraded-probe"],"persist":true}`))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"gate_status":"warn"`) {
		t.Fatalf("run degraded probe status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/chaos-probe/degrade-state/writeback", strings.NewReader(`{"requested_by":"gateway-test","reason":"persist pack-local degrade state"}`))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted || !strings.Contains(w.Body.String(), `"degrade_state_store_ready":true`) || !strings.Contains(w.Body.String(), `"runtime_degrade_state_ready":false`) {
		t.Fatalf("degrade state writeback status=%d body=%s", w.Code, w.Body.String())
	}
}

func newTestGatewayWithChaosProbePack(t *testing.T, status packruntime.PackStatus) (*Gateway, *tenant.Manager) {
	t.Helper()
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           chaosprobepack.PackID,
		Name:         "Chaos Probe Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "disabled",
		Backend: packruntime.BackendManifest{
			Routes: []string{
				"/v1/chaos-probe/status",
				"/v1/chaos-probe/probes",
				"/v1/chaos-probe/run",
				"/v1/chaos-probe/scheduler/plan",
				"/v1/chaos-probe/degrade-state/writeback",
				"/v1/chaos-probe/reports",
				"/v1/chaos-probe/reports/",
				"/v1/chaos-probe/evidence/",
			},
			RouteSpecs: []packruntime.BackendRouteSpec{
				{Method: http.MethodGet, Path: "/v1/chaos-probe/status"},
				{Method: http.MethodGet, Path: "/v1/chaos-probe/probes"},
				{Method: http.MethodPost, Path: "/v1/chaos-probe/probes"},
				{Method: http.MethodPost, Path: "/v1/chaos-probe/run"},
				{Method: http.MethodPost, Path: "/v1/chaos-probe/scheduler/plan"},
				{Method: http.MethodPost, Path: "/v1/chaos-probe/degrade-state/writeback"},
				{Method: http.MethodGet, Path: "/v1/chaos-probe/reports"},
				{Method: http.MethodGet, Path: "/v1/chaos-probe/reports/"},
				{Method: http.MethodGet, Path: "/v1/chaos-probe/evidence/"},
			},
		},
		Frontend: packruntime.FrontendManifest{Menus: []packruntime.FrontendMenu{{Key: "chaos-probe", Label: "Chaos Probe", Path: "/packs/chaos-probe"}}},
		SDK:      packruntime.SDKManifest{TypeScript: "yunque-client/chaos-probe"},
		Update:   packruntime.UpdateManifest{Rollback: true},
	}, "test")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if status == packruntime.PackStatusEnabled {
		if _, err := registry.Enable(chaosprobepack.PackID); err != nil {
			t.Fatalf("Enable: %v", err)
		}
	}
	gw, tm := newTestGatewayWithConfig(GatewayConfig{Packs: registry})
	gw.RegisterBackendPack(chaosprobepack.New(chaosprobepack.Config{DataDir: t.TempDir()}))
	return gw, tm
}
