package taskdistill

import (
	"strings"
	"testing"

	"yunque-agent/internal/ledgercore"
)

func TestToPrompt(t *testing.T) {
	s := &TaskEventSummary{
		TaskID:     "t1",
		Goal:       "search something",
		TaskType:   ledger.TaskTypeGoal,
		Status:     ledger.TaskCompleted,
		StepCount:  5,
		Backtracks: 1,
		ToolsUsed:  []string{"web_search", "read_file"},
		Errors:     []string{"timeout"},
		Thoughts:   []string{"need to search"},
		Decisions:  []string{"use web search"},
	}

	prompt := s.ToPrompt()
	if !strings.Contains(prompt, "search something") {
		t.Error("prompt should contain goal")
	}
	if !strings.Contains(prompt, "web_search") {
		t.Error("prompt should contain tools")
	}
	if !strings.Contains(prompt, "timeout") {
		t.Error("prompt should contain errors")
	}
	if !strings.Contains(prompt, "patterns") {
		t.Error("prompt should mention patterns")
	}
}

func TestHeuristicDistillCompleted(t *testing.T) {
	d := &Distiller{}

	s := &TaskEventSummary{
		TaskID:     "t1",
		Goal:       "do something",
		TaskType:   ledger.TaskTypeGoal,
		Status:     ledger.TaskCompleted,
		StepCount:  2,
		Backtracks: 0,
		ToolsUsed:  []string{"tool1"},
	}

	result := d.heuristicDistill(s)

	// Efficient completion → should have efficient_execution pattern
	found := false
	for _, p := range result.Patterns {
		if p.Name == "efficient_execution" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected efficient_execution pattern for quick completion")
	}

	// Should have tool insights
	if len(result.ToolInsights) == 0 {
		t.Error("expected tool insights")
	}
	for _, ti := range result.ToolInsights {
		if ti.Score != 0.7 {
			t.Errorf("completed task tool score = %f, want 0.7", ti.Score)
		}
	}
}

func TestHeuristicDistillFailedWithBacktracks(t *testing.T) {
	d := &Distiller{}

	s := &TaskEventSummary{
		TaskID:     "t2",
		Goal:       "failed task",
		TaskType:   ledger.TaskTypeGoal,
		Status:     ledger.TaskFailed,
		StepCount:  8,
		Backtracks: 3,
		ToolsUsed:  []string{"tool1"},
		Errors:     []string{"connection refused"},
	}

	result := d.heuristicDistill(s)

	// Failed → should have failure rule
	foundFailRule := false
	for _, r := range result.Rules {
		if strings.Contains(r.Action, "different approach") {
			foundFailRule = true
			break
		}
	}
	if !foundFailRule {
		t.Error("expected failure rule")
	}

	// Failed tools have lower scores
	for _, ti := range result.ToolInsights {
		if ti.Score != 0.3 {
			t.Errorf("failed task tool score = %f, want 0.3", ti.Score)
		}
	}
}

func TestHeuristicDistillBacktrackRecovery(t *testing.T) {
	d := &Distiller{}

	s := &TaskEventSummary{
		TaskID:     "t3",
		Goal:       "recover task",
		TaskType:   ledger.TaskTypeGoal,
		Status:     ledger.TaskCompleted,
		StepCount:  6,
		Backtracks: 2,
	}

	result := d.heuristicDistill(s)

	found := false
	for _, p := range result.Patterns {
		if p.Name == "backtrack_recovery" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected backtrack_recovery pattern")
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello_world"},
		{"simple", "simple"},
		{"ABC DEF", "abc_def"},
	}
	for _, tc := range cases {
		got := slugify(tc.input)
		if got != tc.want {
			t.Errorf("slugify(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestSlugifyLong(t *testing.T) {
	long := strings.Repeat("a", 100)
	got := slugify(long)
	if len(got) > 50 {
		t.Errorf("slugify should truncate to 50 chars, got %d", len(got))
	}
}
