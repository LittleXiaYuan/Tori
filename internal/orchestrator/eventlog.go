package orchestrator

import (
	"sync"
	"time"
)

type EventType string

const (
	EventTaskEnqueued     EventType = "task_enqueued"
	EventTaskAssigned     EventType = "task_assigned"
	EventTaskTimeout      EventType = "task_timeout"
	EventTaskRequeued     EventType = "task_requeued"
	EventWorkerLaunched   EventType = "worker_launched"
	EventWorkerFailed     EventType = "worker_failed"
	EventResultSubmitted  EventType = "result_submitted"
	EventReviewStarted    EventType = "review_started"
	EventReviewApproved   EventType = "review_approved"
	EventReviewRejected   EventType = "review_rejected"
	EventTestPassed       EventType = "test_passed"
	EventTestFailed       EventType = "test_failed"
	EventAutoApproved     EventType = "auto_approved"
	EventPendingApproval  EventType = "pending_approval"
	EventCircuitOpen      EventType = "circuit_open"
	EventCircuitClosed    EventType = "circuit_closed"
	EventDaemonStarted    EventType = "daemon_started"
	EventDaemonStopped    EventType = "daemon_stopped"
)

type Event struct {
	ID        string            `json:"id"`
	Type      EventType         `json:"type"`
	TaskID    string            `json:"task_id,omitempty"`
	WorkerID  string            `json:"worker_id,omitempty"`
	SessionID string            `json:"session_id,omitempty"`
	Message   string            `json:"message"`
	Meta      map[string]any    `json:"meta,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// EventLog is an append-only, bounded in-memory log of orchestration events.
type EventLog struct {
	mu     sync.RWMutex
	events []Event
	maxLen int
	seq    int64
}

func NewEventLog(maxLen int) *EventLog {
	if maxLen <= 0 {
		maxLen = 2000
	}
	return &EventLog{
		events: make([]Event, 0, maxLen),
		maxLen: maxLen,
	}
}

func (l *EventLog) Append(evt Event) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.seq++
	evt.ID = formatSeq(l.seq)
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now()
	}

	if len(l.events) >= l.maxLen {
		copy(l.events, l.events[1:])
		l.events[len(l.events)-1] = evt
	} else {
		l.events = append(l.events, evt)
	}
}

// Recent returns the last N events (newest last).
func (l *EventLog) Recent(n int) []Event {
	l.mu.RLock()
	defer l.mu.RUnlock()

	total := len(l.events)
	if n <= 0 || n > total {
		n = total
	}
	out := make([]Event, n)
	copy(out, l.events[total-n:])
	return out
}

// ForTask returns all events for a specific task ID (timeline).
func (l *EventLog) ForTask(taskID string) []Event {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var out []Event
	for _, e := range l.events {
		if e.TaskID == taskID {
			out = append(out, e)
		}
	}
	return out
}

// Since returns events after a given timestamp.
func (l *EventLog) Since(after time.Time) []Event {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var out []Event
	for _, e := range l.events {
		if e.Timestamp.After(after) {
			out = append(out, e)
		}
	}
	return out
}

// Count returns the total number of stored events.
func (l *EventLog) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.events)
}

func formatSeq(n int64) string {
	const digits = "0123456789"
	if n == 0 {
		return "E0"
	}
	buf := make([]byte, 0, 12)
	for n > 0 {
		buf = append(buf, digits[n%10])
		n /= 10
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return "E" + string(buf)
}
