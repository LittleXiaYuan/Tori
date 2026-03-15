package task

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"yunque-agent/pkg/skills"
)

// TestGroupSteps verifies the step grouping logic.
func TestGroupSteps(t *testing.T) {
	steps := []Step{
		{ID: 1, Group: 0}, // sequential
		{ID: 2, Group: 1}, // parallel group 1
		{ID: 3, Group: 1}, // parallel group 1
		{ID: 4, Group: 0}, // sequential
		{ID: 5, Group: 2}, // parallel group 2
		{ID: 6, Group: 2}, // parallel group 2
		{ID: 7, Group: 2}, // parallel group 2
	}

	groups := groupSteps(steps)
	if len(groups) != 4 {
		t.Fatalf("expected 4 groups, got %d: %v", len(groups), groups)
	}
	// Group 0: [0] (step 1)
	if len(groups[0]) != 1 || groups[0][0] != 0 {
		t.Errorf("group 0: expected [0], got %v", groups[0])
	}
	// Group 1: [1,2] (steps 2,3 — parallel)
	if len(groups[1]) != 2 {
		t.Errorf("group 1: expected 2 steps, got %v", groups[1])
	}
	// Group 2: [3] (step 4 — sequential)
	if len(groups[2]) != 1 || groups[2][0] != 3 {
		t.Errorf("group 2: expected [3], got %v", groups[2])
	}
	// Group 3: [4,5,6] (steps 5,6,7 — parallel)
	if len(groups[3]) != 3 {
		t.Errorf("group 3: expected 3 steps, got %v", groups[3])
	}
}

// TestGroupStepsAllSequential verifies all sequential steps.
func TestGroupStepsAllSequential(t *testing.T) {
	steps := []Step{{ID: 1}, {ID: 2}, {ID: 3}}
	groups := groupSteps(steps)
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}
}

// TestParallelExecution verifies that parallel steps actually run concurrently.
func TestParallelExecution(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	var runCount atomic.Int32

	reg := skills.NewRegistry()
	reg.Register(&countSkill{counter: &runCount, delay: 100 * time.Millisecond})

	runner := NewRunner(store, reg, nil, nil)

	// Create task with pre-defined parallel steps
	task, _ := store.Create(CreateRequest{Description: "parallel test"})
	task.Steps = []Step{
		{ID: 1, Action: "step1", SkillName: "count_skill", Status: StepPending, MaxRetries: 0, Group: 1},
		{ID: 2, Action: "step2", SkillName: "count_skill", Status: StepPending, MaxRetries: 0, Group: 1},
		{ID: 3, Action: "step3", SkillName: "count_skill", Status: StepPending, MaxRetries: 0, Group: 1},
	}
	store.Update(task)

	start := time.Now()
	err := runner.Run(context.Background(), task.ID)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All 3 should have run
	if runCount.Load() != 3 {
		t.Errorf("expected 3 runs, got %d", runCount.Load())
	}
	// Should be ~100ms (parallel), not ~300ms (sequential)
	if elapsed > 250*time.Millisecond {
		t.Errorf("parallel execution took too long: %v (expected ~100ms)", elapsed)
	}

	// Verify task completed
	updated, _ := store.Get(task.ID)
	if updated.Status != StatusCompleted {
		t.Errorf("expected completed, got %s", updated.Status)
	}
}

// TestTemplateCreateAndList tests template CRUD.
func TestTemplateCreateAndList(t *testing.T) {
	dir := t.TempDir()
	ts := NewTemplateStore(dir)

	tpl := &Template{
		Name:        "搜索报告模板",
		Description: "搜索{{topic}}并生成报告",
		Variables: []TemplateVar{
			{Name: "topic", Description: "搜索主题", Required: true},
		},
		Steps: []TemplateStep{
			{Action: "搜索{{topic}}相关信息", SkillName: "web_search", Args: map[string]any{"query": "{{topic}}"}},
			{Action: "总结搜索结果", SkillName: ""},
		},
	}

	err := ts.Create(tpl)
	if err != nil {
		t.Fatalf("create template failed: %v", err)
	}
	if tpl.ID == "" {
		t.Fatal("expected non-empty ID")
	}

	// List
	all := ts.List()
	if len(all) != 1 {
		t.Fatalf("expected 1 template, got %d", len(all))
	}

	// Get
	got, ok := ts.Get(tpl.ID)
	if !ok {
		t.Fatal("template not found")
	}
	if got.Name != "搜索报告模板" {
		t.Errorf("name mismatch: %s", got.Name)
	}
}

