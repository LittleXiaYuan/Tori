package memory

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func newBenchOrchestrator(itemsPerLayer int) *Orchestrator {
	short := NewShortTerm(30 * time.Minute)
	mid := NewMidTerm()
	long := NewLongTerm()
	mgr := NewManager(short, mid, long)
	graph := NewGraph()
	em := NewEditableMemory()
	cfg := DefaultOrchestratorConfig()
	o := NewOrchestrator(cfg, mgr, graph, em)

	ctx := context.Background()
	for i := 0; i < itemsPerLayer; i++ {
		val := fmt.Sprintf("short term memory item number %d about topic %d", i, i%10)
		_ = short.Put(ctx, "bench", Item{Value: val, Key: fmt.Sprintf("s%d", i)})
	}
	for i := 0; i < itemsPerLayer; i++ {
		val := fmt.Sprintf("mid term fact %d: user prefers style %d for coding", i, i%5)
		_ = mgr.AddMid(ctx, "bench", Item{Value: val, Category: "fact"})
	}
	for i := 0; i < itemsPerLayer/5; i++ {
		val := fmt.Sprintf("long term knowledge %d about architecture pattern %d", i, i%3)
		_ = long.Put(ctx, "bench", Item{Value: val, Category: "knowledge"})
	}
	for i := 0; i < itemsPerLayer/10; i++ {
		graph.PutEntity(Entity{
			ID:   fmt.Sprintf("e%d", i),
			Name: fmt.Sprintf("entity_%d", i),
			Type: "concept",
		})
	}
	em.AddBlock("persona", "I am a helpful memory benchmark assistant", 0)
	em.AddBlock("notes", "User likes Go, Rust, and distributed systems", 0)

	return o
}

func BenchmarkRecallSequential(b *testing.B) {
	o := newBenchOrchestrator(200)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		o.Recall(ctx, "bench", "memory item topic", 20)
	}
}

func BenchmarkRecallConcurrent_10(b *testing.B) {
	benchRecallConcurrent(b, 10)
}

func BenchmarkRecallConcurrent_50(b *testing.B) {
	benchRecallConcurrent(b, 50)
}

func BenchmarkRecallConcurrent_100(b *testing.B) {
	benchRecallConcurrent(b, 100)
}

func benchRecallConcurrent(b *testing.B, goroutines int) {
	o := newBenchOrchestrator(200)
	ctx := context.Background()
	queries := []string{
		"memory item topic",
		"user prefers style",
		"architecture pattern",
		"entity concept",
		"helpful assistant",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			q := queries[i%len(queries)]
			o.Recall(ctx, "bench", q, 20)
			i++
		}
	})
}

func BenchmarkRecallMixedReadWrite(b *testing.B) {
	o := newBenchOrchestrator(200)
	ctx := context.Background()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%10 == 0 {
				_ = o.Ingest(ctx, "bench",
					fmt.Sprintf("new fact %d about concurrent access", i),
					"fact", "bench")
			} else {
				o.Recall(ctx, "bench", "concurrent access", 20)
			}
			i++
		}
	})
}

func BenchmarkEmbedGateCacheLookup(b *testing.B) {
	gate := &embeddingGate{
		embed: func(_ context.Context, text string) ([]float32, error) {
			vec := make([]float32, 128)
			for i := range vec {
				vec[i] = float32(i) * 0.01
			}
			return vec, nil
		},
		cfg:         DefaultEmbeddingGateConfig(),
		recentCache: make(map[string]cachedVec),
	}
	for i := range gate.shards {
		gate.shards[i].vecs = make(map[string]*tenantVecCache)
	}

	ctx := context.Background()
	for i := 0; i < 500; i++ {
		text := fmt.Sprintf("cached content %d", i)
		_, _ = gate.embedCached(ctx, text)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			text := fmt.Sprintf("cached content %d", i%500)
			_, _ = gate.embedCached(ctx, text)
			i++
		}
	})
}

func BenchmarkEmbedGateTenantSharded(b *testing.B) {
	gate := &embeddingGate{
		embed: func(_ context.Context, text string) ([]float32, error) {
			vec := make([]float32, 128)
			for i := range vec {
				vec[i] = float32(i) * 0.01
			}
			return vec, nil
		},
		cfg:         DefaultEmbeddingGateConfig(),
		recentCache: make(map[string]cachedVec),
	}
	for i := range gate.shards {
		gate.shards[i].vecs = make(map[string]*tenantVecCache)
	}

	ctx := context.Background()
	for i := 0; i < 20; i++ {
		tid := fmt.Sprintf("tenant_%d", i)
		for j := 0; j < 50; j++ {
			_, _ = gate.embedForTenant(ctx, tid, fmt.Sprintf("content_%d", j))
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			tid := fmt.Sprintf("tenant_%d", i%20)
			content := fmt.Sprintf("content_%d", i%50)
			_, _ = gate.embedForTenant(ctx, tid, content)
			i++
		}
	})
}

func TestRecallConcurrentCorrectness(t *testing.T) {
	o := newBenchOrchestrator(100)
	ctx := context.Background()
	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			results := o.Recall(ctx, "bench", "memory item", 20)
			if len(results) == 0 {
				errors <- fmt.Errorf("goroutine %d: empty results", id)
			}
			for _, r := range results {
				if r.Score < 0 {
					errors <- fmt.Errorf("goroutine %d: negative score", id)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}

func TestEmbedGateShardDistribution(t *testing.T) {
	counts := make([]int, tenantShardCount)
	for i := 0; i < 1000; i++ {
		tid := fmt.Sprintf("tenant_%d", i)
		idx := tenantShardIdx(tid)
		if idx < 0 || idx >= tenantShardCount {
			t.Fatalf("shard index out of range: %d", idx)
		}
		counts[idx]++
	}
	for i, c := range counts {
		ratio := float64(c) / 1000.0
		if ratio < 0.05 || ratio > 0.25 {
			t.Errorf("shard %d has %d tenants (%.1f%%), distribution too uneven", i, c, ratio*100)
		}
	}
}
