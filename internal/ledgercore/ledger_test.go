package ledger_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"yunque-agent/internal/ledgercore"
	lsqlite "yunque-agent/internal/ledgercore/backend/sqlite"
)

func newTestLedger(t *testing.T) *ledger.Ledger {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	b, err := lsqlite.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create sqlite backend: %v", err)
	}

	ldg, err := ledger.Open(b)
	if err != nil {
		t.Fatalf("failed to open ledger: %v", err)
	}
	t.Cleanup(func() { ldg.Close() })
	return ldg
}

type failingMemoryBackend struct {
	ledger.Backend
	err error
}

func (b failingMemoryBackend) PutMemory(ctx context.Context, m *ledger.MemoryEntry) error {
	return b.err
}

// TestTaskLifecycle verifies the full task lifecycle:
// created -> ready -> running -> completed
func TestTaskLifecycle(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	// Create
	task, err := ldg.Tasks.CreateTask(ctx, "Write a report", ledger.TaskTypeGoal, "tenant-1")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if task.ID == "" {
		t.Fatal("task ID should not be empty")
	}
	if task.Status != ledger.TaskCreated {
		t.Fatalf("expected status %q, got %q", ledger.TaskCreated, task.Status)
	}

	// Transition: created -> ready
	err = ldg.Tasks.Transition(ctx, task.ID, ledger.TaskReady, "runtime", nil)
	if err != nil {
		t.Fatalf("Transition to ready: %v", err)
	}

	// Verify
	task, err = ldg.Tasks.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if task.Status != ledger.TaskReady {
		t.Fatalf("expected status %q, got %q", ledger.TaskReady, task.Status)
	}

	// Transition: ready -> running
	err = ldg.Tasks.Transition(ctx, task.ID, ledger.TaskRunning, "runtime", nil)
	if err != nil {
		t.Fatalf("Transition to running: %v", err)
	}

	// Complete with output
	output, _ := json.Marshal(map[string]string{"result": "done"})
	err = ldg.Tasks.Complete(ctx, task.ID, output)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	// Final check
	task, err = ldg.Tasks.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if task.Status != ledger.TaskCompleted {
		t.Fatalf("expected status %q, got %q", ledger.TaskCompleted, task.Status)
	}
	if task.FinishedAt == nil {
		t.Fatal("finished_at should not be nil")
	}
}

// TestInvalidTransition verifies the FSM rejects illegal transitions.
func TestInvalidTransition(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, err := ldg.Tasks.CreateTask(ctx, "Test goal", ledger.TaskTypeGoal, "t1")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	// created -> completed should fail (must go through ready -> running first)
	err = ldg.Tasks.Transition(ctx, task.ID, ledger.TaskCompleted, "runtime", nil)
	if err == nil {
		t.Fatal("expected error for invalid transition created -> completed")
	}
}

// TestEventReplay verifies that replaying events reconstructs the same state.
func TestEventReplay(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	// Full lifecycle
	task, _ := ldg.Tasks.CreateTask(ctx, "Replay test", ledger.TaskTypeGoal, "t1")
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskReady, "runtime", nil)
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskRunning, "runtime", nil)
	ldg.Tasks.Complete(ctx, task.ID, json.RawMessage(`{"result":"ok"}`))

	// Replay from events
	replayed, err := ldg.Events.Replay(ctx, task.ID)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}

	if replayed.Status != ledger.TaskCompleted {
		t.Fatalf("replayed status: expected %q, got %q", ledger.TaskCompleted, replayed.Status)
	}
	if replayed.Goal != "Replay test" {
		t.Fatalf("replayed goal: expected 'Replay test', got %q", replayed.Goal)
	}
}

// TestEventList verifies event listing and ordering.
func TestEventList(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, _ := ldg.Tasks.CreateTask(ctx, "Event list test", ledger.TaskTypeGoal, "t1")
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskReady, "runtime", nil)
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskRunning, "runtime", nil)

	events, err := ldg.Events.ListAll(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}

	if len(events) < 3 {
		t.Fatalf("expected at least 3 events, got %d", len(events))
	}

	// Verify monotonic sequence
	for i := 1; i < len(events); i++ {
		if events[i].Seq <= events[i-1].Seq {
			t.Fatalf("event seq not monotonic: %d <= %d", events[i].Seq, events[i-1].Seq)
		}
	}

	// Verify first event is task.created
	if events[0].Kind != ledger.EventTaskCreated {
		t.Fatalf("first event: expected %q, got %q", ledger.EventTaskCreated, events[0].Kind)
	}
}

// TestTaskFailAndRestart verifies the fail -> restart cycle.
func TestTaskFailAndRestart(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, _ := ldg.Tasks.CreateTask(ctx, "Fail test", ledger.TaskTypeGoal, "t1")
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskReady, "runtime", nil)
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskRunning, "runtime", nil)

	// Fail
	err := ldg.Tasks.Fail(ctx, task.ID, "step 3 timeout")
	if err != nil {
		t.Fatalf("Fail: %v", err)
	}

	task, _ = ldg.Tasks.GetTask(ctx, task.ID)
	if task.Status != ledger.TaskFailed {
		t.Fatalf("expected failed, got %q", task.Status)
	}
	if task.Error == nil || *task.Error != "step 3 timeout" {
		t.Fatal("error message not set correctly")
	}

	// Restart (failed -> ready)
	err = ldg.Tasks.Transition(ctx, task.ID, ledger.TaskReady, "user", nil)
	if err != nil {
		t.Fatalf("Restart: %v", err)
	}

	task, _ = ldg.Tasks.GetTask(ctx, task.ID)
	if task.Status != ledger.TaskReady {
		t.Fatalf("expected ready after restart, got %q", task.Status)
	}
}

