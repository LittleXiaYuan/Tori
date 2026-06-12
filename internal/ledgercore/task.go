package ledger

import (
	"context"
	"encoding/json"
	"time"

	"yunque-agent/internal/ledgercore/internal/ulid"
)

// TaskManager provides high-level task operations that enforce
// event sourcing invariants. Every state mutation goes through events.
type TaskManager struct {
	backend Backend
	events  *EventStore
}

type transactionalTaskBackend interface {
	CreateTaskWithEvent(ctx context.Context, t *Task, e *Event) error
	UpdateTaskWithEvent(ctx context.Context, t *Task, e *Event) error
}

// CreateTask creates a new task and emits a task.created event.
func (tm *TaskManager) CreateTask(ctx context.Context, goal string, taskType TaskType, tenantID string, opts ...TaskOption) (*Task, error) {
	now := time.Now()
	t := &Task{
		ID:         ulid.New(),
		Type:       taskType,
		Goal:       goal,
		Status:     TaskCreated,
		TenantID:   tenantID,
		AgentID:    "default",
		Input:      JSON("{}"),
		Output:     JSON("{}"),
		Metadata:   JSON("{}"),
		MaxRetries: 2,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	for _, opt := range opts {
		opt(t)
	}

	// The created event must carry every initial field so Replay reconstructs
	// the same state as the materialized row (INV-1).
	createdPayload := map[string]interface{}{
		"goal":        t.Goal,
		"type":        t.Type,
		"tenant_id":   t.TenantID,
		"agent_id":    t.AgentID,
		"max_retries": t.MaxRetries,
	}
	if t.UserID != "" {
		createdPayload["user_id"] = t.UserID
	}
	if len(t.Input) > 0 {
		createdPayload["input"] = json.RawMessage(t.Input)
	}
	if len(t.Metadata) > 0 {
		createdPayload["metadata"] = json.RawMessage(t.Metadata)
	}
	if t.Priority != 0 {
		createdPayload["priority"] = t.Priority
	}
	if t.ParentTaskID != nil {
		createdPayload["parent_task_id"] = *t.ParentTaskID
	}
	payload, err := json.Marshal(createdPayload)
	if err != nil {
		return nil, err
	}
	event := &Event{
		ID:        ulid.New(),
		TaskID:    t.ID,
		Kind:      EventTaskCreated,
		Actor:     "runtime",
		Payload:   payload,
		CreatedAt: now,
	}

	if tb, ok := tm.backend.(transactionalTaskBackend); ok {
		if err := tb.CreateTaskWithEvent(ctx, t, event); err != nil {
			return nil, err
		}
		if tm.events.bus != nil {
			tm.events.bus.Publish(event)
		}
		return t, nil
	}

	// Persist task
	if err := tm.backend.CreateTask(ctx, t); err != nil {
		return nil, err
	}
	if err := tm.events.Append(ctx, event); err != nil {
		return nil, err
	}

	return t, nil
}

// GetTask retrieves a task by ID.
func (tm *TaskManager) GetTask(ctx context.Context, id string) (*Task, error) {
	return tm.backend.GetTask(ctx, id)
}

// ListTasks lists tasks matching the given filter.
func (tm *TaskManager) ListTasks(ctx context.Context, f TaskFilter) ([]*Task, error) {
	return tm.backend.ListTasks(ctx, f)
}

// Transition moves a task from one state to another.
// It validates the transition, emits an event, and updates the materialized view.
// Uses optimistic locking with automatic retry on version conflicts.
func (tm *TaskManager) Transition(ctx context.Context, taskID string, to TaskStatus, actor string, payload JSON) error {
	const maxRetries = 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		t, err := tm.backend.GetTask(ctx, taskID)
		if err != nil {
			return err
		}

		if err := ValidateTransition(taskID, t.Status, to); err != nil {
			return err
		}

		now := time.Now()
		from := t.Status
		t.Status = to
		t.UpdatedAt = now

		switch to {
		case TaskRunning:
			if t.StartedAt == nil {
				t.StartedAt = &now
			}
		case TaskCompleted, TaskFailed, TaskCancelled:
			t.FinishedAt = &now
		case TaskRetrying:
			t.RetryCount++
		case TaskReady:
			if from == TaskFailed || from == TaskCancelled {
				t.Error = nil
				t.FinishedAt = nil
			}
		}

		var event *Event
		if tb, ok := tm.backend.(transactionalTaskBackend); ok {
			event, err = buildTransitionEvent(taskID, from, to, actor, payload)
			if err != nil {
				return err
			}
			err = tb.UpdateTaskWithEvent(ctx, t, event)
		} else {
			err = tm.backend.UpdateTask(ctx, t)
		}
		if err == ErrVersionConflict && attempt < maxRetries-1 {
			continue
		}
		if err != nil {
			return err
		}

		if event != nil {
			if tm.events.bus != nil {
				tm.events.bus.Publish(event)
			}
			return nil
		}
		// Emit event only after successful task update to avoid duplicate
		// transition events on version-conflict retries.
		_, err = tm.events.EmitTransition(ctx, taskID, from, to, actor, payload)
		return err
	}
	return ErrVersionConflict
}

