package react

import (
	"context"
	"path/filepath"
	"testing"

	"ledger"
	lsqlite "ledger/backend/sqlite"
)

func setupLedger(t *testing.T) *ledger.Ledger {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	b, err := lsqlite.New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	ldg, err := ledger.Open(b)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ldg.Close() })
	return ldg
}

func createTask(t *testing.T, ldg *ledger.Ledger, name string) string {
	t.Helper()
	task, err := ldg.Tasks.CreateTask(context.Background(), name, ledger.TaskTypeGoal, "test")
	if err != nil {
		t.Fatal(err)
	}
	return task.ID
}

func TestNewRunner(t *testing.T) {
	ldg := setupLedger(t)
	r := NewRunner(ldg)
	if r == nil {
		t.Fatal("NewRunner should not return nil")
	}
}

func TestReActLoopAnswer(t *testing.T) {
	ldg := setupLedger(t)
	ctx := context.Background()

	taskID := createTask(t, ldg, "test goal")

	r := NewRunner(ldg)

	thinkCalls := 0
	think := func(ctx context.Context, history []ledger.ReActStep) (*ledger.ThinkResult, error) {
		thinkCalls++
		// Return answer immediately
		return &ledger.ThinkResult{
			Thought:    "I know the answer",
			Confidence: 0.9,
			Answer:     "42",
		}, nil
	}

	act := func(ctx context.Context, call ledger.ToolCall) (*ledger.ToolResult, error) {
		return &ledger.ToolResult{Output: "ok"}, nil
	}

	cfg := ledger.ReActConfig{}
	result, err := r.ReActLoop(ctx, taskID, "initial obs", cfg, think, act, nil)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Success {
		t.Error("expected success")
	}
	if result.Answer != "42" {
		t.Errorf("Answer = %q, want 42", result.Answer)
	}
	if result.StopReason != "answer" {
		t.Errorf("StopReason = %q, want answer", result.StopReason)
	}
	if result.TotalSteps != 1 {
		t.Errorf("TotalSteps = %d, want 1", result.TotalSteps)
	}
}

func TestReActLoopWithToolCalls(t *testing.T) {
	ldg := setupLedger(t)
	ctx := context.Background()
	taskID := createTask(t, ldg, "tool test")

	r := NewRunner(ldg)

	callNum := 0
	think := func(ctx context.Context, history []ledger.ReActStep) (*ledger.ThinkResult, error) {
		callNum++
		if callNum == 1 {
			return &ledger.ThinkResult{
				Thought:    "need to search",
				Confidence: 0.7,
				Action:     &ledger.ToolCall{Name: "search", Args: map[string]interface{}{"q": "test"}},
			}, nil
		}
		return &ledger.ThinkResult{
			Thought:    "got result",
			Confidence: 0.95,
			Answer:     "found it",
		}, nil
	}

	act := func(ctx context.Context, call ledger.ToolCall) (*ledger.ToolResult, error) {
		return &ledger.ToolResult{Output: "search result: something"}, nil
	}

	cfg := ledger.ReActConfig{}
	result, err := r.ReActLoop(ctx, taskID, "", cfg, think, act, nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.TotalSteps != 2 {
		t.Errorf("TotalSteps = %d, want 2", result.TotalSteps)
	}
	if result.Answer != "found it" {
		t.Errorf("Answer = %q", result.Answer)
	}
}

func TestReActLoopMaxSteps(t *testing.T) {
	ldg := setupLedger(t)
	ctx := context.Background()
	taskID := createTask(t, ldg, "max test")

	r := NewRunner(ldg)

	think := func(ctx context.Context, history []ledger.ReActStep) (*ledger.ThinkResult, error) {
		return &ledger.ThinkResult{
			Thought:    "still thinking",
			Confidence: 0.5,
			Action:     &ledger.ToolCall{Name: "noop"},
		}, nil
	}

	act := func(ctx context.Context, call ledger.ToolCall) (*ledger.ToolResult, error) {
		return &ledger.ToolResult{Output: "ok"}, nil
	}

	cfg := ledger.ReActConfig{MaxSteps: 3}
	result, err := r.ReActLoop(ctx, taskID, "obs", cfg, think, act, nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.StopReason != "max_steps" {
		t.Errorf("StopReason = %q, want max_steps", result.StopReason)
	}
	if result.TotalSteps != 3 {
		t.Errorf("TotalSteps = %d, want 3", result.TotalSteps)
	}
}

func TestReActLoopCancel(t *testing.T) {
	ldg := setupLedger(t)
	ctx, cancel := context.WithCancel(context.Background())
	taskID := createTask(t, ldg, "cancel test")

	r := NewRunner(ldg)

	think := func(ctx context.Context, history []ledger.ReActStep) (*ledger.ThinkResult, error) {
		cancel() // cancel on first call
		return &ledger.ThinkResult{
			Thought: "will be cancelled",
			Action:  &ledger.ToolCall{Name: "test"},
		}, nil
	}

	act := func(ctx context.Context, call ledger.ToolCall) (*ledger.ToolResult, error) {
		return &ledger.ToolResult{Output: "ok"}, nil
	}

	cfg := ledger.ReActConfig{}
	result, _ := r.ReActLoop(ctx, taskID, "obs", cfg, think, act, nil)

	if result.StopReason != "cancelled" {
		t.Errorf("StopReason = %q, want cancelled", result.StopReason)
	}
}

func TestReActLoopOnStep(t *testing.T) {
	ldg := setupLedger(t)
	ctx := context.Background()
	taskID := createTask(t, ldg, "onstep test")

	r := NewRunner(ldg)
	var steps []ledger.ReActStep

	think := func(ctx context.Context, history []ledger.ReActStep) (*ledger.ThinkResult, error) {
		return &ledger.ThinkResult{Thought: "done", Confidence: 1.0, Answer: "ok"}, nil
	}
	act := func(ctx context.Context, call ledger.ToolCall) (*ledger.ToolResult, error) {
		return &ledger.ToolResult{Output: "ok"}, nil
	}

	cfg := ledger.ReActConfig{}
	r.ReActLoop(ctx, taskID, "obs", cfg, think, act, func(step ledger.ReActStep) {
		steps = append(steps, step)
	})

	if len(steps) != 1 {
		t.Errorf("onStep called %d times, want 1", len(steps))
	}
}

func TestReActLoopBacktrack(t *testing.T) {
	ldg := setupLedger(t)
	ctx := context.Background()
	taskID := createTask(t, ldg, "backtrack test")

	r := NewRunner(ldg)

	callNum := 0
	think := func(ctx context.Context, history []ledger.ReActStep) (*ledger.ThinkResult, error) {
		callNum++
		if callNum <= 2 {
			return &ledger.ThinkResult{
				Thought: "try tool",
				Action:  &ledger.ToolCall{Name: "failing_tool"},
			}, nil
		}
		return &ledger.ThinkResult{Thought: "done", Answer: "recovered"}, nil
	}

	act := func(ctx context.Context, call ledger.ToolCall) (*ledger.ToolResult, error) {
		return &ledger.ToolResult{Error: "tool error"}, nil
	}

	cfg := ledger.ReActConfig{BacktrackOnFail: true}
	result, err := r.ReActLoop(ctx, taskID, "", cfg, think, act, nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.Backtracks != 2 {
		t.Errorf("Backtracks = %d, want 2", result.Backtracks)
	}
	if result.Answer != "recovered" {
		t.Errorf("Answer = %q, want recovered", result.Answer)
	}
}
