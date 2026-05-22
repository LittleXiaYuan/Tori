package ledger

import "time"

// ──────────────────────────────────────────────
// Event
// ──────────────────────────────────────────────

// EventKind identifies the type of event.
type EventKind string

// Task lifecycle events.
const (
	EventTaskCreated       EventKind = "task.created"
	EventTaskReady         EventKind = "task.ready"
	EventTaskStarted       EventKind = "task.started"
	EventTaskCompleted     EventKind = "task.completed"
	EventTaskFailed        EventKind = "task.failed"
	EventTaskCancelled     EventKind = "task.cancelled"
	EventTaskRetrying      EventKind = "task.retrying"
	EventTaskBlocked       EventKind = "task.blocked"
	EventTaskWaitingInput  EventKind = "task.waiting_input"
	EventTaskInputReceived EventKind = "task.input_received"
	EventTaskResumed       EventKind = "task.resumed"
)

// Step execution events.
const (
	EventStepStarted   EventKind = "step.started"
	EventStepCompleted EventKind = "step.completed"
	EventStepFailed    EventKind = "step.failed"
	EventStepRetrying  EventKind = "step.retrying"
	EventStepSkipped   EventKind = "step.skipped"
)

// Reasoning trace events ???captures the agent's thought process for
// observability, reflection, experience distillation, and meta-cognition.
const (
	EventReasoningThought    EventKind = "reasoning.thought"
	EventReasoningHypothesis EventKind = "reasoning.hypothesis"
	EventReasoningDecision   EventKind = "reasoning.decision"
	EventReasoningBacktrack  EventKind = "reasoning.backtrack"
	EventReasoningObserve    EventKind = "reasoning.observe"
	EventReasoningPlan       EventKind = "reasoning.plan"
	EventReasoningReflect    EventKind = "reasoning.reflect"
	EventReasoningConfUpdate EventKind = "reasoning.conf"
)

// Infrastructure events.
const (
	EventCheckpointCreated EventKind = "checkpoint.created"
	EventCheckpointLoaded  EventKind = "checkpoint.loaded"
	EventArtifactCreated   EventKind = "artifact.created"
	EventArtifactDeleted   EventKind = "artifact.deleted"
	EventMemoryWritten     EventKind = "memory.written"
	EventMemoryUpdated     EventKind = "memory.updated"
	EventMemoryDeleted     EventKind = "memory.deleted"
	EventMemoryRecalled    EventKind = "memory.recalled"
	EventLLMRequest        EventKind = "llm.request"
	EventLLMResponse       EventKind = "llm.response"
	EventToolInvoked       EventKind = "tool.invoked"
	EventToolCompleted     EventKind = "tool.completed"
	EventToolFailed        EventKind = "tool.failed"
)

// Event is an immutable record in the append-only event log.
// Events are the source of truth; the tasks table is a materialized view.
type Event struct {
	ID         string    `json:"id"          db:"id"` // ULID (time-ordered)
	TaskID     string    `json:"task_id"     db:"task_id"`
	Kind       EventKind `json:"kind"        db:"kind"`
	Seq        int64     `json:"seq"         db:"seq"`   // Monotonically increasing within task
	Actor      string    `json:"actor"       db:"actor"` // "runtime" | "user" | "llm" | "tool:{name}"
	Payload    JSON      `json:"payload"     db:"payload"`
	ParentID   *string   `json:"parent_id"   db:"parent_id"` // Causal chain
	DurationMs *int64    `json:"duration_ms" db:"duration_ms"`
	CreatedAt  time.Time `json:"created_at"  db:"created_at"`
}

// EventQuery provides flexible multi-dimensional event filtering.
// All non-zero fields are combined with AND logic.
type EventQuery struct {
	TaskID   string      `json:"task_id,omitempty"`
	Kinds    []EventKind `json:"kinds,omitempty"`
	Actors   []string    `json:"actors,omitempty"`
	After    *time.Time  `json:"after,omitempty"`
	Before   *time.Time  `json:"before,omitempty"`
	AfterSeq int64       `json:"after_seq,omitempty"`
	Limit    int         `json:"limit,omitempty"`
	Offset   int         `json:"offset,omitempty"`
}
