package eval

import (
	"testing"

	"yunque-agent/internal/ledgercore"

	"yunque-agent/internal/experimental/taskdistill"
)

func TestClamp(t *testing.T) {
	cases := []struct {
		v, min, max, want float64
	}{
		{0.5, 0, 1, 0.5},
		{-1, 0, 1, 0},
		{2, 0, 1, 1},
		{0, 0, 0, 0},
	}
	for _, tc := range cases {
		if got := clamp(tc.v, tc.min, tc.max); got != tc.want {
			t.Errorf("clamp(%v,%v,%v) = %v, want %v", tc.v, tc.min, tc.max, got, tc.want)
		}
	}
}

func TestHeuristicEvalCompleted(t *testing.T) {
	ev := &Evaluator{}
	s := &taskdistill.TaskEventSummary{
		TaskID:     "t1",
		Status:     ledger.TaskCompleted,
		StepCount:  3,
		Backtracks: 0,
	}

	result := ev.heuristicEval(s)

	if result.GoalAchieved < 0.8 {
		t.Errorf("completed no-backtrack GoalAchieved = %f, want >= 0.8", result.GoalAchieved)
	}
	if result.QualityScore < 0.5 {
		t.Errorf("quality score too low: %f", result.QualityScore)
	}
	if result.ShouldDistill {
		t.Error("high-quality task should not need distillation")
	}
}

func TestHeuristicEvalFailed(t *testing.T) {
	ev := &Evaluator{}
	s := &taskdistill.TaskEventSummary{
		TaskID:     "t2",
		Status:     ledger.TaskFailed,
		StepCount:  8,
		Backtracks: 3,
		Errors:     []string{"timeout"},
	}

	result := ev.heuristicEval(s)

	if result.GoalAchieved > 0.5 {
		t.Errorf("failed GoalAchieved = %f, should be low", result.GoalAchieved)
	}
	if !result.ShouldDistill {
		t.Error("low-quality task should recommend distillation")
	}
	if len(result.Suggestions) == 0 {
		t.Error("failed task should have suggestions")
	}
}

func TestHeuristicEvalBacktracks(t *testing.T) {
	ev := &Evaluator{}
	s := &taskdistill.TaskEventSummary{
		TaskID:     "t3",
		Status:     ledger.TaskCompleted,
		StepCount:  5,
		Backtracks: 3,
	}

	result := ev.heuristicEval(s)

	// Should have backtrack suggestion
	foundSuggestion := false
	for _, sug := range result.Suggestions {
		if len(sug) > 0 {
			foundSuggestion = true
		}
	}
	if !foundSuggestion {
		t.Error("expected backtrack-related suggestion")
	}
}

func TestHeuristicEvalManySteps(t *testing.T) {
	ev := &Evaluator{}
	s := &taskdistill.TaskEventSummary{
		TaskID:    "t4",
		Status:    ledger.TaskCompleted,
		StepCount: 15,
	}

	result := ev.heuristicEval(s)

	// Many steps → low efficiency
	if result.Efficiency > 0.5 {
		t.Errorf("15 steps efficiency = %f, should be low", result.Efficiency)
	}

	// Should suggest batching
	foundBatch := false
	for _, sug := range result.Suggestions {
		if len(sug) > 10 {
			foundBatch = true
		}
	}
	if !foundBatch {
		t.Error("expected suggestion for many steps")
	}
}

func TestNewEvaluator(t *testing.T) {
	ev := New(nil)
	if ev == nil {
		t.Fatal("New should not return nil")
	}
}

func TestSetFunctions(t *testing.T) {
	ev := New(nil)
	ev.SetEvalFunc(nil)
	ev.SetDistiller(nil)
	// Should not panic
}