// Complete marks a task as completed with the given output.
func (tm *TaskManager) Complete(ctx context.Context, taskID string, output JSON) error {
	t, err := tm.backend.GetTask(ctx, taskID)
	if err != nil {
		return err
	}

	if err := ValidateTransition(taskID, t.Status, TaskCompleted); err != nil {
		return err
	}

	from := t.Status
	now := time.Now()
	t.Status = TaskCompleted
	t.Output = output
	t.FinishedAt = &now
	t.UpdatedAt = now

	payload, _ := json.Marshal(map[string]interface{}{
		"output": json.RawMessage(output),
	})
	if tb, ok := tm.backend.(transactionalTaskBackend); ok {
		event, err := buildTransitionEvent(taskID, from, TaskCompleted, "runtime", payload)
		if err != nil {
			return err
		}
		if err := tb.UpdateTaskWithEvent(ctx, t, event); err != nil {
			return err
		}
		if tm.events.bus != nil {
			tm.events.bus.Publish(event)
		}
		return nil
	}
	if err := tm.backend.UpdateTask(ctx, t); err != nil {
		return err
	}
	_, err = tm.events.EmitTransition(ctx, taskID, from, TaskCompleted, "runtime", payload)
	return err
}

// Fail marks a task as failed with an error message.
func (tm *TaskManager) Fail(ctx context.Context, taskID string, errMsg string) error {
	t, err := tm.backend.GetTask(ctx, taskID)
	if err != nil {
		return err
	}

	if err := ValidateTransition(taskID, t.Status, TaskFailed); err != nil {
		return err
	}

	from := t.Status
	now := time.Now()
	t.Status = TaskFailed
	t.Error = &errMsg
	t.FinishedAt = &now
	t.UpdatedAt = now

	payload, _ := json.Marshal(map[string]interface{}{
		"error": errMsg,
	})
	if tb, ok := tm.backend.(transactionalTaskBackend); ok {
		event, err := buildTransitionEvent(taskID, from, TaskFailed, "runtime", payload)
		if err != nil {
			return err
		}
		if err := tb.UpdateTaskWithEvent(ctx, t, event); err != nil {
			return err
		}
		if tm.events.bus != nil {
			tm.events.bus.Publish(event)
		}
		return nil
	}
	if err := tm.backend.UpdateTask(ctx, t); err != nil {
		return err
	}
	_, err = tm.events.EmitTransition(ctx, taskID, from, TaskFailed, "runtime", payload)
	return err
}

func buildTransitionEvent(taskID string, from, to TaskStatus, actor string, payload JSON) (*Event, error) {
	if err := ValidateTransition(taskID, from, to); err != nil {
		return nil, err
	}
	kind, err := EventKindForTransition(from, to)
	if err != nil {
		return nil, err
	}
	return &Event{
		ID:        ulid.New(),
		TaskID:    taskID,
		Kind:      kind,
		Actor:     actor,
		Payload:   payload,
		CreatedAt: time.Now(),
	}, nil
}

// Cancel cancels a task.
func (tm *TaskManager) Cancel(ctx context.Context, taskID string, reason string) error {
	payload, err := json.Marshal(map[string]interface{}{
		"reason": reason,
	})
	if err != nil {
		return err
	}
	return tm.Transition(ctx, taskID, TaskCancelled, "user", payload)
}

// TaskOption configures optional fields when creating a task.
type TaskOption func(*Task)

func WithAgentID(id string) TaskOption {
	return func(t *Task) { t.AgentID = id }
}

func WithUserID(id string) TaskOption {
	return func(t *Task) { t.UserID = id }
}

func WithParentTask(id string) TaskOption {
	return func(t *Task) { t.ParentTaskID = &id }
}

func WithInput(input JSON) TaskOption {
	return func(t *Task) { t.Input = input }
}

func WithPriority(p int) TaskOption {
	return func(t *Task) { t.Priority = p }
}

func WithMaxRetries(n int) TaskOption {
	return func(t *Task) { t.MaxRetries = n }
}

func WithMetadata(m JSON) TaskOption {
	return func(t *Task) { t.Metadata = m }
}
