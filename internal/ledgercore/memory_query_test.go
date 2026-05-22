package ledger_test

import (
	"context"
	"testing"

	"yunque-agent/internal/ledgercore"
)

func TestMemorySearchByKey(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	ldg.Memory.PutFact(ctx, "t1", "user.name", "Alice", "user")
	ldg.Memory.PutFact(ctx, "t1", "user.email", "alice@example.com", "user")

	results, err := ldg.Memory.SearchByKey(ctx, "t1", "user.name")
	if err != nil {
		t.Fatalf("SearchByKey: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Content != "Alice" {
		t.Errorf("expected 'Alice', got %q", results[0].Content)
	}
}

func TestMemorySearchMinConfidence(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	ldg.Memory.Put(ctx, &ledger.MemoryEntry{
		TenantID: "t1", Kind: ledger.MemoryFact, Key: "strong",
		Content: "Certain", Confidence: 0.95, Source: "test",
	})
	ldg.Memory.Put(ctx, &ledger.MemoryEntry{
		TenantID: "t1", Kind: ledger.MemoryFact, Key: "weak",
		Content: "Uncertain", Confidence: 0.2, Source: "test",
	})

	results, err := ldg.Memory.Search(ctx, ledger.MemoryQuery{
		TenantID:      "t1",
		MinConfidence: 0.5,
		Limit:         10,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	for _, r := range results {
		if r.Confidence < 0.5 {
			t.Errorf("got memory below threshold: key=%s confidence=%.2f", r.Key, r.Confidence)
		}
	}
}

func TestMemorySearchBySource(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	ldg.Memory.Put(ctx, &ledger.MemoryEntry{
		TenantID: "t1", Kind: ledger.MemoryFact, Key: "from-user",
		Content: "User said something", Confidence: 0.9, Source: "user",
	})
	ldg.Memory.Put(ctx, &ledger.MemoryEntry{
		TenantID: "t1", Kind: ledger.MemoryFact, Key: "from-extraction",
		Content: "Extracted from conversation", Confidence: 0.7, Source: "extraction",
	})

	results, err := ldg.Memory.Search(ctx, ledger.MemoryQuery{
		TenantID: "t1",
		Source:   "user",
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Key != "from-user" {
		t.Errorf("expected from-user, got %s", results[0].Key)
	}
}

func TestMemoryCount(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		ldg.Memory.PutFact(ctx, "t1", "fact-"+string(rune('A'+i)), "content", "test")
	}

	count, err := ldg.Memory.Count(ctx, ledger.MemoryQuery{TenantID: "t1"})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 5 {
		t.Errorf("expected 5, got %d", count)
	}
}

func TestGraphUnlink(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	user := &ledger.GraphNode{Kind: ledger.NodeUser, Label: "bob", RefID: "user:bob", TenantID: "t1", Metadata: ledger.JSON("{}")}
	task := &ledger.GraphNode{Kind: ledger.NodeTask, Label: "task1", RefID: "task:1", TenantID: "t1", Metadata: ledger.JSON("{}")}

	ldg.Graph.Link(ctx, user, task, ledger.EdgeCreatedBy, 1.0)

	if err := ldg.Graph.Unlink(ctx, user.ID); err != nil {
		t.Fatalf("Unlink: %v", err)
	}

	nodes, _, err := ldg.Graph.Neighbors(ctx, task.ID, 1, 10)
	if err != nil {
		t.Fatalf("Neighbors after unlink: %v", err)
	}
	for _, n := range nodes {
		if n.ID == user.ID {
			t.Error("expected user node to be gone after unlink")
		}
	}
}
