package ledger

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	baseledger "github.com/LittleXiaYuan/ledger"
)

func TestTemporalKVStorePutVersionedAndGetRawAt(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 15, 9, 0, 0, 0, time.UTC)
	store := NewTemporalKVStore(ldg, WithTemporalKVNow(func() time.Time { return now }))

	if err := store.PutRawVersionedAt(ctx, "memory_snapshot", "persona", []byte(`"careful"`), now); err != nil {
		t.Fatalf("put v1: %v", err)
	}
	later := now.Add(time.Hour)
	if err := store.PutRawVersionedAt(ctx, "memory_snapshot", "persona", []byte(`"bold"`), later); err != nil {
		t.Fatalf("put v2: %v", err)
	}

	entry, found, err := store.GetRawAt(ctx, "memory_snapshot", "persona", now.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("get at v1: %v", err)
	}
	if !found || string(entry.Value) != `"careful"` {
		t.Fatalf("at v1 got found=%v value=%s", found, entry.Value)
	}

	entry, found, err = store.GetRawAt(ctx, "memory_snapshot", "persona", later.Add(time.Second))
	if err != nil {
		t.Fatalf("get at v2: %v", err)
	}
	if !found || string(entry.Value) != `"bold"` {
		t.Fatalf("at v2 got found=%v value=%s", found, entry.Value)
	}
}

func TestTemporalKVStoreListVersionsAndSnapshotRawAt(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()
	store := NewTemporalKVStore(ldg)
	t0 := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)

	if err := store.PutRawVersionedAt(ctx, "memory_snapshot", "goal", []byte(`"ship"`), t0); err != nil {
		t.Fatalf("put goal v1: %v", err)
	}
	if err := store.PutRawVersionedAt(ctx, "memory_snapshot", "persona", []byte(`"careful"`), t0.Add(10*time.Minute)); err != nil {
		t.Fatalf("put persona: %v", err)
	}
	if err := store.PutRawVersionedAt(ctx, "memory_snapshot", "goal", []byte(`"ship runtime"`), t0.Add(time.Hour)); err != nil {
		t.Fatalf("put goal v2: %v", err)
	}

	versions, err := store.ListVersions(ctx, "memory_snapshot", "goal", 0)
	if err != nil {
		t.Fatalf("versions: %v", err)
	}
	if len(versions) != 2 || !versions[0].Current || string(versions[1].Value) != `"ship"` {
		t.Fatalf("unexpected versions: %#v", versions)
	}

	snapshot, err := store.SnapshotRawAt(ctx, "memory_snapshot", t0.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("snapshot at: %v", err)
	}
	if string(snapshot["goal"]) != `"ship"` || string(snapshot["persona"]) != `"careful"` {
		t.Fatalf("unexpected snapshot at t0+30m: %#v", snapshot)
	}
}