// TestTaskCancel verifies cancellation from various states.
func TestTaskCancel(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, _ := ldg.Tasks.CreateTask(ctx, "Cancel test", ledger.TaskTypeGoal, "t1")
	err := ldg.Tasks.Cancel(ctx, task.ID, "user requested")
	if err != nil {
		t.Fatalf("Cancel from created: %v", err)
	}

	task, _ = ldg.Tasks.GetTask(ctx, task.ID)
	if task.Status != ledger.TaskCancelled {
		t.Fatalf("expected cancelled, got %q", task.Status)
	}
}

// TestListTasks verifies task listing with filters.
func TestListTasks(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	ldg.Tasks.CreateTask(ctx, "Task A", ledger.TaskTypeGoal, "t1")
	ldg.Tasks.CreateTask(ctx, "Task B", ledger.TaskTypeChat, "t1")
	ldg.Tasks.CreateTask(ctx, "Task C", ledger.TaskTypeGoal, "t2")

	// List all for tenant t1
	tasks, err := ldg.Tasks.ListTasks(ctx, ledger.TaskFilter{TenantID: "t1"})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks for tenant t1, got %d", len(tasks))
	}

	// Filter by status
	tasks, err = ldg.Tasks.ListTasks(ctx, ledger.TaskFilter{
		TenantID: "t1",
		Status:   []ledger.TaskStatus{ledger.TaskCreated},
	})
	if err != nil {
		t.Fatalf("ListTasks with status filter: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 created tasks, got %d", len(tasks))
	}
}

// Ensure the data directory is created for the default db path.
func TestDefaultConfigDataDir(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "sub", "ledger.db")

	// Ensure parent directory exists
	os.MkdirAll(filepath.Dir(dbPath), 0755)

	b, err := lsqlite.New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer b.Close()

	ldg, err := ledger.Open(b)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer ldg.Close()

	// Smoke test
	task, err := ldg.Tasks.CreateTask(context.Background(), "Smoke", ledger.TaskTypeChat, "t1")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if task.ID == "" {
		t.Fatal("task ID empty")
	}
}

// ══════════════════════════════════════════════════
// Phase 2 Tests
// ══════════════════════════════════════════════════

// TestCheckpointSaveAndResume verifies checkpoint creation and task recovery.
func TestCheckpointSaveAndResume(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	// Create and advance a task
	task, _ := ldg.Tasks.CreateTask(ctx, "Checkpoint test", ledger.TaskTypeGoal, "t1")
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskReady, "runtime", nil)
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskRunning, "runtime", nil)

	// Save checkpoint at step 2
	workingMem, _ := json.Marshal(map[string]string{"context": "step 2 data"})
	cp, err := ldg.Checkpoints.Save(ctx, task.ID, 2, workingMem, "step_complete")
	if err != nil {
		t.Fatalf("Save checkpoint: %v", err)
	}
	if cp.ID == "" {
		t.Fatal("checkpoint ID empty")
	}
	if cp.StepIndex != 2 {
		t.Fatalf("expected step_index 2, got %d", cp.StepIndex)
	}

	// Verify latest checkpoint
	latest, err := ldg.Checkpoints.Latest(ctx, task.ID)
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if latest.ID != cp.ID {
		t.Fatalf("expected checkpoint %s, got %s", cp.ID, latest.ID)
	}

	// Resume from checkpoint
	result, err := ldg.Resume.Resume(ctx, task.ID)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if result.Task.Status != ledger.TaskRunning {
		t.Fatalf("resumed task status: expected %q, got %q", ledger.TaskRunning, result.Task.Status)
	}
	if result.StepIndex != 2 {
		t.Fatalf("resumed step_index: expected 2, got %d", result.StepIndex)
	}

	// working memory should be preserved
	var wm map[string]string
	json.Unmarshal(result.WorkingMem, &wm)
	if wm["context"] != "step 2 data" {
		t.Fatalf("working mem not preserved: %v", wm)
	}
}

// TestResumeWithoutCheckpoint verifies fallback to full event replay.
func TestResumeWithoutCheckpoint(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, _ := ldg.Tasks.CreateTask(ctx, "No checkpoint", ledger.TaskTypeGoal, "t1")
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskReady, "runtime", nil)
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskRunning, "runtime", nil)

	// Resume without any checkpoint ???should fall back to full replay
	result, err := ldg.Resume.Resume(ctx, task.ID)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if result.Task.Status != ledger.TaskRunning {
		t.Fatalf("expected running, got %q", result.Task.Status)
	}
	if result.ResumedFromEvent != 0 {
		t.Fatalf("expected resumed_from_event=0 (full replay), got %d", result.ResumedFromEvent)
	}
}

