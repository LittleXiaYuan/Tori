package session

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// TaskQueue — per-session serial task channel
//
// Design principle: "默认串行、显式并行"
//   - Each session owns one TaskQueue
//   - Tasks within a session execute sequentially by default
//   - Tasks explicitly marked Safe=true may run in parallel
//   - Queue provides event stream for real-time visualization
//
// This matches the OpenClaw "session channel" pattern:
// one conversation → one ordered task pipeline.
// ──────────────────────────────────────────────

// TaskEntry is a unit of work in the session queue.
type TaskEntry struct {
	ID          string         `json:"id"`
	SessionID   string         `json:"session_id"`
	Prompt      string         `json:"prompt"`               // user message that triggered this
	Status      TaskStatus     `json:"status"`
	Parallel    bool           `json:"parallel"`             // safe to run concurrently
	Priority    int            `json:"priority"`             // 0=normal, 1=high, 2=urgent
	Result      string         `json:"result,omitempty"`
	Error       string         `json:"error,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	FinishedAt  *time.Time     `json:"finished_at,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// TaskStatus represents the lifecycle of a queued task.
type TaskStatus string

const (
	TaskQueued    TaskStatus = "queued"     // waiting in line
	TaskRunning   TaskStatus = "running"    // currently executing
	TaskDone      TaskStatus = "done"       // completed successfully
	TaskFailed    TaskStatus = "failed"     // completed with error
	TaskCancelled TaskStatus = "cancelled"  // user-cancelled
	TaskSkipped   TaskStatus = "skipped"    // skipped by queue logic
)

// QueueEvent is emitted when queue state changes.
type QueueEvent struct {
	Type      string     `json:"type"`       // "enqueued" | "started" | "completed" | "failed" | "cancelled"
	TaskID    string     `json:"task_id"`
	SessionID string    `json:"session_id"`
	Position  int        `json:"position"`   // position in queue (0-based)
	Total     int        `json:"total"`      // total queue depth
	Detail    string     `json:"detail,omitempty"`
	Timestamp time.Time  `json:"timestamp"`
}

// QueueEventListener receives queue events.
type QueueEventListener func(event QueueEvent)

// TaskHandler is the function that actually executes a task.
type TaskHandler func(ctx context.Context, entry *TaskEntry) (result string, err error)

// TaskQueue manages an ordered pipeline of tasks for a single session.
type TaskQueue struct {
	mu            sync.Mutex
	sessionID     string
	queue         []*TaskEntry       // ordered task list
	handler       TaskHandler        // execution callback
	listeners     []QueueEventListener
	running       bool               // is the queue processor active
	cancel        context.CancelFunc // cancel the processor
	wakeup        chan struct{}       // signal new work
	maxSize       int                // max pending tasks (0=unlimited)
	maxConcurrent int                // max concurrent tasks (1=serial, 3-5=concurrent)
	runningTasks  map[string]*TaskEntry // currently executing tasks
}

// NewTaskQueue creates a task queue for a session.
func NewTaskQueue(sessionID string, handler TaskHandler, maxSize int) *TaskQueue {
	if maxSize <= 0 {
		maxSize = 100
	}
	return &TaskQueue{
		sessionID:     sessionID,
		handler:       handler,
		wakeup:        make(chan struct{}, 1),
		maxSize:       maxSize,
		maxConcurrent: 1, // default: serial execution (backward compatible)
		runningTasks:  make(map[string]*TaskEntry),
	}
}

// SetMaxConcurrent configures the concurrency level (1=serial, 3-5=concurrent).
func (q *TaskQueue) SetMaxConcurrent(n int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if n < 1 {
		n = 1
	}
	q.maxConcurrent = n
}

// OnEvent registers a listener for queue state changes.
func (q *TaskQueue) OnEvent(fn QueueEventListener) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.listeners = append(q.listeners, fn)
}

// Enqueue adds a task to the queue. Returns error if queue is full.
func (q *TaskQueue) Enqueue(entry *TaskEntry) error {
	q.mu.Lock()

	pending := 0
	for _, t := range q.queue {
		if t.Status == TaskQueued || t.Status == TaskRunning {
			pending++
		}
	}
	if pending >= q.maxSize {
		q.mu.Unlock()
		return fmt.Errorf("session %s: task queue full (%d pending)", q.sessionID, pending)
	}

	entry.SessionID = q.sessionID
	entry.Status = TaskQueued
	entry.CreatedAt = time.Now()
	q.queue = append(q.queue, entry)

	event := QueueEvent{
		Type:      "enqueued",
		TaskID:    entry.ID,
		SessionID: q.sessionID,
		Position:  pending,
		Total:     pending + 1,
	}
	q.mu.Unlock()

	q.emit(event)

	// Wake up processor
	select {
	case q.wakeup <- struct{}{}:
	default:
	}

	return nil
}

