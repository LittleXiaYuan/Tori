package belief

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// BeliefGraph manages a directed graph of belief nodes.
//
// Thread-safe: all public methods are safe for concurrent use.
// The graph supports:
//   - Add/Get/Remove nodes
//   - Query by kind, source, tension
//   - Conflict detection between nodes
//   - Topological queries (find roots, find leaves)
//
// The graph deliberately does NOT own the update/evaluate logic —
// that belongs to the Engine. Graph is just structured storage.
type BeliefGraph struct {
	mu    sync.RWMutex
	nodes map[string]*BeliefNode

	// indexedByKind allows fast queries by BeliefKind.
	indexedByKind map[BeliefKind][]string

	// rootIDs cache the set of root beliefs for fast access.
	rootIDs []string
}

// NewBeliefGraph creates an empty belief graph.
func NewBeliefGraph() *BeliefGraph {
	return &BeliefGraph{
		nodes:         make(map[string]*BeliefNode),
		indexedByKind: make(map[BeliefKind][]string),
	}
}

// Add inserts a node into the graph. Returns error if validation fails
// or if a node with the same ID already exists.
func (g *BeliefGraph) Add(node *BeliefNode) error {
	if node == nil {
		return fmt.Errorf("belief: cannot add nil node")
	}
	if err := node.Validate(); err != nil {
		return err
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.nodes[node.ID]; exists {
		return fmt.Errorf("belief: node %q already exists", node.ID)
	}

	now := time.Now()
	if node.CreatedAt.IsZero() {
		node.CreatedAt = now
	}
	if node.LastUpdatedAt.IsZero() {
		node.LastUpdatedAt = now
	}

	clipStrength(&node.Strength, &node.Confidence, &node.Valence, &node.Stability, &node.Plasticity)

	g.nodes[node.ID] = node
	g.indexedByKind[node.Kind] = append(g.indexedByKind[node.Kind], node.ID)

	if node.IsRoot() {
		g.rootIDs = append(g.rootIDs, node.ID)
	}

	// Rebuild edge indexes for conflict detection.
	// Edges are stored on the source node; we validate targets exist.
	for _, edge := range node.Related {
		if _, ok := g.nodes[edge.TargetID]; !ok {
			// Edge points to non-existent node — that's okay, the edge
			// was stored with the node. We don't reject the add.
			continue
		}
	}

	return nil
}

// Get retrieves a node by ID. Returns nil if not found.
func (g *BeliefGraph) Get(id string) *BeliefNode {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.nodes[id]
}

// Remove deletes a node and its references from the graph.
func (g *BeliefGraph) Remove(id string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	node, exists := g.nodes[id]
	if !exists {
		return false
	}

	// Remove from kind index.
	kind := node.Kind
	idx := g.indexedByKind[kind]
	for i, nid := range idx {
		if nid == id {
			g.indexedByKind[kind] = append(idx[:i], idx[i+1:]...)
			break
		}
	}

	// Remove from root cache.
	if node.IsRoot() {
		for i, rid := range g.rootIDs {
			if rid == id {
				g.rootIDs = append(g.rootIDs[:i], g.rootIDs[i+1:]...)
				break
			}
		}
	}

	// Remove edges from other nodes that point to this one.
	for _, other := range g.nodes {
		for i, edge := range other.Related {
			if edge.TargetID == id {
				other.Related = append(other.Related[:i], other.Related[i+1:]...)
				break
			}
		}
	}

	delete(g.nodes, id)
	return true
}

// List returns all nodes.
func (g *BeliefGraph) List() []*BeliefNode {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]*BeliefNode, 0, len(g.nodes))
	for _, n := range g.nodes {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ListByKind returns all nodes of a given kind.
func (g *BeliefGraph) ListByKind(kind BeliefKind) []*BeliefNode {
	g.mu.RLock()
	defer g.mu.RUnlock()
	ids := g.indexedByKind[kind]
	out := make([]*BeliefNode, 0, len(ids))
	for _, id := range ids {
		if n := g.nodes[id]; n != nil {
			out = append(out, n)
		}
	}
	return out
}

// Roots returns all root beliefs.
func (g *BeliefGraph) Roots() []*BeliefNode {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]*BeliefNode, 0, len(g.rootIDs))
	for _, id := range g.rootIDs {
		if n := g.nodes[id]; n != nil {
			out = append(out, n)
		}
	}
	return out
}

// Tensions returns all tension nodes.
func (g *BeliefGraph) Tensions() []*BeliefNode {
	return g.ListByKind(KindTension)
}

// ActiveTensions returns tension nodes with both sides still in conflict
// (i.e., both end-point beliefs still exist in the graph).
func (g *BeliefGraph) ActiveTensions() []*BeliefNode {
	tensions := g.ListByKind(KindTension)
	out := make([]*BeliefNode, 0, len(tensions))
	for _, t := range tensions {
		if g.tensionSidesExist(t) {
			out = append(out, t)
		}
	}
	return out
}

func (g *BeliefGraph) tensionSidesExist(t *BeliefNode) bool {
	if len(t.Related) < 2 {
		return false
	}
	alive := 0
	for _, e := range t.Related {
		if g.nodes[e.TargetID] != nil {
			alive++
		}
	}
	return alive >= 2
}

