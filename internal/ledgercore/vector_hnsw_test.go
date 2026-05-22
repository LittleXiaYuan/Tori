package ledger

import (
	"math"
	"math/rand"
	"testing"
)

func TestHNSWInsertAndSearch(t *testing.T) {
	h := NewHNSW(DefaultHNSWConfig())

	h.Insert("a", []float32{1, 0, 0})
	h.Insert("b", []float32{0, 1, 0})
	h.Insert("c", []float32{0, 0, 1})
	h.Insert("d", []float32{0.9, 0.1, 0})

	results := h.Search([]float32{1, 0, 0}, 2, 10)
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	if results[0].ID != "a" {
		t.Errorf("nearest to [1,0,0] should be 'a', got %s", results[0].ID)
	}
	if results[0].Similarity < 0.9 {
		t.Errorf("self-match similarity should be ~1.0, got %.3f", results[0].Similarity)
	}
}

func TestHNSWEmptySearch(t *testing.T) {
	h := NewHNSW(DefaultHNSWConfig())
	results := h.Search([]float32{1, 0}, 5, 10)
	if len(results) != 0 {
		t.Errorf("empty index should return 0 results, got %d", len(results))
	}
}

func TestHNSWRemove(t *testing.T) {
	h := NewHNSW(DefaultHNSWConfig())
	h.Insert("a", []float32{1, 0})
	h.Insert("b", []float32{0, 1})

	if h.Size() != 2 {
		t.Fatalf("expected size 2, got %d", h.Size())
	}

	h.Remove("a")
	if h.Size() != 1 {
		t.Fatalf("expected size 1 after remove, got %d", h.Size())
	}

	results := h.Search([]float32{1, 0}, 5, 10)
	for _, r := range results {
		if r.ID == "a" {
			t.Error("removed node 'a' should not appear in results")
		}
	}
}

func TestHNSWScaleSearch(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	h := NewHNSW(HNSWConfig{M: 16, EfConstruction: 100})

	dims := 8
	n := 500

	var vectors [][]float32
	for i := 0; i < n; i++ {
		v := make([]float32, dims)
		for d := 0; d < dims; d++ {
			v[d] = float32(rng.NormFloat64())
		}
		vectors = append(vectors, v)
		h.Insert(string(rune('A'+i%26))+string(rune('0'+i/26)), v)
	}

	query := vectors[0]
	results := h.Search(query, 5, 50)
	if len(results) == 0 {
		t.Fatal("expected results from 500-vector index")
	}

	if results[0].Distance > 0.01 {
		t.Errorf("self-match distance should be ~0, got %.3f", results[0].Distance)
	}
	t.Logf("found %d results, top similarity=%.3f", len(results), results[0].Similarity)
}

func TestHNSWRecallQuality(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	dims := 4
	n := 200

	var vectors [][]float32
	for i := 0; i < n; i++ {
		v := make([]float32, dims)
		for d := 0; d < dims; d++ {
			v[d] = float32(rng.NormFloat64())
		}
		vectors = append(vectors, v)
	}

	h := NewHNSW(HNSWConfig{M: 16, EfConstruction: 200})
	for i, v := range vectors {
		h.Insert(string(rune(i)), v)
	}

	query := vectors[0]
	trueDists := make([]float64, n)
	for i, v := range vectors {
		trueDists[i] = cosineDistance(query, v)
	}

	trueTopK := make([]int, n)
	for i := range trueTopK {
		trueTopK[i] = i
	}
	sortByDist := func(indices []int) {
		for i := 0; i < len(indices)-1; i++ {
			for j := i + 1; j < len(indices); j++ {
				if trueDists[indices[i]] > trueDists[indices[j]] {
					indices[i], indices[j] = indices[j], indices[i]
				}
			}
		}
	}
	sortByDist(trueTopK)

	results := h.Search(query, 10, 50)
	resultIDs := make(map[int]bool)
	for _, r := range results {
		resultIDs[int([]rune(r.ID)[0])] = true
	}

	hits := 0
	for _, idx := range trueTopK[:10] {
		if resultIDs[idx] {
			hits++
		}
	}
	recall := float64(hits) / 10.0
	t.Logf("recall@10 = %.1f%% (%d/10 true neighbors found)", recall*100, hits)

	if recall < 0.3 {
		t.Errorf("recall@10 too low: %.1f%% (expected >= 30%%)", recall*100)
	}
}

func TestHNSWSize(t *testing.T) {
	h := NewHNSW(DefaultHNSWConfig())
	if h.Size() != 0 {
		t.Error("new index should have size 0")
	}
	h.Insert("x", []float32{1, 2, 3})
	if h.Size() != 1 {
		t.Error("should have size 1 after insert")
	}
}

func TestCosineDistance(t *testing.T) {
	d := cosineDistance([]float32{1, 0}, []float32{1, 0})
	if math.Abs(d) > 0.001 {
		t.Errorf("same vector distance should be ~0, got %.3f", d)
	}

	d = cosineDistance([]float32{1, 0}, []float32{0, 1})
	if math.Abs(d-1.0) > 0.001 {
		t.Errorf("orthogonal distance should be ~1, got %.3f", d)
	}

	d = cosineDistance([]float32{1, 0}, []float32{-1, 0})
	if math.Abs(d-2.0) > 0.001 {
		t.Errorf("opposite distance should be ~2, got %.3f", d)
	}
}
