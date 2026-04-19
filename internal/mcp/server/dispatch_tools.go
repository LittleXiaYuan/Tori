package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/task"
)

// TaskStoreRef provides the minimal interface needed to read/write tasks.
type TaskStoreRef interface {
	Get(id string) (*task.Task, bool)
	List(tenantID string, limit int) []*task.Task
	Update(t *task.Task) error
}

// DispatchContext bundles the dependencies needed by dispatch tools.
type DispatchContext struct {
	Workers   *WorkerRegistry
	TaskStore TaskStoreRef
}

// RegisterDispatchTools adds all 6 dispatch tools to the MCP server.
func RegisterDispatchTools(srv *Server, dc *DispatchContext) {
	srv.RegisterTool(registerWorkerTool(dc))
	srv.RegisterTool(getPendingTasksTool(dc))
	srv.RegisterTool(claimTaskTool(dc))
	srv.RegisterTool(reportProgressTool(dc))
	srv.RegisterTool(submitResultTool(dc))
	srv.RegisterTool(getTaskContextTool(dc))
}

// ──────────────────────────────────────────────
// 1. register_worker
// ──────────────────────────────────────────────

func registerWorkerTool(dc *DispatchContext) ToolDef {
	return ToolDef{
		Name:        "register_worker",
		Description: "Register this worker with the yunque orchestrator. Call once on startup to declare capabilities.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"worker_id":       map[string]any{"type": "string", "description": "Unique ID (optional, auto-generated if empty)"},
				"name":            map[string]any{"type": "string", "description": "Human-readable worker name"},
				"type":            map[string]any{"type": "string", "description": "Worker type: cursor, claude_code, windsurf, custom"},
				"capabilities":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "List of capability tags: coding, testing, review, docs, deploy"},
				"max_concurrency": map[string]any{"type": "integer", "description": "Maximum concurrent tasks (default 1)"},
			},
			"required": []string{"name", "type", "capabilities"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			name := stringArg(args, "name")
			wType := stringArg(args, "type")
			caps := stringSliceArg(args, "capabilities")
			if name == "" || wType == "" || len(caps) == 0 {
				return nil, fmt.Errorf("name, type, and capabilities are required")
			}
			id := stringArg(args, "worker_id")
			maxC := intArg(args, "max_concurrency", 1)

			w := dc.Workers.Register(id, name, wType, caps, maxC, nil)
			return map[string]any{
				"worker_id":    w.ID,
				"status":       w.Status,
				"registered_at": w.RegisteredAt.Format(time.RFC3339),
				"message":      fmt.Sprintf("Worker '%s' registered successfully with capabilities: %s", name, strings.Join(caps, ", ")),
			}, nil
		},
	}
}

// ──────────────────────────────────────────────
// 2. get_pending_tasks
// ──────────────────────────────────────────────

func getPendingTasksTool(dc *DispatchContext) ToolDef {
	return ToolDef{
		Name:        "get_pending_tasks",
		Description: "Get tasks available for this worker to claim, filtered by the worker's capabilities.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"worker_id": map[string]any{"type": "string", "description": "Your registered worker ID"},
				"limit":     map[string]any{"type": "integer", "description": "Maximum number of tasks to return (default 10)"},
			},
			"required": []string{"worker_id"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			workerID := stringArg(args, "worker_id")
			limit := intArg(args, "limit", 10)

			w, ok := dc.Workers.Get(workerID)
			if !ok {
				return nil, fmt.Errorf("worker '%s' not found; call register_worker first", workerID)
			}

			dc.Workers.Heartbeat(workerID)

			if dc.TaskStore == nil {
				return map[string]any{
					"tasks": []any{},
					"count": 0,
				}, nil
			}

			allTasks := dc.TaskStore.List("", 100)
			var pending []map[string]any
			for _, t := range allTasks {
				if !isDispatchable(t) {
					continue
				}
				if !matchesCapabilities(t, w.Capabilities) {
					continue
				}
				pending = append(pending, taskSummary(t))
				if len(pending) >= limit {
					break
				}
			}
			return map[string]any{
				"tasks": pending,
				"count": len(pending),
			}, nil
		},
	}
}

// ──────────────────────────────────────────────
// 3. claim_task
// ──────────────────────────────────────────────

