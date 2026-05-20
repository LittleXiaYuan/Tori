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

	req = httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/retention/prune/execute", strings.NewReader(`{"namespace":"memory_snapshot","candidate_ids":["baseline"],"requested_by":"operator","reason":"gateway approved local cleanup","approval_id":"approval-retention-gateway","approved":true}`))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "pack_local_prune_ready") || !strings.Contains(w.Body.String(), "retention-prune-execute.json") || !strings.Contains(w.Body.String(), `"writes_pack_local_snapshot_store":false`) || !strings.Contains(w.Body.String(), `"no-retention-candidates"`) || !strings.Contains(w.Body.String(), `"writes_ledger_kv":false`) || !strings.Contains(w.Body.String(), `"writes_native_kv_history":false`) || !strings.Contains(w.Body.String(), `"merkle_append_ready":false`) {
		t.Fatalf("pack-local retention prune execute no-op status=%d body=%s", w.Code, w.Body.String())
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

	req = httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/kv-history/cutover/readiness", strings.NewReader(`{"namespace":"memory_snapshot","at":"2026-05-15T12:00:00Z","requested_by":"operator","reason":"gateway readiness smoke","limit":50,"dry_run":true}`))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "cutover_readiness_check_ready") || !strings.Contains(w.Body.String(), "kv-history-cutover-readiness.json") || !strings.Contains(w.Body.String(), `"cutover_ready":false`) || !strings.Contains(w.Body.String(), `"switches_temporal_adapter":false`) || !strings.Contains(w.Body.String(), `"writes_ledger_kv":false`) {
		t.Fatalf("cutover readiness gate status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/audit/links/preview", strings.NewReader(`{"namespace":"memory_snapshot","at":"2026-05-15T12:00:00Z","limit":50,"dry_run":true}`))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "kv_audit_link_preview_ready") || !strings.Contains(w.Body.String(), "audit-link-preview.json") || !strings.Contains(w.Body.String(), `"linkage_ready":false`) || !strings.Contains(w.Body.String(), `"writes_native_kv_history":false`) || !strings.Contains(w.Body.String(), `"merkle_append_ready":false`) {
		t.Fatalf("audit proof-link preview gate status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/audit/links/writeback-plan", strings.NewReader(`{"namespace":"memory_snapshot","at":"2026-05-15T12:00:00Z","limit":50,"requested_by":"operator","reason":"gateway writeback smoke","approval_id":"approval-link-gateway","dry_run":true}`))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "kv_audit_link_writeback_plan_ready") || !strings.Contains(w.Body.String(), "audit-link-writeback-plan.json") || !strings.Contains(w.Body.String(), `"kv_audit_link_writeback_ready":false`) || !strings.Contains(w.Body.String(), `"writes_ledger_kv":false`) || !strings.Contains(w.Body.String(), `"backfills_audit_seq":false`) {
		t.Fatalf("audit proof-link writeback plan status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/audit/links/writeback/store", strings.NewReader(`{"namespace":"memory_snapshot","at":"2026-05-15T12:00:00Z","limit":50,"requested_by":"operator","reason":"gateway writeback store smoke","approval_id":"approval-link-store-gateway","dry_run":true}`))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "kv_audit_link_writeback_store_ready") || !strings.Contains(w.Body.String(), "audit-link-writeback-store.json") || !strings.Contains(w.Body.String(), "audit-link-writeback-record.json") || !strings.Contains(w.Body.String(), `"writes_audit_link_writeback_store":true`) || !strings.Contains(w.Body.String(), `"kv_audit_link_writeback_ready":false`) || !strings.Contains(w.Body.String(), `"writes_ledger_kv":false`) || !strings.Contains(w.Body.String(), `"backfills_audit_seq":false`) {
		t.Fatalf("audit proof-link writeback store status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/audit/links/writeback/executor/plan", strings.NewReader(`{"request_key":"approval-link-store-gateway","requested_by":"operator","reason":"gateway executor plan smoke","dry_run":true}`))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "kv_audit_link_writeback_executor_plan_ready") || !strings.Contains(w.Body.String(), "executor_input_contract_ready") || !strings.Contains(w.Body.String(), "audit-link-writeback-executor-plan.json") || !strings.Contains(w.Body.String(), "audit-link-executor-handoff-plan.json") || !strings.Contains(w.Body.String(), `"consumes_audit_link_writeback_store":true`) || !strings.Contains(w.Body.String(), `"audit_proof_link_executor_ready":false`) || !strings.Contains(w.Body.String(), `"writes_native_kv_history":false`) || !strings.Contains(w.Body.String(), `"backfills_audit_seq":false`) {
		t.Fatalf("audit proof-link executor plan status=%d body=%s", w.Code, w.Body.String())
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
				"/v1/memory-time-travel/retention/prune/execute",
				"/v1/memory-time-travel/kv-history/native-plan",
				"/v1/memory-time-travel/kv-history/migration-preview",
				"/v1/memory-time-travel/kv-history/dual-read/parity",
				"/v1/memory-time-travel/kv-history/cutover/plan",
				"/v1/memory-time-travel/kv-history/cutover/readiness",
				"/v1/memory-time-travel/audit/links",
				"/v1/memory-time-travel/audit/links/preview",
				"/v1/memory-time-travel/audit/links/writeback-plan",
				"/v1/memory-time-travel/audit/links/writeback/store",
				"/v1/memory-time-travel/audit/links/writeback/executor/plan",
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
				{Method: http.MethodPost, Path: "/v1/memory-time-travel/retention/prune/execute"},
				{Method: http.MethodGet, Path: "/v1/memory-time-travel/kv-history/native-plan"},
				{Method: http.MethodGet, Path: "/v1/memory-time-travel/kv-history/migration-preview"},
				{Method: http.MethodPost, Path: "/v1/memory-time-travel/kv-history/dual-read/parity"},
				{Method: http.MethodPost, Path: "/v1/memory-time-travel/kv-history/cutover/plan"},
				{Method: http.MethodPost, Path: "/v1/memory-time-travel/kv-history/cutover/readiness"},
				{Method: http.MethodGet, Path: "/v1/memory-time-travel/audit/links"},
				{Method: http.MethodPost, Path: "/v1/memory-time-travel/audit/links/preview"},
				{Method: http.MethodPost, Path: "/v1/memory-time-travel/audit/links/writeback-plan"},
				{Method: http.MethodPost, Path: "/v1/memory-time-travel/audit/links/writeback/store"},
				{Method: http.MethodPost, Path: "/v1/memory-time-travel/audit/links/writeback/executor/plan"},
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
