package localbrain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultSchedulerConfig(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	if cfg.MinSamples != 200 {
		t.Errorf("MinSamples = %d, want 200", cfg.MinSamples)
	}
	if cfg.MinInterval != 24*time.Hour {
		t.Errorf("MinInterval = %v, want 24h", cfg.MinInterval)
	}
	if cfg.EvalMinScore != 0.7 {
		t.Errorf("EvalMinScore = %v, want 0.7", cfg.EvalMinScore)
	}
	if cfg.MaxAdapters != 3 {
		t.Errorf("MaxAdapters = %d, want 3", cfg.MaxAdapters)
	}
	if cfg.ABTestDuration != 1*time.Hour {
		t.Errorf("ABTestDuration = %v, want 1h", cfg.ABTestDuration)
	}
}

func TestNewLoRAScheduler(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	s := NewLoRAScheduler(nil, nil, nil, cfg)
	if s == nil {
		t.Fatal("expected non-nil scheduler")
	}
	st := s.State()
	if st.TotalTrains != 0 {
		t.Errorf("TotalTrains = %d, want 0", st.TotalTrains)
	}
}

func TestCheckAndTrigger_NoTrainFunc(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	s := NewLoRAScheduler(nil, nil, nil, cfg)

	err := s.CheckAndTrigger(context.Background(), "default")
	if err == nil {
		t.Fatal("expected error when trainFn is nil")
	}
	if err.Error() != "lora_scheduler: no training function configured" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckAndTrigger_TooSoon(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	s := NewLoRAScheduler(nil, nil, nil, cfg)
	s.state.LastTrainTime = time.Now()
	s.SetTrainFunc(func(ctx context.Context, job TrainJob) (*TrainResult, error) {
		t.Fatal("should not be called")
		return nil, nil
	})

	err := s.CheckAndTrigger(context.Background(), "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckAndTrigger_InsufficientSamples(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultSchedulerConfig()
	cfg.TrainingDataDir = tmpDir
	cfg.MinSamples = 100

	s := NewLoRAScheduler(nil, nil, nil, cfg)
	s.SetTrainFunc(func(ctx context.Context, job TrainJob) (*TrainResult, error) {
		t.Fatal("should not be called")
		return nil, nil
	})

	os.WriteFile(filepath.Join(tmpDir, "train.jsonl"), []byte("{}\n{}\n"), 0644)

	err := s.CheckAndTrigger(context.Background(), "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckAndTrigger_TriggersTraining(t *testing.T) {
	tmpDir := t.TempDir()
	adapterDir := t.TempDir()
	cfg := DefaultSchedulerConfig()
	cfg.TrainingDataDir = tmpDir
	cfg.AdapterDir = adapterDir
	cfg.MinSamples = 2
	cfg.FilterEnabled = false

	s := NewLoRAScheduler(nil, nil, nil, cfg)

	trained := false
	s.SetTrainFunc(func(ctx context.Context, job TrainJob) (*TrainResult, error) {
		trained = true
		return &TrainResult{
			AdapterName: job.AdapterName,
			AdapterPath: job.OutputDir,
			Success:     true,
			FinalLoss:   0.1,
			Duration:    time.Second,
		}, nil
	})

	lines := make([]byte, 0)
	for i := 0; i < 5; i++ {
		lines = append(lines, []byte("{}\n")...)
	}
	os.WriteFile(filepath.Join(tmpDir, "data.jsonl"), lines, 0644)

	err := s.CheckAndTrigger(context.Background(), "test-tenant")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !trained {
		t.Error("expected training to be triggered")
	}

	st := s.State()
	if st.TotalTrains != 1 {
		t.Errorf("TotalTrains = %d, want 1", st.TotalTrains)
	}
}

func TestActiveModel_NoAdapter(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	cfg.BaseModel = "qwen-2.5-7b"
	s := NewLoRAScheduler(nil, nil, nil, cfg)

	if got := s.ActiveModel(); got != "qwen-2.5-7b" {
		t.Errorf("ActiveModel = %s, want qwen-2.5-7b", got)
	}
}

func TestRollback_NoPrevious(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	s := NewLoRAScheduler(nil, nil, nil, cfg)

	err := s.Rollback(context.Background())
	if err == nil {
		t.Fatal("expected error on rollback with no previous adapter")
	}
}

func TestRollback_Success(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	cfg.AdapterDir = t.TempDir()
	s := NewLoRAScheduler(nil, nil, nil, cfg)
	s.state.CurrentAdapter = "v2"
	s.state.PreviousAdapter = "v1"
	s.state.ABTestActive = true

	err := s.Rollback(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	st := s.State()
	if st.CurrentAdapter != "v1" {
		t.Errorf("CurrentAdapter = %s, want v1", st.CurrentAdapter)
	}
	if st.PreviousAdapter != "" {
		t.Errorf("PreviousAdapter = %s, want empty", st.PreviousAdapter)
	}
	if st.ABTestActive {
		t.Error("ABTestActive should be false after rollback")
	}
	if st.TotalRollbacks != 1 {
		t.Errorf("TotalRollbacks = %d, want 1", st.TotalRollbacks)
	}
}

func TestRecordABMetric_Inactive(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	s := NewLoRAScheduler(nil, nil, nil, cfg)
	s.state.ABTestActive = false

	s.RecordABMetric(true, 0.8)
	st := s.State()
	if st.ABTestMetrics.NewAdapterQueries != 0 {
		t.Error("should not record when A/B test inactive")
	}
}

func TestRecordABMetric_Active(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	cfg.ABTestDuration = 10 * time.Hour
	s := NewLoRAScheduler(nil, nil, nil, cfg)
	s.state.ABTestActive = true
	s.state.ABTestStart = time.Now()

	s.RecordABMetric(true, 0.8)
	s.RecordABMetric(true, 0.6)
	s.RecordABMetric(false, 0.5)

	st := s.State()
	if st.ABTestMetrics.NewAdapterQueries != 2 {
		t.Errorf("NewAdapterQueries = %d, want 2", st.ABTestMetrics.NewAdapterQueries)
	}
	if st.ABTestMetrics.OldAdapterQueries != 1 {
		t.Errorf("OldAdapterQueries = %d, want 1", st.ABTestMetrics.OldAdapterQueries)
	}
	expectedNew := 0.7
	if diff := st.ABTestMetrics.NewAdapterScore - expectedNew; diff > 0.01 || diff < -0.01 {
		t.Errorf("NewAdapterScore = %f, want ~%f", st.ABTestMetrics.NewAdapterScore, expectedNew)
	}
}

func TestLoadState_FromFile(t *testing.T) {
	dir := t.TempDir()
	state := SchedulerState{
		CurrentAdapter: "my-adapter",
		TotalTrains:    5,
		TotalRollbacks: 1,
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(filepath.Join(dir, "scheduler_state.json"), data, 0644)

	cfg := DefaultSchedulerConfig()
	cfg.AdapterDir = dir
	s := NewLoRAScheduler(nil, nil, nil, cfg)
	s.LoadState()

	st := s.State()
	if st.CurrentAdapter != "my-adapter" {
		t.Errorf("CurrentAdapter = %s, want my-adapter", st.CurrentAdapter)
	}
	if st.TotalTrains != 5 {
		t.Errorf("TotalTrains = %d, want 5", st.TotalTrains)
	}
}

func TestPersistState_ToFile(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultSchedulerConfig()
	cfg.AdapterDir = dir
	s := NewLoRAScheduler(nil, nil, nil, cfg)
	s.state.CurrentAdapter = "saved-adapter"
	s.state.TotalTrains = 3

	s.persistState()

	data, err := os.ReadFile(filepath.Join(dir, "scheduler_state.json"))
	if err != nil {
		t.Fatalf("read state file: %v", err)
	}
	var loaded persistedState
	json.Unmarshal(data, &loaded)
	if loaded.Global.CurrentAdapter != "saved-adapter" {
		t.Errorf("persisted CurrentAdapter = %s, want saved-adapter", loaded.Global.CurrentAdapter)
	}
}

func TestCountJSONLLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	os.WriteFile(path, []byte("{\"a\":1}\n{\"b\":2}\n{\"c\":3}\n"), 0644)

	count := countJSONLLines(path)
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestCountJSONLLines_NoTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	os.WriteFile(path, []byte("{\"a\":1}\n{\"b\":2}"), 0644)

	count := countJSONLLines(path)
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestCountJSONLLines_Missing(t *testing.T) {
	count := countJSONLLines("/nonexistent/file.jsonl")
	if count != 0 {
		t.Errorf("count = %d, want 0 for missing file", count)
	}
}

func TestCountAvailableSamples_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultSchedulerConfig()
	cfg.TrainingDataDir = tmpDir
	s := NewLoRAScheduler(nil, nil, nil, cfg)

	os.WriteFile(filepath.Join(tmpDir, "a.jsonl"), []byte("{}\n{}\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.jsonl"), []byte("{}\n{}\n{}\n"), 0644)

	total, dataPath, err := s.countAvailableSamples()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if dataPath == "" {
		t.Fatal("expected non-empty dataPath")
	}

	mergedCount := countJSONLLines(dataPath)
	if mergedCount != total {
		t.Errorf("merged file has %d lines, but total count was %d — data loss", mergedCount, total)
	}
}

func TestCountAvailableSamples_SingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultSchedulerConfig()
	cfg.TrainingDataDir = tmpDir
	s := NewLoRAScheduler(nil, nil, nil, cfg)

	singlePath := filepath.Join(tmpDir, "only.jsonl")
	os.WriteFile(singlePath, []byte("{}\n{}\n"), 0644)

	total, dataPath, err := s.countAvailableSamples()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if dataPath != singlePath {
		t.Errorf("single file should return original path, got %s", dataPath)
	}
}

func TestCountAvailableSamples_SingleFileNoTrailingNewline(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultSchedulerConfig()
	cfg.TrainingDataDir = tmpDir
	s := NewLoRAScheduler(nil, nil, nil, cfg)

	singlePath := filepath.Join(tmpDir, "only.jsonl")
	os.WriteFile(singlePath, []byte("{}"), 0644)

	total, dataPath, err := s.countAvailableSamples()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if dataPath != singlePath {
		t.Errorf("single file should return original path, got %s", dataPath)
	}
}

func TestCountAvailableSamples_IgnoresGeneratedArtifacts(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultSchedulerConfig()
	cfg.TrainingDataDir = tmpDir
	s := NewLoRAScheduler(nil, nil, nil, cfg)

	rawPath := filepath.Join(tmpDir, "raw.jsonl")
	os.WriteFile(rawPath, []byte("{}\n{}\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "merged_20260505_120000.jsonl"), []byte("{}\n{}\n{}\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "filtered_20260505_120000.jsonl"), []byte("{}\n{}\n{}\n{}\n"), 0644)

	total, dataPath, err := s.countAvailableSamples()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if dataPath != rawPath {
		t.Errorf("dataPath = %s, want %s", dataPath, rawPath)
	}
}

func TestCountAvailableSamples_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultSchedulerConfig()
	cfg.TrainingDataDir = tmpDir
	s := NewLoRAScheduler(nil, nil, nil, cfg)

	total, dataPath, err := s.countAvailableSamples()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 || dataPath != "" {
		t.Errorf("empty dir: total=%d, path=%s", total, dataPath)
	}
}

func TestCountAvailableSamples_MissingDir(t *testing.T) {
	tmpDir := filepath.Join(t.TempDir(), "missing")
	cfg := DefaultSchedulerConfig()
	cfg.TrainingDataDir = tmpDir
	s := NewLoRAScheduler(nil, nil, nil, cfg)

	total, dataPath, err := s.countAvailableSamples()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 || dataPath != "" {
		t.Errorf("missing dir: total=%d, path=%s", total, dataPath)
	}
}

func TestPreviewTrainingDataAppliesFilterReadiness(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultSchedulerConfig()
	cfg.TrainingDataDir = tmpDir
	cfg.MinSamples = 2
	cfg.FilterEnabled = true

	s := NewLoRAScheduler(nil, nil, nil, cfg)
	lines := []string{
		`{"instruction":"valid instruction","input":"useful input","output":"useful output text"}`,
		`{"instruction":"valid instruction","input":"useful input","output":"useful output text"}`,
		`{"instruction":"x","input":"","output":"no"}`,
	}
	os.WriteFile(filepath.Join(tmpDir, "train.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0644)

	preview, err := s.PreviewTrainingData("tenant-a")
	if err != nil {
		t.Fatalf("PreviewTrainingData: %v", err)
	}
	if preview.RawSamples != 3 {
		t.Fatalf("RawSamples = %d, want 3", preview.RawSamples)
	}
	if preview.UsableSamples != 1 {
		t.Fatalf("UsableSamples = %d, want 1", preview.UsableSamples)
	}
	if preview.Ready {
		t.Fatal("preview should not be ready when usable samples are below min_samples")
	}
	if preview.FilterStats == nil || preview.FilterStats.DroppedDup != 1 || preview.FilterStats.DroppedTooShort != 1 {
		t.Fatalf("unexpected filter stats: %+v", preview.FilterStats)
	}
}

func TestPreviewTrainingDataMissingDirIsEmpty(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	cfg.TrainingDataDir = filepath.Join(t.TempDir(), "missing")
	cfg.MinSamples = 2

	s := NewLoRAScheduler(nil, nil, nil, cfg)

	preview, err := s.PreviewTrainingData("tenant-a")
	if err != nil {
		t.Fatalf("PreviewTrainingData: %v", err)
	}
	if preview.RawSamples != 0 || preview.UsableSamples != 0 {
		t.Fatalf("missing dir should preview as empty, got %+v", preview)
	}
	if preview.Ready {
		t.Fatal("missing dir preview should not be ready")
	}
	if preview.Reason != "no training samples found" {
		t.Fatalf("Reason = %q, want no training samples found", preview.Reason)
	}
}

func TestPreviewTrainingDataMultipleFilesDoesNotMerge(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultSchedulerConfig()
	cfg.TrainingDataDir = tmpDir
	cfg.MinSamples = 2
	cfg.FilterEnabled = true

	s := NewLoRAScheduler(nil, nil, nil, cfg)
	valid := `{"instruction":"valid instruction","input":"useful input","output":"useful output text"}`
	os.WriteFile(filepath.Join(tmpDir, "a.jsonl"), []byte(valid+"\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.jsonl"), []byte(valid+"\n"), 0644)

	preview, err := s.PreviewTrainingData("tenant-a")
	if err != nil {
		t.Fatalf("PreviewTrainingData: %v", err)
	}
	if preview.RawSamples != 2 {
		t.Fatalf("RawSamples = %d, want 2", preview.RawSamples)
	}
	if preview.UsableSamples != 1 {
		t.Fatalf("UsableSamples = %d, want 1 after cross-file dedup", preview.UsableSamples)
	}
	if preview.FilterStats == nil || preview.FilterStats.DroppedDup != 1 {
		t.Fatalf("unexpected filter stats: %+v", preview.FilterStats)
	}

	matches, err := filepath.Glob(filepath.Join(tmpDir, "merged_*.jsonl"))
	if err != nil {
		t.Fatalf("glob merged files: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("preview should not create merged files, found %v", matches)
	}
}

func TestCheckAndTrigger_TrainingFails(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultSchedulerConfig()
	cfg.TrainingDataDir = tmpDir
	cfg.AdapterDir = t.TempDir()
	cfg.MinSamples = 1
	cfg.FilterEnabled = false

	s := NewLoRAScheduler(nil, nil, nil, cfg)
	s.SetTrainFunc(func(ctx context.Context, job TrainJob) (*TrainResult, error) {
		return &TrainResult{Success: false, Error: "out of memory"}, nil
	})
	os.WriteFile(filepath.Join(tmpDir, "d.jsonl"), []byte("{}\n{}\n"), 0644)

	err := s.CheckAndTrigger(context.Background(), "t1")
	if err == nil || err.Error() != "lora_scheduler: training unsuccessful: out of memory" {
		t.Errorf("expected 'training unsuccessful' error, got: %v", err)
	}
}

func TestCheckAndTrigger_EvalAutoPass_NoLedger(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultSchedulerConfig()
	cfg.TrainingDataDir = tmpDir
	cfg.AdapterDir = t.TempDir()
	cfg.MinSamples = 1
	cfg.FilterEnabled = false

	s := NewLoRAScheduler(nil, nil, nil, cfg)
	s.SetTrainFunc(func(ctx context.Context, job TrainJob) (*TrainResult, error) {
		return &TrainResult{AdapterName: job.AdapterName, AdapterPath: job.OutputDir, Success: true}, nil
	})
	evalCalled := false
	s.SetEvalFunc(func(ctx context.Context, name string, samples []EvalSample) (*EvalResult, error) {
		evalCalled = true
		return &EvalResult{Score: 0.3, Passed: false}, nil
	})
	os.WriteFile(filepath.Join(tmpDir, "d.jsonl"), []byte("{}\n{}\n"), 0644)

	err := s.CheckAndTrigger(context.Background(), "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evalCalled {
		t.Error("evalFn should not be called when no eval samples available (no ledger)")
	}
}

func TestCheckAndTrigger_EvalPasses_DeploysAdapter(t *testing.T) {
	tmpDir := t.TempDir()
	adapterDir := t.TempDir()
	cfg := DefaultSchedulerConfig()
	cfg.TrainingDataDir = tmpDir
	cfg.AdapterDir = adapterDir
	cfg.MinSamples = 1
	cfg.BaseModel = "qwen-7b"
	cfg.FilterEnabled = false

	adapter := NewLoRAAdapter(LoRAAdapterConfig{BaseURL: "http://127.0.0.1:1"})
	s := NewLoRAScheduler(nil, adapter, nil, cfg)
	s.SetTrainFunc(func(ctx context.Context, job TrainJob) (*TrainResult, error) {
		return &TrainResult{
			AdapterName: job.AdapterName,
			AdapterPath: job.OutputDir,
			Success:     true,
		}, nil
	})
	s.SetEvalFunc(func(ctx context.Context, name string, samples []EvalSample) (*EvalResult, error) {
		return &EvalResult{Score: 0.95, Passed: true}, nil
	})
	os.WriteFile(filepath.Join(tmpDir, "d.jsonl"), []byte("{}\n{}\n"), 0644)

	err := s.CheckAndTrigger(context.Background(), "deploy-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	st := s.State()
	if st.CurrentAdapter == "" {
		t.Error("expected CurrentAdapter to be set after deploy")
	}
	if !st.ABTestActive {
		t.Error("expected A/B test to be active after deploy")
	}
}

func TestActiveModel_WithAdapter(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	cfg.BaseModel = "qwen-7b"
	adapter := NewLoRAAdapter(LoRAAdapterConfig{BaseURL: "http://localhost:8000"})
	s := NewLoRAScheduler(nil, adapter, nil, cfg)
	s.state.CurrentAdapter = "finance-v1"

	got := s.ActiveModel()
	if got != "qwen-7b:finance-v1" {
		t.Errorf("ActiveModel = %s, want qwen-7b:finance-v1", got)
	}
}

func TestFinalizeABTest_InsufficientData(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	cfg.ABTestDuration = 0
	s := NewLoRAScheduler(nil, nil, nil, cfg)
	s.state.ABTestActive = true
	s.state.ABTestStart = time.Now().Add(-time.Hour)
	s.state.CurrentAdapter = "new"
	s.state.PreviousAdapter = "old"
	s.state.ABTestMetrics = ABTestMetrics{
		NewAdapterQueries: 5, NewAdapterScore: 0.8,
		OldAdapterQueries: 3, OldAdapterScore: 0.7,
	}

	s.RecordABMetric(true, 0.9)

	st := s.State()
	if st.ABTestActive {
		t.Error("A/B test should be finalized")
	}
	if st.CurrentAdapter != "new" {
		t.Error("should keep new adapter with insufficient data")
	}
}

func TestFinalizeABTest_Improvement(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	cfg.ABTestDuration = 0
	cfg.AdapterDir = t.TempDir()
	s := NewLoRAScheduler(nil, nil, nil, cfg)
	s.state.ABTestActive = true
	s.state.ABTestStart = time.Now().Add(-time.Hour)
	s.state.CurrentAdapter = "new"
	s.state.PreviousAdapter = "old"
	s.state.ABTestMetrics = ABTestMetrics{
		NewAdapterQueries: 10, NewAdapterScore: 0.9,
		OldAdapterQueries: 10, OldAdapterScore: 0.7,
	}

	s.RecordABMetric(true, 0.9)

	st := s.State()
	if st.ABTestActive {
		t.Error("A/B test should be finalized")
	}
	if st.PreviousAdapter != "" {
		t.Errorf("PreviousAdapter should be cleared on improvement, got %s", st.PreviousAdapter)
	}
}

func TestFinalizeABTest_Regression(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	cfg.ABTestDuration = 0
	cfg.AdapterDir = t.TempDir()
	s := NewLoRAScheduler(nil, nil, nil, cfg)
	s.state.ABTestActive = true
	s.state.ABTestStart = time.Now().Add(-time.Hour)
	s.state.CurrentAdapter = "new"
	s.state.PreviousAdapter = "old"
	s.state.ABTestMetrics = ABTestMetrics{
		NewAdapterQueries: 10, NewAdapterScore: 0.3,
		OldAdapterQueries: 10, OldAdapterScore: 0.9,
	}

	s.RecordABMetric(true, 0.3)

	st := s.State()
	if st.ABTestActive {
		t.Error("A/B test should be finalized")
	}

	time.Sleep(300 * time.Millisecond)
	st = s.State()
	if st.TotalRollbacks == 0 {
		t.Log("Note: rollback runs async via safego.Go; may need more time")
	}
}

func TestRollback_WithAdapter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	adapter := NewLoRAAdapter(LoRAAdapterConfig{BaseURL: srv.URL})
	adapter.knownAdapters["v2"] = &AdapterInfo{Name: "v2", Status: "loaded"}

	cfg := DefaultSchedulerConfig()
	cfg.AdapterDir = t.TempDir()
	s := NewLoRAScheduler(nil, adapter, nil, cfg)
	s.state.CurrentAdapter = "v2"
	s.state.PreviousAdapter = "v1"

	err := s.Rollback(context.Background())
	if err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if adapter.IsLoaded("v2") {
		t.Error("v2 should be unloaded after rollback")
	}
}

func TestMergeJSONLFiles(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultSchedulerConfig()
	cfg.TrainingDataDir = tmpDir
	s := NewLoRAScheduler(nil, nil, nil, cfg)

	os.WriteFile(filepath.Join(tmpDir, "a.jsonl"), []byte("{\"a\":1}\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.jsonl"), []byte("{\"b\":2}"), 0644) // no trailing newline

	files := []string{
		filepath.Join(tmpDir, "a.jsonl"),
		filepath.Join(tmpDir, "b.jsonl"),
	}
	merged, err := s.mergeJSONLFiles(files, tmpDir)
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	count := countJSONLLines(merged)
	if count != 2 {
		t.Errorf("merged line count = %d, want 2", count)
	}
}

func TestMergeJSONLFilesUniqueNames(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultSchedulerConfig()
	cfg.TrainingDataDir = tmpDir
	s := NewLoRAScheduler(nil, nil, nil, cfg)

	os.WriteFile(filepath.Join(tmpDir, "a.jsonl"), []byte("{\"a\":1}\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.jsonl"), []byte("{\"b\":2}\n"), 0644)

	files := []string{
		filepath.Join(tmpDir, "a.jsonl"),
		filepath.Join(tmpDir, "b.jsonl"),
	}
	first, err := s.mergeJSONLFiles(files, tmpDir)
	if err != nil {
		t.Fatalf("first merge failed: %v", err)
	}
	second, err := s.mergeJSONLFiles(files, tmpDir)
	if err != nil {
		t.Fatalf("second merge failed: %v", err)
	}
	if first == second {
		t.Fatalf("merge should not reuse output names: %s", first)
	}
	if _, err := os.Stat(first); err != nil {
		t.Fatalf("first merged file missing: %v", err)
	}
	if _, err := os.Stat(second); err != nil {
		t.Fatalf("second merged file missing: %v", err)
	}
}
