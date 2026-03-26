package ledger

import (
	"context"
	"log/slog"

	"ledger"

	agtask "yunque-agent/internal/agentcore/task"
)

// LedgerSync mirrors task events into the Ledger for event sourcing,
// checkpointing, and structured memory.
type LedgerSync struct {
	ldg       *ledger.Ledger
	taskStore agtask.Store
}

// NewLedgerSync creates a LedgerSync that bridges task events to the Ledger.
func NewLedgerSync(ldg *ledger.Ledger, ts agtask.Store) *LedgerSync {
	return &LedgerSync{ldg: ldg, taskStore: ts}
}

// runnerEventToLedger maps Runner event strings to Ledger EventKinds.
var runnerEventToLedger = map[string]ledger.EventKind{
	"task_started":   ledger.EventTaskStarted,
	"task_completed": ledger.EventTaskCompleted,
	"task_failed":    ledger.EventTaskFailed,
	"task_paused":    ledger.EventTaskWaitingInput,
	"task_resumed":   ledger.EventTaskResumed,
	"task_restarted": ledger.EventTaskResumed,
	"task_cancelled": ledger.EventTaskCancelled,
	"step_completed": ledger.EventStepCompleted,
	"step_started":   ledger.EventStepStarted,
	"step_failed":    ledger.EventStepFailed,
}

// OnEvent handles a task event and records it in the Ledger.
// Matches the Runner.OnTaskEvent callback signature: func(event, taskID, detail string).
func (ls *LedgerSync) OnEvent(event, taskID, detail string) {
	kind, ok := runnerEventToLedger[event]
	if !ok {
		// Unknown event type — log but don't fail
		slog.Debug("ledger sync: unknown event type", "event", event, "task", taskID)
		return
	}

	// Determine actor based on event type
	actor := "runtime"
	switch event {
	case "task_paused", "task_cancelled":
		actor = "user"
	}

	payload := ledger.MakePayload(map[string]interface{}{
		"event":  event,
		"detail": detail,
	})

	ctx := context.Background()
	err := ls.ldg.Events.Append(ctx, &ledger.Event{
		TaskID:  taskID,
		Kind:    kind,
		Actor:   actor,
		Payload: payload,
	})
	if err != nil {
		slog.Warn("ledger sync: failed to record event",
			"event", event, "task", taskID, "err", err)
	}
}

// Ledger returns the underlying Ledger instance for direct access.
func (ls *LedgerSync) Ledger() *ledger.Ledger {
	return ls.ldg
}
