package task

import (
	"container/heap"
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// Dispatcher — capability-aware task dispatcher
//
// Accepts tasks from the Planner, enqueues them with priority,
// and makes them available for external workers to claim.
// Handles timeout detection and automatic re-queue on worker failure.
// ──────────────────────────────────────────────

// DispatchStatus extends task tracking with dispatch metadata.
type DispatchStatus string

const (
	DispatchQueued    DispatchStatus = "queued"
	DispatchAssigned  DispatchStatus = "assigned"
	DispatchTimeout   DispatchStatus = "timeout"
	DispatchCompleted DispatchStatus = "completed"
)

// DispatchEntry wraps a task with dispatch-specific metadata.
type DispatchEntry struct {
	TaskID         string         `json:"task_id"`
	RequiredCaps   []string       `json:"required_caps"`
	Priority       int            `json:"priority"` // higher = more urgent
	DispatchStatus DispatchStatus `json:"dispatch_status"`
	AssignedWorker string         `json:"assigned_worker,omitempty"`
	AssignedAt     *time.Time     `json:"assigned_at,omitempty"`
	TimeoutSec     int            `json:"timeout_sec"` // 0 = no timeout
	RetryCount     int            `json:"retry_count"`
	MaxRetries     int            `json:"max_retries"`
	QueuedAt       time.Time      `json:"queued_at"`

	index int // heap index, managed by the priority queue
}

// Dispatcher manages a priority queue of dispatchable tasks.
type Dispatcher struct {
	mu     sync.Mutex
	queue  priorityQueue
	byID   map[string]*DispatchEntry
	store  Store
	stopCh chan struct{}
}

// NewDispatcher creates a task dispatcher backed by the given store.
func NewDispatcher(store Store) *Dispatcher {
	d := &Dispatcher{
		queue:  make(priorityQueue, 0),
		byID:   make(map[string]*DispatchEntry),
		store:  store,
		stopCh: make(chan struct{}),
	}
	heap.Init(&d.queue)
	go d.timeoutLoop()
	return d
}

// Enqueue adds a task to the dispatch queue.
func (d *Dispatcher) Enqueue(taskID string, requiredCaps []string, priority int, timeoutSec int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.byID[taskID]; exists {
		return fmt.Errorf("task %s already in dispatch queue", taskID)
	}

	entry := &DispatchEntry{
		TaskID:         taskID,
		RequiredCaps:   requiredCaps,
		Priority:       priority,
		DispatchStatus: DispatchQueued,
		TimeoutSec:     timeoutSec,
		MaxRetries:     DefaultMaxRetries,
		QueuedAt:       time.Now(),
	}
	d.byID[taskID] = entry
	heap.Push(&d.queue, entry)

	slog.Info("task enqueued for dispatch", "task_id", taskID, "priority", priority, "caps", requiredCaps)
	return nil
}

// Dequeue removes a task from the queue (e.g. on cancel).
func (d *Dispatcher) Dequeue(taskID string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	entry, ok := d.byID[taskID]
	if !ok {
		return false
	}
	heap.Remove(&d.queue, entry.index)
	delete(d.byID, taskID)
	return true
}

// PeekForWorker finds the highest-priority queued task matching any
// of the given capabilities. Does NOT remove from queue.
func (d *Dispatcher) PeekForWorker(caps []string) []*DispatchEntry {
	d.mu.Lock()
	defer d.mu.Unlock()

	var matches []*DispatchEntry
	for _, entry := range d.byID {
		if entry.DispatchStatus != DispatchQueued {
			continue
		}
		if capsMatch(entry.RequiredCaps, caps) {
			cp := *entry
			matches = append(matches, &cp)
		}
	}
	return matches
}

// Assign marks a task as assigned to a worker.
func (d *Dispatcher) Assign(taskID, workerID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	entry, ok := d.byID[taskID]
	if !ok {
		return fmt.Errorf("task %s not in dispatch queue", taskID)
	}
	if entry.DispatchStatus != DispatchQueued {
		return fmt.Errorf("task %s is %s, not queued", taskID, entry.DispatchStatus)
	}

	entry.DispatchStatus = DispatchAssigned
	entry.AssignedWorker = workerID
	now := time.Now()
	entry.AssignedAt = &now

	slog.Info("task assigned to worker", "task_id", taskID, "worker_id", workerID)
	return nil
}

// Complete marks a dispatched task as finished and removes it from tracking.
func (d *Dispatcher) Complete(taskID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	entry, ok := d.byID[taskID]
	if !ok {
		return
	}
	entry.DispatchStatus = DispatchCompleted
	heap.Remove(&d.queue, entry.index)
	delete(d.byID, taskID)
}

// Requeue returns a timed-out or failed task back to the queue.
func (d *Dispatcher) Requeue(taskID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	entry, ok := d.byID[taskID]
	if !ok {
		return fmt.Errorf("task %s not in dispatch queue", taskID)
	}

	entry.RetryCount++
	if entry.RetryCount > entry.MaxRetries {
		delete(d.byID, taskID)
		slog.Warn("task exceeded max retries, removed from dispatch", "task_id", taskID)
		return fmt.Errorf("task %s exceeded max retries (%d)", taskID, entry.MaxRetries)
	}

	entry.DispatchStatus = DispatchQueued
	entry.AssignedWorker = ""
	entry.AssignedAt = nil
	heap.Fix(&d.queue, entry.index)

	slog.Info("task re-queued", "task_id", taskID, "retry", entry.RetryCount)
	return nil
}

// Pending returns copies of all queued (unassigned) entries.
func (d *Dispatcher) Pending() []DispatchEntry {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []DispatchEntry
	for _, e := range d.byID {
		if e.DispatchStatus == DispatchQueued {
			out = append(out, *e)
		}
	}
	return out
}

// CheckTimeouts detects timed-out assigned tasks and returns their IDs.
func (d *Dispatcher) CheckTimeouts() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	var ids []string
	now := time.Now()
	for _, entry := range d.byID {
		if entry.DispatchStatus != DispatchAssigned || entry.TimeoutSec <= 0 || entry.AssignedAt == nil {
			continue
		}
		if now.After(entry.AssignedAt.Add(time.Duration(entry.TimeoutSec) * time.Second)) {
			entry.DispatchStatus = DispatchTimeout
			ids = append(ids, entry.TaskID)
		}
	}
	return ids
}

