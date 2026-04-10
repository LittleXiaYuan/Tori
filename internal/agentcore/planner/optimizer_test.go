package planner

import (
	"strings"
	"testing"
	"time"
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

func TestShouldSuppress_LowSuccessRate(t *testing.T) {
	opt := NewSkillOptimizer(nil, "")
	now := time.Now().Format(time.RFC3339)
	opt.history = []SkillPerformance{
		{Name: "bad_skill", Total: 20, Success: 2, Failed: 18, SuccessRate: 0.10, LastFailedAt: now},
	}
	if !opt.ShouldSuppress("bad_skill") {
		t.Fatal("expected bad_skill to be suppressed")
	}
}

func TestShouldSuppress_SafeSkill(t *testing.T) {
	opt := NewSkillOptimizer(nil, "")
	now := time.Now().Format(time.RFC3339)
	opt.history = []SkillPerformance{
		{Name: "good_skill", Total: 50, Success: 48, Failed: 2, SuccessRate: 0.96, LastUpdated: now},
		{Name: "new_skill", Total: 3, Success: 0, Failed: 3, SuccessRate: 0.0, LastFailedAt: now}, // too few calls
	}
	if opt.ShouldSuppress("good_skill") {
		t.Fatal("good_skill should not be suppressed")
	}
	if opt.ShouldSuppress("new_skill") {
		t.Fatal("new_skill should not be suppressed (too few total calls)")
	}
	if opt.ShouldSuppress("unknown_skill") {
		t.Fatal("unknown skills should not be suppressed")
	}
}

func TestSuppressedSkills_List(t *testing.T) {
	opt := NewSkillOptimizer(nil, "")
	now := time.Now().Format(time.RFC3339)
	stale := time.Now().Add(-48 * time.Hour).Format(time.RFC3339) // 2 days ago
	opt.history = []SkillPerformance{
		{Name: "dying_a", Total: 10, Success: 1, Failed: 9, SuccessRate: 0.10, LastFailedAt: now},
		{Name: "dying_b", Total: 15, Success: 2, Failed: 13, SuccessRate: 0.13, LastFailedAt: now},
		{Name: "recovered", Total: 10, Success: 1, Failed: 9, SuccessRate: 0.10, LastFailedAt: stale}, // old failures
		{Name: "healthy", Total: 100, Success: 95, Failed: 5, SuccessRate: 0.95, LastFailedAt: now},
	}
	suppressed := opt.SuppressedSkills()
	if len(suppressed) != 2 {
		t.Fatalf("expected 2 suppressed, got %d: %v", len(suppressed), suppressed)
	}
	want := map[string]bool{"dying_a": true, "dying_b": true}
	for _, s := range suppressed {
		if !want[s] {
			t.Fatalf("unexpected suppressed skill: %s", s)
		}
	}
}
