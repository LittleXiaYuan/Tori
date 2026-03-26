package multiagent

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// ──────────────────────────────────────────────
// Supervisor — orchestrates multi-agent collaboration
//
// The Supervisor pattern:
//  1. Receives a goal
//  2. Decomposes it into subtasks
//  3. Delegates subtasks to specialist agents
//  4. Collects and synthesizes results
//  5. Iterates until goal is met or max rounds reached
// ──────────────────────────────────────────────

// AgentFunc is the execution function for an agent role.
// Given a message (task/question), it returns a response.
type AgentFunc func(ctx context.Context, role AgentRole, msg Message) (string, error)

// Supervisor coordinates a team of agents.
type Supervisor struct {
	team     Team
	bus      *Bus
	agentFn  AgentFunc // shared execution function (uses role's prompt to differentiate)
}

// NewSupervisor creates a supervisor for the given team.
func NewSupervisor(team Team, agentFn AgentFunc) *Supervisor {
	return &Supervisor{
		team:    team,
		bus:     NewBus(500),
		agentFn: agentFn,
	}
}

// Run executes the collaboration session for the given goal.
func (s *Supervisor) Run(ctx context.Context, goal string, tenantID string) (*Session, error) {
	session := &Session{
		ID:        uuid.New().String()[:8],
		TeamID:    s.team.ID,
		Goal:      goal,
		Status:    SessionActive,
		TenantID:  tenantID,
		CreatedAt: time.Now(),
	}

	maxRounds := s.team.MaxRounds
	if maxRounds <= 0 {
		maxRounds = 10
	}

	// Find supervisor role
	var supervisorRole *AgentRole
	for i := range s.team.Roles {
		if s.team.Roles[i].ID == s.team.Supervisor {
			supervisorRole = &s.team.Roles[i]
			break
		}
	}
	if supervisorRole == nil && len(s.team.Roles) > 0 {
		supervisorRole = &s.team.Roles[0]
	}
	if supervisorRole == nil {
		session.Status = SessionFailed
		return session, fmt.Errorf("no supervisor role defined")
	}

	slog.Info("multiagent: session started",
		"session", session.ID,
		"team", s.team.Name,
		"goal", goal,
		"roles", len(s.team.Roles),
	)

	// Round 1: Supervisor plans and delegates
	planMsg := Message{
		ID:        uuid.New().String()[:8],
		Type:      MsgTask,
		From:      "system",
		To:        supervisorRole.ID,
		Content:   fmt.Sprintf("你是团队的协调者。目标：%s\n\n请分析目标，决定下一步行动。你可以委派任务给以下角色：\n%s\n\n回复格式：直接回复你的分析和行动计划。", goal, s.describeRoles()),
		Timestamp: time.Now(),
	}
	session.Messages = append(session.Messages, planMsg)

	for round := 0; round < maxRounds; round++ {
		if ctx.Err() != nil {
			session.Status = SessionFailed
			session.Result = "cancelled"
			return session, ctx.Err()
		}

		session.Rounds = round + 1

		// Get latest message to process
		lastMsg := session.Messages[len(session.Messages)-1]

		// Determine which role should respond
		targetRole := s.findRole(lastMsg.To)
		if targetRole == nil {
			targetRole = supervisorRole
		}

		// Execute agent
		response, err := s.agentFn(ctx, *targetRole, lastMsg)
		if err != nil {
			slog.Warn("multiagent: agent error", "role", targetRole.ID, "err", err)
			session.Status = SessionFailed
			session.Result = fmt.Sprintf("agent %s failed: %v", targetRole.ID, err)
			fin := time.Now()
			session.FinishedAt = &fin
			return session, err
		}

		replyMsg := Message{
			ID:        uuid.New().String()[:8],
			Type:      MsgResult,
			From:      targetRole.ID,
			To:        supervisorRole.ID,
			Content:   response,
			ParentID:  lastMsg.ID,
			Timestamp: time.Now(),
		}
		session.Messages = append(session.Messages, replyMsg)

		// Check if supervisor considers the task complete
		// Simple heuristic: if the supervisor's reply contains completion markers
		if targetRole.ID == supervisorRole.ID && isCompletionSignal(response) {
			session.Status = SessionCompleted
			session.Result = response
			fin := time.Now()
			session.FinishedAt = &fin
			slog.Info("multiagent: session completed",
				"session", session.ID,
				"rounds", session.Rounds,
			)
			return session, nil
		}

		// If the response is from a worker, send it back to supervisor for next decision
		if targetRole.ID != supervisorRole.ID {
			nextMsg := Message{
				ID:        uuid.New().String()[:8],
				Type:      MsgResult,
				From:      targetRole.ID,
				To:        supervisorRole.ID,
				Content:   fmt.Sprintf("[%s 的回复]\n%s\n\n请决定下一步行动，或者回复 [DONE] 表示任务完成。", targetRole.Name, response),
				ParentID:  replyMsg.ID,
				Timestamp: time.Now(),
			}
			session.Messages = append(session.Messages, nextMsg)
		}
	}

	// Max rounds reached
	session.Status = SessionTimeout
	session.Result = "max rounds reached"
	fin := time.Now()
	session.FinishedAt = &fin
	return session, nil
}

// describeRoles returns a formatted description of available roles.
func (s *Supervisor) describeRoles() string {
	var desc string
	for _, r := range s.team.Roles {
		if r.ID == s.team.Supervisor {
			continue
		}
		desc += fmt.Sprintf("- %s (%s): %s\n", r.Name, r.ID, r.Description)
	}
	return desc
}

// findRole gets a role by ID.
func (s *Supervisor) findRole(id string) *AgentRole {
	for i := range s.team.Roles {
		if s.team.Roles[i].ID == id {
			return &s.team.Roles[i]
		}
	}
	return nil
}

// isCompletionSignal checks if a response indicates task completion.
func isCompletionSignal(response string) bool {
	markers := []string{"[DONE]", "[完成]", "[COMPLETED]", "任务完成", "已完成所有"}
	for _, m := range markers {
		if containsStr(response, m) {
			return true
		}
	}
	return false
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && indexOfStr(s, substr) >= 0
}

func indexOfStr(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
