package task

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestWorkingMemoryInit(t *testing.T) {
	mgr := NewWorkingMemoryManager(nil)
	tsk := &Task{
		ID:          "t1",
		Description: "Build a web scraper",
	}
	mgr.Init(tsk)

	wm := mgr.Get("t1")
	if wm == nil {
		t.Fatal("expected working memory")
	}
	if wm.Goal != "Build a web scraper" {
		t.Fatalf("expected goal 'Build a web scraper', got '%s'", wm.Goal)
	}
}

func TestWorkingMemoryUpdateAfterStep(t *testing.T) {
	mgr := NewWorkingMemoryManager(nil)
	tsk := &Task{ID: "t2", Description: "Analyze data"}
	mgr.Init(tsk)

	// Complete a step
	step := &Step{
		ID:     1,
		Action: "Fetch data from API",
		Status: StepDone,
		Result: "Retrieved 100 records",
	}
	mgr.UpdateAfterStep(tsk, step)

	wm := mgr.Get("t2")
	if len(wm.CompletedWork) != 1 {
		t.Fatalf("expected 1 completed work, got %d", len(wm.CompletedWork))
	}
	if !strings.Contains(wm.CompletedWork[0], "Fetch data") {
		t.Fatalf("completed work should reference action: %s", wm.CompletedWork[0])
	}
}

func TestWorkingMemoryFailedStep(t *testing.T) {
	mgr := NewWorkingMemoryManager(nil)
	tsk := &Task{ID: "t3", Description: "Deploy service"}
	mgr.Init(tsk)

	// Failed step
	step := &Step{
		ID:     1,
		Action: "Run deploy script",
		Status: StepFailed,
		Error:  "connection refused",
	}
	mgr.UpdateAfterStep(tsk, step)

	wm := mgr.Get("t3")
	if len(wm.Blockers) != 1 {
		t.Fatalf("expected 1 blocker, got %d", len(wm.Blockers))
	}
	if !strings.Contains(wm.Blockers[0], "connection refused") {
		t.Fatalf("blocker should contain error: %s", wm.Blockers[0])
	}
}

func TestWorkingMemoryRender(t *testing.T) {
	wm := &WorkingMemory{
		TaskID:        "t4",
		Goal:          "Generate report",
		CompletedWork: []string{"Collected data → 500 rows"},
		Blockers:      []string{"API rate limited"},
		Confirmed:     []string{"Use CSV format"},
		Pending:       []string{"Include charts?"},
		Artifacts:     []string{"data.csv (data/output/data.csv)"},
		NextAction:    "Format output",
	}

	rendered := wm.Render()
	checks := []string{
		"任务工作记忆",
		"t4",
		"Generate report",
		"已完成",
		"Collected data",
		"阻塞",
		"API rate limited",
		"已确认",
		"Use CSV format",
		"待确认",
		"Include charts?",
		"产物",
		"data.csv",
		"下一步",
		"Format output",
	}
	for _, check := range checks {
		if !strings.Contains(rendered, check) {
			t.Errorf("render missing '%s' in:\n%s", check, rendered)
		}
	}
}

func TestWorkingMemoryAddConfirmedAndPending(t *testing.T) {
	mgr := NewWorkingMemoryManager(nil)
	tsk := &Task{ID: "t5", Description: "Test task"}
	mgr.Init(tsk)

	// Add pending
	mgr.AddPending("t5", "Should we use JSON or CSV?")
	wm := mgr.Get("t5")
	if len(wm.Pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(wm.Pending))
	}

	// Confirm
	mgr.AddConfirmed("t5", "Should we use JSON or CSV?")
	wm = mgr.Get("t5")
	if len(wm.Pending) != 0 {
		t.Fatalf("expected 0 pending after confirm, got %d", len(wm.Pending))
	}
	if len(wm.Confirmed) != 1 {
		t.Fatalf("expected 1 confirmed, got %d", len(wm.Confirmed))
	}
}

