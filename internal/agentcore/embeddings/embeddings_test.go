package embeddings

import (
	"math"
	"testing"
)

func TestCosineSimilarityIdentical(t *testing.T) {
	a := []float32{1.0, 2.0, 3.0}
	sim := CosineSimilarity(a, a)
	if math.Abs(sim-1.0) > 0.001 {
		t.Fatalf("identical vectors should have similarity ~1.0, got %f", sim)
	}
}

func TestCosineSimilarityOrthogonal(t *testing.T) {
	a := []float32{1.0, 0.0}
	b := []float32{0.0, 1.0}
	sim := CosineSimilarity(a, b)
	if math.Abs(sim) > 0.001 {
		t.Fatalf("orthogonal vectors should have similarity ~0.0, got %f", sim)
	}
}

func TestCosineSimilarityOpposite(t *testing.T) {
	a := []float32{1.0, 2.0}
	b := []float32{-1.0, -2.0}
	sim := CosineSimilarity(a, b)
	if math.Abs(sim+1.0) > 0.001 {
		t.Fatalf("opposite vectors should have similarity ~-1.0, got %f", sim)
	}
}

func TestCosineSimilarityDifferentLength(t *testing.T) {
	a := []float32{1.0}
	b := []float32{1.0, 2.0}
	sim := CosineSimilarity(a, b)
	if sim != 0 {
		t.Fatal("different length should return 0")
	}
}

func TestCosineSimilarityEmpty(t *testing.T) {
	sim := CosineSimilarity(nil, nil)
	if sim != 0 {
		t.Fatal("empty should return 0")
	}
}

func TestTopK(t *testing.T) {
	query := []float32{1.0, 0.0, 0.0}
	corpus := [][]float32{
		{0.0, 1.0, 0.0}, // orthogonal
		{1.0, 0.0, 0.0}, // identical
		{0.9, 0.1, 0.0}, // close
		{-1.0, 0.0, 0.0}, // opposite
	}
	results := TopK(query, corpus, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Index != 1 {
		t.Fatalf("top result should be index 1 (identical), got %d", results[0].Index)
	}
	if results[1].Index != 2 {
		t.Fatalf("second result should be index 2 (close), got %d", results[1].Index)
	}
}

func TestTopKLargerThanCorpus(t *testing.T) {
	query := []float32{1.0}
	corpus := [][]float32{{1.0}, {0.5}}
	results := TopK(query, corpus, 10)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestTopKEmpty(t *testing.T) {
	results := TopK([]float32{1.0}, nil, 5)
	if results != nil {
		t.Fatal("empty corpus should return nil")
	}
}

func TestResolverRegisterAndList(t *testing.T) {
	r := NewResolver()
	if len(r.List()) != 0 {
		t.Fatal("should be empty initially")
	}

	// We can't test actual embedding calls without an API, but we can test registry
	_, ok := r.Primary()
	if ok {
		t.Fatal("no primary should exist initially")
	}
}

func TestResolverSetPrimary(t *testing.T) {
	r := NewResolver()
	if r.SetPrimary("nonexistent") {
		t.Fatal("setting nonexistent primary should return false")
	}
}
