package task

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// LifecycleManager — 统一任务生命周期入口
//
// 职责：
//   1. 统一任务/步骤状态变更
//   2. 同步发射生命周期事件
//   3. 验证状态转换合法性
//   4. 保证并发安全（按 task 维度加锁）
//
// 设计原则：
//   - 事件同步发射（避免并发时序问题）
//   - 按 task 维度加锁（避免全局串行）
//   - 监听器只能做轻量副作用（日志、指标、通知）
//   - 不缓存 task，每次从 store 重新获取
// ──────────────────────────────────────────────

// LifecycleEvent 生命周期事件
type LifecycleEvent struct {
	Type      EventType
	TaskID    string
	StepID    int    // -1 表示任务级事件
	OldStatus string // 任务状态变更时有效
	NewStatus string // 任务状态变更时有效
	Result    string // 步骤完成时有效
	Error     error  // 步骤失败时有效
	Timestamp time.Time
}

// EventType 事件类型
type EventType string

const (
	EventTaskStateChanged EventType = "task_state_changed"
	EventStepStarted      EventType = "step_started"
	EventStepCompleted    EventType = "step_completed"
	EventStepFailed       EventType = "step_failed"
)

// LifecycleListener 生命周期监听器（同步调用）
//
// 约束：
//   - 只能做轻量副作用（日志、指标、通知）
//   - 不能阻塞主流程（< 100ms）
//   - 不能修改 task 状态（会导致死锁）
//   - 不能调用 lifecycle 的其他方法（会导致死锁）
type LifecycleListener func(ctx context.Context, event LifecycleEvent)

// LifecycleManager 生命周期管理器
type LifecycleManager struct {
	store        Store
	listeners    []LifecycleListener
	ledgerBridge *LedgerBridge // optional: forwards events to Ledger

	// 按 task 维度加锁，避免全局串行
	taskLocks sync.Map // taskID -> *sync.Mutex
}

// NewLifecycleManager 创建生命周期管理器
func NewLifecycleManager(store Store) *LifecycleManager {
	return &LifecycleManager{
		store: store,
	}
}

// SetEventSink connects a LedgerBridge to receive lifecycle events.
// The bridge is nil-safe, so calling this is optional.
func (lm *LifecycleManager) SetEventSink(bridge *LedgerBridge) {
	lm.ledgerBridge = bridge
}

// OnEvent 注册事件监听器
//
// 约束：
//   - 监听器必须是轻量的（< 100ms）
//   - 监听器不能修改 task 状态
//   - 监听器不能调用 lifecycle 的其他方法
func (lm *LifecycleManager) OnEvent(listener LifecycleListener) {
	lm.listeners = append(lm.listeners, listener)
}

