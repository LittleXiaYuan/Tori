package task

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
)

// ──────────────────────────────────────────────
// TaskEventSink — interface for external event stores (e.g. Ledger)
//
// The LifecycleManager emits lifecycle events through this sink.
// Implementors can persist events to an event-sourced backend,
// save checkpoints, build audit trails, etc.
//
// This is the integration point between task.Runner and ledger.
// ──────────────────────────────────────────────

// TaskEventSink receives lifecycle events from the task runtime.
// All methods must be safe for concurrent use and should not block
// the main execution flow (wrap slow I/O in goroutines if needed).
type TaskEventSink interface {
	// OnTaskCreated is called when a new task is created.
	OnTaskCreated(ctx context.Context, t *Task) error

	// OnTaskTransition is called when a task changes state.
	OnTaskTransition(ctx context.Context, taskID string, oldStatus, newStatus Status) error

	// OnStepStarted is called when a step begins execution.
	OnStepStarted(ctx context.Context, taskID string, stepIndex int, step *Step) error

	// OnStepCompleted is called when a step finishes successfully.
	// Implementations should use this to save checkpoints.
	OnStepCompleted(ctx context.Context, taskID string, stepIndex int, result string) error

	// OnStepFailed is called when a step fails.
	OnStepFailed(ctx context.Context, taskID string, stepIndex int, err error) error
}

// LedgerBridge adapts the Ledger to the TaskEventSink interface.
// It is constructed outside the task package (e.g. in bootstrap/init_gateway)
// and injected into LifecycleManager via SetEventSink().
//
// This bridge pattern keeps the `task` package free of the `ledger` import
// while allowing Ledger to receive full lifecycle telemetry.
type LedgerBridge struct {
	sink TaskEventSink
}

// NewLedgerBridge wraps a TaskEventSink (typically backed by ledger.Ledger).
func NewLedgerBridge(sink TaskEventSink) *LedgerBridge {
	return &LedgerBridge{sink: sink}
}

// forwardTaskCreated safely delegates to the sink.
func (lb *LedgerBridge) forwardTaskCreated(ctx context.Context, t *Task) {
	if lb == nil || lb.sink == nil {
		return
	}
	if err := lb.sink.OnTaskCreated(ctx, t); err != nil {
		slog.Warn("ledger bridge: OnTaskCreated failed", "task", t.ID, "err", err)
	}
}

// forwardTransition safely delegates to the sink.
func (lb *LedgerBridge) forwardTransition(ctx context.Context, taskID string, oldStatus, newStatus Status) {
	if lb == nil || lb.sink == nil {
		return
	}
	if err := lb.sink.OnTaskTransition(ctx, taskID, oldStatus, newStatus); err != nil {
		slog.Warn("ledger bridge: OnTaskTransition failed",
			"task", taskID,
			"from", oldStatus,
			"to", newStatus,
			"err", err,
		)
	}
}

// forwardStepStarted safely delegates to the sink.
func (lb *LedgerBridge) forwardStepStarted(ctx context.Context, taskID string, stepIndex int, step *Step) {
	if lb == nil || lb.sink == nil {
		return
	}
	if err := lb.sink.OnStepStarted(ctx, taskID, stepIndex, step); err != nil {
		slog.Warn("ledger bridge: OnStepStarted failed", "task", taskID, "step", stepIndex, "err", err)
	}
}

// forwardStepCompleted safely delegates to the sink.
func (lb *LedgerBridge) forwardStepCompleted(ctx context.Context, taskID string, stepIndex int, result string) {
	if lb == nil || lb.sink == nil {
		return
	}
	if err := lb.sink.OnStepCompleted(ctx, taskID, stepIndex, result); err != nil {
		slog.Warn("ledger bridge: OnStepCompleted failed", "task", taskID, "step", stepIndex, "err", err)
	}
}

// forwardStepFailed safely delegates to the sink.
func (lb *LedgerBridge) forwardStepFailed(ctx context.Context, taskID string, stepIndex int, stepErr error) {
	if lb == nil || lb.sink == nil {
		return
	}
	if err := lb.sink.OnStepFailed(ctx, taskID, stepIndex, stepErr); err != nil {
		slog.Warn("ledger bridge: OnStepFailed failed", "task", taskID, "step", stepIndex, "err", err)
	}
}

// ──────────────────────────────────────────────
// LogEventSink — built-in sink that writes structured logs
// (useful as a fallback when Ledger is not available)
// ──────────────────────────────────────────────

// LogEventSink implements TaskEventSink using structured logging.
type LogEventSink struct{}

func (s *LogEventSink) OnTaskCreated(_ context.Context, t *Task) error {
	slog.Info("task.event: created", "task", t.ID, "title", t.Title)
	return nil
}

func (s *LogEventSink) OnTaskTransition(_ context.Context, taskID string, oldStatus, newStatus Status) error {
	slog.Info("task.event: transition", "task", taskID, "from", oldStatus, "to", newStatus)
	return nil
}

func (s *LogEventSink) OnStepStarted(_ context.Context, taskID string, stepIndex int, step *Step) error {
	slog.Info("task.event: step_started", "task", taskID, "step", stepIndex, "action", step.Action)
	return nil
}

func (s *LogEventSink) OnStepCompleted(_ context.Context, taskID string, stepIndex int, result string) error {
	preview := result
	if len([]rune(preview)) > 80 {
		preview = string([]rune(preview)[:80]) + "..."
	}
	slog.Info("task.event: step_completed", "task", taskID, "step", stepIndex, "preview", preview)
	return nil
}

func (s *LogEventSink) OnStepFailed(_ context.Context, taskID string, stepIndex int, err error) error {
	slog.Warn("task.event: step_failed", "task", taskID, "step", stepIndex, "err", err)
	return nil
}

// ──────────────────────────────────────────────
// Status mapping helpers (for Ledger adapter usage)
// ──────────────────────────────────────────────

// StatusJSON returns a JSON payload for a task status transition.
func StatusJSON(from, to Status) json.RawMessage {
	payload, _ := json.Marshal(map[string]string{
		"from": string(from),
		"to":   string(to),
	})
	return payload
}

// StepJSON returns a JSON payload for a step event.
func StepJSON(stepIndex int, action, result string, err error) json.RawMessage {
	m := map[string]any{
		"step_index": stepIndex,
		"action":     action,
	}
	if result != "" {
		if len(result) > 200 {
			m["result_preview"] = result[:200] + "..."
		} else {
			m["result"] = result
		}
	}
	if err != nil {
		m["error"] = fmt.Sprintf("%v", err)
	}
	payload, _ := json.Marshal(m)
	return payload
}