// Start begins processing the queue (blocking). Call in a goroutine.
func (q *TaskQueue) Start(ctx context.Context) {
	q.mu.Lock()
	if q.running {
		q.mu.Unlock()
		return
	}
	q.running = true
	ctx, q.cancel = context.WithCancel(ctx)
	q.mu.Unlock()

	defer func() {
		q.mu.Lock()
		q.running = false
		q.mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-q.wakeup:
			q.processNext(ctx)
		}
	}
}

// Stop halts queue processing.
func (q *TaskQueue) Stop() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.cancel != nil {
		q.cancel()
	}
}

// Cancel cancels a specific queued (not yet running) task.
func (q *TaskQueue) Cancel(taskID string) bool {
	q.mu.Lock()
	var found bool
	var event QueueEvent
	for _, t := range q.queue {
		if t.ID == taskID && t.Status == TaskQueued {
			t.Status = TaskCancelled
			now := time.Now()
			t.FinishedAt = &now
			event = QueueEvent{
				Type:      "cancelled",
				TaskID:    taskID,
				SessionID: q.sessionID,
			}
			found = true
			break
		}
	}
	q.mu.Unlock()

	if found {
		q.emit(event)
	}
	return found
}

// Pending returns the number of queued (not yet started) tasks.
func (q *TaskQueue) Pending() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	count := 0
	for _, t := range q.queue {
		if t.Status == TaskQueued {
			count++
		}
	}
	return count
}

// Running returns the number of currently executing tasks.
func (q *TaskQueue) Running() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.runningTasks)
}

// Concurrency returns (current, max) concurrent task counts.
func (q *TaskQueue) Concurrency() (int, int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.runningTasks), q.maxConcurrent
}

// Snapshot returns a copy of the queue for visualization.
func (q *TaskQueue) Snapshot() []TaskEntry {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]TaskEntry, len(q.queue))
	for i, t := range q.queue {
		out[i] = *t
	}
	return out
}

// History returns completed/failed tasks for the session log.
func (q *TaskQueue) History(limit int) []TaskEntry {
	q.mu.Lock()
	defer q.mu.Unlock()
	var out []TaskEntry
	for _, t := range q.queue {
		if t.Status == TaskDone || t.Status == TaskFailed || t.Status == TaskCancelled {
			out = append(out, *t)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

// processNext finds the next queued task and executes it.
// Supports concurrent execution up to maxConcurrent limit.
func (q *TaskQueue) processNext(ctx context.Context) {
	for {
		q.mu.Lock()

		// Check if we've reached concurrency limit
		if len(q.runningTasks) >= q.maxConcurrent {
			q.mu.Unlock()
			return // wait for a slot to free up
		}

		var next *TaskEntry
		pos := 0
		for i, t := range q.queue {
			if t.Status == TaskQueued {
				next = t
				pos = i
				break
			}
		}
		if next == nil {
			q.mu.Unlock()
			return
		}

		// Check if a non-parallel task is already running
		if !next.Parallel && len(q.runningTasks) > 0 {
			q.mu.Unlock()
			return // wait for current tasks to finish
		}

		next.Status = TaskRunning
		now := time.Now()
		next.StartedAt = &now
		q.runningTasks[next.ID] = next

		totalPending := 0
		for _, t := range q.queue {
			if t.Status == TaskQueued {
				totalPending++
			}
		}

		event := QueueEvent{
			Type:      "started",
			TaskID:    next.ID,
			SessionID: q.sessionID,
			Position:  pos,
			Total:     totalPending,
		}
		q.mu.Unlock()

		q.emit(event)

		slog.Info("session_queue: executing",
			"session", q.sessionID, "task", next.ID, "prompt_len", len(next.Prompt),
			"concurrent", len(q.runningTasks), "max", q.maxConcurrent)

		// Execute the task in a goroutine to support concurrency
		go q.executeTask(ctx, next)
	}
}

// executeTask runs a single task and updates its status.
func (q *TaskQueue) executeTask(ctx context.Context, entry *TaskEntry) {
	// Execute the task
	result, err := q.handler(ctx, entry)

	q.mu.Lock()
	finishedAt := time.Now()
	entry.FinishedAt = &finishedAt
	delete(q.runningTasks, entry.ID) // free up the slot

	var event QueueEvent
	if err != nil {
		entry.Status = TaskFailed
		entry.Error = err.Error()
		event = QueueEvent{
			Type:      "failed",
			TaskID:    entry.ID,
			SessionID: q.sessionID,
			Detail:    err.Error(),
		}
	} else {
		entry.Status = TaskDone
		entry.Result = result
		event = QueueEvent{
			Type:      "completed",
			TaskID:    entry.ID,
			SessionID: q.sessionID,
		}
	}
	q.mu.Unlock()

	q.emit(event)

	// Wake up processor to check for more work
	select {
	case q.wakeup <- struct{}{}:
	default:
	}
}

func (q *TaskQueue) emit(event QueueEvent) {
	event.Timestamp = time.Now()
	q.mu.Lock()
	listeners := make([]QueueEventListener, len(q.listeners))
	copy(listeners, q.listeners)
	q.mu.Unlock()
	for _, fn := range listeners {
		fn(event)
	}
}