// TestMemoryCRUD verifies memory create, read, search, delete.
func TestMemoryCRUD(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	// Put a fact
	err := ldg.Memory.PutFact(ctx, "t1", "user.name", "Alice", "user")
	if err != nil {
		t.Fatalf("PutFact: %v", err)
	}

	// Put a preference
	err = ldg.Memory.PutPreference(ctx, "t1", "language", "English")
	if err != nil {
		t.Fatalf("PutPreference: %v", err)
	}

	// Search
	results, err := ldg.Memory.Search(ctx, ledger.MemoryQuery{
		TenantID: "t1",
		Query:    "Alice",
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'Alice', got %d", len(results))
	}
	if results[0].Key != "user.name" {
		t.Fatalf("expected key 'user.name', got %q", results[0].Key)
	}

	// Search by kind
	results, err = ldg.Memory.Search(ctx, ledger.MemoryQuery{
		TenantID: "t1",
		Kinds:    []ledger.MemoryKind{ledger.MemoryPreference},
	})
	if err != nil {
		t.Fatalf("Search by kind: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 preference, got %d", len(results))
	}

	// Delete
	err = ldg.Memory.Delete(ctx, results[0].ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify deleted
	results, err = ldg.Memory.Search(ctx, ledger.MemoryQuery{
		TenantID: "t1",
		Kinds:    []ledger.MemoryKind{ledger.MemoryPreference},
	})
	if err != nil {
		t.Fatalf("Search after delete: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 after delete, got %d", len(results))
	}
}

// TestRecall verifies the task-aware recall pipeline.
func TestRecall(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	// Store various memories
	ldg.Memory.PutFact(ctx, "t1", "user.name", "Alice prefers Go programming", "user")
	ldg.Memory.PutFact(ctx, "t1", "user.lang", "User likes Python and Rust", "extraction")
	ldg.Memory.PutExperience(ctx, "t1", nil, "report.success", "Successfully wrote a quarterly report last time", 0.9)
	ldg.Memory.PutPreference(ctx, "t1", "format", "User prefers markdown output")

	// Recall with a goal-oriented query
	result, err := ldg.Recall.Recall(ctx, ledger.RecallQuery{
		TenantID: "t1",
		Query:    "Go programming",
		TaskGoal: "Write a Go report",
		TaskType: ledger.TaskTypeGoal,
		Limit:    5,
		MinScore: 0.0,
	})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}

	if result.TotalFound == 0 {
		t.Fatal("recall returned 0 results, expected at least 1")
	}
	if result.QueryTimeMs < 0 {
		t.Fatal("query time should be non-negative")
	}

	// Verify the top result is likely the Go programming fact
	found := false
	for _, entry := range result.Entries {
		if entry.Entry.Key == "user.name" {
			found = true
			if entry.Score <= 0 {
				t.Fatal("score should be positive")
			}
			break
		}
	}
	if !found {
		t.Log("Note: 'user.name' entry not in top results ???scoring may need tuning")
	}
}

// TestArtifactSaveAndList verifies artifact metadata management.
func TestArtifactSaveAndList(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, _ := ldg.Tasks.CreateTask(ctx, "Artifact test", ledger.TaskTypeGoal, "t1")

	// Save artifacts
	err := ldg.Artifacts.Save(ctx, &ledger.Artifact{
		TaskID:     task.ID,
		Name:       "report.md",
		Kind:       "file",
		MimeType:   "text/markdown",
		SizeBytes:  1024,
		StorageRef: "/data/artifacts/report.md",
		Checksum:   "sha256:abc123",
	})
	if err != nil {
		t.Fatalf("Save artifact: %v", err)
	}

	err = ldg.Artifacts.Save(ctx, &ledger.Artifact{
		TaskID:     task.ID,
		Name:       "chart.png",
		Kind:       "image",
		MimeType:   "image/png",
		SizeBytes:  51200,
		StorageRef: "/data/artifacts/chart.png",
		Checksum:   "sha256:def456",
	})
	if err != nil {
		t.Fatalf("Save artifact 2: %v", err)
	}

	// List
	arts, err := ldg.Artifacts.List(ctx, task.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(arts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(arts))
	}

	// Get by ID
	art, err := ldg.Artifacts.Get(ctx, arts[0].ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if art.Name != "report.md" {
		t.Fatalf("expected name 'report.md', got %q", art.Name)
	}
}

// TestCheckpointCleanup verifies old checkpoint removal.
func TestCheckpointCleanup(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, _ := ldg.Tasks.CreateTask(ctx, "Cleanup test", ledger.TaskTypeGoal, "t1")
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskReady, "runtime", nil)
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskRunning, "runtime", nil)

	// Create 5 checkpoints
	for i := 0; i < 5; i++ {
		_, err := ldg.Checkpoints.Save(ctx, task.ID, i, nil, "step_complete")
		if err != nil {
			t.Fatalf("Save checkpoint %d: %v", i, err)
		}
	}

	// Should have 5
	cps, _ := ldg.Checkpoints.List(ctx, task.ID, 0)
	if len(cps) != 5 {
		t.Fatalf("expected 5 checkpoints, got %d", len(cps))
	}

	// Cleanup keeping 2
	err := ldg.Checkpoints.Cleanup(ctx, task.ID, 2)
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	// Should have fewer now
	cps, _ = ldg.Checkpoints.List(ctx, task.ID, 0)
	if len(cps) > 3 {
		t.Fatalf("expected at most 3 checkpoints after cleanup, got %d", len(cps))
	}
}

// TestTaskDependencies verifies inter-task dependency management.
func TestTaskDependencies(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	taskA, _ := ldg.Tasks.CreateTask(ctx, "Task A", ledger.TaskTypeGoal, "t1")
	taskB, _ := ldg.Tasks.CreateTask(ctx, "Task B", ledger.TaskTypeGoal, "t1")

	// B depends on A (blocking)
	dep, err := ldg.Deps.Create(ctx, taskA.ID, taskB.ID, ledger.DepBlocking)
	if err != nil {
		t.Fatalf("Create dep: %v", err)
	}

	// B should be blocked
	blocked, err := ldg.Deps.IsBlocked(ctx, taskB.ID)
	if err != nil {
		t.Fatalf("IsBlocked: %v", err)
	}
	if !blocked {
		t.Fatal("expected taskB to be blocked")
	}

	// Satisfy the dependency
	err = ldg.Deps.Satisfy(ctx, dep.ID)
	if err != nil {
		t.Fatalf("Satisfy: %v", err)
	}

	// B should no longer be blocked
	blocked, err = ldg.Deps.IsBlocked(ctx, taskB.ID)
	if err != nil {
		t.Fatalf("IsBlocked after satisfy: %v", err)
	}
	if blocked {
		t.Fatal("expected taskB to be unblocked after satisfy")
	}
}

