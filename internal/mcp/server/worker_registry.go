package server

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Capability tags used by workers to declare what they can do.
const (
	CapCoding  = "coding"
	CapTesting = "testing"
	CapReview  = "review"
	CapDocs    = "docs"
	CapDeploy  = "deploy"
)

// Worker represents a connected external tool (Cursor, Claude Code, etc.).
type Worker struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"` // "cursor", "claude_code", "windsurf", "custom"
	Capabilities []string  `json:"capabilities"`
	MaxConcur    int       `json:"max_concurrency"`
	ActiveTasks  int       `json:"active_tasks"`
	Status       string    `json:"status"` // "online", "busy", "offline"
	RegisteredAt time.Time `json:"registered_at"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// WorkerRegistry manages connected workers with health checking.
type WorkerRegistry struct {
	mu      sync.RWMutex
	workers map[string]*Worker

	heartbeatTimeout time.Duration
	stopCh           chan struct{}
}

// NewWorkerRegistry creates a new registry with a background reaper.
func NewWorkerRegistry(heartbeatTimeout time.Duration) *WorkerRegistry {
	if heartbeatTimeout <= 0 {
		heartbeatTimeout = 60 * time.Second
	}
	wr := &WorkerRegistry{
		workers:          make(map[string]*Worker),
		heartbeatTimeout: heartbeatTimeout,
		stopCh:           make(chan struct{}),
	}
	go wr.reapLoop()
	return wr
}

// Register adds or updates a worker. Returns the assigned ID.
func (wr *WorkerRegistry) Register(id, name, workerType string, caps []string, maxConcur int, meta map[string]string) *Worker {
	wr.mu.Lock()
	defer wr.mu.Unlock()

	if id == "" {
		id = fmt.Sprintf("worker-%d", time.Now().UnixNano())
	}
	if maxConcur <= 0 {
		maxConcur = 1
	}

	now := time.Now()
	w := &Worker{
		ID:            id,
		Name:          name,
		Type:          workerType,
		Capabilities:  caps,
		MaxConcur:     maxConcur,
		Status:        "online",
		RegisteredAt:  now,
		LastHeartbeat: now,
		Metadata:      meta,
	}
	wr.workers[id] = w
	slog.Info("mcp server: worker registered", "id", id, "name", name, "caps", caps)
	return w
}

// Heartbeat updates the worker's last heartbeat timestamp.
func (wr *WorkerRegistry) Heartbeat(id string) bool {
	wr.mu.Lock()
	defer wr.mu.Unlock()
	w, ok := wr.workers[id]
	if !ok {
		return false
	}
	w.LastHeartbeat = time.Now()
	if w.Status == "offline" {
		w.Status = "online"
		slog.Info("mcp server: worker back online", "id", id)
	}
	return true
}

// Unregister removes a worker by ID. Returns released task count.
func (wr *WorkerRegistry) Unregister(id string) bool {
	wr.mu.Lock()
	defer wr.mu.Unlock()
	_, ok := wr.workers[id]
	if ok {
		delete(wr.workers, id)
		slog.Info("mcp server: worker unregistered", "id", id)
	}
	return ok
}

// Get returns a copy of the worker with the given ID.
func (wr *WorkerRegistry) Get(id string) (*Worker, bool) {
	wr.mu.RLock()
	defer wr.mu.RUnlock()
	w, ok := wr.workers[id]
	if !ok {
		return nil, false
	}
	cp := *w
	return &cp, true
}

// List returns all workers.
func (wr *WorkerRegistry) List() []*Worker {
	wr.mu.RLock()
	defer wr.mu.RUnlock()
	out := make([]*Worker, 0, len(wr.workers))
	for _, w := range wr.workers {
		cp := *w
		out = append(out, &cp)
	}
	return out
}

// FindByCapability returns online workers that have the given capability.
func (wr *WorkerRegistry) FindByCapability(cap string) []*Worker {
	wr.mu.RLock()
	defer wr.mu.RUnlock()
	var out []*Worker
	for _, w := range wr.workers {
		if w.Status == "offline" {
			continue
		}
		for _, c := range w.Capabilities {
			if c == cap {
				cp := *w
				out = append(out, &cp)
				break
			}
		}
	}
	return out
}

// IncrementActive marks a worker as having one more active task.
func (wr *WorkerRegistry) IncrementActive(id string) {
	wr.mu.Lock()
	defer wr.mu.Unlock()
	if w, ok := wr.workers[id]; ok {
		w.ActiveTasks++
		if w.ActiveTasks >= w.MaxConcur {
			w.Status = "busy"
		}
	}
}

// DecrementActive marks a worker as having one less active task.
func (wr *WorkerRegistry) DecrementActive(id string) {
	wr.mu.Lock()
	defer wr.mu.Unlock()
	if w, ok := wr.workers[id]; ok {
		if w.ActiveTasks > 0 {
			w.ActiveTasks--
		}
		if w.Status == "busy" && w.ActiveTasks < w.MaxConcur {
			w.Status = "online"
		}
	}
}

// Stop terminates the background reaper.
func (wr *WorkerRegistry) Stop() {
	close(wr.stopCh)
}

func (wr *WorkerRegistry) reapLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-wr.stopCh:
			return
		case <-ticker.C:
			wr.reap()
		}
	}
}

func (wr *WorkerRegistry) reap() {
	wr.mu.Lock()
	defer wr.mu.Unlock()

	deadline := time.Now().Add(-wr.heartbeatTimeout)
	for id, w := range wr.workers {
		if w.LastHeartbeat.Before(deadline) && w.Status != "offline" {
			w.Status = "offline"
			slog.Warn("mcp server: worker timed out", "id", id, "last_heartbeat", w.LastHeartbeat)
		}
	}
}
