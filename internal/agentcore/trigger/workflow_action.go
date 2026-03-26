package trigger

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// ──────────────────────────────────────────────
// WorkflowActionHandler — bridge between Trigger and Workflow Engine
//
// When a trigger fires with ActionRunWorkflow, this handler:
// 1. Extracts workflow_id from trigger.Action.Data
// 2. Creates a workflow instance
// 3. Runs it asynchronously
//
// Usage:
//   wfHandler := trigger.NewWorkflowActionHandler(createAndRun)
//   // In your ActionHandler switch:
//   case ActionRunWorkflow: wfHandler.Handle(ctx, trigger, event)
// ──────────────────────────────────────────────

// WorkflowRunner creates an instance and runs a workflow by definition ID.
// Returns (instanceID, error).
type WorkflowRunner func(ctx context.Context, definitionID, tenantID string, variables map[string]any) (string, error)

// WorkflowActionHandler handles ActionRunWorkflow triggers.
type WorkflowActionHandler struct {
	runner WorkflowRunner
}

// NewWorkflowActionHandler creates a handler for run_workflow actions.
func NewWorkflowActionHandler(runner WorkflowRunner) *WorkflowActionHandler {
	return &WorkflowActionHandler{runner: runner}
}

// Handle executes a workflow from a trigger action.
func (h *WorkflowActionHandler) Handle(ctx context.Context, t *Trigger, event *EventPayload) error {
	if h.runner == nil {
		return fmt.Errorf("workflow runner not configured")
	}

	wfID, _ := t.Action.Data["workflow_id"].(string)
	if wfID == "" {
		return fmt.Errorf("trigger %s: missing workflow_id in action data", t.ID)
	}

	tenantID := ""
	if event != nil {
		tenantID = event.TenantID
	}

	// Merge trigger event data as workflow variables
	vars := make(map[string]any)
	if event != nil && event.Data != nil {
		for k, v := range event.Data {
			vars[k] = v
		}
	}
	// Add trigger context
	vars["_trigger_id"] = t.ID
	vars["_trigger_name"] = t.Name

	instanceID, err := h.runner(ctx, wfID, tenantID, vars)
	if err != nil {
		slog.Error("trigger: workflow execution failed",
			"trigger", t.ID, "workflow", wfID, "err", err)
		return fmt.Errorf("run workflow %s: %w", wfID, err)
	}

	slog.Info("trigger: workflow started",
		"trigger", t.ID, "workflow", wfID, "instance", instanceID)
	return nil
}

// ParseTriggerExpr parses a trigger expression like "cron:0 8 * * *" or "event:task_completed".
// Returns (type, value).
func ParseTriggerExpr(expr string) (TriggerType, string) {
	parts := strings.SplitN(expr, ":", 2)
	if len(parts) != 2 {
		return TriggerTypeEvent, expr
	}
	switch parts[0] {
	case "cron", "time":
		return TriggerTypeTime, parts[1]
	case "event":
		return TriggerTypeEvent, parts[1]
	case "condition":
		return TriggerTypeCondition, parts[1]
	case "keyword":
		return TriggerTypeEvent, parts[1] // keyword triggers are event-based
	default:
		return TriggerTypeEvent, expr
	}
}