func TestWorkingMemoryCleanup(t *testing.T) {
	mgr := NewWorkingMemoryManager(nil)
	tsk := &Task{ID: "t6", Description: "Temp task"}
	mgr.Init(tsk)

	mgr.Cleanup("t6")
	if mgr.Get("t6") != nil {
		t.Fatal("expected nil after cleanup")
	}
}

func TestWorkingMemoryTokenEstimate(t *testing.T) {
	wm := &WorkingMemory{
		TaskID: "t7",
		Goal:   "A very simple task",
	}
	tokens := wm.estimateTokens()
	if tokens <= 0 {
		t.Fatal("expected positive token estimate")
	}
}

func TestWorkingMemoryCompress(t *testing.T) {
	// Use a mock LLM that returns a condensed version
	mgr := NewWorkingMemoryManager(func(ctx context.Context, system, user string) (string, error) {
		return "压缩后的工作记忆摘要", nil
	})
	tsk := &Task{ID: "t8", Description: "Complex task"}
	mgr.Init(tsk)

	// Add lots of completed work to exceed threshold
	mgr.mu.Lock()
	wm := mgr.memories["t8"]
	for i := 0; i < 100; i++ {
		wm.CompletedWork = append(wm.CompletedWork, strings.Repeat("已完成很长的步骤描述", 10))
	}
	mgr.mu.Unlock()

	err := mgr.Compress(context.Background(), "t8")
	if err != nil {
		t.Fatal(err)
	}

	wm = mgr.Get("t8")
	if len(wm.CompletedWork) != 1 {
		t.Fatalf("expected compressed to 1 entry, got %d", len(wm.CompletedWork))
	}
	if wm.CompletedWork[0] != "压缩后的工作记忆摘要" {
		t.Fatalf("unexpected compressed content: %s", wm.CompletedWork[0])
	}
}

func TestCondenseFact(t *testing.T) {
	// Short result
	fact := condenseFact("Search web", "Found 5 results")
	if fact != "Search web → Found 5 results" {
		t.Fatalf("unexpected: %s", fact)
	}

	// Long result gets truncated
	longResult := strings.Repeat("x", 200)
	fact = condenseFact("Long task", longResult)
	if len([]rune(fact)) > 100 {
		t.Fatalf("fact too long: %d runes", len([]rune(fact)))
	}

	// Empty result
	fact = condenseFact("Simple", "")
	if fact != "Simple → 完成" {
		t.Fatalf("unexpected: %s", fact)
	}
}

func TestRenderForTask(t *testing.T) {
	mgr := NewWorkingMemoryManager(nil)

	// Non-existent task
	if mgr.RenderForTask("nonexistent") != "" {
		t.Fatal("expected empty for nonexistent")
	}

	tsk := &Task{ID: "t9", Description: "Test render"}
	mgr.Init(tsk)
	step := &Step{ID: 1, Action: "Step 1", Status: StepDone, Result: "OK", DoneAt: ptr(time.Now())}
	mgr.UpdateAfterStep(tsk, step)

	rendered := mgr.RenderForTask("t9")
	if !strings.Contains(rendered, "Step 1") {
		t.Fatal("missing step in rendered output")
	}
}

func ptr(t time.Time) *time.Time { return &t }

// ── New tests for #34 Task Working Memory enhancements ──

func TestWorkingMemoryPersistence(t *testing.T) {
	dir := t.TempDir()

	// Create manager and add data
	mgr1 := NewWorkingMemoryManagerWithPersistence(nil, dir)
	tsk := &Task{ID: "t-persist", Description: "Persistent task"}
	mgr1.Init(tsk)
	mgr1.AddPending("t-persist", "Use CSV?")
	mgr1.AddConfirmed("t-persist", "Use CSV?")

	// Create new manager from same dir — should restore
	mgr2 := NewWorkingMemoryManagerWithPersistence(nil, dir)
	wm := mgr2.Get("t-persist")
	if wm == nil {
		t.Fatal("expected working memory after reload")
	}
	if wm.Goal != "Persistent task" {
		t.Fatalf("expected goal 'Persistent task', got '%s'", wm.Goal)
	}
	if len(wm.Confirmed) != 1 || wm.Confirmed[0] != "Use CSV?" {
		t.Fatalf("expected confirmed [Use CSV?], got %v", wm.Confirmed)
	}
	if len(wm.Pending) != 0 {
		t.Fatalf("expected 0 pending, got %d", len(wm.Pending))
	}
}

