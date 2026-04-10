package world

import (
	"testing"
	"time"
)

func TestModelUpdateAndGet(t *testing.T) {
	wm := &Model{state: make(map[string]*State)}

	wm.Update("config.db_host", KindConfig, "localhost", "planner", 0.9)

	s, found := wm.Get("config.db_host")
	if !found {
		t.Fatal("expected state")
	}
	if s.Value != "localhost" {
		t.Errorf("value = %s, want localhost", s.Value)
	}
	if s.Kind != KindConfig {
		t.Errorf("kind = %s, want config", s.Kind)
	}
	if s.Confidence != 0.9 {
		t.Errorf("confidence = %v, want 0.9", s.Confidence)
	}
}

func TestModelGetNotFound(t *testing.T) {
	wm := &Model{state: make(map[string]*State)}
	_, found := wm.Get("nonexistent")
	if found {
		t.Error("expected not found")
	}
}

func TestModelGetByKind(t *testing.T) {
	wm := &Model{state: make(map[string]*State)}
	wm.Update("file1.txt", KindFile, "content1", "tools", 0.8)
	wm.Update("file2.txt", KindFile, "content2", "tools", 0.8)
	wm.Update("config.host", KindConfig, "localhost", "planner", 0.9)

	files := wm.GetByKind(KindFile)
	if len(files) != 2 {
		t.Errorf("file count = %d, want 2", len(files))
	}

	configs := wm.GetByKind(KindConfig)
	if len(configs) != 1 {
		t.Errorf("config count = %d, want 1", len(configs))
	}
}

func TestModelSnapshot(t *testing.T) {
	wm := &Model{state: make(map[string]*State)}
	wm.Update("key1", KindCustom, "val1", "test", 0.5)
	wm.Update("key2", KindCustom, "val2", "test", 0.5)

	snap := wm.Snapshot()
	if len(snap) != 2 {
		t.Errorf("snapshot len = %d, want 2", len(snap))
	}

	// Modifying snapshot should not affect original
	snap["key1"].Value = "modified"
	original, _ := wm.Get("key1")
	if original.Value != "val1" {
		t.Error("snapshot modification should not affect original")
	}
}

func TestPredictImpactSimple(t *testing.T) {
	wm := &Model{state: make(map[string]*State)}
	wm.Update("file.txt", KindFile, "old content", "test", 0.9)

	pred := wm.PredictImpact("write_file", []string{"file.txt"})
	if pred.Action != "write_file" {
		t.Errorf("action = %s, want write_file", pred.Action)
	}
	if len(pred.Predictions) != 1 {
		t.Errorf("predictions len = %d, want 1", len(pred.Predictions))
	}
	if pred.Predictions[0].CurrentValue != "old content" {
		t.Errorf("current value = %s, want old content", pred.Predictions[0].CurrentValue)
	}
}

func TestPredictImpactWithDependencies(t *testing.T) {
	wm := &Model{state: make(map[string]*State)}
	wm.state["app.config"] = &State{
		Key: "app.config", Kind: KindConfig, Value: "v1",
		Confidence: 0.9, Dependencies: []string{"app.server"},
	}
	wm.state["app.server"] = &State{
		Key: "app.server", Kind: KindProcess, Value: "running", Confidence: 0.8,
	}

	pred := wm.PredictImpact("update_config", []string{"app.config"})
	if pred.RiskLevel != "medium" {
		t.Errorf("risk = %s, want medium (has dependency)", pred.RiskLevel)
	}
}

func TestPredictImpactHighRisk(t *testing.T) {
	wm := &Model{state: make(map[string]*State)}
	keys := []string{}
	for i := 0; i < 10; i++ {
		key := "key" + string(rune('0'+i))
		wm.Update(key, KindCustom, "v", "test", 0.5)
		keys = append(keys, key)
	}

	pred := wm.PredictImpact("mass_update", keys)
	if pred.RiskLevel != "high" {
		t.Errorf("risk = %s, want high (>5 keys)", pred.RiskLevel)
	}
}

func TestStaleKeys(t *testing.T) {
	wm := &Model{state: make(map[string]*State)}

	wm.state["fresh"] = &State{Key: "fresh", LastVerified: time.Now()}
	wm.state["stale"] = &State{Key: "stale", LastVerified: time.Now().Add(-2 * time.Hour)}

	stale := wm.StaleKeys(1 * time.Hour)
	if len(stale) != 1 {
		t.Errorf("stale count = %d, want 1", len(stale))
	}
	if stale[0] != "stale" {
		t.Errorf("stale key = %s, want stale", stale[0])
	}
}

// ── Predictor tests ──

func TestNewPredictor(t *testing.T) {
	p := NewPredictor(nil, nil)
	if p == nil {
		t.Fatal("expected non-nil predictor")
	}
}

func TestPredictorRecordAndAccuracy(t *testing.T) {
	wm := &Model{state: make(map[string]*State)}
	p := NewPredictor(wm, nil)

	// Simulate a prediction first, then record outcome
	p.mu.Lock()
	p.history = append(p.history, PredictionRecord{Action: "act1"})
	p.history = append(p.history, PredictionRecord{Action: "act2"})
	p.mu.Unlock()

	p.RecordOutcome("act1", "good", true)
	p.RecordOutcome("act2", "bad", false)

	acc := p.Accuracy()
	if acc != 0.5 {
		t.Errorf("accuracy = %v, want 0.5", acc)
	}
}

func TestPredictorAccuracyNoData(t *testing.T) {
	wm := &Model{state: make(map[string]*State)}
	p := NewPredictor(wm, nil)
	acc := p.Accuracy()
	if acc != 0.0 {
		t.Errorf("accuracy = %v, want 0.0 (no data)", acc)
	}
}
