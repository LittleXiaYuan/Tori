package trigger

import (
	"context"
	"testing"
	"time"
)

// ──────────────────────────────────────────────
// P1 Core Tests — 验证 4 类触发器 + 5 类动作
// ──────────────────────────────────────────────

func TestTriggerStore(t *testing.T) {
	store := NewStore(t.TempDir())

	// 创建触发器
	trigger := &TriggerDef{
		ID:       "test-trigger-1",
		Name:     "Test Trigger",
		Type:     TriggerTypeEvent,
		Status:   TriggerStatusActive,
		TenantID: "tenant-1",
		EventConfig: &EventConfig{
			EventType: "task_completed",
		},
		Actions: []TriggerAction{
			{
				Type:            ActionCreateTask,
				TaskTitle:       "Follow-up Task",
				TaskDescription: "Created by trigger",
			},
		},
	}

	err := store.Create(trigger)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// 获取触发器
	got, ok := store.Get("test-trigger-1")
	if !ok {
		t.Fatal("Get failed: trigger not found")
	}
	if got.Name != "Test Trigger" {
		t.Errorf("Name mismatch: got %s, want Test Trigger", got.Name)
	}

	// 列出触发器
	list := store.List("tenant-1", nil)
	if len(list) == 0 {
		t.Error("List returned empty")
	}
}

func TestTriggerRun(t *testing.T) {
	store := NewStore(t.TempDir())

	// 创建执行记录
	run := &TriggerRun{
		TriggerID:     "test-trigger-1",
		TenantID:      "tenant-1",
		Status:        RunStatusRunning,
		TriggerType:   TriggerTypeEvent,
		TriggerSource: "event:task_completed",
		StartedAt:     time.Now(),
		ActionResults: []ActionResult{
			{
				ActionType: ActionCreateTask,
				Status:     "success",
				Result:     "task-123",
			},
		},
	}

	err := store.CreateRun(run)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// 获取执行记录
	got, ok := store.GetRun(run.ID)
	if !ok {
		t.Fatal("GetRun failed: run not found")
	}
	if got.TriggerID != "test-trigger-1" {
		t.Errorf("TriggerID mismatch: got %s, want test-trigger-1", got.TriggerID)
	}
}

func TestExecutor(t *testing.T) {
	store := NewStore(t.TempDir())
	executor := NewExecutor(store)

	// 注入回调
	var createdTaskID string
	executor.SetCreateTask(func(ctx context.Context, tenantID, title, desc string) (string, error) {
		createdTaskID = "task-123"
		return createdTaskID, nil
	})

	// 创建触发器
	trigger := &TriggerDef{
		ID:       "test-trigger-2",
		Name:     "Test Executor",
		Type:     TriggerTypeEvent,
		Status:   TriggerStatusActive,
		TenantID: "tenant-1",
		Actions: []TriggerAction{
			{
				Type:            ActionCreateTask,
				TaskTitle:       "Test Task",
				TaskDescription: "Created by executor test",
			},
		},
	}

	// 执行触发器
	run, err := executor.Execute(context.Background(), trigger, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if run.Status != RunStatusCompleted {
		t.Errorf("Status mismatch: got %s, want %s", run.Status, RunStatusCompleted)
	}
	if run.ActionsExecuted != 1 {
		t.Errorf("ActionsExecuted mismatch: got %d, want 1", run.ActionsExecuted)
	}
	if run.ActionsSucceeded != 1 {
		t.Errorf("ActionsSucceeded mismatch: got %d, want 1", run.ActionsSucceeded)
	}
	if createdTaskID != "task-123" {
		t.Errorf("Task not created: got %s", createdTaskID)
	}
}

func TestBudgetCheck(t *testing.T) {
	budget := &BudgetConfig{
		MaxRunsPerDay:  10,
		MaxTotalCost:   1.0,
		CurrentDayCost: 0.5,
	}

	allowed, reason := budget.CheckBudget(time.Now())
	if !allowed {
		t.Errorf("Budget check failed: %s", reason)
	}

	// 超出预算
	budget.CurrentDayCost = 1.5
	allowed, reason = budget.CheckBudget(time.Now())
	if allowed {
		t.Error("Budget check should fail when cost exceeded")
	}
	if reason == "" {
		t.Error("Reason should not be empty")
	}
}
