package multiagent

import (
	"time"
)

// ──────────────────────────────────────────────
// Multi-Agent Collaboration — types
//
// Enables multiple agent instances to collaborate on complex tasks.
// Uses message-passing rather than shared state.
//
// Collaboration patterns:
//   - Supervisor: one coordinator delegates to specialists
//   - Peer: agents communicate as equals via message bus
//   - Pipeline: sequential handoff between agents
// ──────────────────────────────────────────────

// TeamPattern defines the collaboration model.
type TeamPattern string

const (
	PatternSupervisor TeamPattern = "supervisor" // one coordinator, N workers
	PatternPeer       TeamPattern = "peer"       // all equal, message-based
	PatternPipeline   TeamPattern = "pipeline"   // sequential handoff A → B → C
)

// AgentRole defines a named role with capabilities and constraints.
type AgentRole struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`        // e.g. "researcher", "coder", "reviewer"
	Description string            `json:"description"` // what this agent does
	SystemPrompt string           `json:"system_prompt"`
	Skills      []string          `json:"skills"`      // allowed skill names
	ModelTier   string            `json:"model_tier"`  // "fast", "smart", "expert"
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Team defines a group of agents collaborating on a task.
type Team struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Pattern     TeamPattern `json:"pattern"`
	Roles       []AgentRole `json:"roles"`
	Supervisor  string      `json:"supervisor,omitempty"` // role ID of supervisor (for supervisor pattern)
	MaxRounds   int         `json:"max_rounds"`           // max message rounds before termination
	TenantID    string      `json:"tenant_id"`
	CreatedAt   time.Time   `json:"created_at"`
}

// ── Messages ──

// MessageType classifies inter-agent messages.
type MessageType string

const (
	MsgTask     MessageType = "task"     // delegate a subtask
	MsgResult   MessageType = "result"   // return a result
	MsgQuestion MessageType = "question" // ask for clarification
	MsgFeedback MessageType = "feedback" // provide critique/review
	MsgControl  MessageType = "control"  // system control (start/stop/pause)
)

// Message is the unit of inter-agent communication.
type Message struct {
	ID        string      `json:"id"`
	Type      MessageType `json:"type"`
	From      string      `json:"from"`      // sender role ID
	To        string      `json:"to"`        // receiver role ID ("*" for broadcast)
	Content   string      `json:"content"`
	Metadata  map[string]any `json:"metadata,omitempty"` // structured data
	ParentID  string      `json:"parent_id,omitempty"` // reply-to
	Timestamp time.Time   `json:"timestamp"`
}

// ── Session ──

// SessionStatus tracks the state of a multi-agent session.
type SessionStatus string

const (
	SessionActive    SessionStatus = "active"
	SessionCompleted SessionStatus = "completed"
	SessionFailed    SessionStatus = "failed"
	SessionTimeout   SessionStatus = "timeout"
)

// Session is a single run of a team collaboration.
type Session struct {
	ID        string        `json:"id"`
	TeamID    string        `json:"team_id"`
	Goal      string        `json:"goal"`     // the task to accomplish
	Status    SessionStatus `json:"status"`
	Messages  []Message     `json:"messages"` // full conversation history
	Result    string        `json:"result,omitempty"`
	Rounds    int           `json:"rounds"`   // current round count
	TenantID  string        `json:"tenant_id"`
	CreatedAt time.Time     `json:"created_at"`
	FinishedAt *time.Time   `json:"finished_at,omitempty"`
}