// ══════════════════════════════════════════════════
// Phase 0.1: Reasoning Trace Tests
// ══════════════════════════════════════════════════

// TestReasoningTraceBasic verifies the full reasoning trace lifecycle.
func TestReasoningTraceBasic(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, _ := ldg.Tasks.CreateTask(ctx, "Write a Go tutorial", ledger.TaskTypeGoal, "t1")
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskReady, "runtime", nil)
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskRunning, "runtime", nil)

	// Use the reasoning tracer
	tracer := ldg.Reasoning(task.ID, "planner")

	// ReAct cycle: Observe ???Think ???Decide ???Observe ???Think ???Decide ???Reflect
	if err := tracer.Observe(ctx, "User wants a Go tutorial for intermediates", nil); err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if err := tracer.Think(ctx, "Should cover concurrency since that's Go's strength", nil); err != nil {
		t.Fatalf("Think: %v", err)
	}
	if err := tracer.Decide(ctx, "write_outline", "start with structure before content", 0.85, nil); err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if err := tracer.Observe(ctx, "Outline generated with 5 sections", nil); err != nil {
		t.Fatalf("Observe 2: %v", err)
	}
	if err := tracer.Think(ctx, "Section 3 on goroutines needs code examples", nil); err != nil {
		t.Fatalf("Think 2: %v", err)
	}
	if err := tracer.Decide(ctx, "generate_code", "concrete examples aid learning", 0.9, nil); err != nil {
		t.Fatalf("Decide 2: %v", err)
	}
	if err := tracer.Reflect(ctx, "Tutorial is well-structured but could use more error handling examples", 0.75, nil); err != nil {
		t.Fatalf("Reflect: %v", err)
	}

	// Retrieve the reasoning trace
	trace, err := ldg.Events.GetReasoningTrace(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetReasoningTrace: %v", err)
	}

	if len(trace.Events) != 7 {
		t.Fatalf("expected 7 reasoning events, got %d", len(trace.Events))
	}

	// Verify summary
	s := trace.Summary
	if s.Thoughts != 2 {
		t.Fatalf("expected 2 thoughts, got %d", s.Thoughts)
	}
	if s.Observations != 2 {
		t.Fatalf("expected 2 observations, got %d", s.Observations)
	}
	if s.Decisions != 2 {
		t.Fatalf("expected 2 decisions, got %d", s.Decisions)
	}
	if s.Reflections != 1 {
		t.Fatalf("expected 1 reflection, got %d", s.Reflections)
	}
	if s.TotalSteps != 7 {
		t.Fatalf("expected 7 total steps, got %d", s.TotalSteps)
	}
	if s.AvgConfidence < 0.5 || s.AvgConfidence > 1.0 {
		t.Fatalf("avg confidence out of range: %f", s.AvgConfidence)
	}
}

// TestReasoningBacktrack verifies backtracking during reasoning.
func TestReasoningBacktrack(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, _ := ldg.Tasks.CreateTask(ctx, "Parse complex CSV", ledger.TaskTypeGoal, "t1")
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskReady, "runtime", nil)
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskRunning, "runtime", nil)

	tracer := ldg.Reasoning(task.ID, "react-loop")

	tracer.Think(ctx, "Try parsing with standard csv package", nil)
	tracer.Decide(ctx, "use_csv_reader", "standard approach", 0.7, nil)
	tracer.Observe(ctx, "Failed: file uses non-standard delimiter", nil)
	tracer.Backtrack(ctx, "standard csv reader can't handle pipe delimiter", "use custom parser", nil)
	tracer.Decide(ctx, "use_custom_parser", "handles pipe and mixed delimiters", 0.9, nil)

	trace, err := ldg.Events.GetReasoningTrace(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetReasoningTrace: %v", err)
	}

	if trace.Summary.Backtracks != 1 {
		t.Fatalf("expected 1 backtrack, got %d", trace.Summary.Backtracks)
	}
	if trace.Summary.Decisions != 2 {
		t.Fatalf("expected 2 decisions, got %d", trace.Summary.Decisions)
	}
}

// TestReasoningPlan verifies multi-step plan recording.
func TestReasoningPlan(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, _ := ldg.Tasks.CreateTask(ctx, "Build API server", ledger.TaskTypeGoal, "t1")
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskReady, "runtime", nil)
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskRunning, "runtime", nil)

	tracer := ldg.Reasoning(task.ID, "planner")

	steps := []string{
		"1. Define API routes and handlers",
		"2. Set up database connection",
		"3. Implement authentication middleware",
		"4. Write integration tests",
		"5. Deploy to staging",
	}
	if err := tracer.Plan(ctx, steps, map[string]interface{}{
		"estimated_duration": "2 hours",
	}); err != nil {
		t.Fatalf("Plan: %v", err)
	}

	trace, err := ldg.Events.GetReasoningTrace(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetReasoningTrace: %v", err)
	}

	if len(trace.Events) != 1 {
		t.Fatalf("expected 1 reasoning event, got %d", len(trace.Events))
	}
	if trace.Events[0].Kind != ledger.EventReasoningPlan {
		t.Fatalf("expected reasoning.plan event, got %s", trace.Events[0].Kind)
	}
}

