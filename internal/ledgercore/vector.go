package ledger

import (
	"context"
	"math"
)

// VectorANNBackend selects the approximate-nearest-neighbor backend used by
// VectorIndex.Search when an ANN index is available.
type VectorANNBackend string

const (
	VectorANNBruteForce VectorANNBackend = "bruteforce"
	VectorANNIVF        VectorANNBackend = "ivf"
	VectorANNHNSW       VectorANNBackend = "hnsw"
)

// VectorIndex provides semantic search over memory embeddings.
// It wraps the Backend's vector storage and adds a pluggable EmbedFunc
// for on-the-fly embedding generation.
//
// When IVF is enabled (via EnableIVF), approximate search is used for
// large datasets, falling back to brute-force via the Backend for small ones.
type VectorIndex struct {
	backend Backend
	embedFn EmbedFunc
	dims    int
	ivf     *IVFIndex // nil = brute-force only
	hnsw    *HNSW     // nil = disabled
	ann     VectorANNBackend
}

// SetEmbedFunc configures the embedding function. Must be called before Put/Search.
func (vi *VectorIndex) SetEmbedFunc(fn EmbedFunc) { vi.embedFn = fn }

// SetDimensions sets the expected embedding dimensions for validation.
func (vi *VectorIndex) SetDimensions(d int) { vi.dims = d }

// Dimensions returns the configured embedding dimensions.
func (vi *VectorIndex) Dimensions() int { return vi.dims }

// Enabled returns true if an embedding function is configured.
func (vi *VectorIndex) Enabled() bool { return vi.embedFn != nil }

// SetANNBackend selects which ANN backend Search should prefer.
func (vi *VectorIndex) SetANNBackend(backend VectorANNBackend) {
	switch backend {
	case VectorANNIVF, VectorANNHNSW, VectorANNBruteForce:
		vi.ann = backend
	default:
		vi.ann = VectorANNBruteForce
	}
}

// ANNBackend returns the configured ANN backend.
func (vi *VectorIndex) ANNBackend() VectorANNBackend {
	if vi.ann == "" {
		return VectorANNBruteForce
	}
	return vi.ann
}

// Embed generates a vector embedding for the given text using the configured EmbedFunc.
// Returns nil if no EmbedFunc is configured.
func (vi *VectorIndex) Embed(ctx context.Context, text string) ([]float32, error) {
	if vi.embedFn == nil {
		return nil, nil
	}
	return vi.embedFn(ctx, text)
}

// Put stores an embedding for a memory entry.
// If IVF is enabled, the embedding is also added to the in-memory index.
func (vi *VectorIndex) Put(ctx context.Context, memoryID string, embedding []float32) error {
	if err := vi.backend.PutEmbedding(ctx, memoryID, embedding); err != nil {
		return err
	}
	if vi.ivf != nil && vi.ivf.Trained() {
		vi.ivf.Add(memoryID, embedding)
	}
	if vi.hnsw != nil {
		vi.hnsw.Insert(memoryID, embedding)
	}
	return nil
}

// Search performs semantic nearest-neighbor search.
// Uses the configured ANN backend if available, otherwise falls back to brute-force.
func (vi *VectorIndex) Search(ctx context.Context, q VectorQuery) ([]ScoredEntry, error) {
	switch vi.ANNBackend() {
	case VectorANNHNSW:
		if vi.hnsw != nil && vi.hnsw.Size() > 0 {
			return vi.searchHNSW(ctx, q)
		}
	case VectorANNIVF:
		if vi.ivf != nil && vi.ivf.Trained() {
			return vi.searchIVF(ctx, q)
		}
	}
	if vi.ivf != nil && vi.ivf.Trained() && vi.ann == "" {
		return vi.searchIVF(ctx, q)
	}
	return vi.backend.SearchByVector(ctx, q)
}

// EnableIVF activates the IVF approximate search index.
// Call TrainIVF() after enabling to build the index from existing data.
func (vi *VectorIndex) EnableIVF(cfg IVFConfig) {
	vi.ivf = NewIVFIndex(cfg)
	vi.ann = VectorANNIVF
}

// EnableHNSW activates the HNSW approximate search index.
// Call TrainHNSW() after enabling to build the index from existing data.
func (vi *VectorIndex) EnableHNSW(cfg HNSWConfig) {
	vi.hnsw = NewHNSW(cfg)
	vi.ann = VectorANNHNSW
}

// TrainIVF builds/rebuilds the IVF index from all embeddings in the backend.
// Should be called on startup and periodically for fresh data.
func (vi *VectorIndex) TrainIVF(ctx context.Context, tenantID string) error {
	if vi.ivf == nil {
		return nil
	}

	// Load all embeddings from backend
	results, err := vi.backend.SearchByVector(ctx, VectorQuery{
		TenantID:  tenantID,
		Embedding: make([]float32, vi.dims), // zero vector to get all
		Limit:     100000,
		MinScore:  -1, // no minimum
	})
	if err != nil {
		return err
	}

	vectors := make(map[string][]float32, len(results))
	for _, r := range results {
		if len(r.Entry.Embedding) > 0 {
			vectors[r.Entry.ID] = r.Entry.Embedding
		}
	}

	vi.ivf.Train(vectors)
	return nil
}

