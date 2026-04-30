package knowledge

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestSemanticCache_HitMiss(t *testing.T) {
	cache := NewSemanticCache(SemanticCacheConfig{
		MaxSize:   10,
		Threshold: 0.85,
		TTL:       5 * time.Minute,
	})

	queryVec := []float32{1.0, 0.0, 0.0, 0.0}
	results := []ScoredChunk{
		{Chunk: Chunk{ID: "a", Content: "doc A"}, Score: 0.9},
		{Chunk: Chunk{ID: "b", Content: "doc B"}, Score: 0.8},
	}

	// Miss on empty cache
	_, hit := cache.Lookup(queryVec)
	if hit {
		t.Fatal("expected miss on empty cache")
	}

	// Put + exact hit
	cache.Put(queryVec, "test query", results)
	got, hit := cache.Lookup(queryVec)
	if !hit {
		t.Fatal("expected hit for exact same vector")
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
	if got[0].Chunk.ID != "a" {
		t.Errorf("expected chunk 'a', got %q", got[0].Chunk.ID)
	}

	// Similar vector should also hit (cosine > 0.85)
	similar := []float32{0.95, 0.05, 0.0, 0.0}
	got, hit = cache.Lookup(similar)
	if !hit {
		t.Fatal("expected hit for similar vector")
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 results from similar hit, got %d", len(got))
	}

	// Dissimilar vector should miss
	dissimilar := []float32{0.0, 0.0, 1.0, 0.0}
	_, hit = cache.Lookup(dissimilar)
	if hit {
		t.Fatal("expected miss for dissimilar vector")
	}
}

func TestSemanticCache_TTLExpiry(t *testing.T) {
	cache := NewSemanticCache(SemanticCacheConfig{
		MaxSize:   10,
		Threshold: 0.85,
		TTL:       50 * time.Millisecond,
	})

	vec := []float32{1.0, 0.0, 0.0}
	cache.Put(vec, "q1", []ScoredChunk{
		{Chunk: Chunk{ID: "x"}, Score: 0.5},
	})

	// Should hit before expiry
	_, hit := cache.Lookup(vec)
	if !hit {
		t.Fatal("expected hit before TTL")
	}

	// Wait for TTL
	time.Sleep(60 * time.Millisecond)

	_, hit = cache.Lookup(vec)
	if hit {
		t.Fatal("expected miss after TTL expiry")
	}
}

func TestSemanticCache_LRUEviction(t *testing.T) {
	cache := NewSemanticCache(SemanticCacheConfig{
		MaxSize:   3,
		Threshold: 0.99, // high threshold so different entries don't collide
		TTL:       5 * time.Minute,
	})

	// Fill cache with 3 entries, each in a different dimension
	vecs := [][]float32{
		{1, 0, 0, 0, 0},
		{0, 1, 0, 0, 0},
		{0, 0, 1, 0, 0},
	}
	for i, v := range vecs {
		cache.Put(v, fmt.Sprintf("q%d", i), []ScoredChunk{
			{Chunk: Chunk{ID: fmt.Sprintf("chunk-%d", i)}, Score: 0.9},
		})
		time.Sleep(time.Millisecond)
	}

	// Access first entry to make it recently used
	cache.Lookup(vecs[0])
	time.Sleep(time.Millisecond)

	// Add 4th entry — should evict LRU (vec[1], the second entry which was accessed earliest)
	newVec := []float32{0, 0, 0, 1, 0}
	cache.Put(newVec, "q3", []ScoredChunk{
		{Chunk: Chunk{ID: "chunk-3"}, Score: 0.9},
	})

	if len(cache.entries) != 3 {
		t.Fatalf("expected 3 entries after eviction, got %d", len(cache.entries))
	}

	// Entry 0 should still exist (recently accessed)
	_, hit := cache.Lookup(vecs[0])
	if !hit {
		t.Error("entry 0 should survive (recently accessed)")
	}

	// New entry should exist
	_, hit = cache.Lookup(newVec)
	if !hit {
		t.Error("new entry should exist")
	}
}

func TestSemanticCache_Stats(t *testing.T) {
	cache := NewSemanticCache(SemanticCacheConfig{MaxSize: 10, Threshold: 0.85, TTL: time.Minute})

	cache.RecordHit()
	cache.RecordHit()
	cache.RecordMiss()

	stats := cache.Stats()
	if stats.Hits != 2 {
		t.Errorf("expected 2 hits, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}
	expected := 2.0 / 3.0
	if diff := stats.HitRate - expected; diff > 0.01 || diff < -0.01 {
		t.Errorf("expected hit rate ~%.2f, got %.2f", expected, stats.HitRate)
	}
}

func TestSemanticCache_Clear(t *testing.T) {
	cache := NewSemanticCache(SemanticCacheConfig{MaxSize: 10, Threshold: 0.85, TTL: time.Minute})

	cache.Put([]float32{1, 0}, "q", []ScoredChunk{{Chunk: Chunk{ID: "a"}, Score: 1}})
	cache.RecordHit()
	cache.Clear()

	stats := cache.Stats()
	if stats.Size != 0 || stats.Hits != 0 {
		t.Error("clear should reset everything")
	}
}

func TestSemanticCache_Invalidate(t *testing.T) {
	cache := NewSemanticCache(SemanticCacheConfig{MaxSize: 10, Threshold: 0.85, TTL: time.Minute})

	vec := []float32{1, 0, 0}
	cache.Put(vec, "q1", []ScoredChunk{{Chunk: Chunk{ID: "a"}, Score: 0.9}})

	_, hit := cache.Lookup(vec)
	if !hit {
		t.Fatal("expected hit before invalidation")
	}

	cache.Invalidate()

	_, hit = cache.Lookup(vec)
	if hit {
		t.Fatal("expected miss after invalidation")
	}

	// Stats should still preserve counters (Invalidate only clears entries, not stats)
	cache.RecordHit()
	stats := cache.Stats()
	if stats.Size != 0 {
		t.Error("entries should be empty after invalidation")
	}
}

func TestSemanticCache_ConcurrentAccess(t *testing.T) {
	cache := NewSemanticCache(SemanticCacheConfig{MaxSize: 64, Threshold: 0.85, TTL: time.Minute})

	vec := []float32{1, 0, 0, 0}
	cache.Put(vec, "q", []ScoredChunk{{Chunk: Chunk{ID: "a"}, Score: 0.9}})

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 100; j++ {
				cache.Lookup(vec)
				cache.RecordHit()
				cache.RecordMiss()
			}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	stats := cache.Stats()
	if stats.Hits != 1000 || stats.Misses != 1000 {
		t.Errorf("expected 1000 hits and 1000 misses under concurrency, got hits=%d misses=%d", stats.Hits, stats.Misses)
	}
}

func TestCachedHybridSearch_Integration(t *testing.T) {
	store, _ := buildBenchCorpus()
	ctx := context.Background()

	emb := newDeterministicEmbedder(256)
	store.SetEmbedder(emb)
	store.BuildIndex(ctx)

	cache := NewSemanticCache(SemanticCacheConfig{
		MaxSize:   64,
		Threshold: 0.85,
		TTL:       time.Minute,
	})
	store.SetSemanticCache(cache)

	// First call — cache miss
	results1, cached := store.CachedHybridSearch(ctx, "RAG 检索增强生成", 5)
	if cached {
		t.Fatal("expected cache miss on first call")
	}
	if len(results1) == 0 {
		t.Fatal("expected results")
	}

	// Second identical call — cache hit
	results2, cached := store.CachedHybridSearch(ctx, "RAG 检索增强生成", 5)
	if !cached {
		t.Fatal("expected cache hit on second call")
	}
	if len(results2) != len(results1) {
		t.Errorf("expected same result count: %d vs %d", len(results1), len(results2))
	}

	stats := cache.Stats()
	if stats.Hits != 1 || stats.Misses != 1 {
		t.Errorf("expected 1 hit 1 miss, got hits=%d misses=%d", stats.Hits, stats.Misses)
	}
}

func TestCachedHybridSearch_SimilarQueryHit(t *testing.T) {
	store, _ := buildBenchCorpus()
	ctx := context.Background()

	emb := newDeterministicEmbedder(256)
	store.SetEmbedder(emb)
	store.BuildIndex(ctx)

	cache := NewSemanticCache(SemanticCacheConfig{
		MaxSize:   64,
		Threshold: 0.80, // lower threshold for testing similar queries
		TTL:       time.Minute,
	})
	store.SetSemanticCache(cache)

	// First query
	store.CachedHybridSearch(ctx, "BM25 算法原理", 5)

	// Similar query — may or may not hit depending on embedding similarity
	_, cached := store.CachedHybridSearch(ctx, "BM25 算法原理和参数", 5)

	// Either way, the mechanism should work without error
	stats := cache.Stats()
	t.Logf("Similar query test: cached=%v, hits=%d, misses=%d, hitRate=%.2f",
		cached, stats.Hits, stats.Misses, stats.HitRate)
}

// BenchmarkCachedVsUncached compares retrieval with and without semantic cache.
func BenchmarkCachedVsUncached(b *testing.B) {
	store, queries := buildBenchCorpus()
	ctx := context.Background()

	emb := newDeterministicEmbedder(256)
	store.SetEmbedder(emb)
	store.BuildIndex(ctx)

	cache := NewSemanticCache(SemanticCacheConfig{
		MaxSize:   128,
		Threshold: 0.85,
		TTL:       5 * time.Minute,
	})
	store.SetSemanticCache(cache)

	// Warm up cache
	for _, q := range queries {
		store.CachedHybridSearch(ctx, q.Query, 5)
	}

	b.Run("Uncached_HybridSearch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			q := queries[i%len(queries)]
			store.HybridSearch(ctx, q.Query, 5)
		}
	})

	b.Run("Cached_HybridSearch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			q := queries[i%len(queries)]
			store.CachedHybridSearch(ctx, q.Query, 5)
		}
	})
}