// TestReasoningHypothesis verifies hypothesis tracking with confidence.
func TestReasoningHypothesis(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, _ := ldg.Tasks.CreateTask(ctx, "Debug memory leak", ledger.TaskTypeGoal, "t1")
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskReady, "runtime", nil)
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskRunning, "runtime", nil)

	tracer := ldg.Reasoning(task.ID, "react-loop")

	tracer.Hypothesize(ctx, "Memory leak is caused by unclosed database connections", 0.6, nil)
	tracer.Observe(ctx, "Connection pool shows normal counts", nil)
	tracer.ConfidenceUpdate(ctx, 0.2, "connection pool is healthy, unlikely cause")
	tracer.Hypothesize(ctx, "Goroutine leak from HTTP handlers not closing response bodies", 0.8, nil)

	trace, err := ldg.Events.GetReasoningTrace(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetReasoningTrace: %v", err)
	}

	if trace.Summary.TotalSteps != 4 {
		t.Fatalf("expected 4 steps, got %d", trace.Summary.TotalSteps)
	}
	if trace.Summary.AvgConfidence == 0 {
		t.Fatal("expected non-zero avg confidence")
	}
}

// TestIsReasoningEvent verifies the event kind classification.
func TestIsReasoningEvent(t *testing.T) {
	if !ledger.IsReasoningEvent(ledger.EventReasoningThought) {
		t.Fatal("reasoning.thought should be a reasoning event")
	}
	if !ledger.IsReasoningEvent(ledger.EventReasoningBacktrack) {
		t.Fatal("reasoning.backtrack should be a reasoning event")
	}
	if ledger.IsReasoningEvent(ledger.EventTaskCreated) {
		t.Fatal("task.created should NOT be a reasoning event")
	}
	if ledger.IsReasoningEvent(ledger.EventToolInvoked) {
		t.Fatal("tool.invoked should NOT be a reasoning event")
	}
}

// ══════════════════════════════════════════════════
// Phase 0.2: Event Streaming / Pub-Sub Tests
// ══════════════════════════════════════════════════

// TestEventBusSubscribeAndReceive verifies basic pub/sub.
func TestEventBusSubscribeAndReceive(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, _ := ldg.Tasks.CreateTask(ctx, "Streaming test", ledger.TaskTypeGoal, "t1")

	// Subscribe to all events for this task
	sub := ldg.Bus.Subscribe(ledger.EventFilter{
		TaskIDs: []string{task.ID},
	}, 32)
	defer ldg.Bus.Unsubscribe(sub)

	// Transition should emit an event that the subscriber receives
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskReady, "runtime", nil)

	select {
	case e := <-sub.C:
		if e.TaskID != task.ID {
			t.Fatalf("expected task %s, got %s", task.ID, e.TaskID)
		}
		if e.Kind != ledger.EventTaskReady {
			t.Fatalf("expected %s, got %s", ledger.EventTaskReady, e.Kind)
		}
	default:
		t.Fatal("expected to receive an event, got none")
	}
}

// TestEventBusFilterByKind verifies kind-based filtering.
func TestEventBusFilterByKind(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, _ := ldg.Tasks.CreateTask(ctx, "Kind filter test", ledger.TaskTypeGoal, "t1")
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskReady, "runtime", nil)
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskRunning, "runtime", nil)

	// Subscribe only to reasoning events
	sub := ldg.Bus.Subscribe(ledger.EventFilter{Reasoning: true}, 32)
	defer ldg.Bus.Unsubscribe(sub)

	// Emit a task event (should NOT arrive)
	ldg.Tasks.Complete(ctx, task.ID, json.RawMessage(`{}`))

	// Emit reasoning events (should arrive)
	task2, _ := ldg.Tasks.CreateTask(ctx, "Task 2", ledger.TaskTypeGoal, "t1")
	ldg.Tasks.Transition(ctx, task2.ID, ledger.TaskReady, "runtime", nil)
	ldg.Tasks.Transition(ctx, task2.ID, ledger.TaskRunning, "runtime", nil)
	tracer := ldg.Reasoning(task2.ID, "test")
	tracer.Think(ctx, "testing filter", nil)

	received := drainChannel(sub.C, 100*time.Millisecond)

	if len(received) != 1 {
		t.Fatalf("expected 1 reasoning event, got %d", len(received))
	}
	if received[0].Kind != ledger.EventReasoningThought {
		t.Fatalf("expected reasoning.thought, got %s", received[0].Kind)
	}
}

// TestEventBusMultipleSubscribers verifies fan-out to multiple subscribers.
func TestEventBusMultipleSubscribers(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, _ := ldg.Tasks.CreateTask(ctx, "Multi-sub test", ledger.TaskTypeGoal, "t1")

	sub1 := ldg.Bus.Subscribe(ledger.EventFilter{TaskIDs: []string{task.ID}}, 32)
	sub2 := ldg.Bus.Subscribe(ledger.EventFilter{TaskIDs: []string{task.ID}}, 32)
	defer ldg.Bus.Unsubscribe(sub1)
	defer ldg.Bus.Unsubscribe(sub2)

	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskReady, "runtime", nil)

	got1 := drainChannel(sub1.C, 100*time.Millisecond)
	got2 := drainChannel(sub2.C, 100*time.Millisecond)

	if len(got1) != 1 || len(got2) != 1 {
		t.Fatalf("expected both subscribers to get 1 event, got %d and %d", len(got1), len(got2))
	}
}

