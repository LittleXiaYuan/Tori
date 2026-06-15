package ledger

// Regression tests for the deep-scan adapter fixes (commit e4b55801):
// crash recovery must keep tasks dispatchable, Delete must be a real delete,
// and Update must clear stale errors from previous failed runs.

import (
	"context"
	"testing"

	agtask "yunque-agent/internal/agentcore/task"
)

// RecoverInterrupted used to Fail() running tasks on startup; failed is
// terminal for the dependency scheduler, so dependent tasks hung forever.
// It must mark them Blocked, which round-trips to StatusInterrupted and
// stays dispatchable.
func TestRecoverInterruptedKeepsTasksDispatchable(t *testing.T) {
	ldg := newTestLedger(t)
	store := NewLedgerStore(ldg, t.TempDir())

	task, err := store.Create(agtask.CreateRequest{Description: "crashy", TenantID: "t1"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	task.Status = agtask.StatusRunning
	if err := store.Update(task); err != nil {
		t.Fatalf("Update to running: %v", err)
	}

	if n := store.RecoverInterrupted(); n != 1 {
		t.Fatalf("RecoverInterrupted = %d, want 1", n)
	}

	got, found := store.Get(task.ID)
	if !found {
		t.Fatal("task must still exist after recovery")
	}
	if got.Status != agtask.StatusInterrupted {
		t.Fatalf("recovered status = %s, want %s (failed would strand dependents)", got.Status, agtask.StatusInterrupted)
	}

	// The recovered task must be resumable.
	got.Status = agtask.StatusRunning
	if err := store.Update(got); err != nil {
		t.Fatalf("interrupted task must be dispatchable again: %v", err)
	}
	resumed, _ := store.Get(task.ID)
	if resumed.Status != agtask.StatusRunning {
		t.Fatalf("resumed status = %s, want running", resumed.Status)
	}
}

// Delete used to cancel the task but leave the row and its events behind.
func TestDeleteRemovesTaskPermanently(t *testing.T) {
	ldg := newTestLedger(t)
	store := NewLedgerStore(ldg, t.TempDir())

	task, err := store.Create(agtask.CreateRequest{Description: "to be deleted", TenantID: "t1"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !store.Delete(task.ID) {
		t.Fatal("Delete returned false")
	}
	if _, found := store.Get(task.ID); found {
		t.Fatal("deleted task must not be retrievable")
	}
	if _, err := ldg.Tasks.GetTask(context.Background(), task.ID); err == nil {
		t.Fatal("task row must be gone from the ledger backend")
	}
}

// Update kept a stale Error from a previous failed run when the caller
// resumed the task with an empty error.
func TestUpdateClearsStaleErrorOnResume(t *testing.T) {
	ldg := newTestLedger(t)
	store := NewLedgerStore(ldg, t.TempDir())

	task, err := store.Create(agtask.CreateRequest{Description: "fails once", TenantID: "t1"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	task.Status = agtask.StatusFailed
	task.Error = "boom"
	if err := store.Update(task); err != nil {
		t.Fatalf("Update to failed: %v", err)
	}

	// Resume: error cleared by the caller must clear in the store too.
	task.Status = agtask.StatusRunning
	task.Error = ""
	if err := store.Update(task); err != nil {
		t.Fatalf("Update to running: %v", err)
	}

	got, _ := store.Get(task.ID)
	if got.Error != "" {
		t.Fatalf("stale error survived resume: %q", got.Error)
	}
}