// FindConflicts returns pairs of beliefs that have a "conflicts" edge between them.
func (g *BeliefGraph) FindConflicts() [][2]*BeliefNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var out [][2]*BeliefNode
	for _, a := range g.nodes {
		for _, edge := range a.Related {
			if edge.Relation == RelationConflicts {
				if b := g.nodes[edge.TargetID]; b != nil {
					out = append(out, [2]*BeliefNode{a, b})
				}
			}
		}
	}
	return out
}

// AddEdge adds a directed relationship between two beliefs.
func (g *BeliefGraph) AddEdge(fromID, toID string, rel RelationType) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	from := g.nodes[fromID]
	if from == nil {
		return fmt.Errorf("belief: source node %q not found", fromID)
	}
	if _, ok := g.nodes[toID]; !ok {
		return fmt.Errorf("belief: target node %q not found", toID)
	}

	// Check for duplicate edges.
	for _, e := range from.Related {
		if e.TargetID == toID && e.Relation == rel {
			return nil // already exists, idempotent
		}
	}

	from.Related = append(from.Related, BeliefEdge{
		TargetID: toID,
		Relation: rel,
	})
	from.LastUpdatedAt = time.Now()
	return nil
}

// RemoveEdge removes a directed relationship.
func (g *BeliefGraph) RemoveEdge(fromID, toID string, rel RelationType) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	from := g.nodes[fromID]
	if from == nil {
		return false
	}

	for i, e := range from.Related {
		if e.TargetID == toID && e.Relation == rel {
			from.Related = append(from.Related[:i], from.Related[i+1:]...)
			from.LastUpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// ConnectedBy returns all nodes connected to the given node by a specific relation.
func (g *BeliefGraph) ConnectedBy(id string, rel RelationType) []*BeliefNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	src := g.nodes[id]
	if src == nil {
		return nil
	}

	var out []*BeliefNode
	for _, e := range src.Related {
		if e.Relation == rel {
			if target := g.nodes[e.TargetID]; target != nil {
				out = append(out, target)
			}
		}
	}
	return out
}

// Size returns the number of nodes in the graph.
func (g *BeliefGraph) Size() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodes)
}

// Snapshot returns a deep-ish copy of the current graph state.
// The nodes themselves are copied (not cloned deeply), but the slice
// is a fresh allocation so callers can safely iterate.
func (g *BeliefGraph) Snapshot() []*BeliefNode {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]*BeliefNode, 0, len(g.nodes))
	for _, n := range g.nodes {
		cp := *n
		if n.Related != nil {
			cp.Related = make([]BeliefEdge, len(n.Related))
			copy(cp.Related, n.Related)
		}
		if n.Evidence != nil {
			cp.Evidence = make([]string, len(n.Evidence))
			copy(cp.Evidence, n.Evidence)
		}
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Dot renders the belief graph in Graphviz DOT format for visualization.
func (g *BeliefGraph) Dot() string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var b strings.Builder
	b.WriteString("digraph BeliefGraph {\n")
	b.WriteString("  rankdir=LR;\n")
	b.WriteString("  node [shape=box, style=filled, fontname=\"sans-serif\"];\n\n")

	kindColors := map[BeliefKind]string{
		KindRoot:       "#FFD700",
		KindValue:      "#87CEEB",
		KindRelational: "#98FB98",
		KindPreference: "#DDA0DD",
		KindBoundary:   "#FF6347",
		KindTension:    "#FFA500",
	}

	for _, n := range g.nodes {
		color := kindColors[n.Kind]
		if color == "" {
			color = "#F5F5F5"
		}
		label := strings.ReplaceAll(n.Statement, "\"", "\\\"")
		fmt.Fprintf(&b, "  %q [label=%q, fillcolor=%q];\n", n.ID, label, color)
	}

	b.WriteString("\n")

	for _, n := range g.nodes {
		for _, e := range n.Related {
			style := "solid"
			if e.Relation == RelationConflicts {
				style = "dashed"
				color := "red"
				fmt.Fprintf(&b, "  %q -> %q [style=%s, color=%s, label=%q];\n",
					n.ID, e.TargetID, style, color, string(e.Relation))
			} else if e.Relation == RelationProtects {
				color := "green"
				fmt.Fprintf(&b, "  %q -> %q [style=%s, color=%s, label=%q];\n",
					n.ID, e.TargetID, style, color, string(e.Relation))
			} else {
				fmt.Fprintf(&b, "  %q -> %q [style=%s, label=%q];\n",
					n.ID, e.TargetID, style, string(e.Relation))
			}
		}
	}

	b.WriteString("}\n")
	return b.String()
}

func clipStrength(ptr ...*float64) {
	for _, p := range ptr {
		if *p < 0 {
			*p = 0
		}
		if *p > 1 {
			*p = 1
		}
	}
}

// reset resets the graph to empty (for testing).
func (g *BeliefGraph) reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nodes = make(map[string]*BeliefNode)
	g.indexedByKind = make(map[BeliefKind][]string)
	g.rootIDs = nil
}
