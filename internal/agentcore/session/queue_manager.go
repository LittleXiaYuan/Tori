package session

import (
	"context"
	"log/slog"
	"sync"
)

// ──────────────────────────────────────────────
// QueueManager — orchestrates per-session task queues
//
// Each session gets an isolated TaskQueue on first use.
// Queues auto-start and process tasks sequentially.
// Provides a unified event stream across all sessions.
// ──────────────────────────────────────────────

// QueueManager manages task queues for all sessions.
type QueueManager struct {
	mu            sync.RWMutex
	queues        map[string]*TaskQueue // sessionID → queue
	handler       TaskHandler            // shared task executor
	maxSize       int
	maxConcurrent int                    // default concurrency for new queues
	ctx           context.Context
	cancel        context.CancelFunc
	listener      QueueEventListener     // global event listener
}

// NewQueueManager creates a queue manager with a shared task handler.
func NewQueueManager(handler TaskHandler, maxQueueSize int) *QueueManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &QueueManager{
		queues:        make(map[string]*TaskQueue),
		handler:       handler,
		maxSize:       maxQueueSize,
		maxConcurrent: 1, // default: serial (backward compatible)
		ctx:           ctx,
		cancel:        cancel,
	}
}

// SetDefaultConcurrency sets the default concurrency for new queues.
func (qm *QueueManager) SetDefaultConcurrency(n int) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	if n < 1 {
		n = 1
	}
	qm.maxConcurrent = n
}

// SetSessionConcurrency sets the concurrency for a specific session.
func (qm *QueueManager) SetSessionConcurrency(sessionID string, n int) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	if q, ok := qm.queues[sessionID]; ok {
		q.SetMaxConcurrent(n)
	}
}

// OnEvent sets a global event listener for all queues.
func (qm *QueueManager) OnEvent(fn QueueEventListener) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.listener = fn
}

// GetOrCreate returns the queue for a session, creating if needed.
func (qm *QueueManager) GetOrCreate(sessionID string) *TaskQueue {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	if q, ok := qm.queues[sessionID]; ok {
		return q
	}

	q := NewTaskQueue(sessionID, qm.handler, qm.maxSize)
	q.SetMaxConcurrent(qm.maxConcurrent) // apply default concurrency
	if qm.listener != nil {
		q.OnEvent(qm.listener)
	}
	qm.queues[sessionID] = q

	// Start queue processor in background
	go q.Start(qm.ctx)

	slog.Info("queue_manager: created queue", "session", sessionID, "max_concurrent", qm.maxConcurrent)
	return q
}

// Enqueue submits a task to the appropriate session queue.
func (qm *QueueManager) Enqueue(entry *TaskEntry) error {
	q := qm.GetOrCreate(entry.SessionID)
	return q.Enqueue(entry)
}

// Cancel cancels a task in any session queue.
func (qm *QueueManager) Cancel(sessionID, taskID string) bool {
	qm.mu.RLock()
	q, ok := qm.queues[sessionID]
	qm.mu.RUnlock()
	if !ok {
		return false
	}
	return q.Cancel(taskID)
}

// SessionSnapshot returns the queue state for a session.
func (qm *QueueManager) SessionSnapshot(sessionID string) []TaskEntry {
	qm.mu.RLock()
	q, ok := qm.queues[sessionID]
	qm.mu.RUnlock()
	if !ok {
		return nil
	}
	return q.Snapshot()
}

// AllSessions returns a summary of all active queues.
func (qm *QueueManager) AllSessions() map[string]int {
	qm.mu.RLock()
	defer qm.mu.RUnlock()
	out := make(map[string]int, len(qm.queues))
	for sid, q := range qm.queues {
		out[sid] = q.Pending()
	}
	return out
}

// Remove tears down a session's queue.
func (qm *QueueManager) Remove(sessionID string) {
	qm.mu.Lock()
	q, ok := qm.queues[sessionID]
	if ok {
		q.Stop()
		delete(qm.queues, sessionID)
	}
	qm.mu.Unlock()
}

// Shutdown stops all queues.
func (qm *QueueManager) Shutdown() {
	qm.cancel()
	qm.mu.Lock()
	defer qm.mu.Unlock()
	for _, q := range qm.queues {
		q.Stop()
	}
	qm.queues = make(map[string]*TaskQueue)
}
