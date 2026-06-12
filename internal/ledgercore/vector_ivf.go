package ledger

import (
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"
)

// ── IVF (Inverted File Index) for Approximate Nearest Neighbor Search ──
//
// Instead of brute-force comparing every embedding (O(N)), IVF partitions
// vectors into K clusters using K-means, then at query time only scans
// the nprobe nearest clusters ???reducing search from O(N) to O(N/K * nprobe).
//
// Typical config: K=sqrt(N), nprobe=K/10. For 10K memories: K=100, nprobe=10
// yields ~10x speedup at >95% recall.

// IVFConfig configures the IVF index.
type IVFConfig struct {
	// NumClusters is the number of Voronoi cells (K). Default: sqrt(N), min 8.
	NumClusters int
	// NumProbe is the number of clusters to search at query time.
	// Default: max(ceil(sqrt(K)), K/10).
	NumProbe int
	// MaxIterations for K-means training. Default: 20.
	MaxIterations int
	// MinPointsToTrain: don't build IVF if fewer than this many vectors. Default: 100.
	MinPointsToTrain int
}

// DefaultIVFConfig returns sensible defaults.
func DefaultIVFConfig() IVFConfig {
	return IVFConfig{
		MaxIterations:    20,
		MinPointsToTrain: 100,
	}
}

// IVFIndex is an in-memory inverted file index over embeddings.
// It stores centroids and cluster assignments but delegates actual
// vector storage to the Backend.
type IVFIndex struct {
	mu        sync.RWMutex
	config    IVFConfig
	trained   bool
	centroids [][]float32        // [K][dims]
	cells     map[int][]ivfEntry // clusterID -> entries in this cell
	dims      int
	rng       *rand.Rand // instance-local RNG to avoid global lock contention

	trainedSize int // number of vectors at last Train()
	addedSince  int // vectors added via Add() since last Train()
}

type ivfEntry struct {
	memoryID  string
	embedding []float32
}

// NewIVFIndex creates a new IVF index.
func NewIVFIndex(cfg IVFConfig) *IVFIndex {
	if cfg.MaxIterations == 0 {
		cfg.MaxIterations = 20
	}
	if cfg.MinPointsToTrain == 0 {
		cfg.MinPointsToTrain = 100
	}
	return &IVFIndex{
		config: cfg,
		cells:  make(map[int][]ivfEntry),
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Trained returns true if the index has been trained.
func (idx *IVFIndex) Trained() bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.trained
}

// Train builds the IVF index from a set of vectors using K-means.
// This should be called periodically (e.g. on startup or after bulk inserts).
func (idx *IVFIndex) Train(vectors map[string][]float32) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	n := len(vectors)
	if n < idx.config.MinPointsToTrain {
		idx.trained = false
		return
	}

	// Collect all vectors
	entries := make([]ivfEntry, 0, n)
	for id, vec := range vectors {
		entries = append(entries, ivfEntry{memoryID: id, embedding: vec})
	}

	// Determine K and nprobe
	K := idx.config.NumClusters
	if K == 0 {
		K = int(math.Sqrt(float64(n)))
		if K < 8 {
			K = 8
		}
		if K > 256 {
			K = 256
		}
	}
	if K > n {
		K = n
	}

	nprobe := idx.config.NumProbe
	if nprobe == 0 {
		// The sqrt(K) floor keeps recall stable at small scale, where K-means
		// splits the few natural clusters across many cells and K/10 probes
		// too few of them (measured: K=31, nprobe=3 → recall dips to 50%;
		// nprobe=6 → 100% over 100 builds). At scale K/10 dominates,
		// preserving the documented ~10x speedup for 10K+ vectors.
		nprobe = max(int(math.Ceil(math.Sqrt(float64(K)))), K/10)
	}

	idx.dims = len(entries[0].embedding)
	idx.config.NumClusters = K
	idx.config.NumProbe = nprobe

	// K-means++ initialization
	centroids := kmeansInit(entries, K, idx.rng)

	// K-means iterations
	for iter := 0; iter < idx.config.MaxIterations; iter++ {
		// Assignment step: assign each vector to nearest centroid
		assignments := make([]int, len(entries))
		for i, e := range entries {
			assignments[i] = nearestCentroid(e.embedding, centroids)
		}

		// Update step: recompute centroids
		newCentroids := make([][]float32, K)
		counts := make([]int, K)
		for i := range newCentroids {
			newCentroids[i] = make([]float32, idx.dims)
		}

		for i, e := range entries {
			c := assignments[i]
			counts[c]++
			for d := 0; d < idx.dims; d++ {
				newCentroids[c][d] += e.embedding[d]
			}
		}

		converged := true
		for c := 0; c < K; c++ {
			if counts[c] == 0 {
				// Empty cluster: reinitialize to random vector
				newCentroids[c] = make([]float32, idx.dims)
				copy(newCentroids[c], entries[idx.rng.Intn(len(entries))].embedding)
				converged = false
				continue
			}
			for d := 0; d < idx.dims; d++ {
				newCentroids[c][d] /= float32(counts[c])
			}
			// Check convergence: centroid movement
			if cosDistf32(centroids[c], newCentroids[c]) > 0.001 {
				converged = false
			}
		}
		centroids = newCentroids
		if converged {
			break
		}
	}

	// Build inverted lists
	cells := make(map[int][]ivfEntry, K)
	for _, e := range entries {
		c := nearestCentroid(e.embedding, centroids)
		cells[c] = append(cells[c], e)
	}

	idx.centroids = centroids
	idx.cells = cells
	idx.trained = true
	idx.trainedSize = len(entries)
	idx.addedSince = 0
}

