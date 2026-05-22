package ledger

import (
	"context"

	"yunque-agent/internal/ledgercore/internal/ulid"
)

// ContextGraph manages the entity relationship network.
// Nodes represent users, agents, tasks, memories, artifacts, and topics.
// Edges represent relationships like created_by, used_in, related_to, etc.
type ContextGraph struct {
	backend Backend
}

// Link creates a directed edge between two nodes, creating the nodes if they don't exist.
func (cg *ContextGraph) Link(ctx context.Context, from, to *GraphNode, edgeKind GraphEdgeKind, weight float64) error {
	if from.ID == "" {
		from.ID = ulid.New()
	}
	if to.ID == "" {
		to.ID = ulid.New()
	}
	if from.Metadata == nil {
		from.Metadata = JSON("{}")
	}
	if to.Metadata == nil {
		to.Metadata = JSON("{}")
	}

	if err := cg.backend.PutNode(ctx, from); err != nil {
		return err
	}
	if err := cg.backend.PutNode(ctx, to); err != nil {
		return err
	}

	edge := &GraphEdge{
		ID:       ulid.New(),
		FromID:   from.ID,
		ToID:     to.ID,
		Kind:     edgeKind,
		Weight:   weight,
		Metadata: JSON("{}"),
	}
	return cg.backend.PutEdge(ctx, edge)
}

// LinkMemoryToTask creates a "used_in" edge from a memory node to a task node.
func (cg *ContextGraph) LinkMemoryToTask(ctx context.Context, tenantID, memoryID, taskID string) error {
	memNode := cg.ensureNode(ctx, NodeMemory, memoryID, tenantID)
	taskNode := cg.ensureNode(ctx, NodeTask, taskID, tenantID)
	if memNode == nil || taskNode == nil {
		return nil
	}
	return cg.backend.PutEdge(ctx, &GraphEdge{
		ID: ulid.New(), FromID: memNode.ID, ToID: taskNode.ID,
		Kind: EdgeUsedIn, Weight: 1.0, Metadata: JSON("{}"),
	})
}

// LinkMemoryToTopic creates a "related_to" edge from a memory to a topic.
func (cg *ContextGraph) LinkMemoryToTopic(ctx context.Context, tenantID, memoryID, topic string) error {
	memNode := cg.ensureNode(ctx, NodeMemory, memoryID, tenantID)
	topicNode := &GraphNode{
		ID: ulid.New(), Kind: NodeTopic, Label: topic,
		RefID: "topic:" + topic, TenantID: tenantID, Metadata: JSON("{}"),
	}
	if existing, _ := cg.backend.FindNodeByRef(ctx, tenantID, NodeTopic, "topic:"+topic); existing != nil {
		topicNode = existing
	} else {
		cg.backend.PutNode(ctx, topicNode)
	}
	if memNode == nil {
		return nil
	}
	return cg.backend.PutEdge(ctx, &GraphEdge{
		ID: ulid.New(), FromID: memNode.ID, ToID: topicNode.ID,
		Kind: EdgeRelatedTo, Weight: 0.8, Metadata: JSON("{}"),
	})
}

// Neighbors returns nodes connected within maxDepth hops.
func (cg *ContextGraph) Neighbors(ctx context.Context, nodeID string, maxDepth, limit int) ([]*GraphNode, []*GraphEdge, error) {
	return cg.backend.GetNeighbors(ctx, nodeID, maxDepth, limit)
}

// FindRelatedMemories finds memory nodes connected to a given node (up to 2 hops).
func (cg *ContextGraph) FindRelatedMemories(ctx context.Context, refKind GraphNodeKind, refID string, limit int) ([]string, error) {
	return cg.FindRelatedMemoriesForTenant(ctx, "", refKind, refID, limit)
}

// FindRelatedMemoriesForTenant finds memory nodes connected to a given node,
// scoped to one tenant. Prefer this over FindRelatedMemories in multi-tenant
// callers; the legacy method is retained for compatibility.
func (cg *ContextGraph) FindRelatedMemoriesForTenant(ctx context.Context, tenantID string, refKind GraphNodeKind, refID string, limit int) ([]string, error) {
	node, err := cg.backend.FindNodeByRef(ctx, tenantID, refKind, refID)
	if err != nil || node == nil {
		return nil, nil
	}
	nodes, _, err := cg.backend.GetNeighbors(ctx, node.ID, 2, limit*3)
	if err != nil {
		return nil, err
	}
	var memoryIDs []string
	for _, n := range nodes {
		if n.Kind == NodeMemory && len(memoryIDs) < limit {
			memoryIDs = append(memoryIDs, n.RefID)
		}
	}
	return memoryIDs, nil
}

// Unlink removes a node and all its edges from the graph.
func (cg *ContextGraph) Unlink(ctx context.Context, nodeID string) error {
	return cg.backend.DeleteNode(ctx, nodeID)
}

func (cg *ContextGraph) ensureNode(ctx context.Context, kind GraphNodeKind, refID, tenantID string) *GraphNode {
	existing, err := cg.backend.FindNodeByRef(ctx, tenantID, kind, refID)
	if err == nil && existing != nil {
		return existing
	}
	node := &GraphNode{
		ID: ulid.New(), Kind: kind, Label: string(kind) + ":" + refID,
		RefID: refID, TenantID: tenantID, Metadata: JSON("{}"),
	}
	if err := cg.backend.PutNode(ctx, node); err != nil {
		return nil
	}
	return node
}
