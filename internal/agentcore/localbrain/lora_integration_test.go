package localbrain

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const ollamaURL = "http://localhost:11434"

func ollamaAvailable() bool {
	resp, err := http.Get(ollamaURL + "/api/tags")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

func TestIntegration_LoRAAdapter_OllamaGracefulDegradation(t *testing.T) {
	if !ollamaAvailable() {
		t.Skip("ollama not running")
	}

	adapter := NewLoRAAdapter(LoRAAdapterConfig{
		BaseURL: ollamaURL,
		Timeout: 5 * time.Second,
	})

	// Ollama doesn't have /v1/lora/load — should fallback to local registry
	err := adapter.Load(context.Background(), "test-lora", "/fake/path", "qwen3.5:4b")
	if err != nil {
		t.Fatalf("Load should degrade gracefully, got error: %v", err)
	}

	info, ok := adapter.knownAdapters["test-lora"]
	if !ok {
		t.Fatal("adapter should be registered locally")
	}
	if info.Status != "local_only" {
		t.Errorf("status = %s, want local_only (graceful degradation)", info.Status)
	}

	// List should also degrade gracefully
	list, err := adapter.List(context.Background())
	if err != nil {
		t.Fatalf("List should not error: %v", err)
	}
	if len(list) == 0 {
		t.Error("should return at least the locally registered adapter")
	}

	// ModelName still works
	name := adapter.ModelName("qwen3.5:4b", "test-lora")
	if name != "qwen3.5:4b:test-lora" {
		t.Errorf("ModelName = %s", name)
	}

	// Unload should also degrade gracefully
	err = adapter.Unload(context.Background(), "test-lora")
	if err != nil {
		t.Fatalf("Unload should degrade gracefully: %v", err)
	}
	if _, ok := adapter.knownAdapters["test-lora"]; ok {
		t.Error("adapter should be removed after Unload")
	}
}

func TestIntegration_LoRAScheduler_EndToEnd(t *testing.T) {
	if !ollamaAvailable() {
		t.Skip("ollama not running")
	}

	tmpTraining := t.TempDir()
	tmpAdapters := t.TempDir()

	// Prepare training data
	var lines []byte
	for i := 0; i < 10; i++ {
		lines = append(lines, []byte("{\"input\":\"test\",\"output\":\"ok\"}\n")...)
	}
	os.WriteFile(filepath.Join(tmpTraining, "train.jsonl"), lines, 0644)

	adapter := NewLoRAAdapter(LoRAAdapterConfig{
		BaseURL: ollamaURL,
		Timeout: 5 * time.Second,
	})

	cfg := SchedulerConfig{
		MinSamples:      5,
		MinInterval:     0,
		EvalMinScore:    0.5,
		MaxAdapters:     3,
		BaseModel:       "qwen3.5:4b",
		TrainingDataDir: tmpTraining,
		AdapterDir:      tmpAdapters,
		ABTestDuration:  1 * time.Hour,
	}

	scheduler := NewLoRAScheduler(nil, adapter, nil, cfg)

	trainCalled := false
	scheduler.SetTrainFunc(func(ctx context.Context, job TrainJob) (*TrainResult, error) {
		trainCalled = true
		if job.BaseModel != "qwen3.5:4b" {
			t.Errorf("BaseModel = %s", job.BaseModel)
		}
		if job.NumEpochs != 3 {
			t.Errorf("NumEpochs = %d", job.NumEpochs)
		}
		return &TrainResult{
			AdapterName: job.AdapterName,
			AdapterPath: job.OutputDir,
			Success:     true,
			FinalLoss:   0.15,
			Samples:     10,
			Epochs:      3,
			Duration:    2 * time.Second,
		}, nil
	})

	err := scheduler.CheckAndTrigger(context.Background(), "integration")
	if err != nil {
		t.Fatalf("CheckAndTrigger: %v", err)
	}
	if !trainCalled {
		t.Error("training function should have been called")
	}

	st := scheduler.State()
	if st.TotalTrains != 1 {
		t.Errorf("TotalTrains = %d, want 1", st.TotalTrains)
	}
	if st.CurrentAdapter == "" {
		t.Error("should have a current adapter after deploy")
	}
	if !st.ABTestActive {
		t.Error("A/B test should be active")
	}

	// State should be persisted
	stateFile := filepath.Join(tmpAdapters, "scheduler_state.json")
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Error("scheduler_state.json should be persisted")
	}

	// ActiveModel should include adapter name
	model := scheduler.ActiveModel()
	if model == "qwen3.5:4b" {
		t.Error("ActiveModel should include LoRA suffix")
	}

	// Record A/B metrics
	scheduler.RecordABMetric(true, 0.85)
	scheduler.RecordABMetric(false, 0.75)

	st = scheduler.State()
	if st.ABTestMetrics.NewAdapterQueries != 1 {
		t.Errorf("NewAdapterQueries = %d", st.ABTestMetrics.NewAdapterQueries)
	}

	// First deploy has no previous adapter, rollback should error
	err = scheduler.Rollback(context.Background())
	if err == nil {
		t.Error("Rollback should fail with no previous adapter")
	}

	// Simulate a second deploy to enable rollback
	os.WriteFile(filepath.Join(tmpTraining, "train2.jsonl"), lines, 0644)
	scheduler.SetTrainFunc(func(ctx context.Context, job TrainJob) (*TrainResult, error) {
		return &TrainResult{
			AdapterName: job.AdapterName, AdapterPath: job.OutputDir,
			Success: true, FinalLoss: 0.12,
		}, nil
	})
	// Reset last train time to allow re-trigger
	scheduler.mu.Lock()
	scheduler.state.LastTrainTime = time.Time{}
	scheduler.mu.Unlock()

	err = scheduler.CheckAndTrigger(context.Background(), "integration-v2")
	if err != nil {
		t.Fatalf("second CheckAndTrigger: %v", err)
	}

	st = scheduler.State()
	if st.PreviousAdapter == "" {
		t.Fatal("PreviousAdapter should be set after second deploy")
	}

	err = scheduler.Rollback(context.Background())
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	st = scheduler.State()
	if st.TotalRollbacks != 1 {
		t.Errorf("TotalRollbacks = %d", st.TotalRollbacks)
	}
	if st.ABTestActive {
		t.Error("A/B test should be inactive after rollback")
	}
}

func TestIntegration_LoRAAdapter_MultipleLoadUnload(t *testing.T) {
	if !ollamaAvailable() {
		t.Skip("ollama not running")
	}

	adapter := NewLoRAAdapter(LoRAAdapterConfig{
		BaseURL: ollamaURL,
		Timeout: 5 * time.Second,
	})

	names := []string{"finance-v1", "code-v1", "legal-v1"}
	for _, name := range names {
		err := adapter.Load(context.Background(), name, "/fake/"+name, "qwen3.5:4b")
		if err != nil {
			t.Fatalf("Load %s: %v", name, err)
		}
	}

	list, _ := adapter.List(context.Background())
	if len(list) < 3 {
		t.Errorf("expected 3 adapters, got %d", len(list))
	}

	// Unload one
	adapter.Unload(context.Background(), "code-v1")
	if _, ok := adapter.knownAdapters["code-v1"]; ok {
		t.Error("code-v1 should be removed")
	}
	if _, ok := adapter.knownAdapters["finance-v1"]; !ok {
		t.Error("finance-v1 should still exist")
	}
}