// Add inserts a single vector into the index (online insert after training).
func (idx *IVFIndex) Add(memoryID string, embedding []float32) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if !idx.trained || len(idx.centroids) == 0 {
		return
	}

	c := nearestCentroid(embedding, idx.centroids)
	idx.cells[c] = append(idx.cells[c], ivfEntry{memoryID: memoryID, embedding: embedding})
	idx.addedSince++
}

// NeedsRetrain returns true when incremental inserts have grown beyond
// 30% of the original training set, at which point cluster centroids are
// likely drifted enough to degrade recall quality.
func (idx *IVFIndex) NeedsRetrain() bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	if !idx.trained || idx.trainedSize == 0 {
		return false
	}
	return float64(idx.addedSince)/float64(idx.trainedSize) > 0.30
}

// Remove deletes a vector from the index.
func (idx *IVFIndex) Remove(memoryID string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	for c, entries := range idx.cells {
		for i, e := range entries {
			if e.memoryID == memoryID {
				idx.cells[c] = append(entries[:i], entries[i+1:]...)
				return
			}
		}
	}
}

// IVFSearchResult is a candidate from IVF search.
type IVFSearchResult struct {
	MemoryID string
	Score    float64
}

// Search performs approximate nearest neighbor search using IVF.
// Returns up to `limit` results sorted by descending cosine similarity.
func (idx *IVFIndex) Search(query []float32, limit int, minScore float64) []IVFSearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if !idx.trained || len(idx.centroids) == 0 {
		return nil
	}

	// Find the nprobe nearest centroids
	type centroidDist struct {
		id   int
		dist float64
	}
	dists := make([]centroidDist, len(idx.centroids))
	for i, c := range idx.centroids {
		dists[i] = centroidDist{id: i, dist: cosSimf32(query, c)}
	}
	sort.Slice(dists, func(i, j int) bool { return dists[i].dist > dists[j].dist })

	nprobe := idx.config.NumProbe
	if nprobe > len(dists) {
		nprobe = len(dists)
	}

	// Scan only the nearest clusters
	var results []IVFSearchResult
	for _, cd := range dists[:nprobe] {
		for _, e := range idx.cells[cd.id] {
			sim := cosSimf32(query, e.embedding)
			if sim >= minScore {
				results = append(results, IVFSearchResult{
					MemoryID: e.memoryID,
					Score:    sim,
				})
			}
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })

	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

// Stats returns index statistics.
func (idx *IVFIndex) Stats() (numClusters, totalVectors int) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	numClusters = len(idx.centroids)
	for _, entries := range idx.cells {
		totalVectors += len(entries)
	}
	return
}

// ── Internal helpers ──

func kmeansInit(entries []ivfEntry, K int, rng *rand.Rand) [][]float32 {
	dims := len(entries[0].embedding)
	centroids := make([][]float32, 0, K)

	first := make([]float32, dims)
	copy(first, entries[rng.Intn(len(entries))].embedding)
	centroids = append(centroids, first)

	for len(centroids) < K {
		distances := make([]float64, len(entries))
		totalDist := 0.0
		for i, e := range entries {
			minDist := math.MaxFloat64
			for _, c := range centroids {
				d := 1.0 - cosSimf32(e.embedding, c)
				if d < minDist {
					minDist = d
				}
			}
			distances[i] = minDist * minDist
			totalDist += distances[i]
		}

		if totalDist == 0 {
			break
		}

		target := rng.Float64() * totalDist
		cumulative := 0.0
		chosen := len(entries) - 1
		for i, d := range distances {
			cumulative += d
			if cumulative >= target {
				chosen = i
				break
			}
		}

		c := make([]float32, dims)
		copy(c, entries[chosen].embedding)
		centroids = append(centroids, c)
	}

	return centroids
}

func nearestCentroid(vec []float32, centroids [][]float32) int {
	best := 0
	bestSim := -1.0
	for i, c := range centroids {
		sim := cosSimf32(vec, c)
		if sim > bestSim {
			bestSim = sim
			best = i
		}
	}
	return best
}

// cosSimf32 computes cosine similarity using float32 directly (cache-friendly).
func cosSimf32(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	denom := float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB)))
	if denom == 0 {
		return 0
	}
	return float64(dot / denom)
}

// cosDistf32 is 1 - cosine_similarity.
func cosDistf32(a, b []float32) float64 {
	return 1.0 - cosSimf32(a, b)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
