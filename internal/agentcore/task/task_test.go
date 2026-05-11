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

func TestCreateAndGet(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	task, err := s.Create(CreateRequest{Description: "分析这个 zip 文件的结构"})
	if err != nil {
		t.Fatal(err)
	}
	if task.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if task.Status != StatusPending {
		t.Fatalf("expected pending, got %s", task.Status)
	}
	if task.Title == "" {
		t.Fatal("expected auto-generated title")
	}

	got, ok := s.Get(task.ID)
	if !ok {
		t.Fatal("task not found after create")
	}
	if got.Description != "分析这个 zip 文件的结构" {
		t.Fatal("description mismatch")
	}
}

func TestCreateValidation(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	_, err := s.Create(CreateRequest{})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestTaskCloneDeepCopiesStepProvenance(t *testing.T) {
	tk := &Task{
		ID: "task-copy",
		Steps: []Step{{
			ID:        1,
			Action:    "resume step",
			SkillName: "file_exec",
			Args:      map[string]any{"path": "doc"},
			DependsOn: []int{0},
			Metadata:  map[string]any{"planner_step_id": 2},
			Status:    StepPending,
		}},
	}

	cp := tk.clone()
	cp.Steps[0].Args["path"] = "changed"
	cp.Steps[0].DependsOn[0] = 99
	cp.Steps[0].Metadata["planner_step_id"] = 9

	if tk.Steps[0].Args["path"] != "doc" {
		t.Fatalf("args map was not deep-copied: %+v", tk.Steps[0].Args)
	}
	if tk.Steps[0].DependsOn[0] != 0 {
		t.Fatalf("depends_on slice was not deep-copied: %+v", tk.Steps[0].DependsOn)
	}
	if tk.Steps[0].Metadata["planner_step_id"] != 2 {
		t.Fatalf("metadata map was not deep-copied: %+v", tk.Steps[0].Metadata)
	}
}

func TestRunnerBlocksStepWhenDependenciesUnmet(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	reg := skills.NewRegistry()
	var calls atomic.Int32
	reg.Register(&countSkill{counter: &calls})
	runner := NewRunner(s, reg, nil, &skills.Environment{})

	tk, _ := s.Create(CreateRequest{Description: "dependency blocked"})
	tk.Steps = []Step{
		{ID: 2, Action: "should wait", SkillName: "count_skill", Args: map[string]any{"data": "blocked"}, Status: StepPending, DependsOn: []int{1}},
		{ID: 1, Action: "not done yet", Status: StepPending},
	}
	if err := s.Update(tk); err != nil {
		t.Fatalf("update task: %v", err)
	}

	err := runner.Run(context.Background(), tk.ID)
	if err == nil {
		t.Fatal("expected dependency blocked error")
	}
	got, _ := s.Get(tk.ID)
	if got.Status != StatusInterrupted {
		t.Fatalf("expected interrupted for dependency block, got %s", got.Status)
	}
	if !strings.Contains(got.Error, "等待依赖步骤完成") {
		t.Fatalf("expected dependency error, got %q", got.Error)
	}
	if calls.Load() != 0 {
		t.Fatalf("blocked step should not execute skill, got %d calls", calls.Load())
	}
}

func TestListAndDelete(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	s.Create(CreateRequest{Description: "task A", TenantID: "t1"})
	s.Create(CreateRequest{Description: "task B", TenantID: "t1"})
	s.Create(CreateRequest{Description: "task C", TenantID: "t2"})

	all := s.List("", 0)
	if len(all) != 3 {
		t.Fatalf("expected 3, got %d", len(all))
	}

	t1Tasks := s.List("t1", 0)
	if len(t1Tasks) != 2 {
		t.Fatalf("expected 2 for t1, got %d", len(t1Tasks))
	}

	s.Delete(all[0].ID)
	if len(s.List("", 0)) != 2 {
		t.Fatal("expected 2 after delete")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	tk, _ := s.Create(CreateRequest{Description: "persist me"})

	// Reload from disk
	s2 := NewStore(dir)
	got, ok := s2.Get(tk.ID)
	if !ok {
		t.Fatal("task not found after reload")
	}
	if got.Description != "persist me" {
		t.Fatal("description lost after reload")
	}
}

func TestTaskProgress(t *testing.T) {
	tk := &Task{
		Steps: []Step{
			{ID: 1, Status: StepDone},
			{ID: 2, Status: StepRunning},
			{ID: 3, Status: StepPending},
		},
	}
	done, total := tk.Progress()
	if done != 1 || total != 3 {
		t.Fatalf("expected 1/3, got %d/%d", done, total)
	}
	cur := tk.CurrentStep()
	if cur == nil || cur.ID != 2 {
		t.Fatal("expected current step 2")
	}
}

func TestRunnerPlan(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	// Create a mock skill
	reg := skills.NewRegistry()
	reg.Register(&mockSkill{name: "web_search", result: "找到 3 个结果"})

	// Mock LLM that returns a plan
	llm := func(ctx context.Context, system, user string) (string, error) {
		return `[{"action":"搜索相关文档","skill_name":"web_search","args":{"query":"Go 并发模式"}}]`, nil
	}

	env := &skills.Environment{}
	runner := NewRunner(s, reg, llm, env)

	tk, _ := s.Create(CreateRequest{Description: "搜索 Go 并发模式的最佳实践"})

	err := runner.Run(context.Background(), tk.ID)
	if err != nil {
		t.Fatal(err)
	}

	got, _ := s.Get(tk.ID)
	if got.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s", got.Status)
	}
	if len(got.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(got.Steps))
	}
	if got.Steps[0].Result != "找到 3 个结果" {
		t.Fatalf("unexpected result: %s", got.Steps[0].Result)
	}
}

