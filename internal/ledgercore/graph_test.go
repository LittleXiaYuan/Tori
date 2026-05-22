package ledger_test

import (
	"context"
	"testing"

	"yunque-agent/internal/ledgercore"
)

func TestGraphLinkAndNeighbors(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	user := &ledger.GraphNode{Kind: ledger.NodeUser, Label: "alice", RefID: "user:alice", TenantID: "t1", Metadata: ledger.JSON("{}")}
	task := &ledger.GraphNode{Kind: ledger.NodeTask, Label: "report", RefID: "task:1", TenantID: "t1", Metadata: ledger.JSON("{}")}

	if err := ldg.Graph.Link(ctx, user, task, ledger.EdgeCreatedBy, 1.0); err != nil {
		t.Fatalf("Link: %v", err)
	}

	if user.ID == "" || task.ID == "" {
		t.Fatal("expected IDs to be assigned")
	}

	nodes, edges, err := ldg.Graph.Neighbors(ctx, user.ID, 1, 10)
	if err != nil {
		t.Fatalf("Neighbors: %v", err)
	}
	if len(nodes) == 0 {
		t.Fatal("expected at least one neighbor node")
	}
	if len(edges) == 0 {
		t.Fatal("expected at least one edge")
	}

	found := false
	for _, n := range nodes {
		if n.ID == task.ID {
			found = true
		}
	}
	if !found {
		t.Error("expected task node in neighbors")
	}
}

func TestGraphLinkMemoryToTask(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	if err := ldg.Graph.LinkMemoryToTask(ctx, "t1", "mem:100", "task:200"); err != nil {
		t.Fatalf("LinkMemoryToTask: %v", err)
	}

	memIDs, err := ldg.Graph.FindRelatedMemories(ctx, ledger.NodeTask, "task:200", 10)
	if err != nil {
		t.Fatalf("FindRelatedMemories: %v", err)
	}
	if len(memIDs) == 0 {
		t.Fatal("expected related memory IDs")
	}
	if memIDs[0] != "mem:100" {
		t.Errorf("expected mem:100, got %s", memIDs[0])
	}
}

func TestGraphNodeRefIsTenantScoped(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	if err := ldg.Graph.LinkMemoryToTask(ctx, "tenant-a", "mem:shared", "task:1"); err != nil {
		t.Fatalf("LinkMemoryToTask tenant-a: %v", err)
	}
	if err := ldg.Graph.LinkMemoryToTask(ctx, "tenant-b", "mem:shared", "task:1"); err != nil {
		t.Fatalf("LinkMemoryToTask tenant-b: %v", err)
	}

	nodes, err := ldg.Backend().ListNodes(ctx)
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}

	seen := map[string]bool{}
	for _, n := range nodes {
		if n.Kind == ledger.NodeMemory && n.RefID == "mem:shared" {
			seen[n.TenantID] = true
		}
	}
	if !seen["tenant-a"] || !seen["tenant-b"] {
		t.Fatalf("expected shared ref to have separate tenant nodes, got %v", seen)
	}
}

func TestGraphLinkMemoryToTopic(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	if err := ldg.Graph.LinkMemoryToTopic(ctx, "t1", "mem:300", "AI"); err != nil {
		t.Fatalf("LinkMemoryToTopic: %v", err)
	}

	// Link another memory to the same topic
	if err := ldg.Graph.LinkMemoryToTopic(ctx, "t1", "mem:301", "AI"); err != nil {
		t.Fatalf("LinkMemoryToTopic second: %v", err)
	}

	memIDs, err := ldg.Graph.FindRelatedMemories(ctx, ledger.NodeTopic, "topic:AI", 10)
	if err != nil {
		t.Fatalf("FindRelatedMemories via topic: %v", err)
	}
	if len(memIDs) < 2 {
		t.Errorf("expected at least 2 related memories, got %d", len(memIDs))
	}
}

func TestGraphFindRelatedMemoriesEmpty(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	memIDs, err := ldg.Graph.FindRelatedMemories(ctx, ledger.NodeTask, "nonexistent", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(memIDs) != 0 {
		t.Errorf("expected empty result, got %d", len(memIDs))
	}
}
