package ledger

import (
	"context"
	"encoding/json"
	"time"

	"yunque-agent/internal/ledgercore/internal/ulid"
)

// CheckpointManager handles creation, loading, and cleanup of task checkpoints.
type CheckpointManager struct {
	backend Backend
	events  *EventStore
}

// Save creates a checkpoint for the given task at the current event position.
// It captures the full task state and optional working memory snapshot.
func (cm *CheckpointManager) Save(ctx context.Context, taskID string, stepIndex int, workingMem JSON, reason string) (*Checkpoint, error) {
	// Optimistic snapshot: the task row and the event cursor must describe
	// the same moment, otherwise Resume replays from a cursor that is ahead
	// of the stored state and returns stale data. If the seq moved while we
	// read the task, retry; equal seqs before and after the read mean no
	// event landed in between.
	var task *Task
	var eventSeq int64
	for attempt := 0; ; attempt++ {
		s1, err := cm.backend.LatestEventSeq(ctx, taskID)
		if err != nil {
			return nil, err
		}
		task, err = cm.backend.GetTask(ctx, taskID)
		if err != nil {
			return nil, err
		}
		s2, err := cm.backend.LatestEventSeq(ctx, taskID)
		if err != nil {
			return nil, err
		}
		eventSeq = s2
		if s1 == s2 || attempt >= 3 {
			break
		}
	}

	// Serialize task state
	taskState, err := json.Marshal(task)
	if err != nil {
		return nil, err
	}

	if workingMem == nil {
		workingMem = JSON("{}")
	}

	cp := &Checkpoint{
		ID:         ulid.New(),
		TaskID:     taskID,
		EventSeq:   eventSeq,
		StepIndex:  stepIndex,
		TaskState:  taskState,
		WorkingMem: workingMem,
		SizeBytes:  int64(len(taskState) + len(workingMem)),
		Reason:     reason,
		CreatedAt:  time.Now(),
	}

	if err := cm.backend.SaveCheckpoint(ctx, cp); err != nil {
		return nil, err
	}

	payload, err := json.Marshal(map[string]interface{}{
		"checkpoint_id": cp.ID,
		"event_seq":     cp.EventSeq,
		"reason":        reason,
	})
	if err != nil {
		return cp, err
	}
	if err := cm.events.Append(ctx, &Event{
		ID:        ulid.New(),
		TaskID:    taskID,
		Kind:      EventCheckpointCreated,
		Actor:     "runtime",
		Payload:   payload,
		CreatedAt: cp.CreatedAt,
	}); err != nil {
		return cp, err
	}

	task.CheckpointRef = &cp.ID
	task.UpdatedAt = cp.CreatedAt
	if err := cm.backend.UpdateTask(ctx, task); err != nil {
		return cp, err
	}

	return cp, nil
}

// Latest returns the most recent checkpoint for a task.
func (cm *CheckpointManager) Latest(ctx context.Context, taskID string) (*Checkpoint, error) {
	return cm.backend.LatestCheckpoint(ctx, taskID)
}

// List returns checkpoints for a task, most recent first.
func (cm *CheckpointManager) List(ctx context.Context, taskID string, limit int) ([]*Checkpoint, error) {
	return cm.backend.ListCheckpoints(ctx, taskID, limit)
}

// Cleanup removes old checkpoints, keeping only the N most recent.
func (cm *CheckpointManager) Cleanup(ctx context.Context, taskID string, keepCount int) error {
	cps, err := cm.backend.ListCheckpoints(ctx, taskID, 0)
	if err != nil {
		return err
	}

	if len(cps) <= keepCount {
		return nil // nothing to clean
	}

	// keepCount <= 0 means delete everything (guard against cps[-1] panic).
	if keepCount <= 0 {
		return cm.backend.DeleteCheckpointsBefore(ctx, taskID, cps[0].EventSeq+1)
	}

	// cps is sorted DESC by created_at, keep first keepCount
	cutoff := cps[keepCount-1].EventSeq
	return cm.backend.DeleteCheckpointsBefore(ctx, taskID, cutoff)
}
