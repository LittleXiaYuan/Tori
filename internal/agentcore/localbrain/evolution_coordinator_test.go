package localbrain

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultCoordinatorConfig(t *testing.T) {
	cfg := DefaultCoordinatorConfig()
	if cfg.StrategyInterval != 20 {
		t.Errorf("StrategyInterval = %d, want 20", cfg.StrategyInterval)
	}
	if cfg.WeightHitRateThreshold != 0.7 {
		t.Errorf("WeightHitRateThreshold = %f, want 0.7", cfg.WeightHitRateThreshold)
	}
	if cfg.WeightCooldown != 24*time.Hour {
		t.Errorf("WeightCooldown = %v, want 24h", cfg.WeightCooldown)
	}
}

func TestNewEvolutionCoordinator(t *testing.T) {
	ec := NewEvolutionCoordinator(nil, nil, nil, nil, DefaultCoordinatorConfig())
	if ec == nil {
		t.Fatal("expected non-nil coordinator")
	}
	if ec.State().TotalTasks != 0 {
		t.Errorf("TotalTasks = %d, want 0", ec.State().TotalTasks)
	}
}

func TestOnTaskComplete_IncrementsCounters(t *testing.T) {
	ec := NewEvolutionCoordinator(nil, nil, nil, nil, DefaultCoordinatorConfig())

	for i := 0; i < 5; i++ {
		ec.OnTaskComplete(context.Background(), TaskOutcome{
			TaskID:   "task-" + string(rune('0'+i)),
			TenantID: "default",
			Success:  true,
			Reward:   0.8,
		})
	}

	s := ec.State()
	if s.TotalTasks != 5 {
		t.Errorf("TotalTasks = %d, want 5", s.TotalTasks)
	}
	if s.SuccessTasks != 5 {
		t.Errorf("SuccessTasks = %d, want 5", s.SuccessTasks)
	}
	if s.RollingSuccessRate != 1.0 {
		t.Errorf("RollingSuccessRate = %f, want 1.0", s.RollingSuccessRate)
	}
}

func TestRollingWindow_Capped(t *testing.T) {
	ec := NewEvolutionCoordinator(nil, nil, nil, nil, DefaultCoordinatorConfig())

	for i := 0; i < rollingWindowSize+20; i++ {
		ec.OnTaskComplete(context.Background(), TaskOutcome{
			TaskID:  "t",
			Success: true,
		})
	}

	s := ec.State()
	if len(s.RecentWindow) != rollingWindowSize {
		t.Errorf("RecentWindow len = %d, want %d", len(s.RecentWindow), rollingWindowSize)
	}
}

func TestRollingSuccessRate_Mixed(t *testing.T) {
	ec := NewEvolutionCoordinator(nil, nil, nil, nil, DefaultCoordinatorConfig())

	for i := 0; i < 10; i++ {
		ec.OnTaskComplete(context.Background(), TaskOutcome{
			TaskID:  "t",
			Success: i%2 == 0,
		})
	}

	s := ec.State()
	if s.RollingSuccessRate != 0.5 {
		t.Errorf("RollingSuccessRate = %f, want 0.5", s.RollingSuccessRate)
	}
}

func TestShouldUpdateStrategy_NotEnoughTasks(t *testing.T) {
	cfg := DefaultCoordinatorConfig()
	cfg.StrategyMinObservations = 100
	ec := NewEvolutionCoordinator(nil, nil, nil, nil, cfg)

	for i := 0; i < 30; i++ {
		ec.OnTaskComplete(context.Background(), TaskOutcome{Success: true})
	}

	s := ec.State()
	if s.StrategyUpdates != 0 {
		t.Errorf("strategy should not update before min observations, got %d", s.StrategyUpdates)
	}
}