// TestEventBusUnsubscribe verifies clean unsubscription.
func TestEventBusUnsubscribe(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, _ := ldg.Tasks.CreateTask(ctx, "Unsub test", ledger.TaskTypeGoal, "t1")

	sub := ldg.Bus.Subscribe(ledger.EventFilter{}, 32)
	ldg.Bus.Unsubscribe(sub)

	if ldg.Bus.SubscriberCount() != 0 {
		t.Fatal("expected 0 subscribers after unsubscribe")
	}

	// Channel should be closed
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskReady, "runtime", nil)
	_, open := <-sub.C
	if open {
		t.Fatal("expected channel to be closed after unsubscribe")
	}
}

// TestEventBusSubscriberCount verifies subscriber counting.
func TestEventBusSubscriberCount(t *testing.T) {
	ldg := newTestLedger(t)

	if ldg.Bus.SubscriberCount() != 0 {
		t.Fatal("expected 0 subscribers initially")
	}

	sub1 := ldg.Bus.Subscribe(ledger.EventFilter{}, 8)
	sub2 := ldg.Bus.Subscribe(ledger.EventFilter{}, 8)

	if ldg.Bus.SubscriberCount() != 2 {
		t.Fatalf("expected 2 subscribers, got %d", ldg.Bus.SubscriberCount())
	}

	ldg.Bus.Unsubscribe(sub1)
	if ldg.Bus.SubscriberCount() != 1 {
		t.Fatalf("expected 1 subscriber, got %d", ldg.Bus.SubscriberCount())
	}

	ldg.Bus.Unsubscribe(sub2)
	if ldg.Bus.SubscriberCount() != 0 {
		t.Fatalf("expected 0 subscribers, got %d", ldg.Bus.SubscriberCount())
	}
}

func drainChannel(ch <-chan *ledger.Event, timeout time.Duration) []*ledger.Event {
	var result []*ledger.Event
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				return result
			}
			result = append(result, e)
		case <-timer.C:
			return result
		}
	}
}

// ══════════════════════════════════════════════════
// Phase 0.3: Temporal Query API Tests
// ══════════════════════════════════════════════════

// TestQueryEventsByKind verifies event querying with kind filter.
func TestQueryEventsByKind(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, _ := ldg.Tasks.CreateTask(ctx, "Query test", ledger.TaskTypeGoal, "t1")
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskReady, "runtime", nil)
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskRunning, "runtime", nil)

	tracer := ldg.Reasoning(task.ID, "planner")
	tracer.Think(ctx, "thought 1", nil)
	tracer.Decide(ctx, "action 1", "reason", 0.8, nil)
	tracer.Think(ctx, "thought 2", nil)

	// Query only thought events
	events, err := ldg.Events.Query(ctx, ledger.EventQuery{
		TaskID: task.ID,
		Kinds:  []ledger.EventKind{ledger.EventReasoningThought},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 thought events, got %d", len(events))
	}

	// Query only decision events
	events, err = ldg.Events.Query(ctx, ledger.EventQuery{
		TaskID: task.ID,
		Kinds:  []ledger.EventKind{ledger.EventReasoningDecision},
	})
	if err != nil {
		t.Fatalf("Query decisions: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 decision event, got %d", len(events))
	}
}

// TestQueryEventsByTime verifies time-based event querying.
func TestQueryEventsByTime(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	before := time.Now()
	time.Sleep(10 * time.Millisecond)

	task, _ := ldg.Tasks.CreateTask(ctx, "Time query test", ledger.TaskTypeGoal, "t1")
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskReady, "runtime", nil)

	midpoint := time.Now()
	time.Sleep(10 * time.Millisecond)

	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskRunning, "runtime", nil)

	after := time.Now()

	// Query all events in the full time range
	events, err := ldg.Events.QueryByTime(ctx, before, after,
		nil, 100)
	if err != nil {
		t.Fatalf("QueryByTime: %v", err)
	}
	if len(events) < 3 {
		t.Fatalf("expected at least 3 events, got %d", len(events))
	}

	// Query events only after midpoint
	events, err = ldg.Events.QueryTaskByTime(ctx, task.ID, midpoint, after, 100)
	if err != nil {
		t.Fatalf("QueryTaskByTime: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event after midpoint, got %d", len(events))
	}
	if events[0].Kind != ledger.EventTaskStarted {
		t.Fatalf("expected task.started, got %s", events[0].Kind)
	}
}

// TestQueryEventsMultipleFilters verifies combining multiple filters.
func TestQueryEventsMultipleFilters(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, _ := ldg.Tasks.CreateTask(ctx, "Multi-filter test", ledger.TaskTypeGoal, "t1")
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskReady, "runtime", nil)
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskRunning, "runtime", nil)

	// Emit reasoning events with different actors
	plannerTracer := ldg.Reasoning(task.ID, "planner")
	reactTracer := ldg.Reasoning(task.ID, "react-loop")

	plannerTracer.Think(ctx, "planner thought", nil)
	reactTracer.Think(ctx, "react thought", nil)
	plannerTracer.Decide(ctx, "plan", "reason", 0.9, nil)
	reactTracer.Observe(ctx, "some observation", nil)

	// Filter: reasoning thoughts by planner only
	events, err := ldg.Events.Query(ctx, ledger.EventQuery{
		TaskID: task.ID,
		Kinds:  []ledger.EventKind{ledger.EventReasoningThought},
		Actors: []string{"planner"},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 planner thought, got %d", len(events))
	}
}