// IVFStats returns IVF index statistics. Returns (0,0) if IVF is not enabled.
func (vi *VectorIndex) IVFStats() (numClusters, totalVectors int) {
	if vi.ivf == nil {
		return 0, 0
	}
	return vi.ivf.Stats()
}

// IVFNeedsRetrain returns true when incremental inserts have drifted
// cluster centroids enough (>30% growth) to warrant a full retrain.
// Callers (e.g. MemoryLifecycle.RunAll or a startup hook) should call
// TrainIVF when this returns true.
func (vi *VectorIndex) IVFNeedsRetrain() bool {
	if vi.ivf == nil {
		return false
	}
	return vi.ivf.NeedsRetrain()
}

// TrainHNSW builds/rebuilds the HNSW index from all embeddings in the backend.
func (vi *VectorIndex) TrainHNSW(ctx context.Context, tenantID string, cfg HNSWConfig) error {
	if vi.hnsw == nil {
		vi.EnableHNSW(cfg)
	}

	results, err := vi.backend.SearchByVector(ctx, VectorQuery{
		TenantID:  tenantID,
		Embedding: make([]float32, vi.dims),
		Limit:     100000,
		MinScore:  -1,
	})
	if err != nil {
		return err
	}

	vi.hnsw = NewHNSW(cfg)
	for _, r := range results {
		if len(r.Entry.Embedding) > 0 {
			vi.hnsw.Insert(r.Entry.ID, r.Entry.Embedding)
		}
	}
	vi.ann = VectorANNHNSW
	return nil
}

// HNSWStats returns HNSW index statistics. Returns (0, -1) if disabled.
func (vi *VectorIndex) HNSWStats() (size int, maxLevel int) {
	if vi.hnsw == nil {
		return 0, -1
	}
	return vi.hnsw.Stats()
}

// searchIVF uses the in-memory IVF index for approximate search.
func (vi *VectorIndex) searchIVF(ctx context.Context, q VectorQuery) ([]ScoredEntry, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 10
	}
	minScore := q.MinScore
	if minScore == 0 {
		minScore = 0.3
	}

	// The IVF index spans all tenants, so over-fetch and apply the same
	// tenant/kind filters as searchHNSW; skipping them would leak another
	// tenant's memories into the results.
	ivfResults := vi.ivf.Search(q.Embedding, limit*10, minScore)

	// Load full memory entries from backend
	var results []ScoredEntry
	for _, ir := range ivfResults {
		m, err := vi.backend.GetMemory(ctx, ir.MemoryID)
		if err != nil || m == nil {
			continue
		}
		if q.TenantID != "" && m.TenantID != q.TenantID {
			continue
		}
		if len(q.Kinds) > 0 {
			ok := false
			for _, kind := range q.Kinds {
				if m.Kind == kind {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		results = append(results, ScoredEntry{
			Entry:  *m,
			Score:  ir.Score,
			Reason: "semantic (ivf)",
		})
		if len(results) >= limit {
			break
		}
	}
	return results, nil
}

// searchHNSW uses the in-memory HNSW index for approximate search.
func (vi *VectorIndex) searchHNSW(ctx context.Context, q VectorQuery) ([]ScoredEntry, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 10
	}
	minScore := q.MinScore
	if minScore == 0 {
		minScore = 0.3
	}

	ef := limit * 5
	if vi.hnsw.efSearch > ef {
		ef = vi.hnsw.efSearch
	}
	candidateLimit := limit * 10
	if candidateLimit < 50 {
		candidateLimit = 50
	}
	hits := vi.hnsw.Search(q.Embedding, candidateLimit, ef)

	var results []ScoredEntry
	for _, hit := range hits {
		if hit.Similarity < minScore {
			continue
		}
		m, err := vi.backend.GetMemory(ctx, hit.ID)
		if err != nil || m == nil {
			continue
		}
		if q.TenantID != "" && m.TenantID != q.TenantID {
			continue
		}
		if len(q.Kinds) > 0 {
			ok := false
			for _, kind := range q.Kinds {
				if m.Kind == kind {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		results = append(results, ScoredEntry{
			Entry:  *m,
			Score:  hit.Similarity,
			Reason: "semantic (hnsw)",
		})
		if len(results) >= limit {
			break
		}
	}
	return results, nil
}

// CosineSimilarity computes the cosine similarity between two vectors.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		fa, fb := float64(a[i]), float64(b[i])
		dot += fa * fb
		normA += fa * fa
		normB += fb * fb
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}
