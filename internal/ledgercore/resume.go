package ledger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// ResumeManager handles task recovery from checkpoints.
type ResumeManager struct {
	backend     Backend
	events      *EventStore
	checkpoints *CheckpointManager
}

// ResumeResult contains the recovered task state and metadata.
type ResumeResult struct {
	Task             *Task  `json:"task"`
	WorkingMem       JSON   `json:"working_mem"`
	ResumedFromEvent int64  `json:"resumed_from_event"` // checkpoint's event seq
	EventsReplayed   int    `json:"events_replayed"`    // number of events applied after checkpoint
	StepIndex        int    `json:"step_index"`         // step to resume from
}

// Resume recovers a task from its latest checkpoint + incremental event replay.
//
// The process:
//  1. Load latest checkpoint (task state + working memory + event seq)
//  2. Deserialize task state
//  3. Replay events after checkpoint's seq (incremental recovery)
//  4. Return the recovered state with metadata
//
// If no checkpoint exists, falls back to full event replay.
func (rm *ResumeManager) Resume(ctx context.Context, taskID string) (*ResumeResult, error) {
	cp, err := rm.checkpoints.Latest(ctx, taskID)
	if err != nil {
		if errors.Is(err, ErrCheckpointNotFound) {
			return rm.resumeFromEvents(ctx, taskID)
		}
		return nil, fmt.Errorf("load checkpoint: %w", err)
	}

	// Restore task from checkpoint
	task := &Task{}
	if err := json.Unmarshal(cp.TaskState, task); err != nil {
		return nil, fmt.Errorf("unmarshal checkpoint task state: %w", err)
	}

	// Incremental replay: only events after checkpoint
	events, err := rm.events.List(ctx, taskID, cp.EventSeq, 0)
	if err != nil {
		return nil, fmt.Errorf("list events after checkpoint: %w", err)
	}

	for _, e := range events {
		ApplyEvent(task, e)
	}

	return &ResumeResult{
		Task:             task,
		WorkingMem:       cp.WorkingMem,
		ResumedFromEvent: cp.EventSeq,
		EventsReplayed:   len(events),
		StepIndex:        cp.StepIndex,
	}, nil
}

// resumeFromEvents does a full replay when no checkpoint exists.
func (rm *ResumeManager) resumeFromEvents(ctx context.Context, taskID string) (*ResumeResult, error) {
	task, err := rm.events.Replay(ctx, taskID)
	if err != nil {
		return nil, err
	}

	return &ResumeResult{
		Task:             task,
		WorkingMem:       JSON("{}"),
		ResumedFromEvent: 0,
		EventsReplayed:   -1, // indicates full replay
		StepIndex:        0,
	}, nil
}

// CanResume checks whether a task is in a state that allows resumption.
func (rm *ResumeManager) CanResume(ctx context.Context, taskID string) (bool, error) {
	task, err := rm.backend.GetTask(ctx, taskID)
	if err != nil {
		return false, err
	}

	switch task.Status {
	case TaskFailed, TaskBlocked, TaskRetrying, TaskWaitingInput:
		return true, nil
	default:
		return false, nil
	}
}
