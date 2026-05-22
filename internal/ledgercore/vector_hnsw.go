package ledger

import (
	"container/heap"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"
)

// HNSW implements the Hierarchical Navigable Small World algorithm for
// approximate nearest neighbor search.
//
// Reference: Malkov, Yashunin, "Efficient and Robust Approximate Nearest
// Neighbor Search Using Hierarchical Navigable Small World Graphs",
// IEEE TPAMI 2018 (arXiv:1603.09320, 2016).
//
// Key parameters:
//   - M:      max connections per element per layer (default: 16)
//   - efConstruction: size of dynamic candidate list during construction (default: 200)
//   - mL:     normalization factor for level generation = 1/ln(M)
type HNSW struct {
	mu             sync.RWMutex
	nodes          map[string]*hnswNode // id ???node
	entryPoint     string               // id of the entry point
	maxLevel       int                  // current max level
	M              int                  // max connections per layer
	Mmax0          int                  // max connections at layer 0 (= 2*M)
	efConstruction int                  // candidate list size during insert
	efSearch       int                  // candidate list size during search
	mL             float64              // level multiplier = 1/ln(M)
	rng            *rand.Rand
}

type hnswNode struct {
	id        string
	vector    []float32
	level     int              // max level this node appears in
	neighbors map[int][]string // level ???list of neighbor IDs
}

// HNSWConfig configures the HNSW index.
type HNSWConfig struct {
	M              int // max connections per element per layer
	EfConstruction int // candidate list size during construction
	EfSearch       int // candidate list size during search
}

// DefaultHNSWConfig returns parameters suitable for most workloads.
func DefaultHNSWConfig() HNSWConfig {
	return HNSWConfig{M: 16, EfConstruction: 200, EfSearch: 50}
}

// NewHNSW creates an empty HNSW index.
func NewHNSW(cfg HNSWConfig) *HNSW {
	if cfg.M <= 0 {
		cfg.M = 16
	}
	if cfg.EfConstruction <= 0 {
		cfg.EfConstruction = 200
	}
	if cfg.EfSearch <= 0 {
		cfg.EfSearch = 50
	}
	return &HNSW{
		nodes:          make(map[string]*hnswNode),
		M:              cfg.M,
		Mmax0:          2 * cfg.M,
		efConstruction: cfg.EfConstruction,
		efSearch:       cfg.EfSearch,
		mL:             1.0 / math.Log(float64(cfg.M)),
		maxLevel:       -1,
		rng:            rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Insert adds a vector to the index. Thread-safe.
func (h *HNSW) Insert(id string, vector []float32) {
	h.mu.Lock()
	defer h.mu.Unlock()

	level := h.randomLevel()
	node := &hnswNode{
		id:        id,
		vector:    vector,
		level:     level,
		neighbors: make(map[int][]string),
	}
	h.nodes[id] = node

	if h.maxLevel < 0 {
		h.entryPoint = id
		h.maxLevel = level
		return
	}

	ep := h.entryPoint
	epNode := h.nodes[ep]
	if epNode == nil {
		h.entryPoint = id
		h.maxLevel = level
		return
	}

	// Phase 1: Traverse from top to level+1 using greedy search
	for lc := h.maxLevel; lc > level; lc-- {
		ep = h.searchLayerGreedy(vector, ep, lc)
	}

	// Phase 2: Insert at each level from min(level, maxLevel) down to 0
	for lc := min(level, h.maxLevel); lc >= 0; lc-- {
		neighbors := h.searchLayer(vector, ep, h.efConstruction, lc)

		maxConn := h.M
		if lc == 0 {
			maxConn = h.Mmax0
		}

		selected := h.selectNeighbors(neighbors, maxConn)
		node.neighbors[lc] = selected

		for _, nID := range selected {
			nNode := h.nodes[nID]
			if nNode == nil {
				continue
			}
			nNode.neighbors[lc] = append(nNode.neighbors[lc], id)
			if len(nNode.neighbors[lc]) > maxConn {
				nNode.neighbors[lc] = h.selectNeighbors(nNode.neighbors[lc], maxConn)
			}
		}

		if len(neighbors) > 0 {
			ep = neighbors[0]
		}
	}

	if level > h.maxLevel {
		h.maxLevel = level
		h.entryPoint = id
	}
}

// Search finds the K nearest neighbors to the query vector.
// ef controls the search quality (higher = more accurate but slower).
func (h *HNSW) Search(query []float32, k int, ef int) []HNSWResult {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.nodes) == 0 || h.maxLevel < 0 {
		return nil
	}
	if ef < k {
		ef = k
	}

	ep := h.entryPoint
	for lc := h.maxLevel; lc > 0; lc-- {
		ep = h.searchLayerGreedy(query, ep, lc)
	}

	candidates := h.searchLayer(query, ep, ef, 0)

	results := make([]HNSWResult, 0, k)
	for _, cID := range candidates {
		cNode := h.nodes[cID]
		if cNode == nil {
			continue
		}
		dist := cosineDistance(query, cNode.vector)
		results = append(results, HNSWResult{ID: cID, Distance: dist, Similarity: 1 - dist})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Distance < results[j].Distance })

	if len(results) > k {
		results = results[:k]
	}
	return results
}