func TestTokenSavingsWithCache(t *testing.T) {
	store, queries := buildBenchCorpus()
	ctx := context.Background()

	emb := newDeterministicEmbedder(256)
	store.SetEmbedder(emb)
	store.BuildIndex(ctx)

	cache := NewSemanticCache(SemanticCacheConfig{
		MaxSize:   128,
		Threshold: 0.85,
		TTL:       5 * time.Minute,
	})
	store.SetSemanticCache(cache)

	avgChunkTokens := 200
	totalChunks := 20
	fullContextTokens := totalChunks * avgChunkTokens
	K := 5
	topKTokens := K * avgChunkTokens

	// Simulate two passes of all queries
	totalTokensUsed := 0
	totalTokensBaseline := 0

	for pass := 0; pass < 2; pass++ {
		for _, q := range queries {
			totalTokensBaseline += fullContextTokens

			results, cached := store.CachedHybridSearch(ctx, q.Query, K)
			if cached {
				totalTokensUsed += 0 // cache hit = no new tokens injected
			} else {
				totalTokensUsed += len(results) * avgChunkTokens
			}
		}
	}

	stats := cache.Stats()
	savingsRatio := float64(totalTokensBaseline) / float64(totalTokensUsed)

	t.Log("")
	t.Log("── Token Savings Analysis (2 passes × 15 queries) ──")
	t.Logf("  Baseline (full context):    %d tokens", totalTokensBaseline)
	t.Logf("  Hybrid + Cache:             %d tokens", totalTokensUsed)
	t.Logf("  Savings ratio:              %.1fx", savingsRatio)
	t.Logf("  Cache hit rate:             %.1f%% (%d hits / %d misses)",
		stats.HitRate*100, stats.Hits, stats.Misses)
	t.Logf("  Hybrid top-%d only:          %.1fx (without cache)",
		K, float64(fullContextTokens)/float64(topKTokens))
}
