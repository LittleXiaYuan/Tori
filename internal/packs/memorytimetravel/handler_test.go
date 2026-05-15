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
	if len(routes) != 8 {
		t.Fatalf("expected 8 Memory Time Travel routes, got %d", len(routes))
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
		"/v1/memory-time-travel/status":        {http.MethodGet},
		"/v1/memory-time-travel/snapshots":     {http.MethodGet, http.MethodPost},
		"/v1/memory-time-travel/snapshots/":    {http.MethodGet},
		"/v1/memory-time-travel/snapshot-at":   {http.MethodPost},
		"/v1/memory-time-travel/diff":          {http.MethodPost},
		"/v1/memory-time-travel/rollback-plan": {http.MethodPost},
		"/v1/memory-time-travel/audit/verify":  {http.MethodGet},
		"/v1/memory-time-travel/evidence/":     {http.MethodGet},
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
	req = httptest.NewRequest(http.MethodGet, "/v1/memory-time-travel/evidence/baseline", nil)
	h.Evidence(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "json-memory-time-travel-evidence") || !strings.Contains(w.Body.String(), "snapshot.json") || !strings.Contains(w.Body.String(), "audit-verification.json") {
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
		Files             []string           `json:"files"`
		AuditVerification MerkleVerification `json:"audit_verification"`
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
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