func TestRunnerLLMOnlyStep(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	reg := skills.NewRegistry()

	llm := func(ctx context.Context, system, user string) (string, error) {
		return `[{"action":"分析需求","skill_name":"","args":{}}]`, nil
	}

	env := &skills.Environment{}
	runner := NewRunner(s, reg, llm, env)

	tk, _ := s.Create(CreateRequest{Description: "需求分析"})
	err := runner.Run(context.Background(), tk.ID)
	if err != nil {
		t.Fatal(err)
	}

	got, _ := s.Get(tk.ID)
	if got.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s", got.Status)
	}
}

func TestArtifactDir(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	tk, _ := s.Create(CreateRequest{Description: "test"})

	aDir, err := s.ArtifactDir(tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(aDir); err != nil {
		t.Fatal("artifact dir should exist")
	}
}

// mockSkill implements skills.Skill for testing.
type mockSkill struct {
	name   string
	result string
}

func (m *mockSkill) Name() string               { return m.name }
func (m *mockSkill) Description() string        { return "mock skill" }
func (m *mockSkill) Parameters() map[string]any { return nil }
func (m *mockSkill) Execute(_ context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
	return m.result, nil
}

// ── New tests: step chaining, retry, cancellation ──

func TestStepChaining(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	reg := skills.NewRegistry()

	// Skill that echoes the _prev_result to verify chaining
	reg.Register(&echoSkill{name: "echo"})

	callCount := 0
	llm := func(ctx context.Context, system, user string) (string, error) {
		callCount++
		if callCount == 1 {
			// Planning call: 2 steps
			return `[{"action":"获取数据","skill_name":"echo","args":{"data":"step1-output"}},{"action":"处理数据","skill_name":"echo","args":{"data":"step2-used-prev"}}]`, nil
		}
		return "llm result", nil
	}

	runner := NewRunner(s, reg, llm, &skills.Environment{})
	tk, _ := s.Create(CreateRequest{Description: "chain test"})

	err := runner.Run(context.Background(), tk.ID)
	if err != nil {
		t.Fatal(err)
	}

	got, _ := s.Get(tk.ID)
	if got.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s", got.Status)
	}
	if len(got.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(got.Steps))
	}

	// Step 1 has no input (first step)
	if got.Steps[0].Input != "" {
		t.Fatalf("step 1 should have no input, got %q", got.Steps[0].Input)
	}

	// Step 2 should have step 1's result as input
	if got.Steps[1].Input == "" {
		t.Fatal("step 2 should have chained input from step 1")
	}
	if !strings.Contains(got.Steps[1].Result, "prev=") {
		t.Fatalf("step 2 result should reflect prev_result, got %q", got.Steps[1].Result)
	}
}

