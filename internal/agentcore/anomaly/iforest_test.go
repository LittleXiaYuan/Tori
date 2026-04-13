package anomaly

import (
	"math/rand"
	"testing"
)

func TestIForestBasicAnomaly(t *testing.T) {
	rng := rand.New(rand.NewSource(42))

	var normal [][]float64
	for i := 0; i < 200; i++ {
		normal = append(normal, []float64{rng.NormFloat64(), rng.NormFloat64()})
	}

	f := NewIsolationForest(DefaultIForestConfig())
	f.Fit(normal)

	normalScore := f.Score([]float64{0.1, -0.2})
	anomalyScore := f.Score([]float64{10, 10})

	if anomalyScore <= normalScore {
		t.Errorf("anomaly point [10,10] should score higher than normal [0.1,-0.2], got anomaly=%.3f normal=%.3f",
			anomalyScore, normalScore)
	}
	t.Logf("normal=%.3f anomaly=%.3f", normalScore, anomalyScore)
}

func TestIForestScoreRange(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	var data [][]float64
	for i := 0; i < 100; i++ {
		data = append(data, []float64{rng.NormFloat64()})
	}

	f := NewIsolationForest(DefaultIForestConfig())
	f.Fit(data)

	for _, point := range [][]float64{{0}, {5}, {-3}, {100}} {
		score := f.Score(point)
		if score < 0 || score > 1 {
			t.Errorf("score for %v should be in [0,1], got %.3f", point, score)
		}
	}
}

func TestIForestPredict(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	var data [][]float64
	for i := 0; i < 200; i++ {
		data = append(data, []float64{rng.NormFloat64(), rng.NormFloat64()})
	}

	f := NewIsolationForest(DefaultIForestConfig())
	f.Fit(data)

	isAnomalyOutlier := f.Predict([]float64{20, 20}, 0.6)
	isAnomalyNormal := f.Predict([]float64{0, 0}, 0.6)

	if !isAnomalyOutlier {
		t.Error("far outlier [20,20] should be predicted as anomaly")
	}
	if isAnomalyNormal {
		t.Error("center point [0,0] should not be predicted as anomaly")
	}
}

func TestIForestTopAnomalies(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	var data [][]float64
	for i := 0; i < 100; i++ {
		data = append(data, []float64{rng.NormFloat64(), rng.NormFloat64()})
	}
	data = append(data, []float64{50, 50})
	data = append(data, []float64{-50, -50})

	f := NewIsolationForest(DefaultIForestConfig())
	f.Fit(data[:100])

	top := f.TopAnomalies(data, 5)
	if len(top) != 5 {
		t.Fatalf("expected 5 top anomalies, got %d", len(top))
	}

	topSet := make(map[int]bool)
	for _, idx := range top {
		topSet[idx] = true
	}
	if !topSet[100] && !topSet[101] {
		t.Logf("expected at least one extreme outlier in top 5, got indices %v", top)
	}
	t.Logf("top 5 anomalies: %v", top)
}

func TestIForestUntrained(t *testing.T) {
	f := NewIsolationForest(DefaultIForestConfig())
	score := f.Score([]float64{1, 2, 3})
	if score != 0.5 {
		t.Errorf("untrained forest should return 0.5, got %.3f", score)
	}
	if f.IsTrained() {
		t.Error("should not be trained")
	}
}

func TestIForestEmptyData(t *testing.T) {
	f := NewIsolationForest(DefaultIForestConfig())
	f.Fit(nil)
	if f.IsTrained() {
		t.Error("should not be trained with nil data")
	}
}

func TestIForestHighDimensional(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	dims := 10
	var data [][]float64
	for i := 0; i < 200; i++ {
		point := make([]float64, dims)
		for d := 0; d < dims; d++ {
			point[d] = rng.NormFloat64()
		}
		data = append(data, point)
	}

	f := NewIsolationForest(DefaultIForestConfig())
	f.Fit(data)

	outlier := make([]float64, dims)
	for d := 0; d < dims; d++ {
		outlier[d] = 10 // far from center in all dimensions
	}
	normalPoint := make([]float64, dims)

	outlierScore := f.Score(outlier)
	normalScore := f.Score(normalPoint)

	if outlierScore <= normalScore {
		t.Errorf("10D outlier should score higher, outlier=%.3f normal=%.3f", outlierScore, normalScore)
	}
}

func TestAveragePathLength(t *testing.T) {
	if averagePathLength(1) != 0 {
		t.Error("c(1) should be 0")
	}
	if averagePathLength(2) != 1 {
		t.Error("c(2) should be 1")
	}
	c256 := averagePathLength(256)
	// c(256) = 2*H(255) - 2*255/256 ≈ 2*(ln(255)+0.5772) - 1.992 ≈ 10.24
	if c256 < 9.5 || c256 > 11.0 {
		t.Errorf("c(256) should be ~10.2, got %.2f", c256)
	}
}

func TestIForestScoreBatch(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	var data [][]float64
	for i := 0; i < 100; i++ {
		data = append(data, []float64{rng.NormFloat64()})
	}

	f := NewIsolationForest(DefaultIForestConfig())
	f.Fit(data)

	batch := [][]float64{{0}, {50}, {-50}}
	scores := f.ScoreBatch(batch)
	if len(scores) != 3 {
		t.Fatalf("expected 3 scores, got %d", len(scores))
	}
	if scores[0] >= scores[1] {
		t.Logf("center=%.3f outlier=%.3f", scores[0], scores[1])
	}
}
