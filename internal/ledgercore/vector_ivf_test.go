package ledger_test

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"yunque-agent/internal/ledgercore"
)

// TestIVFBasicSearch verifies that IVF search returns correct approximate results.
func TestIVFBasicSearch(t *testing.T) {
	dims := 32
	numVectors := 500

	// Generate clustered random vectors
	vectors := generateClusteredVectors(numVectors, dims, 5)

	// Build IVF index
	idx := ledger.NewIVFIndex(ledger.IVFConfig{
		MinPointsToTrain: 50,
		MaxIterations:    15,
	})
	idx.Train(vectors)

	if !idx.Trained() {
		t.Fatal("expected index to be trained")
	}

	clusters, total := idx.Stats()
	if clusters == 0 {
		t.Fatal("expected non-zero clusters")
	}
	if total != numVectors {
		t.Fatalf("expected %d vectors, got %d", numVectors, total)
	}
	t.Logf("IVF stats: %d clusters, %d vectors", clusters, total)

	// Search with a known vector
	queryID := "vec-0"
	queryVec := vectors[queryID]
	results := idx.Search(queryVec, 10, 0.5)

	if len(results) == 0 {
		t.Fatal("expected non-empty search results")
	}

	// The query vector itself should be the top result (exact match = 1.0)
	if results[0].MemoryID != queryID {
		t.Logf("top result is %s (score %.4f), expected %s", results[0].MemoryID, results[0].Score, queryID)
	}
	if results[0].Score < 0.99 {
		t.Logf("top result score %.4f, expected ~1.0", results[0].Score)
	}

	// All results should have scores >= minScore
	for _, r := range results {
		if r.Score < 0.5 {
			t.Fatalf("result %s has score %.4f < 0.5", r.MemoryID, r.Score)
		}
	}
}

// TestIVFRecall measures IVF recall vs brute-force.
func TestIVFRecall(t *testing.T) {
	dims := 64
	numVectors := 1000
	topK := 10

	vectors := generateClusteredVectors(numVectors, dims, 8)

	// Brute-force ground truth
	queryVec := vectors["vec-0"]
	bruteForceTruth := bruteForceTopK(queryVec, vectors, topK)

	// IVF search
	idx := ledger.NewIVFIndex(ledger.IVFConfig{
		MinPointsToTrain: 50,
		MaxIterations:    20,
	})
	idx.Train(vectors)

	ivfResults := idx.Search(queryVec, topK, 0.0)

	// Compute recall@K
	truthSet := make(map[string]bool)
	for _, id := range bruteForceTruth {
		truthSet[id] = true
	}

	hits := 0
	for _, r := range ivfResults {
		if truthSet[r.MemoryID] {
			hits++
		}
	}

	recall := float64(hits) / float64(topK)
	t.Logf("IVF recall@%d: %.2f%% (%d/%d hits)", topK, recall*100, hits, topK)

	// We expect at least 70% recall with default settings
	if recall < 0.7 {
		t.Errorf("IVF recall %.2f%% is below acceptable threshold of 70%%", recall*100)
	}
}

// TestIVFOnlineInsert verifies that vectors added after training are searchable.
func TestIVFOnlineInsert(t *testing.T) {
	dims := 16
	vectors := generateClusteredVectors(200, dims, 4)

	idx := ledger.NewIVFIndex(ledger.IVFConfig{MinPointsToTrain: 50})
	idx.Train(vectors)

	// Add a new vector after training
	newVec := make([]float32, dims)
	for i := range newVec {
		newVec[i] = 1.0 // all-ones vector
	}
	idx.Add("new-vec", newVec)

	_, total := idx.Stats()
	if total != 201 {
		t.Fatalf("expected 201 vectors after insert, got %d", total)
	}

	// Search for the new vector
	results := idx.Search(newVec, 5, 0.9)
	found := false
	for _, r := range results {
		if r.MemoryID == "new-vec" {
			found = true
			break
		}
	}
	if !found {
		t.Error("new-vec not found in search results after online insert")
	}
}

// TestIVFRemove verifies vector removal from the index.
func TestIVFRemove(t *testing.T) {
	dims := 16
	vectors := generateClusteredVectors(200, dims, 4)

	idx := ledger.NewIVFIndex(ledger.IVFConfig{MinPointsToTrain: 50})
	idx.Train(vectors)

	_, totalBefore := idx.Stats()

	// Remove a vector
	idx.Remove("vec-0")

	_, totalAfter := idx.Stats()
	if totalAfter != totalBefore-1 {
		t.Fatalf("expected %d vectors after remove, got %d", totalBefore-1, totalAfter)
	}

	// Search should no longer return removed vector
	results := idx.Search(vectors["vec-0"], 200, 0.0)
	for _, r := range results {
		if r.MemoryID == "vec-0" {
			t.Error("removed vector still in search results")
		}
	}
}

// TestIVFSmallDatasetSkipsTraining verifies that IVF doesn't train on small datasets.
func TestIVFSmallDatasetSkipsTraining(t *testing.T) {
	vectors := generateClusteredVectors(10, 16, 2) // below MinPointsToTrain

	idx := ledger.NewIVFIndex(ledger.DefaultIVFConfig())
	idx.Train(vectors)

	if idx.Trained() {
		t.Error("IVF should not train on small datasets")
	}

	// Search should return nil (falls back to brute-force in VectorIndex)
	results := idx.Search(make([]float32, 16), 5, 0.0)
	if results != nil {
		t.Error("expected nil results for untrained index")
	}
}

// ── helpers ──

func generateClusteredVectors(n, dims, numClusters int) map[string][]float32 {
	rng := rand.New(rand.NewSource(42))
	vectors := make(map[string][]float32, n)

	// Generate cluster centers
	centers := make([][]float32, numClusters)
	for c := 0; c < numClusters; c++ {
		center := make([]float32, dims)
		for d := range center {
			center[d] = rng.Float32()*2 - 1
		}
		// Normalize
		var norm float32
		for _, v := range center {
			norm += v * v
		}
		norm = float32(math.Sqrt(float64(norm)))
		for d := range center {
			center[d] /= norm
		}
		centers[c] = center
	}

	// Generate vectors around cluster centers
	for i := 0; i < n; i++ {
		c := i % numClusters
		vec := make([]float32, dims)
		for d := range vec {
			vec[d] = centers[c][d] + rng.Float32()*0.2 - 0.1 // small perturbation
		}
		// Normalize
		var norm float32
		for _, v := range vec {
			norm += v * v
		}
		norm = float32(math.Sqrt(float64(norm)))
		if norm > 0 {
			for d := range vec {
				vec[d] /= norm
			}
		}
		vectors[fmt.Sprintf("vec-%d", i)] = vec
	}

	return vectors
}

func bruteForceTopK(query []float32, vectors map[string][]float32, k int) []string {
	type scored struct {
		id    string
		score float64
	}
	var all []scored
	for id, vec := range vectors {
		all = append(all, scored{id: id, score: ledger.CosineSimilarity(query, vec)})
	}
	// Sort descending
	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			if all[j].score > all[i].score {
				all[i], all[j] = all[j], all[i]
			}
		}
	}
	var result []string
	for i := 0; i < k && i < len(all); i++ {
		result = append(result, all[i].id)
	}
	return result
}
