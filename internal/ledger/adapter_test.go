package ledger

import (
	"testing"

	"github.com/LittleXiaYuan/ledger"

	agtask "yunque-agent/internal/agentcore/task"
)

func TestLedgerStore_CreateAndGet(t *testing.T) {
	ldg := newTestLedger(t)
	store := NewLedgerStore(ldg, t.TempDir())

	task, err := store.Create(agtask.CreateRequest{
		Description: "Test task for ledger",
		TenantID:    "test-tenant",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if task.ID == "" {
		t.Fatal("expected non-empty task ID")
	}
	if task.Status != agtask.StatusPending {
		t.Errorf("status = %s, want pending", task.Status)
	}
	if task.TenantID != "test-tenant" {
		t.Errorf("tenant = %s, want test-tenant", task.TenantID)
	}

	got, found := store.Get(task.ID)
	if !found {
		t.Fatal("expected to find created task")
	}
	if got.Description != "Test task for ledger" {
		t.Errorf("description = %s", got.Description)
	}
}

func TestLedgerStore_GetMissing(t *testing.T) {
	ldg := newTestLedger(t)
	store := NewLedgerStore(ldg, t.TempDir())

	_, found := store.Get("nonexistent-id")
	if found {
		t.Error("expected not found for missing ID")
	}
}

func TestLedgerStore_List(t *testing.T) {
	ldg := newTestLedger(t)
	store := NewLedgerStore(ldg, t.TempDir())

	store.Create(agtask.CreateRequest{Description: "Task A", TenantID: "t1"})
	store.Create(agtask.CreateRequest{Description: "Task B", TenantID: "t1"})
	store.Create(agtask.CreateRequest{Description: "Task C", TenantID: "t2"})

	listT1 := store.List("t1", 10)
	if len(listT1) != 2 {
		t.Errorf("t1 tasks = %d, want 2", len(listT1))
	}

	listT2 := store.List("t2", 10)
	if len(listT2) != 1 {
		t.Errorf("t2 tasks = %d, want 1", len(listT2))
	}
}

func TestLedgerStore_ListLimit(t *testing.T) {
	ldg := newTestLedger(t)
	store := NewLedgerStore(ldg, t.TempDir())

	for i := 0; i < 5; i++ {
		store.Create(agtask.CreateRequest{Description: "task", TenantID: "t1"})
	}

	list := store.List("t1", 3)
	if len(list) != 3 {
		t.Errorf("limited list = %d, want 3", len(list))
	}
}

func TestLedgerStore_Update(t *testing.T) {
	ldg := newTestLedger(t)
	store := NewLedgerStore(ldg, t.TempDir())

	task, _ := store.Create(agtask.CreateRequest{
		Description: "Original description",
		TenantID:    "t1",
	})

	task.Description = "Updated description"
	task.Status = agtask.StatusCompleted
	err := store.Update(task)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := store.Get(task.ID)
	if got.Description != "Updated description" {
		t.Errorf("description = %s, want Updated description", got.Description)
	}
}

func TestLedgerStore_Delete(t *testing.T) {
	ldg := newTestLedger(t)
	store := NewLedgerStore(ldg, t.TempDir())

	task, _ := store.Create(agtask.CreateRequest{
		Description: "To be deleted",
		TenantID:    "t1",
	})

	ok := store.Delete(task.ID)
	if !ok {
		t.Error("Delete should return true")
	}
}

func TestLedgerStore_DeleteMissing(t *testing.T) {
	ldg := newTestLedger(t)
	store := NewLedgerStore(ldg, t.TempDir())

	ok := store.Delete("nonexistent")
	if ok {
		t.Error("Delete of nonexistent should return false")
	}
}

func TestLedgerStore_RecoverInterrupted(t *testing.T) {
	ldg := newTestLedger(t)
	store := NewLedgerStore(ldg, t.TempDir())

	count := store.RecoverInterrupted()
	if count != 0 {
		t.Errorf("count = %d, want 0 (no running tasks)", count)
	}
}

func TestLedgerStore_CreateWithTitle(t *testing.T) {
	ldg := newTestLedger(t)
	store := NewLedgerStore(ldg, t.TempDir())

	task, err := store.Create(agtask.CreateRequest{
		Title:       "My Custom Title",
		Description: "Detailed description here",
		TenantID:    "t1",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if task.Title != "My Custom Title" {
		t.Errorf("title = %s, want My Custom Title", task.Title)
	}
}

func TestStatusConversions(t *testing.T) {
	tests := []struct {
		agent  agtask.Status
		ledger ledger.TaskStatus
	}{
		{agtask.StatusPending, ledger.TaskCreated},
		{agtask.StatusRunning, ledger.TaskRunning},
		{agtask.StatusCompleted, ledger.TaskCompleted},
		{agtask.StatusFailed, ledger.TaskFailed},
		{agtask.StatusCancelled, ledger.TaskCancelled},
		{agtask.StatusPaused, ledger.TaskWaitingInput},
	}
	for _, tt := range tests {
		got := agentStatusToLedger(tt.agent)
		if got != tt.ledger {
			t.Errorf("agentStatusToLedger(%s) = %v, want %v", tt.agent, got, tt.ledger)
		}
	}

	ledgerTests := []struct {
		ledger ledger.TaskStatus
		agent  agtask.Status
	}{
		{ledger.TaskCreated, agtask.StatusPending},
		{ledger.TaskRunning, agtask.StatusRunning},
		{ledger.TaskCompleted, agtask.StatusCompleted},
		{ledger.TaskFailed, agtask.StatusFailed},
		{ledger.TaskCancelled, agtask.StatusCancelled},
		{ledger.TaskWaitingInput, agtask.StatusPaused},
		{ledger.TaskBlocked, agtask.StatusPaused},
		{ledger.TaskRetrying, agtask.StatusRunning},
	}
	for _, tt := range ledgerTests {
		got := ledgerStatusToAgent(tt.ledger)
		if got != tt.agent {
			t.Errorf("ledgerStatusToAgent(%v) = %s, want %s", tt.ledger, got, tt.agent)
		}
	}
}

func TestArtifactDir(t *testing.T) {
	ldg := newTestLedger(t)
	baseDir := t.TempDir()
	store := NewLedgerStore(ldg, baseDir)

	dir, err := store.ArtifactDir("task-123")
	if err != nil {
		t.Fatalf("ArtifactDir: %v", err)
	}
	if dir == "" {
		t.Fatal("expected non-empty artifact dir")
	}
}
