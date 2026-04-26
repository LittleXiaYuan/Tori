package cogni

import (
	"context"
	"testing"
)

func TestEvolutionEngineNoFuncs(t *testing.T) {
	ee := NewEvolutionEngine(DefaultEvolutionConfig(), "")
	_, err := ee.Run(context.Background(), "test")
	if err == nil {
		t.Error("expected error when bench/analyze not configured")
	}
}

func TestEvolutionEngineAlreadyRunning(t *testing.T) {
	ee := NewEvolutionEngine(DefaultEvolutionConfig(), "")
	ee.mu.Lock()
	ee.running["test"] = true
	ee.mu.Unlock()

	_, err := ee.Run(context.Background(), "test")
	if err == nil {
		t.Error("expected error when already running")
	}
}

func TestEvolutionExperiments(t *testing.T) {
	ee := NewEvolutionEngine(DefaultEvolutionConfig(), "")
	if exps := ee.Experiments("nonexistent"); exps != nil {
		t.Error("expected nil for unknown cogni")
	}

	ee.mu.Lock()
	ee.experiments["c1"] = []Experiment{
		{ID: "exp-1", CogniID: "c1", Status: "kept", Delta: 1.5},
	}
	ee.mu.Unlock()

	exps := ee.Experiments("c1")
	if len(exps) != 1 {
		t.Errorf("expected 1 experiment, got %d", len(exps))
	}
	if exps[0].Status != "kept" {
		t.Errorf("expected status 'kept', got %q", exps[0].Status)
	}
}

func TestDefaultEvolutionConfig(t *testing.T) {
	cfg := DefaultEvolutionConfig()
	if cfg.MaxRounds != 5 {
		t.Errorf("expected MaxRounds 5, got %d", cfg.MaxRounds)
	}
	if cfg.StaleRounds != 3 {
		t.Errorf("expected StaleRounds 3, got %d", cfg.StaleRounds)
	}
	if !cfg.AutoRevert {
		t.Error("expected AutoRevert true")
	}
}
