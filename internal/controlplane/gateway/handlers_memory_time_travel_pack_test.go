package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/controlplane/tenant"
	memorytimetravelpack "yunque-agent/internal/packs/memorytimetravel"
	"yunque-agent/pkg/packruntime"
)

func TestMemoryTimeTravelPackGateReturnsNotFoundWhenDisabled(t *testing.T) {
	gw, tm := newTestGatewayWithMemoryTimeTravelPack(t, packruntime.PackStatusDisabled)
	tenant := tm.Register("memory-time-travel-disabled")

	req := httptest.NewRequest(http.MethodGet, "/v1/memory-time-travel/status", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("disabled Memory Time Travel pack should gate status, status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestMemoryTimeTravelPackRoutesStatusWhenEnabled(t *testing.T) {
	gw, tm := newTestGatewayWithMemoryTimeTravelPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("memory-time-travel-enabled")

	req := httptest.NewRequest(http.MethodGet, "/v1/memory-time-travel/status", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "yunque.pack.memory-time-travel") {
		t.Fatalf("enabled Memory Time Travel pack should expose status, status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestMemoryTimeTravelPackRouteSpecsGateByMethod(t *testing.T) {
	gw, tm := newTestGatewayWithMemoryTimeTravelPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("memory-time-travel-method-gate")

	req := httptest.NewRequest(http.MethodGet, "/v1/memory-time-travel/diff", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /v1/memory-time-travel/diff should be blocked by pack method gate, status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestMemoryTimeTravelPackCanSaveSnapshotAndDiff(t *testing.T) {
	gw, tm := newTestGatewayWithMemoryTimeTravelPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("memory-time-travel-flow")

	for _, body := range []string{
		`{"id":"baseline","namespace":"memory_snapshot","values":{"goal":"ship","persona":"careful"}}`,
		`{"id":"candidate","namespace":"memory_snapshot","values":{"goal":"ship","persona":"careful","token":"redacted"}}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/snapshots", strings.NewReader(body))
		req.Header.Set("X-API-Key", tenant.APIKey)
		w := httptest.NewRecorder()
		gw.ServeHTTP(w, req)
		if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), "saved") {
			t.Fatalf("save snapshot status=%d body=%s", w.Code, w.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/diff", strings.NewReader(`{"namespace":"memory_snapshot","base_id":"baseline","target_id":"candidate"}`))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "drift_score") || !strings.Contains(w.Body.String(), "rollback_plan") {
		t.Fatalf("diff memory snapshots status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/rollback/approved-plan", strings.NewReader(`{"namespace":"memory_snapshot","snapshot_id":"baseline","requested_by":"operator","reason":"gateway smoke","approval_id":"approval-gateway-1","dry_run":true}`))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "approved_rollback_plan_ready") || !strings.Contains(w.Body.String(), "rollback_writeback_plan_ready") || !strings.Contains(w.Body.String(), `"writes_ledger_kv":false`) {
		t.Fatalf("approved rollback plan status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/memory-time-travel/kv-history/native-plan?namespace=memory_snapshot", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "native_kv_history_plan_ready") || !strings.Contains(w.Body.String(), "kv-history-migration-plan.json") || !strings.Contains(w.Body.String(), `"writes_native_kv_history":false`) {
		t.Fatalf("native kv_history plan status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/memory-time-travel/kv-history/migration-preview?namespace=memory_snapshot&limit=50", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "kv_history_migration_preview") || !strings.Contains(w.Body.String(), "kv-history-migration-preview.json") || !strings.Contains(w.Body.String(), `"writes_native_kv_history":false`) {
		t.Fatalf("native kv_history migration preview status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/kv-history/cutover/plan", strings.NewReader(`{"namespace":"memory_snapshot","requested_by":"operator","reason":"gateway smoke","limit":50,"dry_run":true}`))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "kv_history_cutover_plan_ready") || !strings.Contains(w.Body.String(), "dual_read_plan_ready") || !strings.Contains(w.Body.String(), "dual_write_plan_ready") || !strings.Contains(w.Body.String(), "kv-history-cutover-plan.json") || !strings.Contains(w.Body.String(), `"cutover_ready":false`) || !strings.Contains(w.Body.String(), `"writes_native_kv_history":false`) {
		t.Fatalf("native kv_history cutover plan status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/kv-history/dual-read/parity", strings.NewReader(`{"namespace":"memory_snapshot","at":"2026-05-15T12:00:00Z","limit":50}`))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "dual_read_parity_check_ready") || !strings.Contains(w.Body.String(), "kv-history-dual-read-parity.json") || !strings.Contains(w.Body.String(), `"switches_temporal_adapter":false`) || !strings.Contains(w.Body.String(), `"writes_ledger_kv":false`) {
		t.Fatalf("dual-read parity gate status=%d body=%s", w.Code, w.Body.String())
	}
}

func newTestGatewayWithMemoryTimeTravelPack(t *testing.T, status packruntime.PackStatus) (*Gateway, *tenant.Manager) {
	t.Helper()
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           memorytimetravelpack.PackID,
		Name:         "Memory Time Travel Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "disabled",
		Backend: packruntime.BackendManifest{
			Routes: []string{
				"/v1/memory-time-travel/status",
				"/v1/memory-time-travel/snapshots",
				"/v1/memory-time-travel/snapshots/",
				"/v1/memory-time-travel/snapshot-at",
				"/v1/memory-time-travel/diff",
				"/v1/memory-time-travel/rollback-plan",
				"/v1/memory-time-travel/rollback/approved-plan",
				"/v1/memory-time-travel/retention/plan",
				"/v1/memory-time-travel/retention/prune-plan",
				"/v1/memory-time-travel/kv-history/native-plan",
				"/v1/memory-time-travel/kv-history/migration-preview",
				"/v1/memory-time-travel/kv-history/dual-read/parity",
				"/v1/memory-time-travel/kv-history/cutover/plan",
				"/v1/memory-time-travel/audit/links",
				"/v1/memory-time-travel/audit/verify",
				"/v1/memory-time-travel/evidence/",
			},
			RouteSpecs: []packruntime.BackendRouteSpec{
				{Method: http.MethodGet, Path: "/v1/memory-time-travel/status"},
				{Method: http.MethodGet, Path: "/v1/memory-time-travel/snapshots"},
				{Method: http.MethodPost, Path: "/v1/memory-time-travel/snapshots"},
				{Method: http.MethodGet, Path: "/v1/memory-time-travel/snapshots/"},
				{Method: http.MethodPost, Path: "/v1/memory-time-travel/snapshot-at"},
				{Method: http.MethodPost, Path: "/v1/memory-time-travel/diff"},
				{Method: http.MethodPost, Path: "/v1/memory-time-travel/rollback-plan"},
				{Method: http.MethodPost, Path: "/v1/memory-time-travel/rollback/approved-plan"},
				{Method: http.MethodGet, Path: "/v1/memory-time-travel/retention/plan"},
				{Method: http.MethodPost, Path: "/v1/memory-time-travel/retention/prune-plan"},
				{Method: http.MethodGet, Path: "/v1/memory-time-travel/kv-history/native-plan"},
				{Method: http.MethodGet, Path: "/v1/memory-time-travel/kv-history/migration-preview"},
				{Method: http.MethodPost, Path: "/v1/memory-time-travel/kv-history/dual-read/parity"},
				{Method: http.MethodPost, Path: "/v1/memory-time-travel/kv-history/cutover/plan"},
				{Method: http.MethodGet, Path: "/v1/memory-time-travel/audit/links"},
				{Method: http.MethodGet, Path: "/v1/memory-time-travel/audit/verify"},
				{Method: http.MethodGet, Path: "/v1/memory-time-travel/evidence/"},
			},
		},
		Frontend: packruntime.FrontendManifest{Menus: []packruntime.FrontendMenu{{Key: "memory-time-travel", Label: "Memory Time Travel", Path: "/packs/memory-time-travel"}}},
		SDK:      packruntime.SDKManifest{TypeScript: "yunque-client/memory-time-travel"},
		Update:   packruntime.UpdateManifest{Rollback: true},
	}, "test")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if status == packruntime.PackStatusEnabled {
		if _, err := registry.Enable(memorytimetravelpack.PackID); err != nil {
			t.Fatalf("Enable: %v", err)
		}
	}
	gw, tm := newTestGatewayWithConfig(GatewayConfig{Packs: registry})
	gw.RegisterBackendPack(memorytimetravelpack.New(memorytimetravelpack.Config{DataDir: t.TempDir()}))
	return gw, tm
}