// ListQueued returns copies of all entries currently in the dispatch queue.
func (d *Dispatcher) ListQueued() []*DispatchEntry {
	d.mu.Lock()
	defer d.mu.Unlock()

	out := make([]*DispatchEntry, 0, len(d.byID))
	for _, e := range d.byID {
		cp := *e
		out = append(out, &cp)
	}
	return out
}

// Stop terminates the background timeout checker.
func (d *Dispatcher) Stop() {
	close(d.stopCh)
}

// timeoutLoop periodically checks for assigned tasks that have exceeded their timeout.
func (d *Dispatcher) timeoutLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.checkTimeouts(context.Background())
		}
	}
}

func (d *Dispatcher) checkTimeouts(_ context.Context) {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	for _, entry := range d.byID {
		if entry.DispatchStatus != DispatchAssigned {
			continue
		}
		if entry.TimeoutSec <= 0 || entry.AssignedAt == nil {
			continue
		}
		deadline := entry.AssignedAt.Add(time.Duration(entry.TimeoutSec) * time.Second)
		if now.After(deadline) {
			slog.Warn("dispatch task timed out", "task_id", entry.TaskID, "worker", entry.AssignedWorker)
			entry.DispatchStatus = DispatchTimeout
			entry.RetryCount++
			if entry.RetryCount <= entry.MaxRetries {
				entry.DispatchStatus = DispatchQueued
				entry.AssignedWorker = ""
				entry.AssignedAt = nil
				heap.Fix(&d.queue, entry.index)
				slog.Info("task re-queued after timeout", "task_id", entry.TaskID)
			}
		}
	}
}

// capsMatch returns true if the required caps are a subset of the offered caps,
// or if requiredCaps is empty (any worker can handle).
func capsMatch(required, offered []string) bool {
	if len(required) == 0 {
		return true
	}
	offerSet := make(map[string]bool, len(offered))
	for _, c := range offered {
		offerSet[c] = true
	}
	for _, r := range required {
		if offerSet[r] {
			return true
		}
	}
	return false
}

// ──────────────────────────────────────────────
// Priority Queue (min-heap by negative priority → highest priority first)
// ──────────────────────────────────────────────

type priorityQueue []*DispatchEntry

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	if pq[i].Priority != pq[j].Priority {
		return pq[i].Priority > pq[j].Priority // higher priority first
	}
	return pq[i].QueuedAt.Before(pq[j].QueuedAt) // FIFO for same priority
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(x any) {
	n := len(*pq)
	entry := x.(*DispatchEntry)
	entry.index = n
	*pq = append(*pq, entry)
}

func (pq *priorityQueue) Pop() any {
	old := *pq
	n := len(old)
	entry := old[n-1]
	old[n-1] = nil
	entry.index = -1
	*pq = old[:n-1]
	return entry
}
