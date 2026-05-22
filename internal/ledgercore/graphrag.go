package ledger

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
)

// GraphRAG extends ContextGraph with community detection and multi-hop retrieval.
// Implements a simplified Leiden-style algorithm for community detection,
// multi-hop BFS traversal, and community-level summarization for RAG.
//
// Reference: Microsoft GraphRAG (microsoft.github.io/graphrag)
type GraphRAG struct {
	mu          sync.RWMutex
	backend     Backend
	communities []Community
	nodeComm    map[string]int // nodeID ???community index
	built       bool
}

// Community represents a detected group of densely connected nodes.
type Community struct {
	ID       int         `json:"id"`
	Nodes    []GraphNode `json:"nodes"`
	Edges    []GraphEdge `json:"edges"`
	Summary  string      `json:"summary,omitempty"`
	Level    int         `json:"level"`    // hierarchy level (0 = leaf)
	Parent   int         `json:"parent"`   // parent community ID (-1 = root)
	Children []int       `json:"children"` // child community IDs
	Score    float64     `json:"score"`    // modularity contribution
}

// SubgraphResult holds the result of a multi-hop traversal.
type SubgraphResult struct {
	Nodes       []GraphNode `json:"nodes"`
	Edges       []GraphEdge `json:"edges"`
	Communities []int       `json:"communities"` // related community IDs
	Depth       int         `json:"depth"`
}

// SummarizeFunc generates a text summary for a community's content.
// Injected by the caller to decouple from LLM provider.
type SummarizeFunc func(ctx context.Context, nodeLabels []string, edgeDescriptions []string) (string, error)

// NewGraphRAG creates a GraphRAG engine backed by the given storage.
func NewGraphRAG(backend Backend) *GraphRAG {
	return &GraphRAG{
		backend:  backend,
		nodeComm: make(map[string]int),
	}
}

// BuildCommunities runs Leiden-style community detection on the graph.
// maxSize controls the maximum community size before splitting.
func (gr *GraphRAG) BuildCommunities(ctx context.Context, maxSize int) error {
	if maxSize <= 0 {
		maxSize = 10
	}

	nodes, err := gr.backend.ListNodes(ctx)
	if err != nil {
		return fmt.Errorf("graphrag: list nodes: %w", err)
	}
	edges, err := gr.backend.ListEdges(ctx)
	if err != nil {
		return fmt.Errorf("graphrag: list edges: %w", err)
	}

	if len(nodes) == 0 {
		gr.mu.Lock()
		gr.communities = nil
		gr.nodeComm = make(map[string]int)
		gr.built = true
		gr.mu.Unlock()
		return nil
	}

	adj := buildAdjacency(nodes, edges)
	communities := leidenDetect(nodes, edges, adj, maxSize)

	gr.mu.Lock()
	gr.communities = communities
	gr.nodeComm = make(map[string]int, len(nodes))
	for i, comm := range communities {
		for _, n := range comm.Nodes {
			gr.nodeComm[n.ID] = i
		}
	}
	gr.built = true
	gr.mu.Unlock()

	return nil
}

// Communities returns the detected communities.
func (gr *GraphRAG) Communities() []Community {
	gr.mu.RLock()
	defer gr.mu.RUnlock()
	return gr.communities
}

// NodeCommunity returns the community ID for a given node.
func (gr *GraphRAG) NodeCommunity(nodeID string) (int, bool) {
	gr.mu.RLock()
	defer gr.mu.RUnlock()
	id, ok := gr.nodeComm[nodeID]
	return id, ok
}

// MultiHopTraversal performs BFS from a seed node up to maxDepth hops.
// Returns all reachable nodes and edges within the traversal frontier.
func (gr *GraphRAG) MultiHopTraversal(ctx context.Context, seedNodeID string, maxDepth int) (*SubgraphResult, error) {
	if maxDepth <= 0 {
		maxDepth = 2
	}

	visited := make(map[string]bool)
	var resultNodes []GraphNode
	var resultEdges []GraphEdge
	commSet := make(map[int]bool)

	frontier := []string{seedNodeID}
	visited[seedNodeID] = true

	for depth := 0; depth < maxDepth && len(frontier) > 0; depth++ {
		var nextFrontier []string

		for _, nodeID := range frontier {
			node, err := gr.backend.GetNode(ctx, nodeID)
			if err != nil || node == nil {
				continue
			}
			resultNodes = append(resultNodes, *node)

			gr.mu.RLock()
			if commID, ok := gr.nodeComm[nodeID]; ok {
				commSet[commID] = true
			}
			gr.mu.RUnlock()

			neighbors, err := gr.backend.Neighbors(ctx, nodeID)
			if err != nil {
				continue
			}
			for _, edge := range neighbors {
				resultEdges = append(resultEdges, *edge)
				otherID := edge.ToID
				if otherID == nodeID {
					otherID = edge.FromID
				}
				if !visited[otherID] {
					visited[otherID] = true
					nextFrontier = append(nextFrontier, otherID)
				}
			}
		}
		frontier = nextFrontier
	}

	// Add final frontier nodes
	for _, nodeID := range frontier {
		node, err := gr.backend.GetNode(ctx, nodeID)
		if err != nil || node == nil {
			continue
		}
		resultNodes = append(resultNodes, *node)
	}

	commIDs := make([]int, 0, len(commSet))
	for id := range commSet {
		commIDs = append(commIDs, id)
	}
	sort.Ints(commIDs)

	return &SubgraphResult{
		Nodes:       resultNodes,
		Edges:       resultEdges,
		Communities: commIDs,
		Depth:       maxDepth,
	}, nil
}

