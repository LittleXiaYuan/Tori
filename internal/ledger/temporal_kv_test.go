package ledger

import (
	"context"
	"encoding/json"
	"testing"
	"time"
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