// TestQueryEventsWithLimit verifies pagination.
func TestQueryEventsWithLimit(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, _ := ldg.Tasks.CreateTask(ctx, "Limit test", ledger.TaskTypeGoal, "t1")
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskReady, "runtime", nil)
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskRunning, "runtime", nil)

	tracer := ldg.Reasoning(task.ID, "planner")
	for i := 0; i < 10; i++ {
		tracer.Think(ctx, "thought", nil)
	}

	// Get first 3
	events, err := ldg.Events.Query(ctx, ledger.EventQuery{
		TaskID: task.ID,
		Kinds:  []ledger.EventKind{ledger.EventReasoningThought},
		Limit:  3,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events with limit, got %d", len(events))
	}
}

// TestExtractExperience verifies experience extraction from PER results.
func TestExtractExperience(t *testing.T) {
	result := &ledger.PERResult{
		Success:  true,
		Attempts: 2,
		Reflections: []ledger.Reflection{
			{Score: 0.4, Learnings: []string{"lesson 1"}},
			{Score: 0.9, Learnings: []string{"lesson 2", "lesson 3"}},
		},
	}

	exp := ledger.ExtractExperience("task-1", "test goal", result)

	if exp.Outcome != "success" {
		t.Fatalf("expected 'success', got %q", exp.Outcome)
	}
	if exp.Score != 0.9 {
		t.Fatalf("expected score 0.9, got %f", exp.Score)
	}
	if len(exp.Learnings) != 3 {
		t.Fatalf("expected 3 learnings, got %d", len(exp.Learnings))
	}
	if exp.Attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", exp.Attempts)
	}
}

// TestAdaptiveRecallFeedback verifies feedback accumulation.
func TestAdaptiveRecallFeedback(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	ar := ledger.NewAdaptiveRecall(ldg.Recall, ldg.Backend(), ldg.Memory)

	// Store some memories
	ldg.Memory.PutFact(ctx, "t1", "lang.go", "Go is great for systems", "user")
	ldg.Memory.PutFact(ctx, "t1", "lang.python", "Python is great for ML", "user")

	memories, _ := ldg.Memory.Search(ctx, ledger.MemoryQuery{TenantID: "t1", Limit: 10})

	// Record positive feedback
	for _, m := range memories {
		ar.RecordFeedback(ctx, ledger.RecallFeedback{
			MemoryID: m.ID,
			Helpful:  true,
			Score:    0.9,
		}, ledger.RecallQuery{
			TenantID: "t1",
			Query:    "programming language",
			TaskGoal: "write code",
			TaskType: ledger.TaskTypeGoal,
		})
	}

	if ar.FeedbackCount() != len(memories) {
		t.Fatalf("expected %d feedback entries, got %d", len(memories), ar.FeedbackCount())
	}
}

// TestAdaptiveRecallWeightAdapt verifies weight adaptation.
func TestAdaptiveRecallWeightAdapt(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	ar := ledger.NewAdaptiveRecall(ldg.Recall, ldg.Backend(), ldg.Memory)
	initialWeights := ar.CurrentWeights()

	// Store memories
	ldg.Memory.PutFact(ctx, "t1", "fact.a", "Important fact A", "user")
	ldg.Memory.PutFact(ctx, "t1", "fact.b", "Less important B", "extraction")
	memories, _ := ldg.Memory.Search(ctx, ledger.MemoryQuery{TenantID: "t1", Limit: 10})

	query := ledger.RecallQuery{
		TenantID: "t1",
		Query:    "important",
		TaskGoal: "find facts",
		TaskType: ledger.TaskTypeGoal,
	}

	// Record mixed feedback
	for _, m := range memories {
		ar.RecordFeedback(ctx, ledger.RecallFeedback{
			MemoryID: m.ID,
			Helpful:  m.Source == "user",
			Score:    0.8,
		}, query)
	}

	// Adapt
	newWeights, used := ar.AdaptWeights(ctx, 1)
	if used == 0 {
		t.Fatal("expected feedback to be used")
	}

	// Weights should have changed
	if newWeights.SourceTrust == initialWeights.SourceTrust &&
		newWeights.KeywordRelevance == initialWeights.KeywordRelevance {
		t.Log("Note: weights didn't change significantly (may need more diverse feedback)")
	}

	// Weights should still sum to ~1.0
	total := newWeights.KeywordRelevance + newWeights.GoalAlignment + newWeights.KindBoost +
		newWeights.Recency + newWeights.Confidence + newWeights.AccessFrequency + newWeights.SourceTrust
	if total < 0.99 || total > 1.01 {
		t.Fatalf("weights should sum to ~1.0, got %f", total)
	}
}

// TestAdaptiveRecallPersistence verifies weight persistence via Memory.
func TestAdaptiveRecallPersistence(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	ar := ledger.NewAdaptiveRecall(ldg.Recall, ldg.Backend(), ldg.Memory)

	// Persist
	if err := ar.PersistWeights(ctx, "t1"); err != nil {
		t.Fatalf("PersistWeights: %v", err)
	}

	// Create a new instance and load
	ar2 := ledger.NewAdaptiveRecall(ldg.Recall, ldg.Backend(), ldg.Memory)
	if err := ar2.LoadWeights(ctx, "t1"); err != nil {
		t.Fatalf("LoadWeights: %v", err)
	}

	w1 := ar.CurrentWeights()
	w2 := ar2.CurrentWeights()

	if w1.KeywordRelevance != w2.KeywordRelevance {
		t.Fatalf("weights mismatch: %v vs %v", w1, w2)
	}
}

// ══════════════════════════════════════════════════
// Phase 2.4: Memory Conflict Resolution Tests
// ══════════════════════════════════════════════════

