// Package anomaly provides unsupervised anomaly detection algorithms.
//
// The primary algorithm is Isolation Forest (Liu, Ting, Zhou, 2008):
//   "Isolation-Based Anomaly Detection", ACM TKDD 6(1), 2012.
//   Original: IEEE ICDM 2008.
//
// Key insight: anomalies are "few and different", so they are easier to isolate
// in random binary trees — requiring fewer splits on average. The anomaly score
// is derived from the average path length across an ensemble of random trees.
package anomaly

import (
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"
)

// IsolationForest is an ensemble of isolation trees for anomaly detection.
type IsolationForest struct {
	mu         sync.RWMutex
	trees      []*iTree
	nTrees     int
	sampleSize int
	trained    bool
	rng        *rand.Rand
}

// IForestConfig configures the isolation forest.
type IForestConfig struct {
	NumTrees   int // number of isolation trees (default: 100)
	SampleSize int // subsample size per tree (default: 256)
}

// DefaultIForestConfig returns standard parameters from the original paper.
func DefaultIForestConfig() IForestConfig {
	return IForestConfig{
		NumTrees:   100,
		SampleSize: 256,
	}
}

// NewIsolationForest creates an untrained isolation forest.
func NewIsolationForest(cfg IForestConfig) *IsolationForest {
	if cfg.NumTrees <= 0 {
		cfg.NumTrees = 100
	}
	if cfg.SampleSize <= 0 {
		cfg.SampleSize = 256
	}
	return &IsolationForest{
		nTrees:     cfg.NumTrees,
		sampleSize: cfg.SampleSize,
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Fit trains the forest on a dataset. Each row is a feature vector.
// Call this with historical "normal" data to establish a baseline.
func (f *IsolationForest) Fit(data [][]float64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(data) == 0 {
		return
	}

	heightLimit := int(math.Ceil(math.Log2(float64(f.sampleSize))))
	f.trees = make([]*iTree, f.nTrees)

	for i := 0; i < f.nTrees; i++ {
		sample := f.subsample(data)
		f.trees[i] = buildITree(sample, 0, heightLimit, f.rng)
	}
	f.trained = true
}

// Score computes the anomaly score for a single observation.
// Returns a value in [0, 1] where:
//   - score close to 1 → anomaly
//   - score close to 0.5 → normal
//   - score close to 0 → very normal (dense region)
//
// Based on Equation 1 in Liu et al. 2008:
//   s(x, n) = 2^(-E(h(x)) / c(n))
// where E(h(x)) is the average path length and c(n) is the expected path length
// of unsuccessful search in a BST.
func (f *IsolationForest) Score(point []float64) float64 {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if !f.trained || len(f.trees) == 0 {
		return 0.5
	}

	var totalPath float64
	for _, tree := range f.trees {
		totalPath += pathLength(point, tree, 0)
	}
	avgPath := totalPath / float64(len(f.trees))

	cn := averagePathLength(float64(f.sampleSize))
	if cn == 0 {
		return 0.5
	}

	return math.Pow(2, -avgPath/cn)
}

// ScoreBatch computes anomaly scores for multiple observations.
func (f *IsolationForest) ScoreBatch(data [][]float64) []float64 {
	scores := make([]float64, len(data))
	for i, point := range data {
		scores[i] = f.Score(point)
	}
	return scores
}

// Predict returns true if the point is classified as an anomaly.
// threshold is typically 0.6-0.7 for moderate sensitivity.
func (f *IsolationForest) Predict(point []float64, threshold float64) bool {
	return f.Score(point) > threshold
}

// TopAnomalies returns the indices of the top-K most anomalous points.
func (f *IsolationForest) TopAnomalies(data [][]float64, k int) []int {
	type scored struct {
		idx   int
		score float64
	}
	results := make([]scored, len(data))
	for i, point := range data {
		results[i] = scored{i, f.Score(point)}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })

	if k > len(results) {
		k = len(results)
	}
	indices := make([]int, k)
	for i := 0; i < k; i++ {
		indices[i] = results[i].idx
	}
	return indices
}

// IsTrained returns whether Fit has been called.
func (f *IsolationForest) IsTrained() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.trained
}

// --- Internal tree structures ---

type iTree struct {
	left       *iTree
	right      *iTree
	splitAttr  int     // feature index used for split
	splitValue float64 // threshold value
	size       int     // number of data points at this node (for leaf adjustment)
	isLeaf     bool
}

// buildITree constructs one isolation tree recursively.
func buildITree(data [][]float64, height, heightLimit int, rng *rand.Rand) *iTree {
	n := len(data)
	if n <= 1 || height >= heightLimit {
		return &iTree{isLeaf: true, size: n}
	}

	nFeatures := len(data[0])
	if nFeatures == 0 {
		return &iTree{isLeaf: true, size: n}
	}

	// Pick a random feature
	attr := rng.Intn(nFeatures)

	// Find min/max for this feature
	minVal, maxVal := data[0][attr], data[0][attr]
	for _, row := range data[1:] {
		if row[attr] < minVal {
			minVal = row[attr]
		}
		if row[attr] > maxVal {
			maxVal = row[attr]
		}
	}

	if minVal == maxVal {
		return &iTree{isLeaf: true, size: n}
	}

	// Random split point between min and max
	splitVal := minVal + rng.Float64()*(maxVal-minVal)

	var left, right [][]float64
	for _, row := range data {
		if row[attr] < splitVal {
			left = append(left, row)
		} else {
			right = append(right, row)
		}
	}

	return &iTree{
		splitAttr:  attr,
		splitValue: splitVal,
		left:       buildITree(left, height+1, heightLimit, rng),
		right:      buildITree(right, height+1, heightLimit, rng),
		size:       n,
	}
}

// pathLength computes the path length for a point in a tree.
func pathLength(point []float64, tree *iTree, currentHeight int) float64 {
	if tree == nil || tree.isLeaf {
		if tree != nil && tree.size > 1 {
			return float64(currentHeight) + averagePathLength(float64(tree.size))
		}
		return float64(currentHeight)
	}

	if tree.splitAttr >= len(point) {
		return float64(currentHeight)
	}

	if point[tree.splitAttr] < tree.splitValue {
		return pathLength(point, tree.left, currentHeight+1)
	}
	return pathLength(point, tree.right, currentHeight+1)
}

// averagePathLength computes c(n), the average path length of unsuccessful
// search in a Binary Search Tree with n elements.
// From Equation 1 in Liu et al.:
//   c(n) = 2*H(n-1) - 2*(n-1)/n
// where H(i) is the harmonic number ≈ ln(i) + 0.5772 (Euler-Mascheroni constant)
func averagePathLength(n float64) float64 {
	if n <= 1 {
		return 0
	}
	if n == 2 {
		return 1
	}
	return 2*(math.Log(n-1)+0.5772156649) - 2*(n-1)/n
}

// subsample draws a random subsample from the data.
func (f *IsolationForest) subsample(data [][]float64) [][]float64 {
	n := len(data)
	size := f.sampleSize
	if size > n {
		size = n
	}

	indices := f.rng.Perm(n)[:size]
	sample := make([][]float64, size)
	for i, idx := range indices {
		sample[i] = data[idx]
	}
	return sample
}
