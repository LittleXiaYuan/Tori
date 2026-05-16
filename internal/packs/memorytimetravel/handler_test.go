package memorytimetravel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeTemporalKV struct {
	snapshot  map[string][]byte
	namespace string
	at        time.Time
}

func (f *fakeTemporalKV) SnapshotRawAt(_ context.Context, namespace string, at time.Time) (map[string][]byte, error) {
	f.namespace = namespace
	f.at = at
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
	if len(routes) != 21 {
		t.Fatalf("expected 21 Memory Time Travel routes, got %d", len(routes))
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
		"/v1/memory-time-travel/status":                              {http.MethodGet},
		"/v1/memory-time-travel/snapshots":                           {http.MethodGet, http.MethodPost},
		"/v1/memory-time-travel/snapshots/":                          {http.MethodGet},
		"/v1/memory-time-travel/snapshot-at":                         {http.MethodPost},
		"/v1/memory-time-travel/diff":                                {http.MethodPost},
		"/v1/memory-time-travel/rollback-plan":                       {http.MethodPost},
		"/v1/memory-time-travel/rollback/approved-plan":              {http.MethodPost},
		"/v1/memory-time-travel/retention/plan":                      {http.MethodGet},
		"/v1/memory-time-travel/retention/prune-plan":                {http.MethodPost},
		"/v1/memory-time-travel/kv-history/native-plan":              {http.MethodGet},
		"/v1/memory-time-travel/kv-history/migration-preview":        {http.MethodGet},
		"/v1/memory-time-travel/kv-history/dual-read/parity":         {http.MethodPost},
		"/v1/memory-time-travel/kv-history/cutover/plan":             {http.MethodPost},
		"/v1/memory-time-travel/kv-history/cutover/readiness":        {http.MethodPost},
		"/v1/memory-time-travel/audit/links":                         {http.MethodGet},
		"/v1/memory-time-travel/audit/links/preview":                 {http.MethodPost},
		"/v1/memory-time-travel/audit/links/writeback-plan":          {http.MethodPost},
		"/v1/memory-time-travel/audit/links/writeback/store":         {http.MethodPost},
		"/v1/memory-time-travel/audit/links/writeback/executor/plan": {http.MethodPost},
		"/v1/memory-time-travel/audit/verify":                        {http.MethodGet},
		"/v1/memory-time-travel/evidence/":                           {http.MethodGet},
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
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "json-memory-time-travel-evidence") || !strings.Contains(w.Body.String(), "snapshot.json") || !strings.Contains(w.Body.String(), "approved-rollback-plan.json") || !strings.Contains(w.Body.String(), "rollback-writeback-plan.json") || !strings.Contains(w.Body.String(), "approval-request-plan.json") || !strings.Contains(w.Body.String(), "retention-plan.json") || !strings.Contains(w.Body.String(), "retention-prune-plan.json") || !strings.Contains(w.Body.String(), "native-kv-history-plan.json") || !strings.Contains(w.Body.String(), "kv-history-migration-plan.json") || !strings.Contains(w.Body.String(), "kv-history-index-plan.json") || !strings.Contains(w.Body.String(), "kv-history-cutover-plan.json") || !strings.Contains(w.Body.String(), "kv-history-dual-read-plan.json") || !strings.Contains(w.Body.String(), "kv-history-dual-write-plan.json") || !strings.Contains(w.Body.String(), "audit-links.json") || !strings.Contains(w.Body.String(), "audit-link-preview.json") || !strings.Contains(w.Body.String(), "audit-link-writeback-plan.json") || !strings.Contains(w.Body.String(), "audit-verification.json") {
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
		TemporalKV: &fakeTemporalKV{snapshot: map[string][]byte{
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
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"approved_rollback_plan_ready":true`) || !strings.Contains(w.Body.String(), `"rollback_writeback_plan_ready":true`) || !strings.Contains(w.Body.String(), `"global_approval_enqueue_ready":false`) || !strings.Contains(w.Body.String(), `"retention_plan_ready":true`) || !strings.Contains(w.Body.String(), `"retention_prune_plan_ready":true`) || !strings.Contains(w.Body.String(), `"native_kv_history_plan_ready":true`) || !strings.Contains(w.Body.String(), `"kv_history_migration_plan_ready":true`) || !strings.Contains(w.Body.String(), `"kv_history_cutover_plan_ready":true`) || !strings.Contains(w.Body.String(), `"kv_history_cutover_readiness_ready":true`) || !strings.Contains(w.Body.String(), `"dual_read_plan_ready":true`) || !strings.Contains(w.Body.String(), `"dual_read_parity_check_ready":false`) || !strings.Contains(w.Body.String(), `"dual_write_plan_ready":true`) || !strings.Contains(w.Body.String(), `"dual_read_ready":false`) || !strings.Contains(w.Body.String(), `"dual_write_ready":false`) || !strings.Contains(w.Body.String(), `"cutover_ready":false`) || !strings.Contains(w.Body.String(), `"native_kv_history_preview_ready":false`) || !strings.Contains(w.Body.String(), `"native_kv_history_ready":false`) || !strings.Contains(w.Body.String(), `"writes_native_kv_history":false`) || !strings.Contains(w.Body.String(), `"migrates_kv_history":false`) || !strings.Contains(w.Body.String(), `"kv_audit_link_schema_ready":true`) || !strings.Contains(w.Body.String(), `"kv_audit_link_preview_ready":true`) || !strings.Contains(w.Body.String(), `"kv_audit_link_writeback_plan_ready":true`) || !strings.Contains(w.Body.String(), `"kv_audit_link_writeback_store_ready":true`) || !strings.Contains(w.Body.String(), `"kv_audit_link_writeback_executor_plan_ready":true`) || !strings.Contains(w.Body.String(), `"executor_input_contract_ready":true`) || !strings.Contains(w.Body.String(), `"audit_proof_link_executor_ready":false`) || !strings.Contains(w.Body.String(), `"kv_audit_link_writeback_ready":false`) || !strings.Contains(w.Body.String(), "memory.rollback.approved_plan") || !strings.Contains(w.Body.String(), "memory.rollback.writeback.plan") || !strings.Contains(w.Body.String(), "memory.retention.plan") || !strings.Contains(w.Body.String(), "memory.retention.prune_plan") || !strings.Contains(w.Body.String(), "memory.kv_history.native_plan") || !strings.Contains(w.Body.String(), "memory.kv_history.migration_preview") || !strings.Contains(w.Body.String(), "memory.kv_history.dual_read.parity") || !strings.Contains(w.Body.String(), "memory.kv_history.cutover.plan") || !strings.Contains(w.Body.String(), "memory.kv_history.cutover.readiness") || !strings.Contains(w.Body.String(), "memory.audit.links.schema") || !strings.Contains(w.Body.String(), "memory.audit.links.preview") || !strings.Contains(w.Body.String(), "memory.audit.links.writeback_plan") || !strings.Contains(w.Body.String(), "memory.audit.links.writeback_store") || !strings.Contains(w.Body.String(), "memory.audit.links.writeback_executor_plan") {
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

func TestMemoryTimeTravelAuditProofLinkPreviewJoinsNativeRowsAndMerkleRecordsWithoutWriting(t *testing.T) {
	now := time.Date(2026, 5, 15, 15, 30, 0, 0, time.UTC)
	previewer := &fakeNativeKVHistoryPreviewer{preview: NativeKVHistoryMigrationPreview{
		Namespace:            "memory_snapshot",
		GeneratedAt:          now,
		SourceNamespace:      "__kv_history__",
		NativeTable:          "kv_history",
		ScannedDocumentCount: 1,
		PreviewRowCount:      2,
		ReturnedRowCount:     2,
		Rows: []NativeKVHistoryRowPreview{
			{ID: "kvh-goal", Namespace: "memory_snapshot", Key: "goal", Version: 1, Value: []byte(`"ship"`), ValueSHA256: valueHash(`"ship"`), UpdatedAt: now.Add(-time.Hour), Current: true, AuditSeq: 7, AuditHash: "audit-hash-7", SourceAdapter: "reserved-ledger-kv-namespace"},
			{ID: "kvh-persona", Namespace: "memory_snapshot", Key: "persona", Version: 1, Value: []byte(`"careful"`), ValueSHA256: valueHash(`"careful"`), UpdatedAt: now.Add(-time.Hour), Current: true, AuditSeq: 8, AuditHash: "missing-hash-8", SourceAdapter: "reserved-ledger-kv-namespace"},
		},
	}}
	verifier := &fakeMerkleVerifier{result: MerkleVerification{
		Ready:        true,
		Valid:        true,
		InvalidIndex: -1,
		RecordCount:  2,
		LastSeq:      8,
		LastHash:     "audit-hash-8",
		RecentRecords: []MerkleAuditRecord{
			{Seq: 7, Timestamp: now.Add(-time.Hour), Type: "memory", Actor: "tenant", Action: "memory.flush", PrevHash: "audit-hash-6", Hash: "audit-hash-7"},
			{Seq: 8, Timestamp: now, Type: "memory", Actor: "tenant", Action: "memory.flush", PrevHash: "audit-hash-7", Hash: "audit-hash-8"},
		},
	}}
	h := New(Config{DataDir: t.TempDir(), Now: func() time.Time { return now }, NativeKVHistoryPreviewer: previewer, MerkleVerifier: verifier})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/audit/links/preview", strings.NewReader(`{"namespace":"memory_snapshot","at":"2026-05-15T15:30:00Z","limit":20,"dry_run":true}`))
	h.AuditLinksPreview(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("audit proof-link preview status=%d body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Preview KVAuditProofLinkPreviewReport `json:"preview"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode audit proof-link preview: %v", err)
	}
	if got.Preview.Stage != "kv-audit-proof-link-preview-before-merkle-writeback" || got.Preview.Status != "preview_only" || !got.Preview.PreviewReady || !got.Preview.KVAuditLinkPreviewReady {
		t.Fatalf("unexpected proof-link preview identity: %#v", got.Preview)
	}
	if got.Preview.LinkageReady || got.Preview.KVAuditLinkageReady || got.Preview.WritesLedgerKV || got.Preview.WritesNativeKVHistory || got.Preview.MerkleAppendReady || got.Preview.MergesAuditProofs {
		t.Fatalf("proof-link preview must stay non-destructive and linkage_ready=false: %#v", got.Preview)
	}
	if previewer.namespace != "memory_snapshot" || previewer.limit != 20 || verifier.limit != 20 {
		t.Fatalf("previewer/verifier should receive normalized namespace and limit, previewer=%q/%d verifier=%d", previewer.namespace, previewer.limit, verifier.limit)
	}
	if got.Preview.CandidateLinkCount != 2 || got.Preview.MatchedLinkCount != 1 || got.Preview.PendingLinkCount != 1 || got.Preview.UnmatchedRowCount != 1 || got.Preview.UnmatchedAuditRecordCount != 1 {
		t.Fatalf("unexpected proof-link counts: %#v", got.Preview)
	}
	if got.Preview.CandidateLinks[0].ProofStatus != "candidate_matched" || got.Preview.CandidateLinks[0].MatchedBy != "audit_seq+audit_hash" || got.Preview.CandidateLinks[0].AuditAction != "memory.flush" {
		t.Fatalf("expected first candidate to match by audit seq/hash: %#v", got.Preview.CandidateLinks)
	}
	if got.Preview.CandidateLinks[1].ProofStatus != "pending_audit_link_backfill" || got.Preview.CandidateLinks[1].NativeRowID != "kvh-persona" {
		t.Fatalf("expected second candidate to remain pending backfill: %#v", got.Preview.CandidateLinks)
	}
	for _, artifact := range []string{"audit-link-preview.json", "audit-links.json", "audit-verification.json", "kv-history-migration-preview.json"} {
		if !containsString(got.Preview.Artifacts, artifact) {
			t.Fatalf("proof-link preview missing artifact %s: %#v", artifact, got.Preview.Artifacts)
		}
	}
	for _, blocker := range []string{"per-kv-merkle-proof-link-not-wired", "merkle-append-not-wired", "native-kv-history-writeback-not-wired"} {
		if !containsString(got.Preview.BlockedBy, blocker) {
			t.Fatalf("proof-link preview missing blocker %s: %#v", blocker, got.Preview.BlockedBy)
		}
	}
}

func TestMemoryTimeTravelAuditProofLinkWritebackPlanConsumesPreviewWithoutWriting(t *testing.T) {
	now := time.Date(2026, 5, 15, 15, 45, 0, 0, time.UTC)
	previewer := &fakeNativeKVHistoryPreviewer{preview: NativeKVHistoryMigrationPreview{
		Namespace:            "memory_snapshot",
		GeneratedAt:          now,
		SourceNamespace:      "__kv_history__",
		NativeTable:          "kv_history",
		ScannedDocumentCount: 1,
		PreviewRowCount:      2,
		ReturnedRowCount:     2,
		Rows: []NativeKVHistoryRowPreview{
			{ID: "kvh-goal", Namespace: "memory_snapshot", Key: "goal", Version: 1, Value: []byte(`"ship"`), ValueSHA256: valueHash(`"ship"`), UpdatedAt: now.Add(-time.Hour), Current: true, AuditSeq: 7, AuditHash: "audit-hash-7", SourceAdapter: "reserved-ledger-kv-namespace"},
			{ID: "kvh-persona", Namespace: "memory_snapshot", Key: "persona", Version: 1, Value: []byte(`"careful"`), ValueSHA256: valueHash(`"careful"`), UpdatedAt: now.Add(-time.Hour), Current: true, AuditSeq: 8, AuditHash: "missing-hash-8", SourceAdapter: "reserved-ledger-kv-namespace"},
		},
	}}
	verifier := &fakeMerkleVerifier{result: MerkleVerification{
		Ready:        true,
		Valid:        true,
		InvalidIndex: -1,
		RecordCount:  1,
		LastSeq:      7,
		LastHash:     "audit-hash-7",
		RecentRecords: []MerkleAuditRecord{
			{Seq: 7, Timestamp: now.Add(-time.Hour), Type: "memory", Actor: "tenant", Action: "memory.flush", Hash: "audit-hash-7"},
		},
	}}
	h := New(Config{DataDir: t.TempDir(), Now: func() time.Time { return now }, NativeKVHistoryPreviewer: previewer, MerkleVerifier: verifier})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/audit/links/writeback-plan", strings.NewReader(`{"namespace":"memory_snapshot","at":"2026-05-15T15:45:00Z","limit":20,"requested_by":"operator","reason":"proof link review","approval_id":"approval-link-1","dry_run":true}`))
	h.AuditLinksWritebackPlan(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("audit proof-link writeback plan status=%d body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Plan KVAuditProofLinkWritebackPlanReport `json:"plan"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode audit proof-link writeback plan: %v", err)
	}
	if got.Plan.Stage != "kv-audit-proof-link-writeback-plan-before-ledger-mutation" || got.Plan.Status != "audit_proof_link_writeback_plan" || !got.Plan.KVAuditLinkWritebackPlanReady || !got.Plan.ConsumesAuditLinkPreview {
		t.Fatalf("unexpected writeback plan identity: %#v", got.Plan)
	}
	if got.Plan.KVAuditLinkWritebackReady || got.Plan.KVAuditLinkageReady || got.Plan.AuditProofLinkReady || got.Plan.WritesLedgerKV || got.Plan.WritesNativeKVHistory || got.Plan.BackfillsAuditSeq || got.Plan.BackfillsAuditHash || got.Plan.MerkleAppendReady || got.Plan.AppendsMerkle || got.Plan.GlobalApprovalEnqueueReady {
		t.Fatalf("writeback plan must remain non-destructive and plan-only: %#v", got.Plan)
	}
	if previewer.limit != 20 || verifier.limit != 20 {
		t.Fatalf("previewer/verifier should receive requested limit, previewer=%d verifier=%d", previewer.limit, verifier.limit)
	}
	if got.Plan.CandidateLinkCount != 2 || got.Plan.MatchedLinkCount != 1 || got.Plan.PendingLinkCount != 1 || got.Plan.ActionCount != 1 || len(got.Plan.WritebackActions) != 1 {
		t.Fatalf("unexpected writeback plan counts/actions: %#v", got.Plan)
	}
	action := got.Plan.WritebackActions[0]
	if action.Operation != "kv_history_audit_proof_link_backfill_preview" || action.NativeRowID != "kvh-goal" || action.AuditSeq != 7 || action.AuditHash != "audit-hash-7" || action.ProofStatus != "would_backfill_audit_seq_hash" || !action.RequiresApproval || action.ApprovalID != "approval-link-1" {
		t.Fatalf("unexpected writeback action: %#v", action)
	}
	if got.Plan.ProposedApprovalRequest.QueueName != "memory_time_travel_audit_proof_link" || got.Plan.ProposedApprovalRequest.SourceArtifact != "audit-link-writeback-plan.json" || got.Plan.ProposedApprovalRequest.GlobalApprovalEnqueueReady {
		t.Fatalf("approval request should be shaped but not enqueued: %#v", got.Plan.ProposedApprovalRequest)
	}
	for _, artifact := range []string{"audit-link-writeback-plan.json", "audit-link-preview.json", "audit-links.json", "audit-verification.json", "kv-history-migration-preview.json"} {
		if !containsString(got.Plan.Artifacts, artifact) {
			t.Fatalf("writeback plan missing artifact %s: %#v", artifact, got.Plan.Artifacts)
		}
	}
	for _, blocker := range []string{"native-kv-history-writeback-not-wired", "merkle-append-not-wired", "audit-proof-link-executor-not-wired", "global-approval-manager-not-consumed"} {
		if !containsString(got.Plan.BlockedBy, blocker) {
			t.Fatalf("writeback plan missing blocker %s: %#v", blocker, got.Plan.BlockedBy)
		}
	}
}

func TestMemoryTimeTravelAuditProofLinkWritebackStorePersistsPackLocalHandoffOnly(t *testing.T) {
	now := time.Date(2026, 5, 15, 15, 50, 0, 0, time.UTC)
	previewer := &fakeNativeKVHistoryPreviewer{preview: NativeKVHistoryMigrationPreview{
		Namespace:            "memory_snapshot",
		GeneratedAt:          now,
		SourceNamespace:      "__kv_history__",
		NativeTable:          "kv_history",
		ScannedDocumentCount: 1,
		PreviewRowCount:      1,
		ReturnedRowCount:     1,
		Rows: []NativeKVHistoryRowPreview{
			{ID: "kvh-goal", Namespace: "memory_snapshot", Key: "goal", Version: 1, Value: []byte(`"ship"`), ValueSHA256: valueHash(`"ship"`), UpdatedAt: now.Add(-time.Hour), Current: true, AuditSeq: 7, AuditHash: "audit-hash-7", SourceAdapter: "reserved-ledger-kv-namespace"},
		},
	}}
	verifier := &fakeMerkleVerifier{result: MerkleVerification{
		Ready:        true,
		Valid:        true,
		InvalidIndex: -1,
		RecordCount:  1,
		LastSeq:      7,
		LastHash:     "audit-hash-7",
		RecentRecords: []MerkleAuditRecord{
			{Seq: 7, Timestamp: now.Add(-time.Hour), Type: "memory", Actor: "tenant", Action: "memory.flush", Hash: "audit-hash-7"},
		},
	}}
	h := New(Config{DataDir: t.TempDir(), Now: func() time.Time { return now }, NativeKVHistoryPreviewer: previewer, MerkleVerifier: verifier})

	body := `{"namespace":"memory_snapshot","at":"2026-05-15T15:50:00Z","limit":20,"requested_by":"operator","reason":"proof link queue","approval_id":"approval-link-store","dry_run":true}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/audit/links/writeback/store", strings.NewReader(body))
	h.AuditLinksWritebackStore(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("audit proof-link writeback store status=%d body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Writeback KVAuditProofLinkWritebackStoreReport `json:"writeback"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode audit proof-link writeback store: %v", err)
	}
	if got.Writeback.Status != "audit_link_writeback_record_stored_pending_executor" || !got.Writeback.KVAuditLinkWritebackStoreReady || !got.Writeback.KVAuditLinkWritebackPlanReady || !got.Writeback.WritesAuditLinkWritebackStore {
		t.Fatalf("unexpected writeback store identity: %#v", got.Writeback)
	}
	if got.Writeback.KVAuditLinkWritebackReady || got.Writeback.KVAuditLinkageReady || got.Writeback.AuditProofLinkReady || got.Writeback.WritesLedgerKV || got.Writeback.WritesNativeKVHistory || got.Writeback.BackfillsAuditSeq || got.Writeback.BackfillsAuditHash || got.Writeback.MerkleAppendReady || got.Writeback.AppendsMerkle || got.Writeback.GlobalApprovalEnqueueReady {
		t.Fatalf("writeback store must persist only pack-local handoff and keep execution blocked: %#v", got.Writeback)
	}
	if got.Writeback.ActionCount != 1 || got.Writeback.AuditLinkWritebackStore.RecordCount != 1 || got.Writeback.ApprovalQueueRecord.Status != "stored_pending_audit_proof_link_executor" {
		t.Fatalf("unexpected writeback store counts or record: %#v", got.Writeback)
	}
	if got.Writeback.ApprovalQueueRecord.WritebackActions[0].ProofStatus != "would_backfill_audit_seq_hash" || got.Writeback.ApprovalQueueRecord.StoreArtifact != "audit-link-writeback-store.json" {
		t.Fatalf("unexpected persisted writeback record: %#v", got.Writeback.ApprovalQueueRecord)
	}
	for _, artifact := range []string{"audit-link-writeback-store.json", "audit-link-writeback-record.json", "audit-link-writeback-plan.json"} {
		if !containsString(got.Writeback.Artifacts, artifact) {
			t.Fatalf("writeback store missing artifact %s: %#v", artifact, got.Writeback.Artifacts)
		}
	}
	if _, err := os.Stat(filepath.Join(h.dataDir, "audit-link-writeback-store.json")); err != nil {
		t.Fatalf("expected pack-local audit-link-writeback-store.json to be written: %v", err)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/audit/links/writeback/store", strings.NewReader(body))
	h.AuditLinksWritebackStore(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("idempotent writeback store status=%d body=%s", w.Code, w.Body.String())
	}
	records, err := h.loadKVAuditProofLinkWritebackRecords()
	if err != nil {
		t.Fatalf("load writeback records: %v", err)
	}
	if len(records) != 1 || records[0].RequestKey != "approval-link-store" {
		t.Fatalf("writeback store should replace by request key, records=%#v", records)
	}
}

func TestMemoryTimeTravelAuditProofLinkExecutorPlanConsumesStoreWithoutWrites(t *testing.T) {
	now := time.Date(2026, 5, 15, 15, 55, 0, 0, time.UTC)
	previewer := &fakeNativeKVHistoryPreviewer{preview: NativeKVHistoryMigrationPreview{
		Namespace:                   "memory_snapshot",
		GeneratedAt:                 now,
		SourceNamespace:             "__kv_history__",
		NativeTable:                 "kv_history",
		ScannedDocumentCount:        1,
		PreviewRowCount:             1,
		ReturnedRowCount:            1,
		NativeKVHistoryPreviewReady: true,
		WritesNativeKVHistory:       false,
		MigratesKVHistory:           false,
		Rows: []NativeKVHistoryRowPreview{{
			ID:          "kvh-memory_snapshot-goal-v1",
			Namespace:   "memory_snapshot",
			Key:         "goal",
			Version:     1,
			ValueSHA256: "goal-sha",
			UpdatedAt:   now.Add(-time.Hour),
			Current:     true,
			AuditSeq:    7,
			AuditHash:   "audit-hash-7",
		}},
	}}
	verifier := &fakeMerkleVerifier{result: MerkleVerification{
		Ready:       true,
		Valid:       true,
		RecordCount: 1,
		LastSeq:     7,
		LastHash:    "audit-hash-7",
		RecentRecords: []MerkleAuditRecord{
			{Seq: 7, Timestamp: now.Add(-time.Hour), Type: "memory", Actor: "tenant", Action: "memory.flush", Hash: "audit-hash-7"},
		},
	}}
	h := New(Config{DataDir: t.TempDir(), Now: func() time.Time { return now }, NativeKVHistoryPreviewer: previewer, MerkleVerifier: verifier})

	body := `{"namespace":"memory_snapshot","at":"2026-05-15T15:55:00Z","requested_by":"operator","reason":"proof link queue","approval_id":"approval-link-executor","dry_run":true}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/audit/links/writeback/store", strings.NewReader(body))
	h.AuditLinksWritebackStore(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("seed writeback store status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/audit/links/writeback/executor/plan", strings.NewReader(`{"request_key":"approval-link-executor","requested_by":"operator","reason":"plan executor handoff","dry_run":true}`))
	h.AuditLinksWritebackExecutorPlan(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("audit proof-link executor plan status=%d body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Plan KVAuditProofLinkWritebackExecutorPlanReport `json:"plan"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode audit proof-link executor plan: %v", err)
	}
	if got.Plan.Status != "audit_proof_link_executor_handoff_plan" || !got.Plan.KVAuditLinkWritebackExecutorPlanReady || !got.Plan.ExecutorInputContractReady || !got.Plan.ConsumesAuditLinkWritebackStore {
		t.Fatalf("unexpected executor plan identity: %#v", got.Plan)
	}
	if got.Plan.AuditProofLinkExecutorReady || got.Plan.KVAuditLinkWritebackReady || got.Plan.KVAuditLinkageReady || got.Plan.AuditProofLinkReady || got.Plan.WritesLedgerKV || got.Plan.WritesNativeKVHistory || got.Plan.BackfillsAuditSeq || got.Plan.BackfillsAuditHash || got.Plan.MerkleAppendReady || got.Plan.AppendsMerkle || got.Plan.GlobalApprovalEnqueueReady || got.Plan.WritesAuditChain {
		t.Fatalf("executor plan must consume store without live writes: %#v", got.Plan)
	}
	if got.Plan.ActionCount != 1 || got.Plan.ExecutorHandoffPlan.Target != "ledger.kv_history.audit_proof_link_executor" || !got.Plan.ExecutorHandoffPlan.ConsumesAuditLinkWritebackStore || got.Plan.ExecutorHandoffPlan.AuditProofLinkExecutorReady {
		t.Fatalf("executor handoff plan should map one stored record to future executor input only: %#v", got.Plan.ExecutorHandoffPlan)
	}
	if len(got.Plan.ExecutorHandoffPlan.DedupKey) == 0 || len(got.Plan.AuditAppendPlan.PayloadDigest) != 64 || got.Plan.AuditAppendPlan.WritesAuditChain {
		t.Fatalf("executor audit plan should expose deterministic non-writing metadata: %#v", got.Plan.AuditAppendPlan)
	}
	for _, artifact := range []string{"audit-link-writeback-executor-plan.json", "audit-link-executor-handoff-plan.json", "audit-link-executor-audit-plan.json", "audit-link-writeback-store.json", "audit-link-writeback-record.json"} {
		if !containsString(got.Plan.Artifacts, artifact) {
			t.Fatalf("executor plan missing artifact %s: %#v", artifact, got.Plan.Artifacts)
		}
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/audit/links/writeback/executor/plan", strings.NewReader(`{"record_id":"missing","request_id":"missing","request_key":"missing","namespace":"missing"}`))
	h.AuditLinksWritebackExecutorPlan(w, req)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "writeback record not found") {
		t.Fatalf("missing executor record should be a clear 400, status=%d body=%s", w.Code, w.Body.String())
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
	if previewer.namespace != "memory_snapshot" || previewer.limit != 2 {
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

func TestMemoryTimeTravelKVHistoryCutoverPlanIsNonDestructive(t *testing.T) {
	now := time.Date(2026, 5, 15, 17, 0, 0, 0, time.UTC)
	previewer := &fakeNativeKVHistoryPreviewer{preview: NativeKVHistoryMigrationPreview{
		Namespace:            "memory_snapshot",
		GeneratedAt:          now,
		SourceNamespace:      "__kv_history__",
		NativeTable:          "kv_history",
		ScannedDocumentCount: 1,
		PreviewRowCount:      1,
		ReturnedRowCount:     1,
		Rows: []NativeKVHistoryRowPreview{
			{ID: "kvh-1", Namespace: "memory_snapshot", Key: "goal", Version: 1, Value: []byte(`"ship"`), ValueSHA256: valueHash(`"ship"`), UpdatedAt: now, Current: true, SourceAdapter: "reserved-ledger-kv-namespace"},
		},
	}}
	h := New(Config{DataDir: t.TempDir(), Now: func() time.Time { return now }, NativeKVHistoryPreviewer: previewer})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/kv-history/cutover/plan", strings.NewReader(`{"namespace":"memory_snapshot","requested_by":"operator","reason":"dual read cutover review","limit":1,"dry_run":true}`))
	h.KVHistoryCutoverPlan(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("kv_history cutover plan status=%d body=%s", w.Code, w.Body.String())
	}

	var got struct {
		Plan KVHistoryCutoverPlanReport `json:"plan"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode kv_history cutover plan: %v", err)
	}
	if got.Plan.Stage != "kv-history-cutover-plan-before-dual-read-write" || got.Plan.Status != "plan_only" || got.Plan.Namespace != "memory-snapshot" {
		t.Fatalf("unexpected cutover identity: %#v", got.Plan)
	}
	if !got.Plan.KVHistoryCutoverPlanReady || !got.Plan.DualReadPlanReady || !got.Plan.DualWritePlanReady || !got.Plan.ConsumesNativeKVHistoryPlan || !got.Plan.ConsumesMigrationPreview {
		t.Fatalf("cutover plan should consume native plan and preview: %#v", got.Plan)
	}
	if got.Plan.NativeKVHistoryReady || got.Plan.WritesNativeKVHistory || got.Plan.MigratesKVHistory || got.Plan.DualReadReady || got.Plan.DualWriteReady || got.Plan.CutoverReady || got.Plan.RollbackReady || got.Plan.CreatesNativeTable || got.Plan.SwitchesTemporalAdapter || got.Plan.DeletesReservedKVNamespace {
		t.Fatalf("cutover plan must stay non-destructive and blocked: %#v", got.Plan)
	}
	if previewer.namespace != "memory_snapshot" || previewer.limit != 1 {
		t.Fatalf("previewer received namespace=%q limit=%d", previewer.namespace, previewer.limit)
	}
	if got.Plan.PreviewRowCount != 1 || got.Plan.ReturnedPreviewRowCount != 1 || !got.Plan.KVHistoryMigrationPreview.NativeKVHistoryPreviewReady {
		t.Fatalf("cutover plan should include migration preview summary: %#v", got.Plan)
	}
	for _, artifact := range []string{"kv-history-cutover-plan.json", "kv-history-dual-read-plan.json", "kv-history-dual-write-plan.json", "kv-history-cutover-rollback-plan.json", "native-kv-history-plan.json", "kv-history-migration-preview.json"} {
		if !containsString(got.Plan.Artifacts, artifact) {
			t.Fatalf("cutover plan missing artifact %s: %#v", artifact, got.Plan.Artifacts)
		}
	}
	if !containsString(got.Plan.BlockedBy, "dual-read-adapter-not-wired") || !containsString(got.Plan.BlockedBy, "dual-write-cutover-not-enabled") || got.Plan.DualWritePlan.WritesLedgerKV || got.Plan.DualWritePlan.WritesNativeKVHistory || got.Plan.DualReadPlan.SwitchesAdapter {
		t.Fatalf("cutover blockers or dual-read/write boundaries drifted: %#v", got.Plan)
	}
}

func TestMemoryTimeTravelKVHistoryDualReadParityComparesReservedSnapshotAndNativePreview(t *testing.T) {
	now := time.Date(2026, 5, 15, 18, 0, 0, 0, time.UTC)
	temporal := &fakeTemporalKV{snapshot: map[string][]byte{
		"goal":    []byte(`"ship"`),
		"persona": []byte(`"careful"`),
	}}
	previewer := &fakeNativeKVHistoryPreviewer{preview: NativeKVHistoryMigrationPreview{
		Namespace:            "memory_snapshot",
		GeneratedAt:          now,
		SourceNamespace:      "__kv_history__",
		NativeTable:          "kv_history",
		ScannedDocumentCount: 1,
		PreviewRowCount:      2,
		ReturnedRowCount:     2,
		Rows: []NativeKVHistoryRowPreview{
			{ID: "kvh-goal", Namespace: "memory_snapshot", Key: "goal", Version: 1, Value: []byte(`"ship"`), ValueSHA256: valueHash(`"ship"`), UpdatedAt: now.Add(-time.Hour), Current: true, SourceAdapter: "reserved-ledger-kv-namespace"},
			{ID: "kvh-persona", Namespace: "memory_snapshot", Key: "persona", Version: 1, Value: []byte(`"careful"`), ValueSHA256: valueHash(`"careful"`), UpdatedAt: now.Add(-time.Hour), Current: true, SourceAdapter: "reserved-ledger-kv-namespace"},
		},
	}}
	h := New(Config{DataDir: t.TempDir(), Now: func() time.Time { return now }, TemporalKV: temporal, NativeKVHistoryPreviewer: previewer})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/memory-time-travel/status", nil)
	h.Status(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"dual_read_parity_check_ready":true`) {
		t.Fatalf("status should expose dual-read parity readiness, status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/kv-history/dual-read/parity", strings.NewReader(`{"namespace":"memory_snapshot","at":"2026-05-15T18:00:00Z","limit":2}`))
	h.KVHistoryDualReadParity(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("dual-read parity status=%d body=%s", w.Code, w.Body.String())
	}

	var got struct {
		Parity KVHistoryDualReadParityReport `json:"parity"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode dual-read parity: %v", err)
	}
	if got.Parity.Stage != "kv-history-dual-read-parity-before-adapter-switch" || got.Parity.Status != "passed" || !got.Parity.ParityPassed || !got.Parity.DualReadParityReady {
		t.Fatalf("unexpected dual-read parity identity: %#v", got.Parity)
	}
	if temporal.namespace != "memory_snapshot" || previewer.namespace != "memory_snapshot" || previewer.limit != 2 {
		t.Fatalf("adapters should receive the reserved temporal namespace, temporal=%q preview=%q limit=%d", temporal.namespace, previewer.namespace, previewer.limit)
	}
	if got.Parity.ReadsNativeKVHistory || got.Parity.SwitchesTemporalAdapter || got.Parity.WritesLedgerKV || got.Parity.WritesNativeKVHistory {
		t.Fatalf("dual-read parity must stay read-only and must not switch adapters: %#v", got.Parity)
	}
	if got.Parity.TemporalKeyCount != 2 || got.Parity.NativePreviewKeyCount != 2 || got.Parity.MatchedKeyCount != 2 || got.Parity.MismatchCount != 0 || !containsString(got.Parity.Artifacts, "kv-history-dual-read-parity.json") {
		t.Fatalf("unexpected dual-read parity counts/artifacts: %#v", got.Parity)
	}
}

func TestMemoryTimeTravelKVHistoryDualReadParityReportsMismatchAndStaysBlocked(t *testing.T) {
	now := time.Date(2026, 5, 15, 18, 30, 0, 0, time.UTC)
	temporal := &fakeTemporalKV{snapshot: map[string][]byte{
		"goal": []byte(`"ship"`),
	}}
	previewer := &fakeNativeKVHistoryPreviewer{preview: NativeKVHistoryMigrationPreview{
		Namespace:            "memory_snapshot",
		GeneratedAt:          now,
		SourceNamespace:      "__kv_history__",
		NativeTable:          "kv_history",
		ScannedDocumentCount: 1,
		PreviewRowCount:      1,
		ReturnedRowCount:     1,
		Rows: []NativeKVHistoryRowPreview{
			{ID: "kvh-goal", Namespace: "memory_snapshot", Key: "goal", Version: 1, Value: []byte(`"drift"`), ValueSHA256: valueHash(`"drift"`), UpdatedAt: now.Add(-time.Hour), Current: true, SourceAdapter: "reserved-ledger-kv-namespace"},
		},
	}}
	h := New(Config{DataDir: t.TempDir(), Now: func() time.Time { return now }, TemporalKV: temporal, NativeKVHistoryPreviewer: previewer})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/kv-history/dual-read/parity", strings.NewReader(`{"namespace":"memory_snapshot","at":"2026-05-15T18:30:00Z","limit":5}`))
	h.KVHistoryDualReadParity(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("dual-read parity mismatch status=%d body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Parity KVHistoryDualReadParityReport `json:"parity"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode mismatch parity: %v", err)
	}
	if got.Parity.Status != "mismatch" || got.Parity.ParityPassed || got.Parity.DualReadParityReady || got.Parity.MismatchCount != 1 || got.Parity.ValueMismatchCount != 1 {
		t.Fatalf("unexpected mismatch parity report: %#v", got.Parity)
	}
	if !containsString(got.Parity.BlockedBy, "dual-read-parity-mismatch") || got.Parity.SwitchesTemporalAdapter || got.Parity.ReadsNativeKVHistory || got.Parity.WritesLedgerKV {
		t.Fatalf("mismatch must block future adapter switch and writes: %#v", got.Parity)
	}
	if got.Parity.Mismatches[0].Key != "goal" || got.Parity.Mismatches[0].Kind != "value_mismatch" || got.Parity.Mismatches[0].ReservedValue != "ship" || got.Parity.Mismatches[0].NativePreviewValue != "drift" {
		t.Fatalf("unexpected mismatch detail: %#v", got.Parity.Mismatches)
	}
}

func TestMemoryTimeTravelKVHistoryCutoverReadinessAggregatesPlanAndParityWithoutSwitching(t *testing.T) {
	now := time.Date(2026, 5, 15, 19, 0, 0, 0, time.UTC)
	temporal := &fakeTemporalKV{snapshot: map[string][]byte{
		"goal": []byte(`"ship"`),
	}}
	previewer := &fakeNativeKVHistoryPreviewer{preview: NativeKVHistoryMigrationPreview{
		Namespace:            "memory_snapshot",
		GeneratedAt:          now,
		SourceNamespace:      "__kv_history__",
		NativeTable:          "kv_history",
		ScannedDocumentCount: 1,
		PreviewRowCount:      1,
		ReturnedRowCount:     1,
		Rows: []NativeKVHistoryRowPreview{
			{ID: "kvh-goal", Namespace: "memory_snapshot", Key: "goal", Version: 1, Value: []byte(`"ship"`), ValueSHA256: valueHash(`"ship"`), UpdatedAt: now.Add(-time.Hour), Current: true, SourceAdapter: "reserved-ledger-kv-namespace"},
		},
	}}
	h := New(Config{DataDir: t.TempDir(), Now: func() time.Time { return now }, TemporalKV: temporal, NativeKVHistoryPreviewer: previewer})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/memory-time-travel/kv-history/cutover/readiness", strings.NewReader(`{"namespace":"memory_snapshot","at":"2026-05-15T19:00:00Z","requested_by":"operator","reason":"cutover gate review","limit":5,"dry_run":true}`))
	h.KVHistoryCutoverReadiness(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("cutover readiness status=%d body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Readiness KVHistoryCutoverReadinessReport `json:"readiness"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode cutover readiness: %v", err)
	}
	if got.Readiness.Stage != "kv-history-cutover-readiness-before-adapter-switch" || got.Readiness.Status != "blocked" || !got.Readiness.CutoverReadinessCheckReady {
		t.Fatalf("unexpected readiness identity: %#v", got.Readiness)
	}
	if !got.Readiness.ConsumesCutoverPlan || !got.Readiness.ConsumesDualReadParity || !got.Readiness.ParityPassed || !got.Readiness.DualReadParityReady {
		t.Fatalf("readiness should consume plan and parity evidence: %#v", got.Readiness)
	}
	if got.Readiness.CutoverReady || got.Readiness.SwitchesTemporalAdapter || got.Readiness.WritesLedgerKV || got.Readiness.WritesNativeKVHistory || got.Readiness.NativeReadAdapterReady || got.Readiness.NativeWritePathReady {
		t.Fatalf("readiness gate must not enable cutover, adapter switch or writes: %#v", got.Readiness)
	}
	if got.Readiness.RequiredGateCount != 7 || got.Readiness.PassedGateCount != 3 || got.Readiness.BlockedGateCount != 4 {
		t.Fatalf("unexpected readiness gate counts: %#v", got.Readiness)
	}
	for _, artifact := range []string{"kv-history-cutover-readiness.json", "kv-history-cutover-plan.json", "kv-history-dual-read-parity.json"} {
		if !containsString(got.Readiness.Artifacts, artifact) {
			t.Fatalf("readiness missing artifact %s: %#v", artifact, got.Readiness.Artifacts)
		}
	}
	for _, blocker := range []string{"native-kv-history-read-adapter-not-wired", "dual-write-cutover-not-enabled", "cutover-rollback-executor-not-wired", "per-kv-merkle-proof-link-not-wired"} {
		if !containsString(got.Readiness.BlockedBy, blocker) {
			t.Fatalf("readiness missing blocker %s: %#v", blocker, got.Readiness.BlockedBy)
		}
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
		RecentRecords: []MerkleAuditRecord{
			{Seq: 1, Timestamp: now, Type: "memory", Actor: "tenant", Action: "memory.flush", Hash: "hash-1"},
		},
	}}
	previewer := &fakeNativeKVHistoryPreviewer{preview: NativeKVHistoryMigrationPreview{
		Namespace:            "memory_snapshot",
		GeneratedAt:          now,
		SourceNamespace:      "__kv_history__",
		NativeTable:          "kv_history",
		ScannedDocumentCount: 1,
		PreviewRowCount:      1,
		ReturnedRowCount:     1,
		Rows: []NativeKVHistoryRowPreview{
			{ID: "kvh-goal", Namespace: "memory_snapshot", Key: "goal", Version: 1, Value: []byte(`"ship"`), ValueSHA256: valueHash(`"ship"`), UpdatedAt: now, Current: true, AuditSeq: 1, AuditHash: "hash-1", SourceAdapter: "reserved-ledger-kv-namespace"},
		},
	}}
	h := New(Config{DataDir: t.TempDir(), Now: func() time.Time { return now }, NativeKVHistoryPreviewer: previewer, MerkleVerifier: verifier})

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
		Files                     []string                                    `json:"files"`
		ApprovedRollbackPlan      ApprovedRollbackExecutionPlanReport         `json:"approved_rollback_plan"`
		RollbackWritebackPlan     []RollbackWritebackActionPlan               `json:"rollback_writeback_plan"`
		ApprovalRequestPlan       GlobalApprovalRequestPlan                   `json:"approval_request_plan"`
		RetentionPlan             RetentionPlanReport                         `json:"retention_plan"`
		RetentionPrunePlan        RetentionPrunePlanReport                    `json:"retention_prune_plan"`
		NativeKVHistoryPlan       NativeKVHistoryPlanReport                   `json:"native_kv_history_plan"`
		KVHistoryMigrationPlan    []KVHistoryMigrationStepPlan                `json:"kv_history_migration_plan"`
		KVHistoryIndexPlan        []NativeKVHistoryIndexPlan                  `json:"kv_history_index_plan"`
		KVHistoryCutoverPlan      KVHistoryCutoverPlanReport                  `json:"kv_history_cutover_plan"`
		KVHistoryCutoverReadiness KVHistoryCutoverReadinessReport             `json:"kv_history_cutover_readiness"`
		KVHistoryDualReadParity   KVHistoryDualReadParityReport               `json:"kv_history_dual_read_parity"`
		KVHistoryDualReadPlan     KVHistoryDualReadPlan                       `json:"kv_history_dual_read_plan"`
		KVHistoryDualWritePlan    KVHistoryDualWritePlan                      `json:"kv_history_dual_write_plan"`
		KVHistoryMigrationPreview NativeKVHistoryMigrationPreview             `json:"kv_history_migration_preview"`
		KVAuditLinkSchema         KVAuditLinksReport                          `json:"kv_audit_link_schema"`
		KVAuditLinks              []KVAuditProofLink                          `json:"kv_audit_links"`
		KVAuditLinkPreview        KVAuditProofLinkPreviewReport               `json:"kv_audit_link_preview"`
		KVAuditLinkWritebackPlan  KVAuditProofLinkWritebackPlanReport         `json:"kv_audit_link_writeback_plan"`
		KVAuditLinkWritebacks     []KVAuditProofLinkWritebackActionPlan       `json:"kv_audit_link_writeback_actions"`
		KVAuditLinkWritebackStore KVAuditProofLinkWritebackStoreSummary       `json:"kv_audit_link_writeback_store"`
		KVAuditLinkWritebackRecs  []KVAuditProofLinkWritebackRecord           `json:"kv_audit_link_writeback_records"`
		KVAuditLinkExecutorPlan   KVAuditProofLinkWritebackExecutorPlanReport `json:"kv_audit_link_writeback_executor_plan"`
		AuditLinkExecutorHandoff  KVAuditProofLinkExecutorHandoffPlan         `json:"audit_link_executor_handoff_plan"`
		AuditLinkExecutorAudit    KVAuditProofLinkExecutorAuditAppendPlan     `json:"audit_link_executor_audit_plan"`
		KVAuditLinkExecutorErr    string                                      `json:"kv_audit_link_writeback_executor_plan_error"`
		AuditVerification         MerkleVerification                          `json:"audit_verification"`
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
	if !containsString(got.Files, "audit-link-preview.json") || !got.KVAuditLinkPreview.KVAuditLinkPreviewReady || got.KVAuditLinkPreview.KVAuditLinkageReady || got.KVAuditLinkPreview.WritesLedgerKV || got.KVAuditLinkPreview.WritesNativeKVHistory || got.KVAuditLinkPreview.MerkleAppendReady {
		t.Fatalf("evidence should include read-only KV audit proof-link preview: files=%#v preview=%#v", got.Files, got.KVAuditLinkPreview)
	}
	if got.KVAuditLinkPreview.MatchedLinkCount != 1 || got.KVAuditLinkPreview.CandidateLinks[0].ProofStatus != "candidate_matched" {
		t.Fatalf("proof-link preview should expose matched candidate without write-back: %#v", got.KVAuditLinkPreview)
	}
	if !containsString(got.Files, "audit-link-writeback-plan.json") || !got.KVAuditLinkWritebackPlan.KVAuditLinkWritebackPlanReady || got.KVAuditLinkWritebackPlan.KVAuditLinkWritebackReady || got.KVAuditLinkWritebackPlan.WritesLedgerKV || got.KVAuditLinkWritebackPlan.WritesNativeKVHistory || got.KVAuditLinkWritebackPlan.BackfillsAuditSeq || got.KVAuditLinkWritebackPlan.MerkleAppendReady {
		t.Fatalf("evidence should include plan-only KV audit proof-link writeback bridge: files=%#v plan=%#v", got.Files, got.KVAuditLinkWritebackPlan)
	}
	if got.KVAuditLinkWritebackPlan.ActionCount != 1 || len(got.KVAuditLinkWritebacks) != 1 || got.KVAuditLinkWritebacks[0].ProofStatus != "would_backfill_audit_seq_hash" {
		t.Fatalf("evidence should expose one proposed proof-link writeback action: plan=%#v actions=%#v", got.KVAuditLinkWritebackPlan, got.KVAuditLinkWritebacks)
	}
	if !containsString(got.Files, "audit-link-writeback-store.json") || !containsString(got.Files, "audit-link-writeback-record.json") || !got.KVAuditLinkWritebackStore.KVAuditLinkWritebackStoreReady || got.KVAuditLinkWritebackStore.KVAuditLinkWritebackReady || got.KVAuditLinkWritebackStore.WritesLedgerKV || got.KVAuditLinkWritebackStore.WritesNativeKVHistory || got.KVAuditLinkWritebackStore.BackfillsAuditSeq {
		t.Fatalf("evidence should include pack-local proof-link writeback store summary without execution: files=%#v store=%#v", got.Files, got.KVAuditLinkWritebackStore)
	}
	if len(got.KVAuditLinkWritebackRecs) != 0 {
		t.Fatalf("evidence should not synthesize writeback store records unless the store route has persisted them: %#v", got.KVAuditLinkWritebackRecs)
	}
	if !containsString(got.Files, "audit-link-writeback-executor-plan.json") || !containsString(got.Files, "audit-link-executor-handoff-plan.json") || !containsString(got.Files, "audit-link-executor-audit-plan.json") {
		t.Fatalf("evidence files should declare proof-link executor handoff artifacts: %#v", got.Files)
	}
	if got.KVAuditLinkExecutorErr == "" || got.KVAuditLinkExecutorPlan.KVAuditLinkWritebackExecutorPlanReady || got.AuditLinkExecutorHandoff.ExecutorInputContractReady || got.AuditLinkExecutorAudit.AuditAppendPlanReady {
		t.Fatalf("evidence should not synthesize executor handoff without a persisted writeback record: err=%q plan=%#v handoff=%#v audit=%#v", got.KVAuditLinkExecutorErr, got.KVAuditLinkExecutorPlan, got.AuditLinkExecutorHandoff, got.AuditLinkExecutorAudit)
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
	if !containsString(got.Files, "kv-history-dual-read-parity.json") || got.KVHistoryDualReadParity.SwitchesTemporalAdapter || got.KVHistoryDualReadParity.WritesLedgerKV || got.KVHistoryDualReadParity.WritesNativeKVHistory {
		t.Fatalf("evidence should include read-only dual-read parity gate: files=%#v parity=%#v", got.Files, got.KVHistoryDualReadParity)
	}
	if !containsString(got.Files, "kv-history-cutover-readiness.json") || !got.KVHistoryCutoverReadiness.CutoverReadinessCheckReady || got.KVHistoryCutoverReadiness.CutoverReady || got.KVHistoryCutoverReadiness.SwitchesTemporalAdapter || got.KVHistoryCutoverReadiness.WritesLedgerKV || got.KVHistoryCutoverReadiness.WritesNativeKVHistory {
		t.Fatalf("evidence should include read-only cutover readiness gate: files=%#v readiness=%#v", got.Files, got.KVHistoryCutoverReadiness)
	}
	if !containsString(got.Files, "kv-history-cutover-plan.json") || !containsString(got.Files, "kv-history-dual-read-plan.json") || !containsString(got.Files, "kv-history-dual-write-plan.json") || !got.KVHistoryCutoverPlan.KVHistoryCutoverPlanReady || got.KVHistoryCutoverPlan.CutoverReady || got.KVHistoryCutoverPlan.WritesNativeKVHistory || got.KVHistoryDualReadPlan.Ready || got.KVHistoryDualWritePlan.Ready || got.KVHistoryDualWritePlan.WritesLedgerKV {
		t.Fatalf("evidence should include plan-only kv_history cutover artifacts: files=%#v cutover=%#v read=%#v write=%#v", got.Files, got.KVHistoryCutoverPlan, got.KVHistoryDualReadPlan, got.KVHistoryDualWritePlan)
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