// SummarizeCommunities generates text summaries for all communities using the provided function.
func (gr *GraphRAG) SummarizeCommunities(ctx context.Context, fn SummarizeFunc) error {
	if fn == nil {
		return fmt.Errorf("summarize function is nil")
	}

	// Snapshot community metadata under read lock (avoid holding lock during LLM calls).
	gr.mu.RLock()
	type commInput struct {
		idx       int
		labels    []string
		edgeDescs []string
	}
	inputs := make([]commInput, len(gr.communities))
	for i := range gr.communities {
		comm := &gr.communities[i]
		labels := make([]string, len(comm.Nodes))
		for j, n := range comm.Nodes {
			labels[j] = fmt.Sprintf("%s:%s", n.Kind, n.Label)
		}
		edgeDescs := make([]string, len(comm.Edges))
		for j, e := range comm.Edges {
			edgeDescs[j] = fmt.Sprintf("%s -[%s]-> %s", e.FromID, e.Kind, e.ToID)
		}
		inputs[i] = commInput{idx: i, labels: labels, edgeDescs: edgeDescs}
	}
	gr.mu.RUnlock()

	// Call LLM without holding the lock.
	summaries := make(map[int]string, len(inputs))
	for _, in := range inputs {
		summary, err := fn(ctx, in.labels, in.edgeDescs)
		if err != nil {
			continue // best effort
		}
		summaries[in.idx] = summary
	}

	// Write results back under write lock.
	gr.mu.Lock()
	for idx, summary := range summaries {
		if idx < len(gr.communities) {
			gr.communities[idx].Summary = summary
		}
	}
	gr.mu.Unlock()
	return nil
}

// SearchByCommunity retrieves community summaries relevant to a query.
// Uses keyword matching against community node labels and summaries.
func (gr *GraphRAG) SearchByCommunity(query string, limit int) []Community {
	gr.mu.RLock()
	defer gr.mu.RUnlock()

	if len(gr.communities) == 0 {
		return nil
	}

	queryTerms := strings.Fields(strings.ToLower(query))
	type scored struct {
		comm  Community
		score float64
	}
	var results []scored

	for _, comm := range gr.communities {
		score := 0.0
		text := strings.ToLower(comm.Summary)
		for _, n := range comm.Nodes {
			text += " " + strings.ToLower(n.Label)
		}
		for _, qt := range queryTerms {
			if strings.Contains(text, qt) {
				score += 1.0
			}
		}
		if score > 0 {
			results = append(results, scored{comm, score / float64(len(queryTerms))})
		}
	}

	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	out := make([]Community, len(results))
	for i, r := range results {
		out[i] = r.comm
	}
	return out
}

// --- Leiden-style community detection ---

type adjacency struct {
	neighbors map[string][]weightedEdge
	totalWeight float64
}

type weightedEdge struct {
	nodeID string
	weight float64
}

func buildAdjacency(nodes []GraphNode, edges []GraphEdge) adjacency {
	adj := adjacency{neighbors: make(map[string][]weightedEdge)}
	for _, n := range nodes {
		if _, ok := adj.neighbors[n.ID]; !ok {
			adj.neighbors[n.ID] = nil
		}
	}
	for _, e := range edges {
		w := e.Weight
		if w <= 0 {
			w = 1.0
		}
		adj.neighbors[e.FromID] = append(adj.neighbors[e.FromID], weightedEdge{e.ToID, w})
		adj.neighbors[e.ToID] = append(adj.neighbors[e.ToID], weightedEdge{e.FromID, w})
		adj.totalWeight += w
	}
	return adj
}

