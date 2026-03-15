package task

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestLifecycleManager_TransitionTo(t *testing.T) {
	store := NewStore("") // 使用空字符串，不持久化到磁盘
	lm := NewLifecycleManager(store)

	// 创建测试任务
	task, err := store.Create(CreateRequest{
		Title:       "Test Task",
		Description: "Test description",
		TenantID:    "tenant-1",
	})
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	taskID := task.ID

	// 测试合法状态转换
	t.Run("valid transition", func(t *testing.T) {
		err := lm.TransitionTo(context.Background(), taskID, StatusPlanning)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		updatedTask, _ := store.Get(taskID)
		if updatedTask.Status != StatusPlanning {
			t.Errorf("expected status %s, got %s", StatusPlanning, updatedTask.Status)
		}
	})

	// 测试非法状态转换
	t.Run("invalid transition", func(t *testing.T) {
		err := lm.TransitionTo(context.Background(), taskID, StatusCompleted)
		if err == nil {
			t.Fatal("expected error for invalid transition, got nil")
		}
	})

	// 测试幂等性
	t.Run("idempotent transition", func(t *testing.T) {
		err := lm.TransitionTo(context.Background(), taskID, StatusPlanning)
		if err != nil {
			t.Fatalf("expected no error for idempotent transition, got %v", err)
		}
	})

	// 测试任务不存在
	t.Run("task not found", func(t *testing.T) {
		err := lm.TransitionTo(context.Background(), "non-existent", StatusRunning)
		if err == nil {
			t.Fatal("expected error for non-existent task, got nil")
		}
	})
}

func TestLifecycleManager_StepLifecycle(t *testing.T) {
	store := NewStore("")
	lm := NewLifecycleManager(store)

	// 创建测试任务
	task, err := store.Create(CreateRequest{
		Title:       "Test Task",
		Description: "Test description",
		TenantID:    "tenant-1",
	})
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	taskID := task.ID

	// 添加步骤并更新任务状态
	task.Status = StatusRunning
	task.Steps = []Step{
		{ID: 0, Action: "Step 1", Status: StepPending},
		{ID: 1, Action: "Step 2", Status: StepPending},
	}
	store.Update(task)

	// 测试步骤开始
	t.Run("step start", func(t *testing.T) {
		err := lm.OnStepStart(context.Background(), taskID, 0)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		updatedTask, _ := store.Get(taskID)
		if updatedTask.Steps[0].Status != StepRunning {
			t.Errorf("expected step status %s, got %s", StepRunning, updatedTask.Steps[0].Status)
		}
		if updatedTask.Steps[0].StartedAt == nil {
			t.Error("expected StartedAt to be set")
		}
	})

	// 测试步骤完成
	t.Run("step complete", func(t *testing.T) {
		err := lm.OnStepComplete(context.Background(), taskID, 0, "Step 1 result")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		updatedTask, _ := store.Get(taskID)
		if updatedTask.Steps[0].Status != StepDone {
			t.Errorf("expected step status %s, got %s", StepDone, updatedTask.Steps[0].Status)
		}
		if updatedTask.Steps[0].Result != "Step 1 result" {
			t.Errorf("expected result 'Step 1 result', got '%s'", updatedTask.Steps[0].Result)
		}
		if updatedTask.Steps[0].DoneAt == nil {
			t.Error("expected DoneAt to be set")
		}
	})

	// 测试步骤失败
	t.Run("step failed", func(t *testing.T) {
		err := lm.OnStepFailed(context.Background(), taskID, 1, errors.New("step failed"))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		updatedTask, _ := store.Get(taskID)
		if updatedTask.Steps[1].Status != StepFailed {
			t.Errorf("expected step status %s, got %s", StepFailed, updatedTask.Steps[1].Status)
		}
		if updatedTask.Steps[1].Error != "step failed" {
			t.Errorf("expected error 'step failed', got '%s'", updatedTask.Steps[1].Error)
		}
	})

	// 测试无效步骤 ID
	t.Run("invalid step id", func(t *testing.T) {
		err := lm.OnStepStart(context.Background(), taskID, 99)
		if err == nil {
			t.Fatal("expected error for invalid step ID, got nil")
		}
	})
}