func TestExtractConfirmFromThread(t *testing.T) {
	mgr := NewWorkingMemoryManager(nil)
	tsk := &Task{ID: "t-confirm", Description: "Test confirm"}
	mgr.Init(tsk)
	mgr.AddPending("t-confirm", "使用 JSON 还是 CSV 格式?")

	// Non-confirmation message: should not extract
	ok := mgr.ExtractConfirmFromThread("t-confirm", "这个任务怎么样了？")
	if ok {
		t.Fatal("should not detect confirmation from neutral message")
	}

	// Confirmation message: should extract
	ok = mgr.ExtractConfirmFromThread("t-confirm", "好的，确认使用 JSON")
	if !ok {
		t.Fatal("should detect confirmation from '好的'")
	}

	wm := mgr.Get("t-confirm")
	if len(wm.Pending) != 0 {
		t.Fatalf("expected 0 pending after confirm, got %d", len(wm.Pending))
	}
	if len(wm.Confirmed) != 1 {
		t.Fatalf("expected 1 confirmed, got %d", len(wm.Confirmed))
	}
	if wm.Confirmed[0] != "使用 JSON 还是 CSV 格式?" {
		t.Fatalf("unexpected confirmed: %s", wm.Confirmed[0])
	}
}

func TestExtractConfirmNoPending(t *testing.T) {
	mgr := NewWorkingMemoryManager(nil)
	tsk := &Task{ID: "t-no-pending", Description: "No pending"}
	mgr.Init(tsk)

	// No pending items → should return false even with confirmation
	ok := mgr.ExtractConfirmFromThread("t-no-pending", "确认")
	if ok {
		t.Fatal("should not extract when no pending items")
	}
}

func TestSummarizerInterface(t *testing.T) {
	// Create a custom summarizer
	customSummarizer := &mockSummarizer{result: "自定义压缩结果"}

	mgr := NewWorkingMemoryManager(nil)
	mgr.SetSummarizer(customSummarizer)

	tsk := &Task{ID: "t-custom", Description: "Custom summarizer test"}
	mgr.Init(tsk)

	// Add lots of data to exceed threshold
	mgr.mu.Lock()
	wm := mgr.memories["t-custom"]
	for i := 0; i < 100; i++ {
		wm.CompletedWork = append(wm.CompletedWork, strings.Repeat("长步骤描述内容", 10))
	}
	mgr.mu.Unlock()

	err := mgr.Compress(context.Background(), "t-custom")
	if err != nil {
		t.Fatal(err)
	}

	wm = mgr.Get("t-custom")
	if wm.CompletedWork[0] != "自定义压缩结果" {
		t.Fatalf("expected custom summarizer result, got: %s", wm.CompletedWork[0])
	}
}

type mockSummarizer struct {
	result string
}

func (s *mockSummarizer) Summarize(ctx context.Context, text string) (string, error) {
	return s.result, nil
}

func TestGetAll(t *testing.T) {
	mgr := NewWorkingMemoryManager(nil)
	mgr.Init(&Task{ID: "a", Description: "Task A"})
	mgr.Init(&Task{ID: "b", Description: "Task B"})

	all := mgr.GetAll()
	if len(all) != 2 {
		t.Fatalf("expected 2 memories, got %d", len(all))
	}
	if all["a"] == nil || all["b"] == nil {
		t.Fatal("expected both tasks in GetAll")
	}
}
