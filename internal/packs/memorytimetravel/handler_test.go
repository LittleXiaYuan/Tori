package memorytimetravel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeTemporalKV struct {
	snapshot map[string][]byte
}

func (f fakeTemporalKV) SnapshotRawAt(context.Context, string, time.Time) (map[string][]byte, error) {
	return f.snapshot, nil
}

type fakeNativeKVHistoryPreviewer struct {
	preview   NativeKVHistoryMigrationPreview
	namespace string
	limit     int
}

func (f *fakeNativeKVHistoryPreviewer) PreviewNativeKVHistoryRows(_ context.Context, namespace string, limit int) (NativeKVHistoryMigrationPreview, error) {
	f.namespace = namespace
	f.limit = limit
	return f.preview, nil
}

type fakeMerkleVerifier struct {
	result MerkleVerification
	limit  int
}

func (f *fakeMerkleVerifier) VerifyMerkleAuditChain(_ context.Context, limit int) (MerkleVerification, error) {
	f.limit = limit
	return f.result, nil
}

func TestMemoryTimeTravelHandlerRoutesExposePackShellSurface(t *testing.T) {
	h := New(Config{DataDir: t.TempDir()})
	if h.PackID() != PackID {
		t.Fatalf("PackID = %q, want %q", h.PackID(), PackID)
	}
	routes := h.Routes()
	if len(routes) != 14 {
		t.Fatalf("expected 14 Memory Time Travel routes, got %d", len(routes))
	}
	byPath := map[string][]string{}
	for _, route := range routes {
		if route.Path == "" || route.Handler == nil {
			t.Fatalf("route must declare path and handler: %#v", route)
		}
		methods := append([]string{}, route.Methods...)
		if route.Method != "" {
			methods = append([]string{route.Method}, methods...)
		}
		if len(methods) == 0 {
			t.Fatalf("route must declare method(s): %#v", route)
		}
		byPath[route.Path] = methods
	}
	expected := map[string][]string{
		"/v1/memory-time-travel/status":                       {http.MethodGet},
		"/v1/memory-time-travel/snapshots":                    {http.MethodGet, http.MethodPost},
		"/v1/memory-time-travel/snapshots/":                   {http.MethodGet},
		"/v1/memory-time-travel/snapshot-at":                  {http.MethodPost},
		"/v1/memory-time-travel/diff":                         {http.MethodPost},
		"/v1/memory-time-travel/rollback-plan":                {http.MethodPost},
		"/v1/memory-time-travel/rollback/approved-plan":       {http.MethodPost},
		"/v1/memory-time-travel/retention/plan":               {http.MethodGet},
		"/v1/memory-time-travel/retention/prune-plan":         {http.MethodPost},
		"/v1/memory-time-travel/kv-history/native-plan":       {http.MethodGet},
		"/v1/memory-time-travel/kv-history/migration-preview": {http.MethodGet},
		"/v1/memory-time-travel/audit/links":                  {http.MethodGet},
		"/v1/memory-time-travel/audit/verify":                 {http.MethodGet},
		"/v1/memory-time-travel/evidence/":                    {http.MethodGet},
	}
	for path, methods := range expected {
		got := strings.Join(byPath[path], ",")
		want := strings.Join(methods, ",")
		if got != want {
			t.Fatalf("expected %s methods %s, got %s", path, want, got)
		}
	}
}

