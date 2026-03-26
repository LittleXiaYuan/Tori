package observe

import (
	"sync"
)

// ──────────────────────────────────────────────
// AuditTrail — in-memory event store for execution trace replay
//
// Records all AgentEvents from every subsystem (planner, workflow,
// approval) and provides query-by-trace-id / query-by-task-id
// for the /v1/trace API and web execution trace panel.
//
// Design choice: in-memory with rolling window (10k events, ~24h).
// For production persistence, swap with a DB-backed implementation.
// ──────────────────────────────────────────────

// AuditTrail stores agent events for tracing and replay.
type AuditTrail struct {
	mu     sync.RWMutex
	events []AgentEvent
	maxLen int
}

// NewAuditTrail creates an AuditTrail with a given max capacity.
func NewAuditTrail(maxLen int) *AuditTrail {
	if maxLen <= 0 {
		maxLen = 10000
	}
	return &AuditTrail{
		events: make([]AgentEvent, 0, maxLen),
		maxLen: maxLen,
	}
}

// Record appends an event to the trail.
func (t *AuditTrail) Record(event AgentEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, event)
	if len(t.events) > t.maxLen {
		// Trim oldest 20% to avoid constant trimming
		trim := t.maxLen / 5
		copy(t.events, t.events[trim:])
		t.events = t.events[:len(t.events)-trim]
	}
}

// QueryByTraceID returns all events for a given trace ID.
func (t *AuditTrail) QueryByTraceID(traceID string) []AgentEvent {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var result []AgentEvent
	for _, e := range t.events {
		if e.TraceID == traceID {
			result = append(result, e)
		}
	}
	return result
}

// QueryByTaskID returns all events for a given task ID.
func (t *AuditTrail) QueryByTaskID(taskID string) []AgentEvent {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var result []AgentEvent
	for _, e := range t.events {
		if e.Meta.TaskID == taskID {
			result = append(result, e)
		}
	}
	return result
}

// Recent returns the most recent N events.
func (t *AuditTrail) Recent(limit int) []AgentEvent {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if limit <= 0 || limit > len(t.events) {
		limit = len(t.events)
	}
	start := len(t.events) - limit
	result := make([]AgentEvent, limit)
	copy(result, t.events[start:])
	return result
}

// Len returns current event count.
func (t *AuditTrail) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.events)
}
