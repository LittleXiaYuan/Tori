package ledger

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// CompactConfig controls event log compaction behavior.
type CompactConfig struct {
	// MinAge: only compact tasks finished at least this long ago (default 7d).
	MinAge time.Duration
	// KeepKinds: event kinds to preserve even after compaction (always kept: task lifecycle).
	KeepKinds []EventKind
	// MaxEventsPerTask: if a task has more events than this, compact (default 200).
	MaxEventsPerTask int
	// DryRun: if true, only log what would be compacted without deleting.
	DryRun bool
}

// DefaultCompactConfig returns sensible defaults for event compaction.
func DefaultCompactConfig() CompactConfig {
	return CompactConfig{
		MinAge:           7 * 24 * time.Hour,
		MaxEventsPerTask: 200,
		KeepKinds: []EventKind{
			EventTaskCreated, EventTaskCompleted, EventTaskFailed, EventTaskCancelled,
			EventReasoningPlan, EventReasoningReflect,
		},
	}
}

// CompactResult summarizes the result of a compaction run.
type CompactResult struct {
	TasksScanned     int `json:"tasks_scanned"`
	TasksCompacted   int `json:"tasks_compacted"`
	EventsRemoved    int `json:"events_removed"`
	EventsRetained   int `json:"events_retained"`
	SnapshotsCreated int `json:"snapshots_created"`
}

// CompactSummary is the snapshot written when compacting a task's events.
type CompactSummary struct {
	TaskID          string         `json:"task_id"`
	TotalEvents     int            `json:"total_events"`
	RetainedEvents  int            `json:"retained_events"`
	RemovedEvents   int            `json:"removed_events"`
	FirstEvent      time.Time      `json:"first_event"`
	LastEvent       time.Time      `json:"last_event"`
	EventKindCounts map[string]int `json:"event_kind_counts"`
}

const EventCompacted EventKind = "compaction.snapshot"

// CompactEvents performs event log compaction on completed/failed/cancelled tasks.
//
// For each qualifying task:
//  1. Counts events; skips if below MaxEventsPerTask
//  2. Creates a compaction snapshot event summarizing what was removed
//  3. Removes non-essential events (keeping task lifecycle + reasoning landmarks)
//  4. Preserves last checkpoint for crash recovery
//
// This prevents unbounded event log growth for long-running agents.
func (l *Ledger) CompactEvents(ctx context.Context, tenantID string, cfg CompactConfig) (*CompactResult, error) {
	if cfg.MinAge == 0 {
		cfg.MinAge = 7 * 24 * time.Hour
	}
	if cfg.MaxEventsPerTask == 0 {
		cfg.MaxEventsPerTask = 200
	}

	cutoff := time.Now().Add(-cfg.MinAge)
	result := &CompactResult{}

	// Find terminal tasks older than MinAge
	tasks, err := l.backend.ListTasks(ctx, TaskFilter{
		TenantID: tenantID,
		Status:   []TaskStatus{TaskCompleted, TaskFailed, TaskCancelled},
		Limit:    500,
	})
	if err != nil {
		return nil, fmt.Errorf("compact: list tasks: %w", err)
	}

	keepSet := make(map[EventKind]bool)
	for _, k := range cfg.KeepKinds {
		keepSet[k] = true
	}
	// Always keep task lifecycle + checkpoint events
	for _, k := range []EventKind{
		EventTaskCreated, EventTaskStarted, EventTaskCompleted,
		EventTaskFailed, EventTaskCancelled, EventCompacted,
		EventCheckpointCreated,
	} {
		keepSet[k] = true
	}

	for _, t := range tasks {
		if t.FinishedAt == nil || t.FinishedAt.After(cutoff) {
			continue
		}
		result.TasksScanned++

		events, err := l.Events.ListAll(ctx, t.ID)
		if err != nil {
			slog.Warn("compact: list events failed", "task", t.ID, "err", err)
			continue
		}

		if len(events) <= cfg.MaxEventsPerTask {
			result.EventsRetained += len(events)
			continue
		}

		// Build summary before compaction
		summary := CompactSummary{
			TaskID:          t.ID,
			TotalEvents:     len(events),
			EventKindCounts: make(map[string]int),
		}
		if len(events) > 0 {
			summary.FirstEvent = events[0].CreatedAt
			summary.LastEvent = events[len(events)-1].CreatedAt
		}

		// Resume replays events with seq > checkpoint.EventSeq on top of the
		// checkpoint state (failed tasks are resumable), so the window after
		// the last checkpoint must survive compaction.
		var cpSeq int64 = -1
		if cp, err := l.backend.LatestCheckpoint(ctx, t.ID); err == nil && cp != nil {
			cpSeq = cp.EventSeq
		}

		var toRemove []string
		for _, e := range events {
			summary.EventKindCounts[string(e.Kind)]++
			if keepSet[e.Kind] {
				continue
			}
			if cpSeq >= 0 && e.Seq > cpSeq {
				continue
			}
			toRemove = append(toRemove, e.ID)
		}

		summary.RemovedEvents = len(toRemove)
		summary.RetainedEvents = len(events) - len(toRemove)

		if cfg.DryRun {
			slog.Info("compact: dry-run",
				"task", t.ID,
				"total", len(events),
				"would_remove", len(toRemove),
				"would_retain", summary.RetainedEvents,
			)
			result.TasksCompacted++
			result.EventsRemoved += len(toRemove)
			result.EventsRetained += summary.RetainedEvents
			continue
		}

		// Write compaction snapshot event
		snapPayload, _ := json.Marshal(summary)
		l.Events.Append(ctx, &Event{
			TaskID:    t.ID,
			Kind:      EventCompacted,
			Actor:     "compactor",
			Payload:   snapPayload,
			CreatedAt: time.Now(),
		})
		result.SnapshotsCreated++

		// Delete non-essential events
		// NOTE: This requires backend support for event deletion.
		// For backends that truly need append-only semantics, this is a no-op.
		if batchDeleter, ok := l.backend.(EventBatchDeleter); ok {
			// Prefer batch deletion for efficiency
			if err := batchDeleter.DeleteEvents(ctx, toRemove); err != nil {
				slog.Warn("compact: batch delete failed", "task", t.ID, "err", err)
			}
		} else if deleter, ok := l.backend.(EventDeleter); ok {
			for _, id := range toRemove {
				deleter.DeleteEvent(ctx, id)
			}
		} else {
			slog.Warn("compact: backend does not support event deletion, skipping physical removal",
				"task", t.ID, "would_remove", len(toRemove))
		}

		result.TasksCompacted++
		result.EventsRemoved += len(toRemove)
		result.EventsRetained += summary.RetainedEvents

		slog.Info("compact: task compacted",
			"task", t.ID,
			"removed", len(toRemove),
			"retained", summary.RetainedEvents,
		)
	}

	slog.Info("compact: run complete",
		"scanned", result.TasksScanned,
		"compacted", result.TasksCompacted,
		"events_removed", result.EventsRemoved,
		"events_retained", result.EventsRetained,
		"snapshots", result.SnapshotsCreated,
	)
	return result, nil
}

// EventDeleter is an optional interface for backends that support event deletion.
// Not all backends will support this (truly append-only stores won't).
type EventDeleter interface {
	DeleteEvent(ctx context.Context, eventID string) error
}

// EventBatchDeleter is an optional interface for backends that support batch event deletion.
// More efficient than deleting one by one.
type EventBatchDeleter interface {
	DeleteEvents(ctx context.Context, eventIDs []string) error
}