func claimTaskTool(dc *DispatchContext) ToolDef {
	return ToolDef{
		Name:        "claim_task",
		Description: "Claim a pending task for execution. Only one worker can claim each task.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"worker_id": map[string]any{"type": "string", "description": "Your registered worker ID"},
				"task_id":   map[string]any{"type": "string", "description": "Task ID to claim"},
			},
			"required": []string{"worker_id", "task_id"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			workerID := stringArg(args, "worker_id")
			taskID := stringArg(args, "task_id")

			w, ok := dc.Workers.Get(workerID)
			if !ok {
				return nil, fmt.Errorf("worker '%s' not found", workerID)
			}

			if w.ActiveTasks >= w.MaxConcur {
				return nil, fmt.Errorf("worker at max concurrency (%d); finish a task first", w.MaxConcur)
			}

			if dc.TaskStore == nil {
				return nil, fmt.Errorf("task store not available")
			}

			t, ok := dc.TaskStore.Get(taskID)
			if !ok {
				return nil, fmt.Errorf("task '%s' not found", taskID)
			}
			if !isDispatchable(t) {
				return nil, fmt.Errorf("task '%s' is not available (status: %s)", taskID, t.Status)
			}

			if t.Constraints == nil {
				t.Constraints = &task.TaskConstraints{}
			}
			if t.Constraints.Extra == nil {
				t.Constraints.Extra = make(map[string]any)
			}
			t.Constraints.Extra["claimed_by"] = workerID
			t.Constraints.Extra["claimed_at"] = time.Now().Format(time.RFC3339)
			t.Status = task.StatusRunning
			now := time.Now()
			t.StartedAt = &now

			if err := dc.TaskStore.Update(t); err != nil {
				return nil, fmt.Errorf("failed to update task: %w", err)
			}

			dc.Workers.IncrementActive(workerID)
			dc.Workers.Heartbeat(workerID)

			slog.Info("task claimed by worker", "task_id", taskID, "worker_id", workerID)

			return map[string]any{
				"task_id":     taskID,
				"title":       t.Title,
				"description": t.Description,
				"status":      "running",
				"message":     fmt.Sprintf("Task '%s' claimed successfully. Use get_task_context for details.", t.Title),
			}, nil
		},
	}
}

// ──────────────────────────────────────────────
// 4. report_progress
// ──────────────────────────────────────────────

func reportProgressTool(dc *DispatchContext) ToolDef {
	return ToolDef{
		Name:        "report_progress",
		Description: "Report progress on a claimed task. Call periodically to keep the orchestrator informed.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"worker_id": map[string]any{"type": "string", "description": "Your registered worker ID"},
				"task_id":   map[string]any{"type": "string", "description": "Task being worked on"},
				"progress":  map[string]any{"type": "string", "description": "Progress description or percentage"},
				"step_completed": map[string]any{"type": "integer", "description": "Index of the step just completed (optional)"},
			},
			"required": []string{"worker_id", "task_id", "progress"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			workerID := stringArg(args, "worker_id")
			taskID := stringArg(args, "task_id")
			progress := stringArg(args, "progress")

			if !dc.Workers.Heartbeat(workerID) {
				return nil, fmt.Errorf("worker '%s' not found", workerID)
			}

			if dc.TaskStore == nil {
				return nil, fmt.Errorf("task store not available")
			}

			t, ok := dc.TaskStore.Get(taskID)
			if !ok {
				return nil, fmt.Errorf("task '%s' not found", taskID)
			}

			stepIdx := intArg(args, "step_completed", -1)
			if stepIdx >= 0 && stepIdx < len(t.Steps) {
				t.Steps[stepIdx].Status = task.StepDone
				now := time.Now()
				t.Steps[stepIdx].DoneAt = &now
				t.Steps[stepIdx].Result = progress
			}

			if t.Constraints == nil {
				t.Constraints = &task.TaskConstraints{}
			}
			if t.Constraints.Extra == nil {
				t.Constraints.Extra = make(map[string]any)
			}
			t.Constraints.Extra["last_progress"] = progress
			t.Constraints.Extra["last_progress_at"] = time.Now().Format(time.RFC3339)

			if err := dc.TaskStore.Update(t); err != nil {
				return nil, fmt.Errorf("failed to update: %w", err)
			}

			done, total := t.Progress()
			slog.Info("worker progress", "task_id", taskID, "worker_id", workerID, "progress", progress)

			return map[string]any{
				"acknowledged":  true,
				"steps_done":    done,
				"steps_total":   total,
			}, nil
		},
	}
}

// ──────────────────────────────────────────────
// 5. submit_result
// ──────────────────────────────────────────────

