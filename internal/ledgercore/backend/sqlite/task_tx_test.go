package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"yunque-agent/internal/ledgercore"
)

func testTask(id string) *ledger.Task {
	now := time.Now()
	return &ledger.Task{
		ID:         id,
		Type:       ledger.TaskTypeGoal,
		Goal:       "verify transactional task event writes",
		Status:     ledger.TaskCreated,
		TenantID:   "tenant-tx",
		AgentID:    "agent",
		Input:      ledger.JSON("{}"),
		Output:     ledger.JSON("{}"),
		Metadata:   ledger.JSON("{}"),
		MaxRetries: 2,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

func badEvent(taskID string) *ledger.Event {
	return &ledger.Event{
		ID:        "evt-bad",
		TaskID:    taskID,
		Kind:      ledger.EventTaskReady,
		Actor:     "test",
		Payload:   ledger.JSON("{}"),
		CreatedAt: time.Now(),
	}
}

func TestCreateTaskWithEventRollsBackWhenEventInsertFails(t *testing.T) {
	b, err := New(filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	ctx := context.Background()
	if err := b.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	task := testTask("task-create-rollback")
	err = b.CreateTaskWithEvent(ctx, task, badEvent("missing-task"))
	if err == nil {
		t.Fatal("expected foreign-key error from bad event")
	}

	if _, err := b.GetTask(ctx, task.ID); err == nil {
		t.Fatal("task insert should have rolled back when event insert failed")
	}
}

func TestUpdateTaskWithEventRollsBackWhenEventInsertFails(t *testing.T) {
	b, err := New(filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	ctx := context.Background()
	if err := b.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	task := testTask("task-update-rollback")
	if err := b.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	task.Status = ledger.TaskReady
	task.UpdatedAt = time.Now()
	err = b.UpdateTaskWithEvent(ctx, task, badEvent("missing-task"))
	if err == nil {
		t.Fatal("expected foreign-key error from bad event")
	}

	stored, err := b.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if stored.Status != ledger.TaskCreated {
		t.Fatalf("task status = %s, want %s after rollback", stored.Status, ledger.TaskCreated)
	}
	if stored.Version != 0 {
		t.Fatalf("task version = %d, want 0 after rollback", stored.Version)
	}
}

func TestAppendEventRestoresImplicitSeqAfterInsertFailure(t *testing.T) {
	b, err := New(filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	ctx := context.Background()
	if err := b.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	failed := badEvent("task-seq-restore")
	if err := b.AppendEvent(ctx, failed); err == nil {
		t.Fatal("expected foreign-key error from missing task")
	}
	if failed.Seq != 0 {
		t.Fatalf("failed event seq = %d, want reset to 0", failed.Seq)
	}

	task := testTask("task-seq-restore")
	if err := b.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	next := &ledger.Event{
		ID:        "evt-next",
		TaskID:    task.ID,
		Kind:      ledger.EventTaskReady,
		Actor:     "test",
		Payload:   ledger.JSON("{}"),
		CreatedAt: time.Now(),
	}
	if err := b.AppendEvent(ctx, next); err != nil {
		t.Fatalf("AppendEvent after failed insert: %v", err)
	}
	if next.Seq != 1 {
		t.Fatalf("next event seq = %d, want 1", next.Seq)
	}
}
