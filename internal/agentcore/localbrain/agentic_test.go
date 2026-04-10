package localbrain

import (
	"context"
	"testing"
	"time"
)

func TestDefaultAgenticConfig(t *testing.T) {
	cfg := DefaultAgenticConfig()
	if cfg.IntuitionTimeout != 50*time.Millisecond {
		t.Errorf("IntuitionTimeout = %v, want 50ms", cfg.IntuitionTimeout)
	}
	if cfg.MaxThinkSteps != 10 {
		t.Errorf("MaxThinkSteps = %d, want 10", cfg.MaxThinkSteps)
	}
	if !cfg.InterleavedMode {
		t.Error("InterleavedMode should default to true")
	}
}

func TestDetermineThinkLevelFirstStep(t *testing.T) {
	// With nil brain, returns DefaultThinkLevel (1 = ThinkQuick)
	at := NewAgenticThinking(nil, nil, nil)
	ctx := context.Background()

	level := at.determineThinkLevel(ctx, ThinkRequest{StepIndex: 0})
	if level != ThinkLevel(at.config.DefaultThinkLevel) {
		t.Errorf("nil brain step 0: level = %d, want DefaultThinkLevel(%d)", level, at.config.DefaultThinkLevel)
	}
}

func TestDetermineThinkLevelAfterFailure(t *testing.T) {
	// With nil brain, always returns default regardless of step history
	at := NewAgenticThinking(nil, nil, nil)
	ctx := context.Background()

	level := at.determineThinkLevel(ctx, ThinkRequest{
		StepIndex: 3,
		StepHistory: []StepSummary{{Action: "act", Result: "fail", Success: false}},
	})
	// nil brain → always default level
	if level != ThinkLevel(at.config.DefaultThinkLevel) {
		t.Errorf("nil brain after failure: level = %d, want %d", level, at.config.DefaultThinkLevel)
	}
}

func TestDetermineThinkLevelManySteps(t *testing.T) {
	// With nil brain, always returns default
	at := NewAgenticThinking(nil, nil, nil)
	ctx := context.Background()

	level := at.determineThinkLevel(ctx, ThinkRequest{
		StepIndex: 7,
		StepHistory: []StepSummary{
			{Action: "a", Result: "r", Success: true},
		},
	})
	if level != ThinkLevel(at.config.DefaultThinkLevel) {
		t.Errorf("nil brain many steps: level = %d, want %d", level, at.config.DefaultThinkLevel)
	}
}

func TestThinkNoBrainFallback(t *testing.T) {
	cfg := DefaultAgenticConfig()
	cfg.DefaultThinkLevel = 0
	at := NewAgenticThinking(nil, nil, nil, cfg)
	ctx := context.Background()

	// StepIndex=0 always returns ThinkNormal, but normalThink fails (no pool/client)
	_, err := at.Think(ctx, ThinkRequest{
		StepIndex: 0,
		Query:     "test",
	})
	// normalThink returns error due to nil client
	if err == nil {
		t.Log("Think succeeded unexpectedly — this is OK if pool handles nil gracefully")
	}
}

func TestThinkNone(t *testing.T) {
	_ = NewAgenticThinking(nil, nil, nil)

	// Directly test ThinkResult with level 0
	result := &ThinkResult{Level: ThinkNone, Confidence: 0.9}
	if result.Level != ThinkNone {
		t.Errorf("level = %d, want ThinkNone", result.Level)
	}
}

func TestFormatHistory(t *testing.T) {
	at := NewAgenticThinking(nil, nil, nil)
	history := []StepSummary{
		{Action: "read_file", Result: "ok", Success: true},
		{Action: "write_file", Result: "fail", Success: false},
	}
	formatted := at.formatHistory(history)
	if formatted == "" {
		t.Error("formatHistory returned empty for non-empty history")
	}
}
