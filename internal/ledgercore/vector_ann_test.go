package ledger_test

import (
	"context"
	"strings"
	"testing"

	"yunque-agent/internal/ledgercore"
)

func TestVectorIndexHNSWBackendSearch(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()
	ldg.Vector.SetDimensions(3)
	ldg.Vector.EnableHNSW(ledger.HNSWConfig{M: 8, EfConstruction: 50, EfSearch: 20})

	memA := putVectorMemory(t, ldg, "t1", "alpha", []float32{1, 0, 0})
	putVectorMemory(t, ldg, "t1", "beta", []float32{0, 1, 0})
	putVectorMemory(t, ldg, "t2", "other tenant", []float32{1, 0, 0})

	results, err := ldg.Vector.Search(ctx, ledger.VectorQuery{
		TenantID:  "t1",
		Embedding: []float32{1, 0, 0},
		Limit:     3,
		MinScore:  0.5,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected HNSW search results")
	}
	if results[0].Entry.ID != memA.ID {
		t.Fatalf("expected alpha memory first, got %s", results[0].Entry.ID)
	}
	if !strings.Contains(results[0].Reason, "hnsw") {
		t.Fatalf("expected HNSW reason, got %q", results[0].Reason)
	}
	for _, result := range results {
		if result.Entry.TenantID != "t1" {
			t.Fatalf("HNSW search leaked tenant %q", result.Entry.TenantID)
		}
	}

	size, _ := ldg.Vector.HNSWStats()
	if size != 3 {
		t.Fatalf("expected 3 indexed vectors, got %d", size)
	}
}

func TestVectorIndexTrainHNSWFromBackend(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()
	ldg.Vector.SetDimensions(3)

	memA := putVectorMemory(t, ldg, "t1", "trained alpha", []float32{1, 0, 0})
	putVectorMemory(t, ldg, "t1", "trained beta", []float32{0, 1, 0})

	if err := ldg.Vector.TrainHNSW(ctx, "t1", ledger.HNSWConfig{M: 8, EfConstruction: 50, EfSearch: 20}); err != nil {
		t.Fatalf("TrainHNSW: %v", err)
	}
	if got := ldg.Vector.ANNBackend(); got != ledger.VectorANNHNSW {
		t.Fatalf("expected HNSW backend, got %s", got)
	}
	if size, _ := ldg.Vector.HNSWStats(); size != 2 {
		t.Fatalf("expected trained HNSW size 2, got %d", size)
	}

	results, err := ldg.Vector.Search(ctx, ledger.VectorQuery{
		TenantID:  "t1",
		Embedding: []float32{1, 0, 0},
		Limit:     1,
		MinScore:  0.5,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 || results[0].Entry.ID != memA.ID {
		t.Fatalf("expected trained HNSW top hit %s, got %#v", memA.ID, results)
	}
	if !strings.Contains(results[0].Reason, "hnsw") {
		t.Fatalf("expected trained HNSW search reason, got %q", results[0].Reason)
	}
}

func putVectorMemory(t *testing.T, ldg *ledger.Ledger, tenantID, content string, embedding []float32) *ledger.MemoryEntry {
	t.Helper()
	ctx := context.Background()
	m := &ledger.MemoryEntry{
		TenantID:   tenantID,
		Kind:       ledger.MemoryFact,
		Key:        content,
		Content:    content,
		Source:     "test",
		Confidence: 0.9,
	}
	if err := ldg.Memory.Put(ctx, m); err != nil {
		t.Fatalf("Put memory: %v", err)
	}
	if err := ldg.Vector.Put(ctx, m.ID, embedding); err != nil {
		t.Fatalf("Put vector: %v", err)
	}
	return m
}
