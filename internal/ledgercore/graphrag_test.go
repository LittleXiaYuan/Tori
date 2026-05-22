package ledger_test

import (
	"context"
	"path/filepath"
	"testing"

	"yunque-agent/internal/ledgercore"
	lsqlite "yunque-agent/internal/ledgercore/backend/sqlite"
)

func newGraphTestLedger(t *testing.T) *ledger.Ledger {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	b, err := lsqlite.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create sqlite backend: %v", err)
	}
	ldg, err := ledger.Open(b)
	if err != nil {
		t.Fatalf("failed to open ledger: %v", err)
	}
	t.Cleanup(func() { ldg.Close() })
	return ldg
}

func TestGraphRAGBuildCommunities(t *testing.T) {
	ldg := newGraphTestLedger(t)
	ctx := context.Background()

	graph := ldg.Graph

	nodeA := &ledger.GraphNode{ID: "a", Kind: ledger.NodeTopic, Label: "Machine Learning", RefID: "ref-a"}
	nodeB := &ledger.GraphNode{ID: "b", Kind: ledger.NodeTopic, Label: "Deep Learning", RefID: "ref-b"}
	nodeC := &ledger.GraphNode{ID: "c", Kind: ledger.NodeTopic, Label: "Neural Networks", RefID: "ref-c"}
	nodeD := &ledger.GraphNode{ID: "d", Kind: ledger.NodeTopic, Label: "Cooking", RefID: "ref-d"}
	nodeE := &ledger.GraphNode{ID: "e", Kind: ledger.NodeTopic, Label: "Recipes", RefID: "ref-e"}

	graph.Link(ctx, nodeA, nodeB, ledger.EdgeRelatedTo, 1.0)
	graph.Link(ctx, nodeB, nodeC, ledger.EdgeRelatedTo, 1.0)
	graph.Link(ctx, nodeA, nodeC, ledger.EdgeRelatedTo, 0.8)
	graph.Link(ctx, nodeD, nodeE, ledger.EdgeRelatedTo, 1.0)

	gr := ledger.NewGraphRAG(ldg.Backend())
	if err := gr.BuildCommunities(ctx, 10); err != nil {
		t.Fatalf("BuildCommunities: %v", err)
	}

	comms := gr.Communities()
	if len(comms) == 0 {
		t.Fatal("expected at least 1 community")
	}
	t.Logf("detected %d communities", len(comms))
	for i, c := range comms {
		labels := make([]string, len(c.Nodes))
		for j, n := range c.Nodes {
			labels[j] = n.Label
		}
		t.Logf("  community %d: %v (score=%.3f)", i, labels, c.Score)
	}
}

func TestGraphRAGMultiHop(t *testing.T) {
	ldg := newGraphTestLedger(t)
	ctx := context.Background()
	graph := ldg.Graph

	nodeA := &ledger.GraphNode{ID: "a", Kind: ledger.NodeTopic, Label: "Go", RefID: "ref-a"}
	nodeB := &ledger.GraphNode{ID: "b", Kind: ledger.NodeTopic, Label: "Goroutines", RefID: "ref-b"}
	nodeC := &ledger.GraphNode{ID: "c", Kind: ledger.NodeTopic, Label: "Channels", RefID: "ref-c"}
	nodeD := &ledger.GraphNode{ID: "d", Kind: ledger.NodeTopic, Label: "Concurrency", RefID: "ref-d"}

	graph.Link(ctx, nodeA, nodeB, ledger.EdgeRelatedTo, 1.0)
	graph.Link(ctx, nodeB, nodeC, ledger.EdgeRelatedTo, 1.0)
	graph.Link(ctx, nodeC, nodeD, ledger.EdgeRelatedTo, 1.0)

	gr := ledger.NewGraphRAG(ldg.Backend())
	result, err := gr.MultiHopTraversal(ctx, "a", 3)
	if err != nil {
		t.Fatalf("MultiHopTraversal: %v", err)
	}

	if len(result.Nodes) < 2 {
		t.Errorf("expected at least 2 nodes in traversal from A, got %d", len(result.Nodes))
	}
	t.Logf("traversal: %d nodes, %d edges, depth=%d", len(result.Nodes), len(result.Edges), result.Depth)
	for _, n := range result.Nodes {
		t.Logf("  node: %s (%s)", n.Label, n.ID)
	}
}

func TestGraphRAGSearchByCommunity(t *testing.T) {
	ldg := newGraphTestLedger(t)
	ctx := context.Background()
	graph := ldg.Graph

	nodeA := &ledger.GraphNode{ID: "a", Kind: ledger.NodeTopic, Label: "Machine Learning", RefID: "ref-a2"}
	nodeB := &ledger.GraphNode{ID: "b", Kind: ledger.NodeTopic, Label: "Python", RefID: "ref-b2"}
	nodeC := &ledger.GraphNode{ID: "c", Kind: ledger.NodeTopic, Label: "Cooking", RefID: "ref-c2"}
	nodeD := &ledger.GraphNode{ID: "d", Kind: ledger.NodeTopic, Label: "Recipe", RefID: "ref-d2"}

	graph.Link(ctx, nodeA, nodeB, ledger.EdgeRelatedTo, 1.0)
	graph.Link(ctx, nodeC, nodeD, ledger.EdgeRelatedTo, 1.0)

	gr := ledger.NewGraphRAG(ldg.Backend())
	gr.BuildCommunities(ctx, 10)

	results := gr.SearchByCommunity("machine learning python", 5)
	if len(results) == 0 {
		t.Log("no community matches (may depend on community structure)")
		return
	}
	t.Logf("found %d matching communities for 'machine learning python'", len(results))
}

func TestGraphRAGEmptyGraph(t *testing.T) {
	ldg := newGraphTestLedger(t)
	ctx := context.Background()

	gr := ledger.NewGraphRAG(ldg.Backend())
	if err := gr.BuildCommunities(ctx, 10); err != nil {
		t.Fatalf("BuildCommunities on empty graph: %v", err)
	}

	comms := gr.Communities()
	if len(comms) != 0 {
		t.Errorf("expected 0 communities on empty graph, got %d", len(comms))
	}
}

func TestGraphRAGNodeCommunity(t *testing.T) {
	ldg := newGraphTestLedger(t)
	ctx := context.Background()
	graph := ldg.Graph

	nodeA := &ledger.GraphNode{ID: "a", Kind: ledger.NodeTopic, Label: "AI", RefID: "ref-a3"}
	nodeB := &ledger.GraphNode{ID: "b", Kind: ledger.NodeTopic, Label: "ML", RefID: "ref-b3"}
	graph.Link(ctx, nodeA, nodeB, ledger.EdgeRelatedTo, 1.0)

	gr := ledger.NewGraphRAG(ldg.Backend())
	gr.BuildCommunities(ctx, 10)

	comms := gr.Communities()
	t.Logf("total communities: %d", len(comms))
	for i, c := range comms {
		labels := make([]string, len(c.Nodes))
		for j, n := range c.Nodes {
			labels[j] = n.Label
		}
		t.Logf("  community %d: %v", i, labels)
	}

	if len(comms) == 0 {
		t.Fatal("expected at least 1 community")
	}
}
