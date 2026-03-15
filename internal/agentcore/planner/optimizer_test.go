package planner

import (
	"strings"
	"testing"
)

func TestSkillOptimizerEmptyHints(t *testing.T) {
	opt := NewSkillOptimizer(nil, "")
	hints := opt.OptimizationHints()
	if hints != "" {
		t.Fatalf("expected empty hints, got: %s", hints)
	}
}

func TestSkillOptimizerHintsWithHistory(t *testing.T) {
	opt := NewSkillOptimizer(nil, "")
	opt.history = []SkillPerformance{
		{Name: "web_search", Total: 25, Success: 23, Failed: 2, SuccessRate: 0.92},
		{Name: "translate", Total: 15, Success: 14, Failed: 1, SuccessRate: 0.93},
		{Name: "broken_skill", Total: 10, Success: 3, Failed: 7, SuccessRate: 0.30},
		{Name: "rare_skill", Total: 2, Success: 2, Failed: 0, SuccessRate: 1.0}, // too few calls
	}

	hints := opt.OptimizationHints()
	if hints == "" {
		t.Fatal("expected non-empty hints")
	}

	// Should mention frequent skills
	if !strings.Contains(hints, "web_search") {
		t.Fatal("expected web_search in frequent skills")
	}
	if !strings.Contains(hints, "translate") {
		t.Fatal("expected translate in frequent skills")
	}

	// Should warn about low-success-rate skill
	if !strings.Contains(hints, "broken_skill") {
		t.Fatal("expected broken_skill in low-success hints")
	}

	// Should NOT mention rare_skill (too few calls)
	if strings.Contains(hints, "rare_skill") {
		t.Fatal("should not mention rare_skill with only 2 calls")
	}
}

func TestSkillOptimizerPerformanceReport(t *testing.T) {
	opt := NewSkillOptimizer(nil, "")
	opt.history = []SkillPerformance{
		{Name: "a", Total: 5},
		{Name: "b", Total: 20},
		{Name: "c", Total: 10},
	}

	report := opt.PerformanceReport()
	if len(report) != 3 {
		t.Fatalf("expected 3 skills, got %d", len(report))
	}
	// Should be sorted by total descending
	if report[0].Name != "b" || report[1].Name != "c" || report[2].Name != "a" {
		t.Fatal("expected sorted by total descending")
	}
}

func TestSkillOptimizerAnalyzeThrottling(t *testing.T) {
	opt := NewSkillOptimizer(nil, "")

	// Should not panic with nil metrics
	opt.Analyze()
	opt.Analyze()
	opt.Analyze()
}
