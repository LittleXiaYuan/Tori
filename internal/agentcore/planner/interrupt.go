package planner

import (
	"log/slog"
	"strings"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/session"
)

// RunStateAccessor provides access to the active RunState for interrupt checking.
type RunStateAccessor func(sessionID string) *session.RunState

func (p *Planner) checkInterrupt(req PlanRequest, messages []llm.Message) (bool, []llm.Message) {
	if p.runState == nil {
		return false, nil
	}
	sessionID := req.TaskID
	if sessionID == "" {
		return false, nil
	}
	rs := p.runState(sessionID)
	if rs == nil {
		return false, nil
	}

	interrupted, kind, msg := rs.CheckInterrupt()
	if !interrupted {
		supplements := rs.DrainSupplements()
		if len(supplements) > 0 {
			return false, supplementMessages(supplements)
		}
		return false, nil
	}

	switch kind {
	case session.InterruptCorrection:
		slog.Info("planner: correction interrupt", "session", sessionID)
		return true, nil
	case session.InterruptSupplement:
		supplements := rs.DrainSupplements()
		if msg != "" {
			supplements = append([]string{msg}, supplements...)
		}
		return false, supplementMessages(supplements)
	default:
		return false, nil
	}
}

func supplementMessages(supplements []string) []llm.Message {
	if len(supplements) == 0 {
		return nil
	}
	return []llm.Message{
		{Role: "user", Content: "[补充信息] " + strings.Join(supplements, "\n")},
	}
}
