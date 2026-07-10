package planner

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/task"
	"yunque-agent/pkg/safego"
)

// backgroundedNotice is the fixed, deterministic reply the current chat turn
// gets the instant a handoff is backgrounded. It exists so the top-level
// model always has a true sentence to relay instead of improvising an excuse
// when the sub-agent call is slow or fails — the exact fabrication bug this
// async path was built to close.
func backgroundedNotice(agentName string) string {
	return fmt.Sprintf("已经把「%s」这个任务转成后台任务在跑，完成后我会通知你。", agentName)
}

// executeHandoffAsync creates a task record for the delegation, launches the
// actual sub-agent execution (the unchanged, synchronous hr.Execute) on a
// detached context in a panic-safe goroutine, and returns immediately so the
// calling chat turn is never blocked on it. Completion — success or failure
// — is reported back through notifyFn/broadcastFn, never left for the model
// to guess at.
func (s *DelegationRuntimeService) executeHandoffAsync(ctx context.Context, req PlanRequest, toolName, agentName, input string, hooks HandoffExecutionHooks) HandoffExecutionResult {
	t, err := s.taskStore.Create(task.CreateRequest{
		Title:       fmt.Sprintf("委派任务：%s", agentName),
		Description: input,
		TenantID:    req.TenantID,
		Constraints: &task.TaskConstraints{
			Tags: []string{"handoff", "async", agentName},
			Extra: map[string]any{
				"agent_name": agentName,
				"session_id": req.SessionID,
				"trace_id":   req.TraceID,
				"tool_name":  toolName,
			},
		},
	})
	if err != nil {
		// Task bookkeeping failed — fall back to the notice anyway rather than
		// blocking the turn on a bookkeeping error; the sub-agent still runs.
		slog.Warn("handoff: async task creation failed, proceeding without task record", "agent", agentName, "err", err)
	}

	sessionID := req.SessionID
	tenantID := req.TenantID
	providerOverride := req.EffectiveModelTier()

	// The current chat turn must never block on a backgrounded handoff — that is
	// the whole reason this path exists. So we try to take the per-session
	// concurrency slot without blocking: if one is free we hold it before the
	// goroutine even starts (bounding creation); if the session is already at its
	// ceiling we still background the work, and the goroutine blocks on the slot
	// itself. Either way the caller returns instantly with backgroundedNotice.
	slot := s.sessionSlot(sessionID)
	acquired := false
	select {
	case slot <- struct{}{}:
		acquired = true
	default:
	}

	taskID := ""
	if t != nil {
		taskID = t.ID
	}

	safego.Go("handoff-async-"+agentName, func() {
		if !acquired {
			slot <- struct{}{} // session at ceiling: queue here, off the caller's turn
		}
		defer s.releaseSessionSlot(sessionID, slot)

		runCtx, cancel := context.WithTimeout(context.Background(), 360*time.Second)
		defer cancel()

		startedAt := time.Now()
		hr, runErr := s.ExecuteHandoff(runCtx, tenantID, agentName, input, providerOverride)
		duration := time.Since(startedAt)

		if hooks.Metrics != nil {
			hooks.Metrics(toolName, duration, runErr)
		}
		if hooks.RecordExecutionFailure != nil {
			hooks.RecordExecutionFailure(runErr != nil)
		}

		reply, partial := "", ""
		if hr != nil {
			reply = hr.Reply
			partial = hr.PartialResult
		}
		msg := s.finishHandoffTask(taskID, agentName, reply, partial, runErr)

		if s.notifyFn != nil && sessionID != "" {
			s.notifyFn(sessionID, llm.Message{Role: "assistant", Content: msg})
		}
		event := "completed"
		if runErr != nil {
			event = "failed"
		}
		if s.broadcastFn != nil && taskID != "" {
			s.broadcastFn(event, taskID, msg)
		}
		slog.Info("handoff: async delegation finished", "agent", agentName, "duration", duration, "err", runErr)
	})

	return HandoffExecutionResult{
		Handled:   true,
		ToolName:  toolName,
		AgentName: agentName,
		Input:     input,
		Reply:     backgroundedNotice(agentName),
	}
}

// finishHandoffTask records the terminal state of a finished async handoff and
// returns an honest, specific chat message describing it.
//
// It closes three defects the earlier handoffCompletionMessage had:
//   - Success now marks the task StatusCompleted and persists it, instead of
//     leaving every successful delegation as a permanent StatusPending leak.
//   - When the sub-agent timed out (Reply empty) but recovered partial work
//     (#33), that partial result is surfaced instead of "没有返回内容".
//   - The task is loaded and written back through a fresh store clone rather
//     than mutating the store-shared *Task pointer with no lock held.
//
// The direct StatusCompleted/StatusFailed write here intentionally bypasses the
// lifecycle transition validator (pending→completed is otherwise disallowed):
// an async handoff has no planning/running phase of its own, so the task record
// is bookkeeping only, and JSONStore.Update is the sanctioned raw-write path.
func (s *DelegationRuntimeService) finishHandoffTask(taskID, agentName, reply, partial string, runErr error) string {
	if runErr == nil {
		body := reply
		if body == "" {
			body = partial // #33: prefer recovered work over a "nothing returned" placeholder
		}
		s.markHandoffTask(taskID, task.StatusCompleted, "")
		if body == "" {
			return fmt.Sprintf("后台任务「%s」已完成，但没有返回内容。", agentName)
		}
		return fmt.Sprintf("后台任务「%s」已完成：\n\n%s", agentName, body)
	}

	t := s.markHandoffTask(taskID, task.StatusFailed, runErr.Error())
	var msg string
	if t != nil {
		if hint := task.InferRecoveryHint(t, "handoff-async"); hint != nil {
			msg = fmt.Sprintf("后台任务「%s」失败：%s", agentName, hint.Summary)
			if hint.PrimaryAction.Label != "" {
				msg += fmt.Sprintf("\n建议：%s", hint.PrimaryAction.Label)
				if hint.PrimaryAction.Href != "" {
					msg += fmt.Sprintf("（%s）", hint.PrimaryAction.Href)
				}
			}
		}
	}
	if msg == "" {
		msg = fmt.Sprintf("后台任务「%s」失败：%s", agentName, runErr.Error())
	}
	// #33: recovered partial work is appended to whatever failure message we
	// produced — including hint-based ones — so a timeout never discards the
	// evidence the sub-agent managed to collect before it ran out of time.
	if partial != "" {
		msg += fmt.Sprintf("\n\n已完成的部分结果：\n%s", partial)
	}
	return msg
}

// markHandoffTask loads a fresh copy of the task, sets its terminal status, and
// persists it. Returning the updated copy lets the caller feed it to
// InferRecoveryHint. Loading via Get (a clone) rather than reusing the pointer
// returned from Create keeps every write to the store-owned struct under the
// store's lock. Missing store or task is a no-op that returns nil.
func (s *DelegationRuntimeService) markHandoffTask(taskID string, status task.Status, errMsg string) *task.Task {
	if s.taskStore == nil || taskID == "" {
		return nil
	}
	t, ok := s.taskStore.Get(taskID)
	if !ok || t == nil {
		return nil
	}
	t.Status = status
	t.Error = errMsg
	if err := s.taskStore.Update(t); err != nil {
		slog.Warn("handoff: async task status update failed", "task", taskID, "status", status, "err", err)
	}
	return t
}