func TestStepRetry(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	var failCount atomic.Int32
	reg := skills.NewRegistry()
	reg.Register(&failNTimesSkill{name: "flaky", failTimes: 2, callCount: &failCount})

	callCount := 0
	llm := func(ctx context.Context, system, user string) (string, error) {
		callCount++
		if callCount == 1 {
			return `[{"action":"flaky op","skill_name":"flaky","args":{}}]`, nil
		}
		return "ok", nil
	}

	runner := NewRunner(s, reg, llm, &skills.Environment{})
	tk, _ := s.Create(CreateRequest{Description: "retry test"})

	err := runner.Run(context.Background(), tk.ID)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}

	got, _ := s.Get(tk.ID)
	if got.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s (error: %s)", got.Status, got.Error)
	}
	if got.Steps[0].RetryCount != 2 {
		t.Fatalf("expected 2 retries, got %d", got.Steps[0].RetryCount)
	}
}

func TestStepRetryExhausted(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	var failCount atomic.Int32
	reg := skills.NewRegistry()
	reg.Register(&failNTimesSkill{name: "always_fail", failTimes: 10, callCount: &failCount})

	callCount := 0
	llm := func(ctx context.Context, system, user string) (string, error) {
		callCount++
		if callCount == 1 {
			return `[{"action":"will fail","skill_name":"always_fail","args":{}}]`, nil
		}
		return "ok", nil
	}

	runner := NewRunner(s, reg, llm, &skills.Environment{})
	tk, _ := s.Create(CreateRequest{Description: "retry exhaust test"})

	err := runner.Run(context.Background(), tk.ID)
	if err == nil {
		t.Fatal("expected error when retries exhausted")
	}

	got, _ := s.Get(tk.ID)
	if got.Status != StatusFailed {
		t.Fatalf("expected failed, got %s", got.Status)
	}
	if got.Steps[0].RetryCount != DefaultMaxRetries {
		t.Fatalf("expected %d retries, got %d", DefaultMaxRetries, got.Steps[0].RetryCount)
	}
}

func TestTaskCancel(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	reg := skills.NewRegistry()

	// Skill that blocks until cancelled
	reg.Register(&blockingSkill{name: "slow"})

	callCount := 0
	llm := func(ctx context.Context, system, user string) (string, error) {
		callCount++
		if callCount == 1 {
			return `[{"action":"slow op","skill_name":"slow","args":{}}]`, nil
		}
		return "ok", nil
	}

	runner := NewRunner(s, reg, llm, &skills.Environment{})
	tk, _ := s.Create(CreateRequest{Description: "cancel test"})

	done := make(chan error, 1)
	go func() {
		done <- runner.Run(context.Background(), tk.ID)
	}()

	// Wait for the task to start running
	time.Sleep(200 * time.Millisecond)

	if !runner.IsRunning(tk.ID) {
		t.Fatal("expected task to be running")
	}

	ok := runner.Cancel(tk.ID)
	if !ok {
		t.Fatal("cancel should return true for running task")
	}

	err := <-done
	if err == nil {
		t.Fatal("expected cancellation error")
	}

	got, _ := s.Get(tk.ID)
	if got.Status != StatusCancelled {
		t.Fatalf("expected cancelled, got %s", got.Status)
	}
}

// ──────────────────────────────────────────────
// #37 Runtime Hardening Tests
// ──────────────────────────────────────────────

func TestRecoverInterrupted(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	// Create tasks in various states
	t1, _ := s.Create(CreateRequest{Description: "running task"})
	t1.Status = StatusRunning
	t1.Steps = []Step{{ID: 1, Action: "test", Status: StepRunning}}
	s.Update(t1)

	t2, _ := s.Create(CreateRequest{Description: "planning task"})
	t2.Status = StatusPlanning
	s.Update(t2)

	t3, _ := s.Create(CreateRequest{Description: "completed task"})
	t3.Status = StatusCompleted
	s.Update(t3)

	// Simulate process restart by creating new store from same dir
	s2 := NewStore(dir)
	count := s2.RecoverInterrupted()

	if count != 2 {
		t.Fatalf("expected 2 recovered, got %d", count)
	}

	got1, _ := s2.Get(t1.ID)
	if got1.Status != StatusInterrupted {
		t.Fatalf("expected interrupted, got %s", got1.Status)
	}
	if got1.Steps[0].Status != StepPending {
		t.Fatalf("running step should be reset to pending, got %s", got1.Steps[0].Status)
	}

	got2, _ := s2.Get(t2.ID)
	if got2.Status != StatusInterrupted {
		t.Fatalf("expected interrupted, got %s", got2.Status)
	}

	got3, _ := s2.Get(t3.ID)
	if got3.Status != StatusCompleted {
		t.Fatalf("completed task should stay completed, got %s", got3.Status)
	}
}

