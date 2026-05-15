package ledger

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"yunque-agent/internal/agentcore/memory"
)

func TestLedgerPersisterFlushMirrorsMemoryIntoTemporalKV(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()
	mid := memory.NewMidTerm()
	long := memory.NewLongTerm()
	now := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	temporal := NewTemporalKVStore(ldg, WithTemporalKVNow(func() time.Time { return now }))
	p := NewLedgerPersister(
		ldg,
		mid,
		long,
		"",
		WithLedgerPersisterTemporalKV(temporal),
		WithLedgerPersisterNow(func() time.Time { return now }),
	)
	defer p.Stop()

	if !p.TemporalWritebackReady() {
		t.Fatal("expected temporal write-back to be ready")
	}
	if err := mid.Put(ctx, "tenant-a", memory.Item{Key: "goal", Value: "ship runtime", Source: "test", Category: "fact"}); err != nil {
		t.Fatalf("put mid: %v", err)
	}
	if err := long.Put(ctx, "tenant-a", memory.Item{ID: "exp-1", Key: "lesson", Value: "prefer reversible slices", Source: "test", Category: "experience"}); err != nil {
		t.Fatalf("put long: %v", err)
	}

	p.MarkDirty()
	p.flush()

	snapshot, err := temporal.SnapshotRawAt(ctx, "memory_snapshot", now.Add(time.Second))
	if err != nil {
		t.Fatalf("snapshot temporal memory: %v", err)
	}
	for key, want := range map[string]string{
		"tenant-a/mid/goal":   "ship runtime",
		"tenant-a/long/exp-1": "prefer reversible slices",
	} {
		raw, ok := snapshot[key]
		if !ok {
			t.Fatalf("missing temporal memory key %s in %#v", key, snapshot)
		}
		var got temporalMemoryRecord
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("decode %s: %v", key, err)
		}
		if got.Value != want || got.FlushedAt != now {
			t.Fatalf("unexpected temporal memory record for %s: %#v", key, got)
		}
	}
}