func TestLifecycleManager_EventEmission(t *testing.T) {
	store := NewStore("")
	lm := NewLifecycleManager(store)

	// 创建测试任务
	task, err := store.Create(CreateRequest{
		Title:       "Test Task",
		Description: "Test description",
		TenantID:    "tenant-1",
	})
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	taskID := task.ID

	// 注册监听器
	var receivedEvents []LifecycleEvent
	var mu sync.Mutex
	lm.OnEvent(func(ctx context.Context, event LifecycleEvent) {
		mu.Lock()
		defer mu.Unlock()
		receivedEvents = append(receivedEvents, event)
	})

	// 触发状态变更
	lm.TransitionTo(context.Background(), taskID, StatusPlanning)

	// 验证事件
	mu.Lock()
	defer mu.Unlock()
	if len(receivedEvents) != 1 {
		t.Fatalf("expected 1 event, got %d", len(receivedEvents))
	}

	event := receivedEvents[0]
	if event.Type != EventTaskStateChanged {
		t.Errorf("expected event type %s, got %s", EventTaskStateChanged, event.Type)
	}
	if event.TaskID != taskID {
		t.Errorf("expected task ID '%s', got '%s'", taskID, event.TaskID)
	}
	if event.OldStatus != string(StatusPending) {
		t.Errorf("expected old status %s, got %s", StatusPending, event.OldStatus)
	}
	if event.NewStatus != string(StatusPlanning) {
		t.Errorf("expected new status %s, got %s", StatusPlanning, event.NewStatus)
	}
}

func TestLifecycleManager_ConcurrentUpdates(t *testing.T) {
	store := NewStore("")
	lm := NewLifecycleManager(store)

	// 创建多个测试任务
	taskIDs := make([]string, 10)
	for i := 0; i < 10; i++ {
		task, err := store.Create(CreateRequest{
			Title:       fmt.Sprintf("Test Task %d", i),
			Description: "Test description",
			TenantID:    "tenant-1",
		})
		if err != nil {
			t.Fatalf("failed to create task %d: %v", i, err)
		}
		taskIDs[i] = task.ID
	}

	// 并发更新不同任务
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(taskID string) {
			defer wg.Done()
			lm.TransitionTo(context.Background(), taskID, StatusPlanning)
			lm.TransitionTo(context.Background(), taskID, StatusRunning)
			lm.TransitionTo(context.Background(), taskID, StatusCompleted)
		}(taskIDs[i])
	}

	wg.Wait()

	// 验证所有任务都成功更新
	for i := 0; i < 10; i++ {
		task, ok := store.Get(taskIDs[i])
		if !ok {
			t.Errorf("task %d not found", i)
			continue
		}
		if task.Status != StatusCompleted {
			t.Errorf("task %d: expected status %s, got %s", i, StatusCompleted, task.Status)
		}
	}
}

func TestLifecycleManager_ListenerPanic(t *testing.T) {
	store := NewStore("")
	lm := NewLifecycleManager(store)

	// 创建测试任务
	task, err := store.Create(CreateRequest{
		Title:       "Test Task",
		Description: "Test description",
		TenantID:    "tenant-1",
	})
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	taskID := task.ID

	// 注册会 panic 的监听器
	lm.OnEvent(func(ctx context.Context, event LifecycleEvent) {
		panic("listener panic")
	})

	// 注册正常的监听器
	var called bool
	lm.OnEvent(func(ctx context.Context, event LifecycleEvent) {
		called = true
	})

	// 触发状态变更（不应该因为 panic 而失败）
	err = lm.TransitionTo(context.Background(), taskID, StatusPlanning)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// 验证第二个监听器仍然被调用
	if !called {
		t.Error("expected second listener to be called despite first listener panic")
	}
}

func TestValidateTransition(t *testing.T) {
	tests := []struct {
		name      string
		old       Status
		new       Status
		wantError bool
	}{
		{"pending to planning", StatusPending, StatusPlanning, false},
		{"planning to running", StatusPlanning, StatusRunning, false},
		{"running to completed", StatusRunning, StatusCompleted, false},
		{"running to failed", StatusRunning, StatusFailed, false},
		{"failed to running (retry)", StatusFailed, StatusRunning, false},
		{"pending to completed (invalid)", StatusPending, StatusCompleted, true},
		{"completed to running (invalid)", StatusCompleted, StatusRunning, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTransition(tt.old, tt.new)
			if (err != nil) != tt.wantError {
				t.Errorf("validateTransition() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