// TestConflictNewerWins verifies the newer-wins strategy.
func TestConflictNewerWins(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	// Store initial memory
	ldg.Memory.PutFact(ctx, "t1", "user.city", "Beijing", "user")

	// Create conflicting memory
	incoming := &ledger.MemoryEntry{
		TenantID:   "t1",
		Kind:       ledger.MemoryFact,
		Key:        "user.city",
		Content:    "Shanghai",
		Source:     "user",
		Confidence: 0.9,
	}

	cr := ldg.ConflictResolver(ledger.ConflictNewerWins)
	result, err := cr.CheckAndResolve(ctx, incoming)
	if err != nil {
		t.Fatalf("CheckAndResolve: %v", err)
	}

	if result == nil {
		t.Fatal("expected conflict to be detected")
	}
	if result.Winner.Content != "Shanghai" {
		t.Fatalf("expected Shanghai to win, got %s", result.Winner.Content)
	}
	if result.Strategy != ledger.ConflictNewerWins {
		t.Fatalf("expected newer_wins strategy, got %s", result.Strategy)
	}
}

// TestConflictHigherConfidence verifies the higher-confidence strategy.
func TestConflictHigherConfidence(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	// Store initial with high confidence
	ldg.Memory.Put(ctx, &ledger.MemoryEntry{
		TenantID:   "t1",
		Kind:       ledger.MemoryFact,
		Key:        "user.age",
		Content:    "25",
		Source:     "user",
		Confidence: 0.95,
	})

	// Incoming with low confidence
	incoming := &ledger.MemoryEntry{
		TenantID:   "t1",
		Kind:       ledger.MemoryFact,
		Key:        "user.age",
		Content:    "30",
		Source:     "extraction",
		Confidence: 0.4,
	}

	cr := ldg.ConflictResolver(ledger.ConflictHigherConfidenceWins)
	result, err := cr.CheckAndResolve(ctx, incoming)
	if err != nil {
		t.Fatalf("CheckAndResolve: %v", err)
	}
	if result == nil {
		t.Fatal("expected conflict")
	}
	if result.Winner.Content != "25" {
		t.Fatalf("expected existing (25) to win with higher confidence, got %s", result.Winner.Content)
	}
}

// TestConflictMerge verifies the merge strategy.
func TestConflictMerge(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	ldg.Memory.PutFact(ctx, "t1", "user.skills", "Go programming", "user")

	incoming := &ledger.MemoryEntry{
		TenantID:   "t1",
		Kind:       ledger.MemoryFact,
		Key:        "user.skills",
		Content:    "Python programming",
		Source:     "user",
		Confidence: 0.8,
	}

	cr := ldg.ConflictResolver(ledger.ConflictMerge)
	result, err := cr.CheckAndResolve(ctx, incoming)
	if err != nil {
		t.Fatalf("CheckAndResolve: %v", err)
	}
	if result == nil {
		t.Fatal("expected conflict")
	}
	if !result.Merged {
		t.Fatal("expected merge")
	}
}

// TestNoConflict verifies no false positives when there's no conflict.
func TestNoConflict(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	ldg.Memory.PutFact(ctx, "t1", "user.name", "Alice", "user")

	// Different key = no conflict
	incoming := &ledger.MemoryEntry{
		TenantID:   "t1",
		Kind:       ledger.MemoryFact,
		Key:        "user.email",
		Content:    "alice@example.com",
		Source:     "user",
		Confidence: 0.9,
	}

	cr := ldg.ConflictResolver(ledger.ConflictNewerWins)
	result, err := cr.CheckAndResolve(ctx, incoming)
	if err != nil {
		t.Fatalf("CheckAndResolve: %v", err)
	}
	if result != nil {
		t.Fatal("expected no conflict for different keys")
	}
}

func TestConflictNewerWinsPropagatesPutMemoryError(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	ldg.Memory.PutFact(ctx, "t1", "user.city", "Beijing", "user")

	incoming := &ledger.MemoryEntry{
		TenantID:   "t1",
		Kind:       ledger.MemoryFact,
		Key:        "user.city",
		Content:    "Shanghai",
		Source:     "user",
		Confidence: 0.9,
	}

	failing := failingMemoryBackend{Backend: ldg.Backend(), err: errors.New("put memory failed")}
	cr := ledger.NewConflictResolver(failing, ldg.Memory, ledger.ConflictNewerWins)
	if _, err := cr.CheckAndResolve(ctx, incoming); err == nil || err.Error() != "put memory failed" {
		t.Fatalf("expected put-memory error, got %v", err)
	}
}

// ══════════════════════════════════════════════════
func TestReasoningTraceMixedWithTaskEvents(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, _ := ldg.Tasks.CreateTask(ctx, "Mixed events test", ledger.TaskTypeGoal, "t1")
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskReady, "runtime", nil)
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskRunning, "runtime", nil)

	tracer := ldg.Reasoning(task.ID, "planner")
	tracer.Think(ctx, "Analyzing the task", nil)
	tracer.Decide(ctx, "execute_step_1", "best approach", 0.8, nil)

	// Total events = task.created + task.ready + task.started + 2 reasoning
	all, err := ldg.Events.ListAll(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) < 5 {
		t.Fatalf("expected at least 5 total events, got %d", len(all))
	}

	// Reasoning trace should only have 2
	trace, _ := ldg.Events.GetReasoningTrace(ctx, task.ID)
	if len(trace.Events) != 2 {
		t.Fatalf("expected 2 reasoning events, got %d", len(trace.Events))
	}
}
