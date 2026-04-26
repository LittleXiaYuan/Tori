package localbrain

import (
	"context"
	"testing"

	ldg "github.com/LittleXiaYuan/ledger"
)

func TestDefaultRewardModel_SuccessfulTask(t *testing.T) {
	rm := &DefaultRewardModel{}
	trace := &ldg.ReasoningTrace{
		Summary: &ldg.TraceSummary{
			TotalSteps:    3,
			Backtracks:    0,
			AvgConfidence: 0.8,
			Reflections:   1,
		},
	}
	score := rm.Score(context.Background(), trace, true)
	// success(0.5) + efficient(0.2) + no backtracks(0.15) + high conf(0.1) + reflection(0.05) = 1.0
	if score != 1.0 {
		t.Errorf("score = %f, want 1.0", score)
	}
}

func TestDefaultRewardModel_FailedTask(t *testing.T) {
	rm := &DefaultRewardModel{}
	trace := &ldg.ReasoningTrace{
		Summary: &ldg.TraceSummary{
			TotalSteps:    10,
			Backtracks:    5,
			AvgConfidence: 0.2,
		},
	}
	score := rm.Score(context.Background(), trace, false)
	// no success(0) + many steps(0) + many backtracks(-0.1) + low conf(-0.1) = clamped to 0
	if score != 0.0 {
		t.Errorf("score = %f, want 0.0", score)
	}
}

func TestDefaultRewardModel_MediumTask(t *testing.T) {
	rm := &DefaultRewardModel{}
	trace := &ldg.ReasoningTrace{
		Summary: &ldg.TraceSummary{
			TotalSteps:    5,
			Backtracks:    0,
			AvgConfidence: 0.5,
		},
	}
	score := rm.Score(context.Background(), trace, true)
	// success(0.5) + medium steps(0.1) + no backtracks(0.15) + medium conf(0) = 0.75
	if score < 0.7 || score > 0.8 {
		t.Errorf("score = %f, expected ~0.75", score)
	}
}

func TestDefaultRewardModel_NilTrace(t *testing.T) {
	rm := &DefaultRewardModel{}
	score := rm.Score(context.Background(), nil, true)
	if score != 0.0 {
		t.Errorf("score = %f, want 0.0 for nil trace", score)
	}
}

func TestDefaultRewardModel_NilSummary(t *testing.T) {
	rm := &DefaultRewardModel{}
	trace := &ldg.ReasoningTrace{Summary: nil}
	score := rm.Score(context.Background(), trace, true)
	if score != 0.0 {
		t.Errorf("score = %f, want 0.0 for nil summary", score)
	}
}

func TestNewTrainingPipeline(t *testing.T) {
	tp := NewTrainingPipeline(nil, nil, "/tmp/data")
	if tp == nil {
		t.Fatal("expected non-nil pipeline")
	}
	if tp.dataDir != "/tmp/data" {
		t.Errorf("dataDir = %s", tp.dataDir)
	}
	if tp.rewarder == nil {
		t.Error("expected default rewarder")
	}
}

func TestSetRewardModel(t *testing.T) {
	tp := NewTrainingPipeline(nil, nil, "/tmp")
	custom := &DefaultRewardModel{}
	tp.SetRewardModel(custom)
	if tp.rewarder != custom {
		t.Error("SetRewardModel did not replace rewarder")
	}
}

func TestTrajectoryStepTypes(t *testing.T) {
	kinds := map[string]string{
		"reasoning.thought":    "think",
		"reasoning.observe":    "observe",
		"reasoning.decision":   "decide",
		"reasoning.backtrack":  "backtrack",
		"reasoning.hypothesis": "hypothesize",
		"reasoning.reflect":    "reflect",
	}
	for kind, expected := range kinds {
		if expected == "" {
			t.Errorf("missing mapping for %s", kind)
		}
		_ = kind
	}
}

func TestExtractTrajectory_NilLedger_Panics(t *testing.T) {
	tp := NewTrainingPipeline(nil, nil, "/tmp")
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic with nil ledger")
		}
	}()
	tp.ExtractTrajectory(context.Background(), "task-1", true)
}