// getTaskLock 获取指定 task 的锁
func (lm *LifecycleManager) getTaskLock(taskID string) *sync.Mutex {
	lock, _ := lm.taskLocks.LoadOrStore(taskID, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

// TransitionTo 统一的任务状态变更入口
func (lm *LifecycleManager) TransitionTo(ctx context.Context, taskID string, newStatus Status) error {
	// 按 task 维度加锁
	lock := lm.getTaskLock(taskID)
	lock.Lock()
	defer lock.Unlock()

	// 每次从 store 重新获取，避免快照覆盖
	task, ok := lm.store.Get(taskID)
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	oldStatus := task.Status
	if oldStatus == newStatus {
		return nil // 无需变更
	}

	// 验证状态转换合法性
	if err := validateTransition(oldStatus, newStatus); err != nil {
		return err
	}

	// 更新状态
	task.Status = newStatus
	task.UpdatedAt = time.Now()

	// 设置时间戳
	switch newStatus {
	case StatusRunning:
		if task.StartedAt == nil {
			now := time.Now()
			task.StartedAt = &now
		}
	case StatusCompleted, StatusFailed, StatusCancelled:
		now := time.Now()
		task.FinishedAt = &now
	}

	// 持久化
	if err := lm.store.Update(task); err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	// 解锁后同步发射事件（避免监听器中的死锁）
	lock.Unlock()
	lm.emitSync(ctx, LifecycleEvent{
		Type:      EventTaskStateChanged,
		TaskID:    taskID,
		StepID:    -1,
		OldStatus: string(oldStatus),
		NewStatus: string(newStatus),
		Timestamp: time.Now(),
	})
	lock.Lock() // 重新加锁，保证 defer unlock 正常工作

	// Forward to Ledger bridge (non-blocking, nil-safe)
	lm.ledgerBridge.forwardTransition(ctx, taskID, oldStatus, newStatus)

	slog.Info("task status changed",
		"task_id", taskID,
		"old_status", oldStatus,
		"new_status", newStatus,
	)

	return nil
}

// OnStepStart 步骤开始钩子
func (lm *LifecycleManager) OnStepStart(ctx context.Context, taskID string, stepID int) error {
	// 按 task 维度加锁
	lock := lm.getTaskLock(taskID)
	lock.Lock()
	defer lock.Unlock()

	// 每次从 store 重新获取
	task, ok := lm.store.Get(taskID)
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	if stepID < 0 || stepID >= len(task.Steps) {
		return fmt.Errorf("invalid step ID %d", stepID)
	}

	step := &task.Steps[stepID]

	// 幂等性检查：如果已经是 running，跳过
	if step.Status == StepRunning {
		return nil
	}

	step.Status = StepRunning
	now := time.Now()
	step.StartedAt = &now

	if err := lm.store.Update(task); err != nil {
		return err
	}

	// 解锁后同步发射事件
	lock.Unlock()
	lm.emitSync(ctx, LifecycleEvent{
		Type:      EventStepStarted,
		TaskID:    taskID,
		StepID:    stepID,
		Timestamp: time.Now(),
	})

	// Forward to Ledger bridge
	lm.ledgerBridge.forwardStepStarted(ctx, taskID, stepID, step)
	lock.Lock()

	return nil
}

// OnStepComplete 步骤完成钩子
func (lm *LifecycleManager) OnStepComplete(ctx context.Context, taskID string, stepID int, result string) error {
	// 按 task 维度加锁
	lock := lm.getTaskLock(taskID)
	lock.Lock()
	defer lock.Unlock()

	// 每次从 store 重新获取
	task, ok := lm.store.Get(taskID)
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	if stepID < 0 || stepID >= len(task.Steps) {
		return fmt.Errorf("invalid step ID %d", stepID)
	}

	step := &task.Steps[stepID]

	// 幂等性检查：如果已经是 done，跳过
	if step.Status == StepDone {
		return nil
	}

	step.Status = StepDone
	step.Result = result
	now := time.Now()
	step.DoneAt = &now

	if err := lm.store.Update(task); err != nil {
		return err
	}

	// 解锁后同步发射事件
	lock.Unlock()
	lm.emitSync(ctx, LifecycleEvent{
		Type:      EventStepCompleted,
		TaskID:    taskID,
		StepID:    stepID,
		Result:    result,
		Timestamp: time.Now(),
	})

	// Forward to Ledger bridge (triggers checkpoint save)
	lm.ledgerBridge.forwardStepCompleted(ctx, taskID, stepID, result)
	lock.Lock()

	return nil
}

// OnStepFailed 步骤失败钩子
func (lm *LifecycleManager) OnStepFailed(ctx context.Context, taskID string, stepID int, err error) error {
	// 按 task 维度加锁
	lock := lm.getTaskLock(taskID)
	lock.Lock()
	defer lock.Unlock()

	// 每次从 store 重新获取
	task, ok := lm.store.Get(taskID)
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	if stepID < 0 || stepID >= len(task.Steps) {
		return fmt.Errorf("invalid step ID %d", stepID)
	}

	step := &task.Steps[stepID]

	// 幂等性检查：如果已经是 failed，跳过
	if step.Status == StepFailed {
		return nil
	}

	step.Status = StepFailed
	step.Error = err.Error()
	now := time.Now()
	step.DoneAt = &now

	if storeErr := lm.store.Update(task); storeErr != nil {
		return storeErr
	}

	// 解锁后同步发射事件
	lock.Unlock()
	lm.emitSync(ctx, LifecycleEvent{
		Type:      EventStepFailed,
		TaskID:    taskID,
		StepID:    stepID,
		Error:     err,
		Timestamp: time.Now(),
	})

	// Forward to Ledger bridge
	lm.ledgerBridge.forwardStepFailed(ctx, taskID, stepID, err)
	lock.Lock()

	return nil
}

// emitSync 同步发射事件给所有监听器
func (lm *LifecycleManager) emitSync(ctx context.Context, event LifecycleEvent) {
	for _, listener := range lm.listeners {
		// 同步调用，保证时序
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("lifecycle listener panic", "error", r, "event", event.Type)
				}
			}()
			listener(ctx, event)
		}()
	}
}

// validateTransition 验证状态转换合法性
func validateTransition(old, new Status) error {
	// 定义合法的状态转换
	validTransitions := map[Status][]Status{
		StatusPending:     {StatusPlanning, StatusRunning, StatusCancelled}, // running: 预定义步骤跳过 planning
		StatusPlanning:    {StatusRunning, StatusFailed, StatusCancelled},
		StatusRunning:     {StatusPaused, StatusCompleted, StatusFailed, StatusInterrupted, StatusCancelled},
		StatusPaused:      {StatusRunning, StatusCancelled},
		StatusInterrupted: {StatusRunning, StatusFailed, StatusCancelled},
		StatusFailed:      {StatusRunning}, // 允许重试
	}

	allowed, ok := validTransitions[old]
	if !ok {
		return fmt.Errorf("invalid old status: %s", old)
	}

	for _, s := range allowed {
		if s == new {
			return nil
		}
	}

	return fmt.Errorf("invalid transition from %s to %s", old, new)
}
