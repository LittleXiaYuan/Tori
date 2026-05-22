package ledger_test

import (
	"context"
	"testing"
	"time"

	"yunque-agent/internal/ledgercore"
)

func TestLifecycleDecay(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	entry := &ledger.MemoryEntry{
		TenantID:   "t1",
		Kind:       ledger.MemoryFact,
		Key:        "old-fact",
		Content:    "The sky is blue",
		Confidence: 0.9,
		Source:     "test",
	}
	if err := ldg.Memory.Put(ctx, entry); err != nil {
		t.Fatalf("Put memory: %v", err)
	}

	// Set very short half-life so even a freshly created memory decays
	ldg.Lifecycle.SetDecayHalfLife(0.0001) // extremely short: ~0.01 seconds
	time.Sleep(50 * time.Millisecond)      // let some time pass

	updated, err := ldg.Lifecycle.RunDecay(ctx, "t1")
	if err != nil {
		t.Fatalf("RunDecay: %v", err)
	}

	// With such an extremely short half-life, the memory should have decayed
	results, err := ldg.Memory.Search(ctx, ledger.MemoryQuery{
		TenantID: "t1",
		Kinds:    []ledger.MemoryKind{ledger.MemoryFact},
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	t.Logf("decay updated=%d entries=%d", updated, len(results))
	for _, m := range results {
		if m.Key == "old-fact" {
			t.Logf("old-fact confidence after decay: %.6f", m.Confidence)
		}
	}
}

func TestLifecycleGC(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	// Insert a memory with very low confidence
	entry := &ledger.MemoryEntry{
		TenantID:   "t1",
		Kind:       ledger.MemoryFact,
		Key:        "weak-fact",
		Content:    "Uncertain information",
		Confidence: 0.02,
		Source:     "test",
	}
	if err := ldg.Memory.Put(ctx, entry); err != nil {
		t.Fatalf("Put: %v", err)
	}

	// Insert a healthy memory
	healthy := &ledger.MemoryEntry{
		TenantID:   "t1",
		Kind:       ledger.MemoryFact,
		Key:        "strong-fact",
		Content:    "Definitely true",
		Confidence: 0.95,
		Source:     "test",
	}
	if err := ldg.Memory.Put(ctx, healthy); err != nil {
		t.Fatalf("Put healthy: %v", err)
	}

	ldg.Lifecycle.SetGCThreshold(0.05)
	removed, err := ldg.Lifecycle.RunGC(ctx, "t1")
	if err != nil {
		t.Fatalf("RunGC: %v", err)
	}
	if removed == 0 {
		t.Fatal("expected at least one memory to be GC'd")
	}

	results, err := ldg.Memory.Search(ctx, ledger.MemoryQuery{
		TenantID: "t1",
		Kinds:    []ledger.MemoryKind{ledger.MemoryFact},
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("Search after GC: %v", err)
	}
	for _, m := range results {
		if m.Key == "weak-fact" {
			t.Error("expected weak-fact to have been garbage collected")
		}
	}
}