// leidenDetect performs greedy modularity optimization with recursive splitting.
func leidenDetect(nodes []GraphNode, edges []GraphEdge, adj adjacency, maxSize int) []Community {
	nodeMap := make(map[string]GraphNode, len(nodes))
	for _, n := range nodes {
		nodeMap[n.ID] = n
	}

	// Phase 1: Each node starts in its own community
	assignment := make(map[string]int, len(nodes))
	for i, n := range nodes {
		assignment[n.ID] = i
	}

	// Phase 2: Greedy modularity optimization (multiple passes)
	m2 := 2 * adj.totalWeight
	if m2 == 0 {
		m2 = 1
	}

	// Compute node degrees
	degree := make(map[string]float64, len(nodes))
	for nodeID, neighbors := range adj.neighbors {
		for _, ne := range neighbors {
			degree[nodeID] += ne.weight
		}
	}

	changed := true
	for pass := 0; pass < 10 && changed; pass++ {
		changed = false
		for _, n := range nodes {
			currentComm := assignment[n.ID]
			bestComm := currentComm
			bestDelta := 0.0

			// Calculate modularity gain for moving to each neighbor's community
			neighborComms := make(map[int]float64)
			for _, ne := range adj.neighbors[n.ID] {
				nComm := assignment[ne.nodeID]
				neighborComms[nComm] += ne.weight
			}

			ki := degree[n.ID]
			for targetComm, sumWin := range neighborComms {
				if targetComm == currentComm {
					continue
				}
				// Simplified modularity delta
				sigmaTot := communityDegree(assignment, degree, targetComm)
				delta := sumWin/m2 - ki*sigmaTot/(m2*m2)

				if delta > bestDelta {
					bestDelta = delta
					bestComm = targetComm
				}
			}

			if bestComm != currentComm {
				assignment[n.ID] = bestComm
				changed = true
			}
		}
	}

	// Phase 3: Collect communities
	commNodes := make(map[int][]GraphNode)
	for _, n := range nodes {
		comm := assignment[n.ID]
		commNodes[comm] = append(commNodes[comm], n)
	}

	// Build edge lists per community
	commEdges := make(map[int][]GraphEdge)
	for _, e := range edges {
		fromComm := assignment[e.FromID]
		toComm := assignment[e.ToID]
		if fromComm == toComm {
			commEdges[fromComm] = append(commEdges[fromComm], e)
		}
	}

	// Phase 4: Split oversized communities
	var communities []Community
	for commID, cNodes := range commNodes {
		if len(cNodes) <= maxSize {
			communities = append(communities, Community{
				ID:     commID,
				Nodes:  cNodes,
				Edges:  commEdges[commID],
				Level:  0,
				Parent: -1,
				Score:  modularityContribution(cNodes, commEdges[commID], adj),
			})
		} else {
			subs := splitCommunity(cNodes, commEdges[commID], adj, maxSize)
			parentID := commID
			childIDs := make([]int, len(subs))
			for i, sub := range subs {
				sub.ID = len(communities) + 1000 + i
				sub.Parent = parentID
				sub.Level = 1
				childIDs[i] = sub.ID
				communities = append(communities, sub)
			}
			communities = append(communities, Community{
				ID:       parentID,
				Nodes:    cNodes,
				Edges:    commEdges[commID],
				Level:    0,
				Parent:   -1,
				Children: childIDs,
				Score:    modularityContribution(cNodes, commEdges[commID], adj),
			})
		}
	}

	sort.Slice(communities, func(i, j int) bool { return len(communities[i].Nodes) > len(communities[j].Nodes) })

	// Build old→new ID mapping, then fix all Parent/Children references.
	oldToNew := make(map[int]int, len(communities))
	for i, c := range communities {
		oldToNew[c.ID] = i
	}
	for i := range communities {
		communities[i].ID = i
		if communities[i].Parent >= 0 {
			if newID, ok := oldToNew[communities[i].Parent]; ok {
				communities[i].Parent = newID
			}
		}
		for j, childID := range communities[i].Children {
			if newID, ok := oldToNew[childID]; ok {
				communities[i].Children[j] = newID
			}
		}
	}

	return communities
}

func communityDegree(assignment map[string]int, degree map[string]float64, commID int) float64 {
	total := 0.0
	for nodeID, comm := range assignment {
		if comm == commID {
			total += degree[nodeID]
		}
	}
	return total
}

func modularityContribution(nodes []GraphNode, edges []GraphEdge, adj adjacency) float64 {
	if adj.totalWeight == 0 {
		return 0
	}
	internalWeight := 0.0
	for _, e := range edges {
		internalWeight += e.Weight
	}
	totalDegree := 0.0
	for _, n := range nodes {
		for _, ne := range adj.neighbors[n.ID] {
			totalDegree += ne.weight
		}
	}
	m2 := 2 * adj.totalWeight
	return internalWeight/m2 - math.Pow(totalDegree/m2, 2)
}

// splitCommunity divides an oversized community using the same greedy approach.
func splitCommunity(nodes []GraphNode, edges []GraphEdge, adj adjacency, maxSize int) []Community {
	if len(nodes) <= maxSize {
		return []Community{{Nodes: nodes, Edges: edges}}
	}

	assignment := make(map[string]int, len(nodes))
	numSplits := (len(nodes) + maxSize - 1) / maxSize
	for i, n := range nodes {
		assignment[n.ID] = i % numSplits
	}

	subNodes := make(map[int][]GraphNode)
	subEdges := make(map[int][]GraphEdge)
	for _, n := range nodes {
		comm := assignment[n.ID]
		subNodes[comm] = append(subNodes[comm], n)
	}
	for _, e := range edges {
		fromComm := assignment[e.FromID]
		toComm := assignment[e.ToID]
		if fromComm == toComm {
			subEdges[fromComm] = append(subEdges[fromComm], e)
		}
	}

	var result []Community
	for commID, cn := range subNodes {
		result = append(result, Community{
			Nodes: cn,
			Edges: subEdges[commID],
		})
	}
	return result
}
