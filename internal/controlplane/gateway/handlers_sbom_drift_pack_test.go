package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/controlplane/tenant"
	sbomdriftpack "yunque-agent/internal/packs/sbomdrift"
	"yunque-agent/pkg/packruntime"
)

func TestSBOMDriftPackGateReturnsNotFoundWhenDisabled(t *testing.T) {
	gw, tm := newTestGatewayWithSBOMDriftPack(t, packruntime.PackStatusDisabled)
	tenant := tm.Register("sbom-drift-disabled")

	req := httptest.NewRequest(http.MethodGet, "/v1/sbom-drift/status", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("disabled SBOM Drift pack should gate status, status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestSBOMDriftPackRoutesStatusWhenEnabled(t *testing.T) {
	gw, tm := newTestGatewayWithSBOMDriftPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("sbom-drift-enabled")

	req := httptest.NewRequest(http.MethodGet, "/v1/sbom-drift/status", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "yunque.pack.sbom-drift") {
		t.Fatalf("enabled SBOM Drift pack should expose status, status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestSBOMDriftPackRouteSpecsGateByMethod(t *testing.T) {
	gw, tm := newTestGatewayWithSBOMDriftPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("sbom-drift-method-gate")

	req := httptest.NewRequest(http.MethodGet, "/v1/sbom-drift/diff", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /v1/sbom-drift/diff should be blocked by pack method gate, status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestSBOMDriftPackCanCreateSnapshotAndDiff(t *testing.T) {
	gw, tm := newTestGatewayWithSBOMDriftPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("sbom-drift-flow")

	req := httptest.NewRequest(http.MethodPost, "/v1/sbom-drift/snapshots", strings.NewReader(`{"id":"baseline","source":"gateway-test"}`))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), "baseline") {
		t.Fatalf("create snapshot status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/sbom-drift/diff", strings.NewReader(`{"base_id":"baseline","target_current":true}`))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "risk_level") {
		t.Fatalf("diff status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestSBOMDriftPackCanExportCycloneDXAndPlanCIGate(t *testing.T) {
	gw, tm := newTestGatewayWithSBOMDriftPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("sbom-drift-ci-gate")

	req := httptest.NewRequest(http.MethodPost, "/v1/sbom-drift/snapshots", strings.NewReader(`{"id":"baseline","source":"gateway-test"}`))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create snapshot status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/sbom-drift/cyclonedx/baseline", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"bomFormat":"CycloneDX"`) {
		t.Fatalf("cyclonedx status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/sbom-drift/ci-gate/plan", strings.NewReader(`{"base_id":"baseline","target_current":true,"fail_on_risk":"high"}`))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"ci_gate_plan_ready":true`) || !strings.Contains(w.Body.String(), `"ci_gate_ready":false`) || !strings.Contains(w.Body.String(), `"govulncheck_plan_ready":true`) || !strings.Contains(w.Body.String(), `"govulncheck_ready":false`) {
		t.Fatalf("ci gate plan status=%d body=%s", w.Code, w.Body.String())
	}
}

func newTestGatewayWithSBOMDriftPack(t *testing.T, status packruntime.PackStatus) (*Gateway, *tenant.Manager) {
	t.Helper()
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           sbomdriftpack.PackID,
		Name:         "SBOM Drift Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "disabled",
		Backend: packruntime.BackendManifest{
			Routes: []string{
				"/v1/sbom-drift/status",
				"/v1/sbom-drift/snapshots",
				"/v1/sbom-drift/snapshots/",
				"/v1/sbom-drift/diff",
				"/v1/sbom-drift/cyclonedx/",
				"/v1/sbom-drift/ci-gate/plan",
				"/v1/sbom-drift/evidence/",
			},
			RouteSpecs: []packruntime.BackendRouteSpec{
				{Method: http.MethodGet, Path: "/v1/sbom-drift/status"},
				{Method: http.MethodGet, Path: "/v1/sbom-drift/snapshots"},
				{Method: http.MethodPost, Path: "/v1/sbom-drift/snapshots"},
				{Method: http.MethodGet, Path: "/v1/sbom-drift/snapshots/"},
				{Method: http.MethodPost, Path: "/v1/sbom-drift/diff"},
				{Method: http.MethodGet, Path: "/v1/sbom-drift/cyclonedx/"},
				{Method: http.MethodPost, Path: "/v1/sbom-drift/ci-gate/plan"},
				{Method: http.MethodGet, Path: "/v1/sbom-drift/evidence/"},
			},
		},
		Frontend: packruntime.FrontendManifest{Menus: []packruntime.FrontendMenu{{Key: "sbom-drift", Label: "SBOM 漂移", Path: "/packs/sbom-drift"}}},
		SDK:      packruntime.SDKManifest{TypeScript: "yunque-client/sbom-drift"},
		Update:   packruntime.UpdateManifest{Rollback: true},
	}, "test")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if status == packruntime.PackStatusEnabled {
		if _, err := registry.Enable(sbomdriftpack.PackID); err != nil {
			t.Fatalf("Enable: %v", err)
		}
	}
	gw, tm := newTestGatewayWithConfig(GatewayConfig{Packs: registry})
	gw.RegisterBackendPack(sbomdriftpack.New(sbomdriftpack.Config{RepoRoot: t.TempDir(), DataDir: t.TempDir()}))
	return gw, tm
}
