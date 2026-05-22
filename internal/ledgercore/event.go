package ledger

import (
	"context"
	"encoding/json"
	"time"

	"yunque-agent/internal/ledgercore/internal/ulid"
)

// EventStore provides event sourcing operations over the Backend.
type EventStore struct {
	backend Backend
	bus     *EventBus
}

// Append writes an event to the log. The event ID and CreatedAt are auto-assigned
// if not already set. The Seq is auto-assigned by the backend.
// After persisting, the event is published to the EventBus for real-time subscribers.
func (es *EventStore) Append(ctx context.Context, e *Event) error {
	if e.ID == "" {
		e.ID = ulid.New()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	if err := es.backend.AppendEvent(ctx, e); err != nil {
		return err
	}
	if es.bus != nil {
		es.bus.Publish(e)
	}
	return nil
}

// List returns events for a task after the given sequence number.
func (es *EventStore) List(ctx context.Context, taskID string, afterSeq int64, limit int) ([]*Event, error) {
	return es.backend.ListEvents(ctx, taskID, afterSeq, limit)
}

// ListAll returns all events for a task.
func (es *EventStore) ListAll(ctx context.Context, taskID string) ([]*Event, error) {
	return es.backend.ListEvents(ctx, taskID, 0, 0)
}

// Replay reconstructs a Task's current state by replaying all its events.
// This is the canonical way to verify state consistency.
func (es *EventStore) Replay(ctx context.Context, taskID string) (*Task, error) {
	events, err := es.ListAll(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, ErrTaskNotFound
	}

	t := &Task{ID: taskID}
	for _, e := range events {
		ApplyEvent(t, e)
	}
	return t, nil
}

// ReplayFrom reconstructs state starting from a checkpoint, applying only
// events after the checkpoint's sequence number.
func (es *EventStore) ReplayFrom(ctx context.Context, cp *Checkpoint) (*Task, error) {
	// Restore task from checkpoint
	t := &Task{}
	if err := json.Unmarshal(cp.TaskState, t); err != nil {
		return nil, err
	}

	// Apply incremental events
	events, err := es.List(ctx, cp.TaskID, cp.EventSeq, 0)
	if err != nil {
		return nil, err
	}
	for _, e := range events {
		ApplyEvent(t, e)
	}
	return t, nil
}

// Query performs a flexible event query across multiple dimensions.
func (es *EventStore) Query(ctx context.Context, q EventQuery) ([]*Event, error) {
	return es.backend.QueryEvents(ctx, q)
}

// QueryByTime returns events within a time range, optionally filtered by kinds.
func (es *EventStore) QueryByTime(ctx context.Context, after, before time.Time, kinds []EventKind, limit int) ([]*Event, error) {
	q := EventQuery{
		After:  &after,
		Before: &before,
		Kinds:  kinds,
		Limit:  limit,
	}
	return es.backend.QueryEvents(ctx, q)
}

// QueryTaskByTime returns events for a specific task within a time range.
func (es *EventStore) QueryTaskByTime(ctx context.Context, taskID string, after, before time.Time, limit int) ([]*Event, error) {
	q := EventQuery{
		TaskID: taskID,
		After:  &after,
		Before: &before,
		Limit:  limit,
	}
	return es.backend.QueryEvents(ctx, q)
}

// EmitTransition creates and appends a state transition event.
// It validates the transition, assigns the event kind, and persists it.
func (es *EventStore) EmitTransition(ctx context.Context, taskID string, from, to TaskStatus, actor string, payload JSON) (*Event, error) {
	if err := ValidateTransition(taskID, from, to); err != nil {
		return nil, err
	}

	kind, err := EventKindForTransition(from, to)
	if err != nil {
		return nil, err
	}

	e := &Event{
		ID:        ulid.New(),
		TaskID:    taskID,
		Kind:      kind,
		Actor:     actor,
		Payload:   payload,
		CreatedAt: time.Now(),
	}
	if err := es.Append(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}
