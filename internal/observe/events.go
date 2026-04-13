package observe

import (
	"fmt"
	"sync/atomic"
	"time"
)

// ──────────────────────────────────────────────
// Unified Agent Event Protocol
//
// Every observable event across planner, workflow, approval,
// and multi-agent systems is represented as an AgentEvent.
// This provides:
//   - Consistent SSE payload format
//   - trace_id linkage for audit trail
//   - Domain-typed filtering (planner.thinking, workflow.node_done, etc.)
// ──────────────────────────────────────────────

// AgentEvent is the universal event structure for all agent subsystems.
type AgentEvent struct {
	ID        string    `json:"id"`                   // unique event ID (evt-...)
	TraceID   string    `json:"trace_id"`             // trace context
	SpanID    string    `json:"span_id,omitempty"`    // span context
	Timestamp time.Time `json:"ts"`                   // event time
	Domain    string    `json:"domain"`               // "planner", "workflow", "approval", "agent"
	Type      string    `json:"type"`                 // "thinking", "tool_start", "node_done", etc.
	Summary   string    `json:"summary"`              // human-readable one-liner
	Detail    any       `json:"detail,omitempty"`      // structured payload (skill args, node config, etc.)
	Meta      EventMeta `json:"meta"`                 // correlation metadata
}

// EventMeta carries correlation IDs for cross-system linkage.
type EventMeta struct {
	TenantID   string `json:"tenant_id,omitempty"`
	SessionID  string `json:"session_id,omitempty"`
	TaskID     string `json:"task_id,omitempty"`
	Skill      string `json:"skill,omitempty"`
	NodeID     string `json:"node_id,omitempty"`
	NodeName   string `json:"node_name,omitempty"`
	InstanceID string `json:"instance_id,omitempty"`
}

// QualifiedType returns "domain.type" for SSE event name, e.g. "planner.thinking".
func (e AgentEvent) QualifiedType() string {
	if e.Domain == "" {
		return e.Type
	}
	return e.Domain + "." + e.Type
}

// ── Event Domains ──

const (
	DomainPlanner  = "planner"
	DomainWorkflow = "workflow"
	DomainApproval = "approval"
	DomainAgent    = "agent"
)

// ── Planner Event Types ──

const (
	EventThinking     = "thinking"
	EventToolStart    = "tool_start"
	EventToolResult   = "tool_result"
	EventReflect      = "reflect"
	EventPlan         = "plan"
	EventHandoffStart = "handoff_start"
	EventHandoffDone  = "handoff_done"
)

// ── Workflow Event Types ──

const (
	EventNodeStart  = "node_start"
	EventNodeDone   = "node_done"
	EventNodeFailed = "node_failed"
)

// ── Approval Event Types ──

const (
	EventApprovalRequest  = "request"
	EventApprovalApproved = "approved"
	EventApprovalDenied   = "denied"
)

// ── Event ID generator ──

var eventSeq atomic.Uint64

// NewEventID generates a unique event ID.
func NewEventID() string {
	seq := eventSeq.Add(1)
	return fmt.Sprintf("evt-%d-%d", time.Now().UnixMilli(), seq)
}

// NewEvent is a convenience constructor for AgentEvent.
func NewEvent(traceID, domain, eventType, summary string) AgentEvent {
	return AgentEvent{
		ID:        NewEventID(),
		TraceID:   traceID,
		Timestamp: time.Now(),
		Domain:    domain,
		Type:      eventType,
		Summary:   summary,
	}
}

// HandoffDetail is the Detail payload for handoff_start / handoff_done events.
type HandoffDetail struct {
	Agent string `json:"agent"`
	Input string `json:"input,omitempty"`
	Reply string `json:"reply,omitempty"`
	Error string `json:"error,omitempty"`
	DurMs int64  `json:"dur_ms,omitempty"`
}

// ToolStartDetail is the Detail payload for tool_start events.
type ToolStartDetail struct {
	Skill string         `json:"skill"`
	Args  map[string]any `json:"args,omitempty"`
}

// ToolResultDetail is the Detail payload for tool_result events.
type ToolResultDetail struct {
	Skill  string      `json:"skill"`
	Result string      `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
	Files  []FileEntry `json:"files,omitempty"`
}

// FileEntry describes a file produced by a tool invocation.
type FileEntry struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	Name string `json:"name"`
}