func submitResultTool(dc *DispatchContext) ToolDef {
	return ToolDef{
		Name:        "submit_result",
		Description: "Submit the final result of a completed task.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"worker_id": map[string]any{"type": "string", "description": "Your registered worker ID"},
				"task_id":   map[string]any{"type": "string", "description": "Task being completed"},
				"success":   map[string]any{"type": "boolean", "description": "Whether the task succeeded"},
				"result":    map[string]any{"type": "string", "description": "Result summary or output"},
				"error":     map[string]any{"type": "string", "description": "Error message if failed"},
				"artifacts": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "object"},
					"description": "List of produced artifacts [{name, path, type}]",
				},
			},
			"required": []string{"worker_id", "task_id", "success"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			workerID := stringArg(args, "worker_id")
			taskID := stringArg(args, "task_id")
			success := boolArg(args, "success")

			if !dc.Workers.Heartbeat(workerID) {
				return nil, fmt.Errorf("worker '%s' not found", workerID)
			}

			if dc.TaskStore == nil {
				return nil, fmt.Errorf("task store not available")
			}

			t, ok := dc.TaskStore.Get(taskID)
			if !ok {
				return nil, fmt.Errorf("task '%s' not found", taskID)
			}

			now := time.Now()
			t.FinishedAt = &now

			if success {
				t.Status = task.StatusCompleted
				resultText := stringArg(args, "result")
				for i := range t.Steps {
					if t.Steps[i].Status != task.StepDone {
						t.Steps[i].Status = task.StepDone
						t.Steps[i].DoneAt = &now
						t.Steps[i].Result = resultText
					}
				}
			} else {
				t.Status = task.StatusFailed
				t.Error = stringArg(args, "error")
				if t.Error == "" {
					t.Error = "task failed (no details provided)"
				}
			}

			if err := dc.TaskStore.Update(t); err != nil {
				return nil, fmt.Errorf("failed to update: %w", err)
			}

			dc.Workers.DecrementActive(workerID)

			slog.Info("task result submitted", "task_id", taskID, "worker_id", workerID, "success", success)

			return map[string]any{
				"task_id": taskID,
				"status":  string(t.Status),
				"message": fmt.Sprintf("Task '%s' marked as %s", t.Title, t.Status),
			}, nil
		},
	}
}

// ──────────────────────────────────────────────
// 6. get_task_context
// ──────────────────────────────────────────────

func getTaskContextTool(dc *DispatchContext) ToolDef {
	return ToolDef{
		Name:        "get_task_context",
		Description: "Get detailed context for a claimed task, including description, steps, and constraints.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"worker_id": map[string]any{"type": "string", "description": "Your registered worker ID"},
				"task_id":   map[string]any{"type": "string", "description": "Task to get context for"},
			},
			"required": []string{"worker_id", "task_id"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			workerID := stringArg(args, "worker_id")
			taskID := stringArg(args, "task_id")

			if !dc.Workers.Heartbeat(workerID) {
				return nil, fmt.Errorf("worker '%s' not found", workerID)
			}

			if dc.TaskStore == nil {
				return nil, fmt.Errorf("task store not available")
			}

			t, ok := dc.TaskStore.Get(taskID)
			if !ok {
				return nil, fmt.Errorf("task '%s' not found", taskID)
			}

			steps := make([]map[string]any, 0, len(t.Steps))
			for _, s := range t.Steps {
				steps = append(steps, map[string]any{
					"id":     s.ID,
					"action": s.Action,
					"skill":  s.SkillName,
					"status": string(s.Status),
					"result": s.Result,
				})
			}

			result := map[string]any{
				"task_id":     t.ID,
				"title":       t.Title,
				"description": t.Description,
				"status":      string(t.Status),
				"steps":       steps,
				"created_at":  t.CreatedAt.Format(time.RFC3339),
			}

			if t.Constraints != nil {
				result["constraints"] = map[string]any{
					"max_steps":        t.Constraints.MaxSteps,
					"timeout_sec":      t.Constraints.TimeoutSec,
					"success_criteria": t.Constraints.SuccessCriteria,
					"test_command":     t.Constraints.TestCommand,
					"priority":         t.Constraints.Priority,
					"tags":             t.Constraints.Tags,
				}
			}

			return result, nil
		},
	}
}

// ──────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────

func isDispatchable(t *task.Task) bool {
	if t.Status == task.StatusPending || t.Status == task.StatusInterrupted {
		return true
	}
	if t.Status == task.StatusRunning {
		if t.Constraints != nil && t.Constraints.Extra != nil {
			if _, claimed := t.Constraints.Extra["claimed_by"]; !claimed {
				return true
			}
		}
	}
	return false
}

func matchesCapabilities(t *task.Task, workerCaps []string) bool {
	if t.Constraints == nil || len(t.Constraints.Tags) == 0 {
		return true // no tags required → any worker can handle
	}
	for _, tag := range t.Constraints.Tags {
		for _, cap := range workerCaps {
			if tag == cap {
				return true
			}
		}
	}
	return false
}

func taskSummary(t *task.Task) map[string]any {
	done, total := t.Progress()
	priority := ""
	if t.Constraints != nil {
		priority = t.Constraints.Priority
	}
	return map[string]any{
		"task_id":     t.ID,
		"title":       t.Title,
		"description": truncate(t.Description, 200),
		"status":      string(t.Status),
		"priority":    priority,
		"steps_done":  done,
		"steps_total": total,
		"created_at":  t.CreatedAt.Format(time.RFC3339),
	}
}

func truncate(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen]) + "..."
}

func stringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	v, ok := args[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprintf("%v", v))
}

func intArg(args map[string]any, key string, def int) int {
	if args == nil {
		return def
	}
	v, ok := args[key]
	if !ok {
		return def
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, err := n.Int64()
		if err == nil {
			return int(i)
		}
	}
	return def
}

func boolArg(args map[string]any, key string) bool {
	if args == nil {
		return false
	}
	v, ok := args[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func stringSliceArg(args map[string]any, key string) []string {
	if args == nil {
		return nil
	}
	v, ok := args[key]
	if !ok {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