func TestTemporalKVStorePreviewNativeKVHistoryRowsIsReadOnly(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()
	t0 := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	store := NewTemporalKVStore(ldg, WithTemporalKVNow(func() time.Time { return t0 }))

	if err := store.PutRawVersionedAt(ctx, "memory_snapshot", "goal", []byte(`"ship"`), t0); err != nil {
		t.Fatalf("put goal v1: %v", err)
	}
	if err := store.PutRawVersionedAt(ctx, "memory_snapshot", "goal", []byte(`"ship runtime"`), t0.Add(time.Hour)); err != nil {
		t.Fatalf("put goal v2: %v", err)
	}
	if err := store.PutRawVersionedAt(ctx, "memory_snapshot", "persona", []byte(`"careful"`), t0.Add(10*time.Minute)); err != nil {
		t.Fatalf("put persona: %v", err)
	}
	if err := store.PutRawVersionedAt(ctx, "other_namespace", "goal", []byte(`"ignore-v1"`), t0); err != nil {
		t.Fatalf("put other v1: %v", err)
	}
	if err := store.PutRawVersionedAt(ctx, "other_namespace", "goal", []byte(`"ignore-v2"`), t0.Add(time.Hour)); err != nil {
		t.Fatalf("put other v2: %v", err)
	}

	beforeHistory := rawKVByKey(t, ctx, ldg, temporalKVHistoryNamespace)
	preview, err := store.PreviewNativeKVHistoryRows(ctx, "memory_snapshot", 0)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	afterHistory := rawKVByKey(t, ctx, ldg, temporalKVHistoryNamespace)

	if preview.Namespace != "memory_snapshot" || preview.SourceNamespace != temporalKVHistoryNamespace || preview.NativeTable != "kv_history" {
		t.Fatalf("unexpected preview identity: %#v", preview)
	}
	if preview.WritesNativeKVHistory || preview.MigratesKVHistory || !preview.UsesReservedKVNamespace {
		t.Fatalf("preview must stay non-destructive: %#v", preview)
	}
	if preview.ScannedDocumentCount != 1 || preview.PreviewRowCount != 3 || preview.ReturnedRowCount != 3 {
		t.Fatalf("unexpected preview counts: %#v", preview)
	}
	if len(preview.Rows) != 3 {
		t.Fatalf("expected 3 preview rows, got %#v", preview.Rows)
	}
	if preview.Rows[0].Key != "goal" || preview.Rows[0].Version != 1 || preview.Rows[0].Current || string(preview.Rows[0].Value) != `"ship"` {
		t.Fatalf("unexpected historical row: %#v", preview.Rows[0])
	}
	if preview.Rows[0].ValueSHA256 != sha256Hex([]byte(`"ship"`)) || preview.Rows[0].SourceAdapter != "reserved-ledger-kv-namespace" {
		t.Fatalf("unexpected historical row metadata: %#v", preview.Rows[0])
	}
	if preview.Rows[1].Key != "goal" || preview.Rows[1].Version != 2 || !preview.Rows[1].Current || string(preview.Rows[1].Value) != `"ship runtime"` {
		t.Fatalf("unexpected current goal row: %#v", preview.Rows[1])
	}
	if preview.Rows[2].Key != "persona" || preview.Rows[2].Version != 1 || !preview.Rows[2].Current || string(preview.Rows[2].Value) != `"careful"` {
		t.Fatalf("unexpected current persona row: %#v", preview.Rows[2])
	}
	if preview.Rows[0].AuditSeq != 0 || preview.Rows[0].AuditHash != "" {
		t.Fatalf("audit linkage should remain placeholder-only: %#v", preview.Rows[0])
	}
	if !sameRawKV(beforeHistory, afterHistory) {
		t.Fatalf("preview changed reserved history namespace: before=%#v after=%#v", beforeHistory, afterHistory)
	}

	again, err := store.PreviewNativeKVHistoryRows(ctx, "memory_snapshot", 2)
	if err != nil {
		t.Fatalf("preview with limit: %v", err)
	}
	if again.PreviewRowCount != 3 || again.ReturnedRowCount != 2 || len(again.Rows) != 2 || again.Limit != 2 {
		t.Fatalf("limit should cap returned rows only: %#v", again)
	}
	if again.Rows[0].ID != preview.Rows[0].ID || again.Rows[1].ID != preview.Rows[1].ID {
		t.Fatalf("preview row IDs should be deterministic: first=%#v again=%#v", preview.Rows[:2], again.Rows)
	}
}

func TestTemporalKVStorePutVersionedJSON(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()
	store := NewTemporalKVStore(ldg)

	if err := store.PutVersioned(ctx, "config", "providers", map[string]string{"default": "local"}); err != nil {
		t.Fatalf("put versioned json: %v", err)
	}

	entry, found, err := store.GetRawAt(ctx, "config", "providers", time.Now().Add(time.Second))
	if err != nil {
		t.Fatalf("get raw at: %v", err)
	}
	if !found {
		t.Fatal("expected config/providers")
	}
	var got map[string]string
	if err := json.Unmarshal(entry.Value, &got); err != nil {
		t.Fatalf("unmarshal value: %v", err)
	}
	if got["default"] != "local" {
		t.Fatalf("unexpected json value: %#v", got)
	}
}

func rawKVByKey(t *testing.T, ctx context.Context, ldg *baseledger.Ledger, namespace string) map[string]string {
	t.Helper()
	entries, err := ldg.KV.List(ctx, namespace)
	if err != nil {
		t.Fatalf("list raw kv %s: %v", namespace, err)
	}
	out := make(map[string]string, len(entries))
	for _, entry := range entries {
		out[entry.Key] = string(entry.Value)
	}
	return out
}

func sameRawKV(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if b[key] != value {
			return false
		}
	}
	return true
}