func TestShouldTriggerWeights_NoScheduler(t *testing.T) {
	cfg := DefaultCoordinatorConfig()
	cfg.WeightMinNewTasks = 5
	cfg.WeightHitRateThreshold = 0.9
	ec := NewEvolutionCoordinator(nil, nil, nil, nil, cfg)

	for i := 0; i < 60; i++ {
		ec.OnTaskComplete(context.Background(), TaskOutcome{Success: false})
	}

	s := ec.State()
	if s.WeightTriggers != 0 {
		t.Errorf("should not trigger weights without scheduler, got %d", s.WeightTriggers)
	}
}

func TestShouldTriggerWeights_HighSuccessRate(t *testing.T) {
	cfg := DefaultCoordinatorConfig()
	cfg.WeightMinNewTasks = 5
	cfg.WeightHitRateThreshold = 0.5
	scheduler := NewLoRAScheduler(nil, nil, nil, DefaultSchedulerConfig())
	ec := NewEvolutionCoordinator(nil, nil, scheduler, nil, cfg)

	for i := 0; i < 60; i++ {
		ec.OnTaskComplete(context.Background(), TaskOutcome{Success: true})
	}

	s := ec.State()
	if s.WeightTriggers != 0 {
		t.Errorf("should not trigger weights with high success rate, got %d", s.WeightTriggers)
	}
}

func TestCoarseIntentKey(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"hello world", "hello"},
		{"查询天气", "查询天气"},
		{"", ""},
		{"  leading", ""},
		{"averylongwordwithoutspaceshouldbecapped", "averylongwordwit"},
	}
	for _, tt := range tests {
		if got := coarseIntentKey(tt.input); got != tt.want {
			t.Errorf("coarseIntentKey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestComplexityFromSteps(t *testing.T) {
	tests := []struct {
		steps int
		want  string
	}{
		{1, "simple"},
		{3, "simple"},
		{5, "medium"},
		{8, "medium"},
		{10, "hard"},
	}
	for _, tt := range tests {
		if got := complexityFromSteps(tt.steps); got != tt.want {
			t.Errorf("complexityFromSteps(%d) = %q, want %q", tt.steps, got, tt.want)
		}
	}
}

func TestPersistAndLoadState(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultCoordinatorConfig()
	cfg.StateDir = dir

	ec := NewEvolutionCoordinator(nil, nil, nil, nil, cfg)
	for i := 0; i < 5; i++ {
		ec.OnTaskComplete(context.Background(), TaskOutcome{
			TaskID:   "t",
			TenantID: "tenant-a",
			Success:  true,
		})
	}

	statePath := filepath.Join(dir, "evolution_state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("state file not written: %v", err)
	}
	var saved CoordinatorState
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("state file malformed: %v", err)
	}
	if saved.TotalTasks != 5 {
		t.Errorf("saved TotalTasks = %d, want 5", saved.TotalTasks)
	}

	ec2 := NewEvolutionCoordinator(nil, nil, nil, nil, cfg)
	s := ec2.State()
	if s.TotalTasks != 5 {
		t.Errorf("loaded TotalTasks = %d, want 5", s.TotalTasks)
	}
	if s.ByTenant["tenant-a"] != 5 {
		t.Errorf("loaded ByTenant[tenant-a] = %d, want 5", s.ByTenant["tenant-a"])
	}
}

func TestOnTaskComplete_ByTenant(t *testing.T) {
	ec := NewEvolutionCoordinator(nil, nil, nil, nil, DefaultCoordinatorConfig())

	ec.OnTaskComplete(context.Background(), TaskOutcome{TenantID: "a", Success: true})
	ec.OnTaskComplete(context.Background(), TaskOutcome{TenantID: "a", Success: true})
	ec.OnTaskComplete(context.Background(), TaskOutcome{TenantID: "b", Success: false})

	s := ec.State()
	if s.ByTenant["a"] != 2 {
		t.Errorf("tenant a = %d, want 2", s.ByTenant["a"])
	}
	if s.ByTenant["b"] != 1 {
		t.Errorf("tenant b = %d, want 1", s.ByTenant["b"])
	}
}