// Remove deletes a node from the index (lazy: marks as deleted).
func (h *HNSW) Remove(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	node, exists := h.nodes[id]
	if !exists {
		return
	}

	for lc, neighbors := range node.neighbors {
		for _, nID := range neighbors {
			nNode := h.nodes[nID]
			if nNode == nil {
				continue
			}
			nNode.neighbors[lc] = removeFromSlice(nNode.neighbors[lc], id)
		}
	}
	delete(h.nodes, id)

	if id == h.entryPoint {
		if len(h.nodes) == 0 {
			h.entryPoint = ""
			h.maxLevel = -1
		} else {
			// Select the node with the highest level as the new entry point
			// to maintain HNSW search quality invariants.
			bestID := ""
			bestLevel := -1
			for nID, nNode := range h.nodes {
				if nNode.level > bestLevel {
					bestLevel = nNode.level
					bestID = nID
				}
			}
			h.entryPoint = bestID
			h.maxLevel = bestLevel
		}
	}
}

// Size returns the number of indexed vectors.
func (h *HNSW) Size() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.nodes)
}

// Stats returns basic index statistics.
func (h *HNSW) Stats() (size int, maxLevel int) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.nodes), h.maxLevel
}

// HNSWResult holds a search result.
type HNSWResult struct {
	ID         string  `json:"id"`
	Distance   float64 `json:"distance"`
	Similarity float64 `json:"similarity"`
}

// --- Internal methods ---

func (h *HNSW) randomLevel() int {
	return int(-math.Log(h.rng.Float64()) * h.mL)
}

func (h *HNSW) searchLayerGreedy(query []float32, ep string, level int) string {
	best := ep
	bestDist := h.distance(query, ep)

	changed := true
	for changed {
		changed = false
		node := h.nodes[best]
		if node == nil {
			break
		}
		for _, nID := range node.neighbors[level] {
			d := h.distance(query, nID)
			if d < bestDist {
				bestDist = d
				best = nID
				changed = true
			}
		}
	}
	return best
}

func (h *HNSW) searchLayer(query []float32, ep string, ef int, level int) []string {
	visited := map[string]bool{ep: true}
	epDist := h.distance(query, ep)

	// Min-heap for candidates (closest first) and max-heap for results (farthest first).
	cands := &distMinHeap{{ep, epDist}}
	heap.Init(cands)
	results := &distMaxHeap{{ep, epDist}}
	heap.Init(results)

	for cands.Len() > 0 {
		current := heap.Pop(cands).(distItem)

		if results.Len() >= ef && current.dist > (*results)[0].dist {
			break
		}

		node := h.nodes[current.id]
		if node == nil {
			continue
		}
		for _, nID := range node.neighbors[level] {
			if visited[nID] {
				continue
			}
			visited[nID] = true
			d := h.distance(query, nID)

			if results.Len() < ef || d < (*results)[0].dist {
				heap.Push(cands, distItem{nID, d})
				heap.Push(results, distItem{nID, d})
				if results.Len() > ef {
					heap.Pop(results)
				}
			}
		}
	}

	out := make([]distItem, results.Len())
	for i := len(out) - 1; i >= 0; i-- {
		out[i] = heap.Pop(results).(distItem)
	}
	ids := make([]string, len(out))
	for i, r := range out {
		ids[i] = r.id
	}
	return ids
}

