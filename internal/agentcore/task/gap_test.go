package task

import (
	"context"
	"testing"

	"yunque-agent/pkg/skills"
)

func TestGapClassifySkillMissing(t *testing.T) {
	g := NewGapAnalyzer(nil) // no LLM

	tk := &Task{ID: "t1"}
	step := &Step{
		ID:        1,
		Action:    "send email",
		SkillName: "email_send",
		Error:     `skill "email_send" not found`,
		Status:    StepFailed,
	}

	rec := g.Analyze(context.Background(), tk, step)
	if rec.GapType != GapSkillMissing {
		t.Fatalf("expected skill_missing, got %s", rec.GapType)
	}
}

func TestGapClassifyParamError(t *testing.T) {
	g := NewGapAnalyzer(nil)

	tk := &Task{ID: "t2"}
	step := &Step{
		ID:        1,
		Action:    "translate text",
		SkillName: "translate",
		Error:     "target_lang is required",
		Status:    StepFailed,
	}

	rec := g.Analyze(context.Background(), tk, step)
	if rec.GapType != GapParamError {
		t.Fatalf("expected param_error, got %s", rec.GapType)
	}
}

func TestGapClassifyEnvError(t *testing.T) {
	g := NewGapAnalyzer(nil)

	tk := &Task{ID: "t3"}
	step := &Step{
		ID:        1,
		Action:    "search web",
		SkillName: "web_search",
		Error:     "connection timeout: cannot reach api.example.com",
		Status:    StepFailed,
	}

	rec := g.Analyze(context.Background(), tk, step)
	if rec.GapType != GapEnvError {
		t.Fatalf("expected env_error, got %s", rec.GapType)
	}
}

func TestGapClassifyUnknown(t *testing.T) {
	g := NewGapAnalyzer(nil)

	tk := &Task{ID: "t4"}
	step := &Step{
		ID:     1,
		Action: "mysterious step",
		Error:  "something went wrong",
		Status: StepFailed,
	}

	rec := g.Analyze(context.Background(), tk, step)
	if rec.GapType != GapUnknown {
		t.Fatalf("expected unknown, got %s", rec.GapType)
	}
}

func TestGapRecordsAndStats(t *testing.T) {
	g := NewGapAnalyzer(nil)

	tk := &Task{ID: "t5"}

	g.Analyze(context.Background(), tk, &Step{ID: 1, SkillName: "foo", Error: `skill "foo" not found`, Status: StepFailed})
	g.Analyze(context.Background(), tk, &Step{ID: 2, SkillName: "bar", Error: "text is required", Status: StepFailed})
	g.Analyze(context.Background(), tk, &Step{ID: 3, SkillName: "baz", Error: "connection timeout", Status: StepFailed})

	// Stats
	stats := g.Stats()
	if stats["total"] != 3 {
		t.Fatalf("expected 3 total, got %d", stats["total"])
	}
	if stats["skill_missing"] != 1 {
		t.Fatalf("expected 1 skill_missing, got %d", stats["skill_missing"])
	}
	if stats["param_error"] != 1 {
		t.Fatalf("expected 1 param_error, got %d", stats["param_error"])
	}
	if stats["env_error"] != 1 {
		t.Fatalf("expected 1 env_error, got %d", stats["env_error"])
	}

	// Filter by type
	missing := g.Records(GapSkillMissing, false)
	if len(missing) != 1 {
		t.Fatalf("expected 1 skill_missing record, got %d", len(missing))
	}

	// Unresolved filter
	all := g.Records("", true)
	if len(all) != 3 {
		t.Fatalf("expected 3 unresolved, got %d", len(all))
	}

	// Resolve one
	ok := g.Resolve(all[0].ID)
	if !ok {
		t.Fatal("resolve failed")
	}
	unresolved := g.Records("", true)
	if len(unresolved) != 2 {
		t.Fatalf("expected 2 unresolved after resolve, got %d", len(unresolved))
	}
}

func TestGapWithLLMSuggestion(t *testing.T) {
	mockLLM := func(ctx context.Context, system, user string) (string, error) {
		return "建议安装 email_send 技能来支持邮件发送功能", nil
	}

	g := NewGapAnalyzer(mockLLM)
	tk := &Task{ID: "t6"}
	step := &Step{
		ID:        1,
		Action:    "send email notification",
		SkillName: "email_send",
		Error:     `skill "email_send" not found`,
		Status:    StepFailed,
	}

	rec := g.Analyze(context.Background(), tk, step)
	if rec.Suggestion == "" {
		t.Fatal("expected LLM suggestion")
	}
	if rec.GapType != GapSkillMissing {
		t.Fatalf("expected skill_missing, got %s", rec.GapType)
	}
}

func TestGapIntegratedWithRunner(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	reg := skills.NewRegistry() // empty registry — no skills

	callCount := 0
	llm := func(ctx context.Context, system, user string) (string, error) {
		callCount++
		if callCount == 1 {
			return `[{"action":"send notification","skill_name":"email","args":{}}]`, nil
		}
		return "ok", nil
	}

	g := NewGapAnalyzer(nil)
	runner := NewRunner(s, reg, llm, nil)
	runner.SetGapAnalyzer(g)

	tk, _ := s.Create(CreateRequest{Description: "send a notification"})
	err := runner.Run(context.Background(), tk.ID)
	if err == nil {
		t.Fatal("expected error")
	}

	// Gap should be recorded
	stats := g.Stats()
	if stats["total"] != 1 {
		t.Fatalf("expected 1 gap, got %d", stats["total"])
	}
	if stats["skill_missing"] != 1 {
		t.Fatalf("expected skill_missing gap, got: %v", stats)
	}

	// Step should have gap_type
	got, _ := s.Get(tk.ID)
	if got.Steps[0].GapType != string(GapSkillMissing) {
		t.Fatalf("expected step gap_type=skill_missing, got %s", got.Steps[0].GapType)
	}
}
