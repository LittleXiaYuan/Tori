package knowledge

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"yunque-agent/internal/agentcore/embeddings"
)

// SemanticIndex adds vector search capability to the knowledge store.
type SemanticIndex struct {
	mu       sync.RWMutex
	embedder embeddings.Embedder
	vectors  [][]float32 // parallel to store.chunks
	ready    bool
}

// SetEmbedder attaches an embedder and builds the vector index for all existing chunks.
func (s *Store) SetEmbedder(emb embeddings.Embedder) {
	if emb == nil {
		return
	}
	s.mu.Lock()
	if s.semantic == nil {
		s.semantic = &SemanticIndex{}
	}
	s.semantic.embedder = emb
	s.mu.Unlock()
}

// BuildIndex computes embeddings for all current chunks. Call after bulk ingest.
func (s *Store) BuildIndex(ctx context.Context) error {
	s.mu.RLock()
	sem := s.semantic
	chunks := make([]Chunk, len(s.chunks))
	copy(chunks, s.chunks)
	s.mu.RUnlock()

	if sem == nil || sem.embedder == nil {
		return nil
	}

	// Batch embed all chunks
	texts := make([]string, len(chunks))
	for i, c := range chunks {
		texts[i] = c.Content
	}

	const batchSize = 32
	allVecs := make([][]float32, 0, len(texts))
	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		vecs, err := sem.embedder.EmbedBatch(ctx, texts[i:end])
		if err != nil {
			slog.Warn("knowledge: embedding batch failed", "offset", i, "err", err)
			// Fill with nil vectors for failed batch
			for j := 0; j < end-i; j++ {
				allVecs = append(allVecs, nil)
			}
			continue
		}
		allVecs = append(allVecs, vecs...)
	}

	sem.mu.Lock()
	sem.vectors = allVecs
	sem.ready = true
	sem.mu.Unlock()

	slog.Info("knowledge: semantic index built", "chunks", len(allVecs))
	return nil
}

// SemanticSearch finds the most relevant chunks using vector similarity.
// Falls back to keyword Search if no embedder is configured.
func (s *Store) SemanticSearch(ctx context.Context, query string, limit int) []Chunk {
	if limit <= 0 {
		limit = 5
	}

	s.mu.RLock()
	sem := s.semantic
	s.mu.RUnlock()

	// Fallback to keyword search
	if sem == nil || !sem.ready || sem.embedder == nil {
		return s.Search(query, limit)
	}

	// Embed query
	qVec, err := sem.embedder.Embed(ctx, query)
	if err != nil {
		slog.Warn("knowledge: query embedding failed, falling back to keyword", "err", err)
		return s.Search(query, limit)
	}

	sem.mu.RLock()
	corpus := sem.vectors
	sem.mu.RUnlock()

	// TopK similarity
	scored := embeddings.TopK(qVec, corpus, limit)

	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]Chunk, 0, len(scored))
	for _, si := range scored {
		if si.Score < 0.3 { // minimum similarity threshold
			continue
		}
		if si.Index < len(s.chunks) {
			results = append(results, s.chunks[si.Index])
		}
	}
	return results
}

// HasSemanticIndex returns whether the semantic index is ready.
func (s *Store) HasSemanticIndex() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.semantic != nil && s.semantic.ready
}

// HybridSearch combines vector (dense) and BM25 (sparse) search using RRF fusion.
// Falls back gracefully: if no embedder → BM25 only; if no chunks → empty.
func (s *Store) HybridSearch(ctx context.Context, query string, limit int) []ScoredChunk {
	start := time.Now()
	if limit <= 0 {
		limit = 5
	}

	candidateLimit := limit * 5
	if candidateLimit < 20 {
		candidateLimit = 20
	}

	denseChunks := s.SemanticSearch(ctx, query, candidateLimit)

	// Sparse retrieval (BM25) — use cached index, rebuild only when chunks change.
	s.mu.Lock()
	if s.bm25Cache == nil || s.bm25Built != s.bm25Version {
		s.bm25Cache = NewBM25Index(s.chunks)
		s.bm25Built = s.bm25Version
	}
	bm25Idx := s.bm25Cache
	chunks := make([]Chunk, len(s.chunks))
	copy(chunks, s.chunks)
	s.mu.Unlock()

	bm25Results := bm25Idx.Search(query, candidateLimit)

	sparseChunks := make([]Chunk, len(bm25Results))
	for i, r := range bm25Results {
		if r.ChunkIndex < len(chunks) {
			sparseChunks[i] = chunks[r.ChunkIndex]
		}
	}

	results := FuseRRF(denseChunks, sparseChunks, 60, limit)
	if s.onSearch != nil {
		s.onSearch("hybrid", time.Since(start), len(results))
	}
	return results
}