func TestMemoryTimeTravelSnapshotDiffRollbackAndEvidence(t *testing.T) {
	now := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	h := New(Config{DataDir: t.TempDir(), Now: func() time.Time { return now }})

	baseBody := `{"id":"baseline","namespace":"memory_snapshot","source":"test","values":{"goal":"ship pack runtime","persona":"careful"}}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/snapshots", strings.NewReader(baseBody))
	h.Snapshots(w, req)
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), "baseline") {
		t.Fatalf("save baseline status=%d body=%s", w.Code, w.Body.String())
	}

	now = now.Add(time.Hour)
	targetBody := `{"id":"after-drift","namespace":"memory_snapshot","source":"test","values":{"goal":"ship pack runtime","persona":"careful","api_token":"redacted"}}`
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/snapshots", strings.NewReader(targetBody))
	h.Snapshots(w, req)
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), "after-drift") {
		t.Fatalf("save target status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/diff", strings.NewReader(`{"namespace":"memory_snapshot","base_id":"baseline","target_id":"after-drift"}`))
	h.Diff(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "api_token") || !strings.Contains(w.Body.String(), "high") {
		t.Fatalf("diff status=%d body=%s", w.Code, w.Body.String())
	}

	var diff struct {
		Diff DiffReport `json:"diff"`
	}
	if err := json.NewDecoder(w.Body).Decode(&diff); err != nil {
		t.Fatalf("decode diff: %v", err)
	}
	if diff.Diff.AddedCount != 1 || diff.Diff.RiskLevel != "high" || len(diff.Diff.RollbackPlan) == 0 {
		t.Fatalf("unexpected diff report: %#v", diff.Diff)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/snapshot-at", strings.NewReader(`{"namespace":"memory_snapshot","at":"2026-05-15T12:30:00Z"}`))
	h.SnapshotAt(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "reconstructed") || !strings.Contains(w.Body.String(), "baseline") {
		t.Fatalf("snapshot-at status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/rollback-plan", strings.NewReader(`{"namespace":"memory_snapshot","snapshot_id":"baseline","dry_run":true}`))
	h.RollbackPlan(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "put goal=ship pack runtime") {
		t.Fatalf("rollback plan status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/rollback/approved-plan", strings.NewReader(`{"namespace":"memory_snapshot","snapshot_id":"baseline","requested_by":"operator","reason":"restore known-good memory","approval_id":"approval-123","dry_run":true}`))
	h.ApprovedRollbackPlan(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "approved-rollback-plan.json") || !strings.Contains(w.Body.String(), "rollback-writeback-plan.json") || !strings.Contains(w.Body.String(), "approval_request_plan_ready") {
		t.Fatalf("approved rollback plan status=%d body=%s", w.Code, w.Body.String())
	}
	var approved struct {
		Plan ApprovedRollbackExecutionPlanReport `json:"plan"`
	}
	if err := json.NewDecoder(w.Body).Decode(&approved); err != nil {
		t.Fatalf("decode approved rollback plan: %v", err)
	}
	if !approved.Plan.ApprovedRollbackPlanReady || !approved.Plan.RollbackWritebackPlanReady || approved.Plan.RollbackWritebackReady || approved.Plan.WritesLedgerKV || approved.Plan.WritesTemporalKV || approved.Plan.GlobalApprovalEnqueueReady {
		t.Fatalf("approved rollback plan must stay plan-only and non-destructive: %#v", approved.Plan)
	}
	if approved.Plan.ProposedApprovalRequest.RiskLevel != "high" || approved.Plan.ProposedApprovalRequest.Category != "data_mutation" || approved.Plan.ProposedApprovalRequest.Requester != "operator" {
		t.Fatalf("approval request should align global Approval Manager field shape: %#v", approved.Plan.ProposedApprovalRequest)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/memory-time-travel/evidence/baseline", nil)
	h.Evidence(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "json-memory-time-travel-evidence") || !strings.Contains(w.Body.String(), "snapshot.json") || !strings.Contains(w.Body.String(), "approved-rollback-plan.json") || !strings.Contains(w.Body.String(), "rollback-writeback-plan.json") || !strings.Contains(w.Body.String(), "approval-request-plan.json") || !strings.Contains(w.Body.String(), "retention-plan.json") || !strings.Contains(w.Body.String(), "retention-prune-plan.json") || !strings.Contains(w.Body.String(), "native-kv-history-plan.json") || !strings.Contains(w.Body.String(), "kv-history-migration-plan.json") || !strings.Contains(w.Body.String(), "kv-history-index-plan.json") || !strings.Contains(w.Body.String(), "audit-links.json") || !strings.Contains(w.Body.String(), "audit-verification.json") {
		t.Fatalf("evidence status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestMemoryTimeTravelDryRunDoesNotPersistSnapshot(t *testing.T) {
	h := New(Config{DataDir: t.TempDir()})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/snapshots", strings.NewReader(`{"id":"dry","values":{"k":"v"},"dry_run":true}`))
	h.Snapshots(w, req)
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), "dry_run") {
		t.Fatalf("dry-run save status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/memory-time-travel/snapshots/dry", nil)
	h.SnapshotDetail(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("dry-run snapshot should not persist, status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestMemoryTimeTravelSnapshotAtUsesLedgerTemporalKVWhenAttached(t *testing.T) {
	h := New(Config{
		DataDir:                  t.TempDir(),
		MemoryPersisterWriteback: true,
		TemporalKV: fakeTemporalKV{snapshot: map[string][]byte{
			"goal":    []byte(`"ship temporal kv"`),
			"persona": []byte(`{"mode":"careful"}`),
		}},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/snapshot-at", strings.NewReader(`{"namespace":"memory_snapshot","at":"2026-05-15T12:30:00Z"}`))
	h.SnapshotAt(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("snapshot-at status=%d body=%s", w.Code, w.Body.String())
	}

	var got SnapshotAtResponse
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode snapshot-at: %v", err)
	}
	if got.Source != "ledger-kv-history" || got.Values["goal"] != "ship temporal kv" || got.Values["persona"] != `{"mode":"careful"}` {
		t.Fatalf("unexpected temporal kv snapshot-at response: %#v", got)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/memory-time-travel/status", nil)
	h.Status(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"memory_persister_writeback_ready":true`) {
		t.Fatalf("status should expose Memory Persister temporal write-back readiness, status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestMemoryTimeTravelRetentionPlanIsDryRun(t *testing.T) {
	now := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	h := New(Config{
		DataDir: t.TempDir(),
		Now:     func() time.Time { return now },
		Policy:  RetentionPolicy{RetentionDays: 7, MaxSnapshotsPerNamespace: 2},
	})

	for _, item := range []struct {
		id string
		at time.Time
	}{
		{id: "old-baseline", at: now.AddDate(0, 0, -10)},
		{id: "middle-baseline", at: now.Add(-time.Hour)},
		{id: "new-baseline", at: now},
	} {
		now = item.at
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/snapshots", strings.NewReader(`{"id":"`+item.id+`","namespace":"memory_snapshot","values":{"goal":"ship"}}`))
		h.Snapshots(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("save %s status=%d body=%s", item.id, w.Code, w.Body.String())
		}
	}
	now = time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/memory-time-travel/status", nil)
	h.Status(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"approved_rollback_plan_ready":true`) || !strings.Contains(w.Body.String(), `"rollback_writeback_plan_ready":true`) || !strings.Contains(w.Body.String(), `"global_approval_enqueue_ready":false`) || !strings.Contains(w.Body.String(), `"retention_plan_ready":true`) || !strings.Contains(w.Body.String(), `"retention_prune_plan_ready":true`) || !strings.Contains(w.Body.String(), `"native_kv_history_plan_ready":true`) || !strings.Contains(w.Body.String(), `"kv_history_migration_plan_ready":true`) || !strings.Contains(w.Body.String(), `"native_kv_history_preview_ready":false`) || !strings.Contains(w.Body.String(), `"native_kv_history_ready":false`) || !strings.Contains(w.Body.String(), `"writes_native_kv_history":false`) || !strings.Contains(w.Body.String(), `"migrates_kv_history":false`) || !strings.Contains(w.Body.String(), `"kv_audit_link_schema_ready":true`) || !strings.Contains(w.Body.String(), "memory.rollback.approved_plan") || !strings.Contains(w.Body.String(), "memory.rollback.writeback.plan") || !strings.Contains(w.Body.String(), "memory.retention.plan") || !strings.Contains(w.Body.String(), "memory.retention.prune_plan") || !strings.Contains(w.Body.String(), "memory.kv_history.native_plan") || !strings.Contains(w.Body.String(), "memory.kv_history.migration_preview") || !strings.Contains(w.Body.String(), "memory.audit.links.schema") {
		t.Fatalf("status should expose retention dry-run readiness, status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/memory-time-travel/retention/plan?namespace=memory_snapshot", nil)
	h.RetentionPlan(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("retention plan status=%d body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Plan RetentionPlanReport `json:"plan"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode retention plan: %v", err)
	}
	if !got.Plan.DryRun || got.Plan.Status != "dry_run" || got.Plan.CandidateCount != 1 || got.Plan.KeepCount != 2 {
		t.Fatalf("unexpected retention plan: %#v", got.Plan)
	}
	if got.Plan.Candidates[0].ID != "old-baseline" || !containsString(got.Plan.Candidates[0].Reasons, "older_than_retention_days:7") || !strings.Contains(got.Plan.Actions[0], "would delete pack-local snapshot") {
		t.Fatalf("unexpected retention candidate: %#v", got.Plan.Candidates)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/memory-time-travel/snapshots/old-baseline", nil)
	h.SnapshotDetail(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("retention plan must not delete snapshots, status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestMemoryTimeTravelRetentionPrunePlanRequiresApprovalAndDoesNotDelete(t *testing.T) {
	now := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	h := New(Config{
		DataDir: t.TempDir(),
		Now:     func() time.Time { return now },
		Policy:  RetentionPolicy{RetentionDays: 7, MaxSnapshotsPerNamespace: 2},
	})

	now = now.AddDate(0, 0, -10)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/snapshots", strings.NewReader(`{"id":"old-baseline","namespace":"memory_snapshot","values":{"goal":"ship"}}`))
	h.Snapshots(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("save old snapshot status=%d body=%s", w.Code, w.Body.String())
	}
	now = time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/retention/prune-plan", strings.NewReader(`{"namespace":"memory_snapshot","candidate_ids":["old-baseline"],"requested_by":"operator","reason":"policy review","dry_run":true}`))
	h.RetentionPrunePlan(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("retention prune plan status=%d body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Plan RetentionPrunePlanReport `json:"plan"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode retention prune plan: %v", err)
	}
	if !got.Plan.DryRun || got.Plan.Status != "approval_plan" || !got.Plan.ApprovalRequired || got.Plan.PruneReady || got.Plan.SelectedCandidateCount != 1 {
		t.Fatalf("unexpected retention prune plan: %#v", got.Plan)
	}
	if !strings.Contains(got.Plan.Actions[0], "requires approval before deleting pack-local snapshot memory-snapshot/old-baseline") {
		t.Fatalf("unexpected prune action: %#v", got.Plan.Actions)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/memory-time-travel/snapshots/old-baseline", nil)
	h.SnapshotDetail(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("retention prune plan must not delete snapshots, status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestMemoryTimeTravelAuditLinksExposeSchemaPlaceholder(t *testing.T) {
	now := time.Date(2026, 5, 15, 15, 0, 0, 0, time.UTC)
	h := New(Config{DataDir: t.TempDir(), Now: func() time.Time { return now }})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/memory-time-travel/audit/links?namespace=memory_snapshot", nil)
	h.AuditLinks(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("audit links status=%d body=%s", w.Code, w.Body.String())
	}

	var got struct {
		Links KVAuditLinksReport `json:"links"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode audit links: %v", err)
	}
	if !got.Links.SchemaReady || got.Links.LinkageReady || got.Links.NativeKVHistoryReady || got.Links.Namespace != "memory-snapshot" {
		t.Fatalf("unexpected audit links placeholder: %#v", got.Links)
	}
	if len(got.Links.KVAuditLinks) != 0 || !containsString(got.Links.RequiredFields, "audit_hash") || got.Links.Source != "schema-placeholder-before-native-kv-history" {
		t.Fatalf("unexpected audit link schema: %#v", got.Links)
	}
}

func TestMemoryTimeTravelNativeKVHistoryPlanIsNonDestructive(t *testing.T) {
	now := time.Date(2026, 5, 15, 16, 0, 0, 0, time.UTC)
	h := New(Config{DataDir: t.TempDir(), Now: func() time.Time { return now }})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/memory-time-travel/kv-history/native-plan?namespace=memory_snapshot", nil)
	h.NativeKVHistoryPlan(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("native kv_history plan status=%d body=%s", w.Code, w.Body.String())
	}

	var got struct {
		Plan NativeKVHistoryPlanReport `json:"plan"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode native kv_history plan: %v", err)
	}
	if got.Plan.Stage != "native-kv-history-plan-before-schema-migration" || got.Plan.Namespace != "memory-snapshot" {
		t.Fatalf("unexpected native kv_history plan identity: %#v", got.Plan)
	}
	if !got.Plan.NativeKVHistoryPlanReady || !got.Plan.KVHistoryMigrationPlanReady || !got.Plan.KVHistoryIndexPlanReady || got.Plan.NativeKVHistoryReady || got.Plan.WritesNativeKVHistory || got.Plan.MigratesKVHistory {
		t.Fatalf("native kv_history plan must stay plan-only and non-destructive: %#v", got.Plan)
	}
	if got.Plan.CurrentHistoryNamespace != "__kv_history__" || got.Plan.NativeTable != "kv_history" || !containsString(got.Plan.Artifacts, "native-kv-history-plan.json") || !containsString(got.Plan.Artifacts, "kv-history-migration-plan.json") || !containsString(got.Plan.Artifacts, "kv-history-index-plan.json") {
		t.Fatalf("native kv_history artifacts or adapter details drifted: %#v", got.Plan)
	}
	if len(got.Plan.SchemaPlan) == 0 || len(got.Plan.KVHistoryMigrationPlan) == 0 || len(got.Plan.KVHistoryIndexPlan) == 0 || !containsString(got.Plan.BlockedBy, "ledger-native-kv-history-schema-not-wired") {
		t.Fatalf("native kv_history plan should include schema/index/migration blockers: %#v", got.Plan)
	}
}

func TestMemoryTimeTravelNativeKVHistoryMigrationPreviewIsNonDestructive(t *testing.T) {
	now := time.Date(2026, 5, 15, 16, 30, 0, 0, time.UTC)
	previewer := &fakeNativeKVHistoryPreviewer{preview: NativeKVHistoryMigrationPreview{
		Namespace:             "memory_snapshot",
		GeneratedAt:           now,
		SourceNamespace:       "__kv_history__",
		NativeTable:           "kv_history",
		ScannedDocumentCount:  1,
		PreviewRowCount:       2,
		ReturnedRowCount:      2,
		WritesNativeKVHistory: false,
		MigratesKVHistory:     false,
		Rows: []NativeKVHistoryRowPreview{
			{ID: "kvh-1", Namespace: "memory_snapshot", Key: "goal", Version: 1, Value: []byte(`"ship"`), ValueSHA256: valueHash(`"ship"`), UpdatedAt: now.Add(-time.Hour), SourceAdapter: "reserved-ledger-kv-namespace"},
			{ID: "kvh-2", Namespace: "memory_snapshot", Key: "goal", Version: 2, Value: []byte(`"ship runtime"`), ValueSHA256: valueHash(`"ship runtime"`), UpdatedAt: now, Current: true, SourceAdapter: "reserved-ledger-kv-namespace"},
		},
	}}
	h := New(Config{DataDir: t.TempDir(), Now: func() time.Time { return now }, NativeKVHistoryPreviewer: previewer})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/memory-time-travel/status", nil)
	h.Status(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"native_kv_history_preview_ready":true`) {
		t.Fatalf("status should expose native preview adapter readiness, status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/memory-time-travel/kv-history/migration-preview?namespace=memory_snapshot&limit=2", nil)
	h.NativeKVHistoryMigrationPreview(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("native kv_history preview status=%d body=%s", w.Code, w.Body.String())
	}

	var got struct {
		Preview NativeKVHistoryMigrationPreview `json:"kv_history_migration_preview"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode native kv_history preview: %v", err)
	}
	if previewer.namespace != "memory-snapshot" || previewer.limit != 2 {
		t.Fatalf("previewer received namespace=%q limit=%d", previewer.namespace, previewer.limit)
	}
	if got.Preview.PackID != PackID || got.Preview.Stage != "native-kv-history-migration-preview-before-native-write" || got.Preview.Status != "preview_only" || !got.Preview.NativeKVHistoryPreviewReady {
		t.Fatalf("unexpected preview identity: %#v", got.Preview)
	}
	if got.Preview.WritesNativeKVHistory || got.Preview.MigratesKVHistory || !got.Preview.UsesReservedKVNamespace || len(got.Preview.Rows) != 2 {
		t.Fatalf("native kv_history preview must stay read-only: %#v", got.Preview)
	}
	if !containsString(got.Preview.Artifacts, "kv-history-migration-preview.json") || got.Preview.Rows[1].ValueSHA256 != valueHash(`"ship runtime"`) {
		t.Fatalf("unexpected preview artifacts or row digest: %#v", got.Preview)
	}
}

func TestMemoryTimeTravelAuditVerifyUsesMerkleVerifier(t *testing.T) {
	now := time.Date(2026, 5, 15, 13, 0, 0, 0, time.UTC)
	verifier := &fakeMerkleVerifier{result: MerkleVerification{
		Ready:        true,
		Valid:        true,
		InvalidIndex: -1,
		RecordCount:  2,
		LastSeq:      2,
		LastHash:     "hash-2",
		RecentRecords: []MerkleAuditRecord{
			{Seq: 2, Timestamp: now, Type: "memory", Actor: "tenant", Action: "flush", PrevHash: "hash-1", Hash: "hash-2"},
		},
	}}
	h := New(Config{DataDir: t.TempDir(), Now: func() time.Time { return now }, MerkleVerifier: verifier})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/memory-time-travel/status", nil)
	h.Status(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"merkle_verification_ready":true`) || !strings.Contains(w.Body.String(), "memory.audit.verify") {
		t.Fatalf("status should expose Merkle verifier readiness, status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/memory-time-travel/audit/verify?limit=3", nil)
	h.AuditVerify(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("audit verify status=%d body=%s", w.Code, w.Body.String())
	}
	var got MerkleVerification
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode audit verify: %v", err)
	}
	if verifier.limit != 3 || !got.Ready || !got.Valid || got.RecordCount != 2 || got.LastHash != "hash-2" || len(got.RecentRecords) != 1 {
		t.Fatalf("unexpected Merkle verification response: limit=%d got=%#v", verifier.limit, got)
	}
	if got.CheckedAt != now {
		t.Fatalf("expected handler to fill checked_at from Now when verifier omits it, got %s", got.CheckedAt)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/memory-time-travel/evidence/missing", nil)
	h.Evidence(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("missing evidence should still 404 before audit verification, status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestMemoryTimeTravelEvidenceIncludesMerkleAuditVerificationWhenAttached(t *testing.T) {
	now := time.Date(2026, 5, 15, 14, 0, 0, 0, time.UTC)
	verifier := &fakeMerkleVerifier{result: MerkleVerification{
		Ready:        true,
		Valid:        true,
		InvalidIndex: -1,
		RecordCount:  1,
		LastSeq:      1,
		LastHash:     "hash-1",
	}}
	h := New(Config{DataDir: t.TempDir(), Now: func() time.Time { return now }, MerkleVerifier: verifier})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/snapshots", strings.NewReader(`{"id":"baseline","values":{"goal":"ship"}}`))
	h.Snapshots(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("save snapshot status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/memory-time-travel/evidence/baseline", nil)
	h.Evidence(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("evidence status=%d body=%s", w.Code, w.Body.String())
	}

	var got struct {
		Files                     []string                            `json:"files"`
		ApprovedRollbackPlan      ApprovedRollbackExecutionPlanReport `json:"approved_rollback_plan"`
		RollbackWritebackPlan     []RollbackWritebackActionPlan       `json:"rollback_writeback_plan"`
		ApprovalRequestPlan       GlobalApprovalRequestPlan           `json:"approval_request_plan"`
		RetentionPlan             RetentionPlanReport                 `json:"retention_plan"`
		RetentionPrunePlan        RetentionPrunePlanReport            `json:"retention_prune_plan"`
		NativeKVHistoryPlan       NativeKVHistoryPlanReport           `json:"native_kv_history_plan"`
		KVHistoryMigrationPlan    []KVHistoryMigrationStepPlan        `json:"kv_history_migration_plan"`
		KVHistoryIndexPlan        []NativeKVHistoryIndexPlan          `json:"kv_history_index_plan"`
		KVHistoryMigrationPreview NativeKVHistoryMigrationPreview     `json:"kv_history_migration_preview"`
		KVAuditLinkSchema         KVAuditLinksReport                  `json:"kv_audit_link_schema"`
		KVAuditLinks              []KVAuditProofLink                  `json:"kv_audit_links"`
		AuditVerification         MerkleVerification                  `json:"audit_verification"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode evidence: %v", err)
	}
	if verifier.limit != 10 || got.AuditVerification.LastHash != "hash-1" || got.AuditVerification.CheckedAt != now {
		t.Fatalf("unexpected audit verification evidence: limit=%d got=%#v", verifier.limit, got.AuditVerification)
	}
	if !containsString(got.Files, "audit-verification.json") {
		t.Fatalf("evidence files should include audit-verification.json: %#v", got.Files)
	}
	if !containsString(got.Files, "approved-rollback-plan.json") || !containsString(got.Files, "rollback-writeback-plan.json") || !containsString(got.Files, "approval-request-plan.json") || !got.ApprovedRollbackPlan.ApprovedRollbackPlanReady || got.ApprovedRollbackPlan.RollbackWritebackReady || got.ApprovalRequestPlan.GlobalApprovalEnqueueReady || len(got.RollbackWritebackPlan) == 0 {
		t.Fatalf("evidence should include approved rollback writeback plan preview: files=%#v approved=%#v approval=%#v writebacks=%#v", got.Files, got.ApprovedRollbackPlan, got.ApprovalRequestPlan, got.RollbackWritebackPlan)
	}
	if !containsString(got.Files, "audit-links.json") || !got.KVAuditLinkSchema.SchemaReady || got.KVAuditLinkSchema.LinkageReady || len(got.KVAuditLinks) != 0 {
		t.Fatalf("evidence should include KV audit link schema placeholder: files=%#v schema=%#v links=%#v", got.Files, got.KVAuditLinkSchema, got.KVAuditLinks)
	}
	if !containsString(got.Files, "retention-prune-plan.json") || !got.RetentionPrunePlan.DryRun || got.RetentionPrunePlan.PruneReady {
		t.Fatalf("evidence should include dry-run retention prune plan: files=%#v plan=%#v", got.Files, got.RetentionPrunePlan)
	}
	if !containsString(got.Files, "retention-plan.json") || !got.RetentionPlan.DryRun {
		t.Fatalf("evidence should include dry-run retention plan: files=%#v plan=%#v", got.Files, got.RetentionPlan)
	}
	if !containsString(got.Files, "native-kv-history-plan.json") || !containsString(got.Files, "kv-history-migration-plan.json") || !containsString(got.Files, "kv-history-index-plan.json") || !got.NativeKVHistoryPlan.NativeKVHistoryPlanReady || got.NativeKVHistoryPlan.NativeKVHistoryReady || got.NativeKVHistoryPlan.WritesNativeKVHistory || len(got.KVHistoryMigrationPlan) == 0 || len(got.KVHistoryIndexPlan) == 0 {
		t.Fatalf("evidence should include native kv_history plan-only artifacts: files=%#v plan=%#v migration=%#v indexes=%#v", got.Files, got.NativeKVHistoryPlan, got.KVHistoryMigrationPlan, got.KVHistoryIndexPlan)
	}
	if !containsString(got.Files, "kv-history-migration-preview.json") {
		t.Fatalf("evidence files should include native kv_history migration preview artifact: %#v", got.Files)
	}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