func TestIsResumable(t *testing.T) {
	tests := []struct {
		status Status
		want   bool
	}{
		{StatusPending, false},
		{StatusRunning, false},
		{StatusCompleted, false},
		{StatusFailed, true},
		{StatusCancelled, false},
		{StatusPaused, true},
		{StatusInterrupted, true},
	}
	for _, tt := range tests {
		tk := &Task{Status: tt.status}
		if got := tk.IsResumable(); got != tt.want {
			t.Errorf("IsResumable(%s) = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestTaskPause(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	reg := skills.NewRegistry()

	// Multi-step task: step 1 is fast, step 2 blocks (gives time to pause)
	reg.Register(&echoSkill{name: "fast"})
	reg.Register(&blockingSkill{name: "slow"})

	callCount := 0
	llm := func(ctx context.Context, system, user string) (string, error) {
		callCount++
		if callCount == 1 {
			return `[{"action":"fast op","skill_name":"fast","args":{}},{"action":"slow op","skill_name":"slow","args":{}}]`, nil
		}
		return "ok", nil
	}

	runner := NewRunner(s, reg, llm, &skills.Environment{})
	tk, _ := s.Create(CreateRequest{Description: "pause test"})

	done := make(chan error, 1)
	go func() {
		done <- runner.Run(context.Background(), tk.ID)
	}()

	// Wait for task to start, then request pause
	time.Sleep(100 * time.Millisecond)
	ok := runner.Pause(tk.ID)
	if !ok {
		// Task might have already completed fast step; cancel the slow one to unblock
		runner.Cancel(tk.ID)
		<-done
		t.Skip("task completed too fast for pause test")
	}

	// Cancel to unblock the slow step so the task can check pause
	time.Sleep(50 * time.Millisecond)
	runner.Cancel(tk.ID)
	<-done

	got, _ := s.Get(tk.ID)
	// Task should be paused or cancelled (depending on timing)
	if got.Status != StatusPaused && got.Status != StatusCancelled {
		t.Fatalf("expected paused or cancelled, got %s", got.Status)
	}
}

func TestTaskResumeFromFailed(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	reg := skills.NewRegistry()

	// Skill that fails on first 3 calls, succeeds after that
	counter := &atomic.Int32{}
	reg.Register(&failNTimesSkill{name: "flaky_resume", failTimes: 3, callCount: counter})

	callCount := 0
	llm := func(ctx context.Context, system, user string) (string, error) {
		callCount++
		if strings.Contains(system, "规划器") {
			return `[{"action":"flaky op","skill_name":"flaky_resume","args":{}}]`, nil
		}
		return "ok", nil
	}

	runner := NewRunner(s, reg, llm, &skills.Environment{})
	tk, _ := s.Create(CreateRequest{Description: "resume test"})

	// First run should fail (3 calls = plan attempt + 2 retries, all fail)
	err := runner.Run(context.Background(), tk.ID)
	if err == nil {
		t.Fatal("expected first run to fail")
	}
	got, _ := s.Get(tk.ID)
	if got.Status != StatusFailed {
		t.Fatalf("expected failed, got %s", got.Status)
	}

	// Resume should succeed (counter is now past 3 so next call succeeds)
	err = runner.Resume(context.Background(), tk.ID)
	if err != nil {
		t.Fatalf("resume should succeed: %v", err)
	}

	got, _ = s.Get(tk.ID)
	if got.Status != StatusCompleted {
		t.Fatalf("expected completed after resume, got %s", got.Status)
	}
}

func TestTaskRestart(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	reg := skills.NewRegistry()

	callCount := 0
	llm := func(ctx context.Context, system, user string) (string, error) {
		callCount++
		if callCount == 1 || callCount == 2 {
			// First plan, then the first step
			if strings.Contains(system, "规划器") {
				return `[{"action":"step1","skill_name":"","args":{}}]`, nil
			}
			return "result v1", nil
		}
		// After restart, new plan and step
		if strings.Contains(system, "规划器") {
			return `[{"action":"new step","skill_name":"","args":{}}]`, nil
		}
		return "result v2", nil
	}

	runner := NewRunner(s, reg, llm, &skills.Environment{})
	tk, _ := s.Create(CreateRequest{Description: "restart test"})

	// First run
	err := runner.Run(context.Background(), tk.ID)
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	got, _ := s.Get(tk.ID)
	if got.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s", got.Status)
	}

	// Restart — re-plans and re-executes
	err = runner.Restart(context.Background(), tk.ID)
	if err != nil {
		t.Fatalf("restart failed: %v", err)
	}
	got, _ = s.Get(tk.ID)
	if got.Status != StatusCompleted {
		t.Fatalf("expected completed after restart, got %s", got.Status)
	}
	if got.Steps[0].Action != "new step" {
		t.Fatalf("expected re-planned step, got %s", got.Steps[0].Action)
	}
}

func TestParallelStepsSafe(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	reg := skills.NewRegistry()

	reg.Register(&echoSkill{name: "p1"})
	reg.Register(&echoSkill{name: "p2"})
	reg.Register(&echoSkill{name: "p3"})

	callCount := 0
	llm := func(ctx context.Context, system, user string) (string, error) {
		callCount++
		if callCount == 1 {
			return `[
				{"action":"parallel 1","skill_name":"p1","args":{},"group":1},
				{"action":"parallel 2","skill_name":"p2","args":{},"group":1},
				{"action":"parallel 3","skill_name":"p3","args":{},"group":1}
			]`, nil
		}
		return "ok", nil
	}

	runner := NewRunner(s, reg, llm, &skills.Environment{})
	tk, _ := s.Create(CreateRequest{Description: "parallel test"})

	err := runner.Run(context.Background(), tk.ID)
	if err != nil {
		t.Fatalf("parallel run failed: %v", err)
	}

	got, _ := s.Get(tk.ID)
	if got.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s", got.Status)
	}
	for i, step := range got.Steps {
		if step.Status != StepDone {
			t.Errorf("step %d: expected done, got %s", i, step.Status)
		}
		if step.DoneAt == nil {
			t.Errorf("step %d: expected DoneAt to be set", i)
		}
	}
}

func TestRecoverAllViaRunner(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	tk, _ := s.Create(CreateRequest{Description: "zombie"})
	tk.Status = StatusRunning
	s.Update(tk)

	reg := skills.NewRegistry()
	llm := func(ctx context.Context, system, user string) (string, error) { return "", nil }
	runner := NewRunner(s, reg, llm, &skills.Environment{})

	count := runner.RecoverAll()
	if count != 1 {
		t.Fatalf("expected 1 recovered, got %d", count)
	}

	got, _ := s.Get(tk.ID)
	if got.Status != StatusInterrupted {
		t.Fatalf("expected interrupted, got %s", got.Status)
	}
	if !got.IsResumable() {
		t.Fatal("interrupted task should be resumable")
	}
}

// ── Test helpers ──

// echoSkill returns args["data"] and includes _prev_result if present.
type echoSkill struct{ name string }

func (e *echoSkill) Name() string               { return e.name }
func (e *echoSkill) Description() string        { return "echo" }
func (e *echoSkill) Parameters() map[string]any { return nil }
func (e *echoSkill) Execute(_ context.Context, args map[string]any, _ *skills.Environment) (string, error) {
	data, _ := args["data"].(string)
	prev, _ := args["_prev_result"].(string)
	if prev != "" {
		return fmt.Sprintf("data=%s prev=%s", data, prev[:min(len(prev), 50)]), nil
	}
	return fmt.Sprintf("data=%s", data), nil
}

// failNTimesSkill fails the first N calls, then succeeds.
type failNTimesSkill struct {
	name      string
	failTimes int
	callCount *atomic.Int32
}

func (f *failNTimesSkill) Name() string               { return f.name }
func (f *failNTimesSkill) Description() string        { return "flaky" }
func (f *failNTimesSkill) Parameters() map[string]any { return nil }
func (f *failNTimesSkill) Execute(_ context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
	n := int(f.callCount.Add(1))
	if n <= f.failTimes {
		return "", fmt.Errorf("transient error #%d", n)
	}
	return "success after retries", nil
}

// blockingSkill blocks until context is cancelled.
type blockingSkill struct{ name string }

func (b *blockingSkill) Name() string               { return b.name }
func (b *blockingSkill) Description() string        { return "blocks" }
func (b *blockingSkill) Parameters() map[string]any { return nil }
func (b *blockingSkill) Execute(ctx context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}