// TestTemplateInstantiate tests variable substitution.
func TestTemplateInstantiate(t *testing.T) {
	dir := t.TempDir()
	ts := NewTemplateStore(dir)

	tpl := &Template{
		Name:        "搜索{{topic}}",
		Description: "搜索{{topic}}相关资料，输出到{{output}}",
		Variables: []TemplateVar{
			{Name: "topic", Required: true},
			{Name: "output", Default: "data/output/report.txt"},
		},
		Steps: []TemplateStep{
			{Action: "搜索{{topic}}", SkillName: "web_search", Args: map[string]any{"query": "{{topic}} 最新进展"}},
			{Action: "写入{{output}}", SkillName: "file_create", Args: map[string]any{"path": "{{output}}"}},
		},
	}
	ts.Create(tpl)

	task, err := ts.Instantiate(tpl.ID, map[string]string{"topic": "AI Agent"}, "test-tenant")
	if err != nil {
		t.Fatalf("instantiate failed: %v", err)
	}
	if task.Title != "搜索AI Agent" {
		t.Errorf("title substitution failed: %s", task.Title)
	}
	if !strings.Contains(task.Description, "AI Agent") {
		t.Errorf("description substitution failed: %s", task.Description)
	}
	if len(task.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(task.Steps))
	}
	if task.Steps[0].Args["query"] != "AI Agent 最新进展" {
		t.Errorf("args substitution failed: %v", task.Steps[0].Args)
	}
	// Default value for output
	if task.Steps[1].Args["path"] != "data/output/report.txt" {
		t.Errorf("default substitution failed: %v", task.Steps[1].Args)
	}
}

// TestTemplateRequiredVar tests that missing required variables are rejected.
func TestTemplateRequiredVar(t *testing.T) {
	dir := t.TempDir()
	ts := NewTemplateStore(dir)

	tpl := &Template{
		Name: "test",
		Variables: []TemplateVar{
			{Name: "topic", Required: true},
		},
		Steps: []TemplateStep{{Action: "do {{topic}}"}},
	}
	ts.Create(tpl)

	_, err := ts.Instantiate(tpl.ID, map[string]string{}, "test")
	if err == nil {
		t.Fatal("expected error for missing required variable")
	}
}

// TestTemplatePersistence tests templates survive reload.
func TestTemplatePersistence(t *testing.T) {
	dir := t.TempDir()
	ts := NewTemplateStore(dir)
	ts.Create(&Template{Name: "persistent", Steps: []TemplateStep{{Action: "do something"}}})

	// Reload
	ts2 := NewTemplateStore(dir)
	all := ts2.List()
	if len(all) != 1 {
		t.Fatalf("expected 1 template after reload, got %d", len(all))
	}
	if all[0].Name != "persistent" {
		t.Errorf("name mismatch after reload: %s", all[0].Name)
	}
}

// TestTemplateDelete tests template deletion.
func TestTemplateDelete(t *testing.T) {
	dir := t.TempDir()
	ts := NewTemplateStore(dir)
	tpl := &Template{Name: "deleteme", Steps: []TemplateStep{{Action: "x"}}}
	ts.Create(tpl)

	if !ts.Delete(tpl.ID) {
		t.Fatal("delete should return true")
	}
	if _, ok := ts.Get(tpl.ID); ok {
		t.Fatal("template should not exist after delete")
	}
	if ts.Delete(tpl.ID) {
		t.Fatal("second delete should return false")
	}
}

// TestTemplateParallelSteps tests templates with parallel groups.
func TestTemplateParallelSteps(t *testing.T) {
	dir := t.TempDir()
	ts := NewTemplateStore(dir)

	tpl := &Template{
		Name: "parallel-template",
		Steps: []TemplateStep{
			{Action: "搜索A", SkillName: "web_search", Group: 1},
			{Action: "搜索B", SkillName: "web_search", Group: 1},
			{Action: "合并结果", Group: 0},
		},
	}
	ts.Create(tpl)

	task, err := ts.Instantiate(tpl.ID, nil, "test")
	if err != nil {
		t.Fatalf("instantiate failed: %v", err)
	}
	if len(task.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(task.Steps))
	}
	if task.Steps[0].Group != 1 || task.Steps[1].Group != 1 {
		t.Error("parallel group not preserved")
	}
	if task.Steps[2].Group != 0 {
		t.Error("sequential group not preserved")
	}
}

// ── Helper: a skill that counts invocations with optional delay ──

type countSkill struct {
	counter *atomic.Int32
	delay   time.Duration
}

func (s *countSkill) Name() string               { return "count_skill" }
func (s *countSkill) Description() string        { return "counts invocations" }
func (s *countSkill) Parameters() map[string]any { return nil }
func (s *countSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	n := s.counter.Add(1)
	return fmt.Sprintf("count: %d", n), nil
}

func init() {
	_ = os.MkdirAll("data", 0755)
}
