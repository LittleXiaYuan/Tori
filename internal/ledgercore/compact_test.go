package ledger_test

import (
	"context"
	"testing"
	"time"

	"yunque-agent/internal/ledgercore"
)

func TestCompactDryRun(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, err := ldg.Tasks.CreateTask(ctx, "compact-test", ledger.TaskTypeGoal, "t1")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskReady, "test", nil)
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskRunning, "test", nil)

	for i := 0; i < 250; i++ {
		ldg.Events.Append(ctx, &ledger.Event{
			TaskID:    task.ID,
			Kind:      ledger.EventStepStarted,
			Actor:     "test",
			Payload:   []byte(`{"step": "bulk"}`),
			CreatedAt: time.Now().Add(-8 * 24 * time.Hour),
		})
	}

	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskCompleted, "test", nil)

	cfg := ledger.CompactConfig{
		MinAge:           1 * time.Millisecond,
		MaxEventsPerTask: 200,
		DryRun:           true,
	}
	time.Sleep(5 * time.Millisecond)

	result, err := ldg.CompactEvents(ctx, "t1", cfg)
	if err != nil {
		t.Fatalf("CompactEvents: %v", err)
	}
	t.Logf("compact: scanned=%d compacted=%d removed=%d retained=%d",
		result.TasksScanned, result.TasksCompacted, result.EventsRemoved, result.EventsRetained)
}

func TestCompactRealDeletion(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, err := ldg.Tasks.CreateTask(ctx, "compact-real", ledger.TaskTypeGoal, "t1")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskReady, "test", nil)
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskRunning, "test", nil)

	for i := 0; i < 250; i++ {
		ldg.Events.Append(ctx, &ledger.Event{
			TaskID:  task.ID,
			Kind:    ledger.EventStepStarted,
			Actor:   "test",
			Payload: []byte(`{"step": "bulk"}`),
		})
	}

	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskCompleted, "test", nil)

	// Count events before compaction
	beforeEvents, _ := ldg.Events.ListAll(ctx, task.ID)
	beforeCount := len(beforeEvents)

	cfg := ledger.CompactConfig{
		MinAge:           1 * time.Millisecond,
		MaxEventsPerTask: 200,
		DryRun:           false,
	}
	time.Sleep(5 * time.Millisecond)

	result, err := ldg.CompactEvents(ctx, "t1", cfg)
	if err != nil {
		t.Fatalf("CompactEvents: %v", err)
	}

	afterEvents, _ := ldg.Events.ListAll(ctx, task.ID)
	afterCount := len(afterEvents)

	t.Logf("before=%d after=%d removed=%d", beforeCount, afterCount, result.EventsRemoved)
	if afterCount >= beforeCount {
		t.Errorf("expected fewer events after compaction: before=%d after=%d", beforeCount, afterCount)
	}
	if result.SnapshotsCreated == 0 {
		t.Error("expected at least one snapshot to be created")
	}
}

func TestCompactNoOpBelowThreshold(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, err := ldg.Tasks.CreateTask(ctx, "small-task", ledger.TaskTypeGoal, "t1")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskReady, "test", nil)
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskRunning, "test", nil)

	// Add only a few events (below threshold)
	for i := 0; i < 10; i++ {
		ldg.Events.Append(ctx, &ledger.Event{
			TaskID:    task.ID,
			Kind:      ledger.EventStepStarted,
			Actor:     "test",
			Payload:   []byte(`{}`),
			CreatedAt: time.Now().Add(-8 * 24 * time.Hour),
		})
	}

	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskCompleted, "test", nil)

	cfg := ledger.CompactConfig{
		MinAge:           1 * time.Millisecond,
		MaxEventsPerTask: 200,
		DryRun:           true,
	}
	time.Sleep(5 * time.Millisecond)

	result, err := ldg.CompactEvents(ctx, "t1", cfg)
	if err != nil {
		t.Fatalf("CompactEvents: %v", err)
	}
	if result.EventsRemoved != 0 {
		t.Errorf("expected no events removed for small task, got %d", result.EventsRemoved)
	}
}