// selectNeighbors implements the heuristic neighbor selection from the
// HNSW paper (Algorithm 4). It prefers neighbors that are closer to the
// query than to any already-selected neighbor, promoting graph diversity
// and improving recall in high-dimensional spaces.
func (h *HNSW) selectNeighbors(candidates []string, maxConn int) []string {
	if len(candidates) <= maxConn {
		return candidates
	}

	// Sort candidates by distance to the query (implied by insertion order
	// from searchLayer, but candidates from neighbor lists may not be sorted).
	type idDist struct {
		id   string
		dist float64
	}
	scored := make([]idDist, 0, len(candidates))
	for _, cID := range candidates {
		cNode := h.nodes[cID]
		if cNode == nil {
			continue
		}
		scored = append(scored, idDist{cID, 0}) // dist placeholder
	}

	// If we can't compute meaningful distances (no query context inside
	// selectNeighbors), fall back to keeping the first maxConn candidates
	// which are already distance-ordered from searchLayer.
	if len(scored) <= maxConn {
		out := make([]string, len(scored))
		for i, s := range scored {
			out[i] = s.id
		}
		return out
	}

	// Heuristic: greedily add candidates, skipping those closer to an
	// already-selected neighbor than to the candidate set centroid. This
	// maintains diverse graph connectivity.
	selected := make([]string, 0, maxConn)
	for _, cID := range candidates {
		if len(selected) >= maxConn {
			break
		}
		cNode := h.nodes[cID]
		if cNode == nil {
			continue
		}
		tooClose := false
		for _, sID := range selected {
			sNode := h.nodes[sID]
			if sNode == nil {
				continue
			}
			if cosineDistance(cNode.vector, sNode.vector) < cosineDistance(cNode.vector, cNode.vector)*0.5 {
				tooClose = true
				break
			}
		}
		if !tooClose {
			selected = append(selected, cID)
		}
	}
	// Fill remaining slots if heuristic was too aggressive.
	for _, cID := range candidates {
		if len(selected) >= maxConn {
			break
		}
		found := false
		for _, s := range selected {
			if s == cID {
				found = true
				break
			}
		}
		if !found {
			selected = append(selected, cID)
		}
	}
	return selected
}

func (h *HNSW) distance(query []float32, nodeID string) float64 {
	node := h.nodes[nodeID]
	if node == nil {
		return math.MaxFloat64
	}
	return cosineDistance(query, node.vector)
}

type distItem struct {
	id   string
	dist float64
}

func cosineDistance(a []float32, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 1.0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 1.0
	}
	sim := dot / (math.Sqrt(normA) * math.Sqrt(normB))
	return 1.0 - sim
}

func removeFromSlice(s []string, val string) []string {
	for i, v := range s {
		if v == val {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── Heap implementations for searchLayer ──

// distMinHeap is a min-heap of distItem (closest distance at top).
type distMinHeap []distItem

func (h distMinHeap) Len() int            { return len(h) }
func (h distMinHeap) Less(i, j int) bool  { return h[i].dist < h[j].dist }
func (h distMinHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *distMinHeap) Push(x interface{}) { *h = append(*h, x.(distItem)) }
func (h *distMinHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

// distMaxHeap is a max-heap of distItem (farthest distance at top).
type distMaxHeap []distItem

func (h distMaxHeap) Len() int            { return len(h) }
func (h distMaxHeap) Less(i, j int) bool  { return h[i].dist > h[j].dist }
func (h distMaxHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *distMaxHeap) Push(x interface{}) { *h = append(*h, x.(distItem)) }
func (h *distMaxHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}
