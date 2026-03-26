package task

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Store is the persistence layer interface for tasks.
type Store interface {
	Create(req CreateRequest) (*Task, error)
	Get(id string) (*Task, bool)
	List(tenantID string, limit int) []*Task
	Update(t *Task) error
	Delete(id string) bool
	ArtifactDir(taskID string) (string, error)
	RecoverInterrupted() int
}

// JSONStore persists tasks to individual JSON files under data/tasks/.
type JSONStore struct {
	mu      sync.RWMutex
	tasks   map[string]*Task
	baseDir string
}

// NewJSONStore creates a task store rooted at dir (e.g. "data/tasks").
func NewJSONStore(dir string) *JSONStore {
	s := &JSONStore{
		tasks:   make(map[string]*Task),
		baseDir: dir,
	}
	s.loadAll()
	return s
}

// Create persists a new task and returns it.
func (s *JSONStore) Create(req CreateRequest) (*Task, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	now := time.Now()
	t := &Task{
		ID:          uuid.New().String()[:8],
		Title:       req.Title,
		Description: req.Description,
		Status:      StatusPending,
		TenantID:    req.TenantID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if t.Title == "" {
		// Auto-generate title from first 50 chars of description
		r := []rune(t.Description)
		if len(r) > 50 {
			t.Title = string(r[:50]) + "..."
		} else {
			t.Title = t.Description
		}
	}

	s.mu.Lock()
	s.tasks[t.ID] = t
	s.mu.Unlock()

	if err := s.save(t); err != nil {
		return nil, fmt.Errorf("persist task: %w", err)
	}
	return t, nil
}

// Get returns a deep copy of a task by ID.
func (s *JSONStore) Get(id string) (*Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[id]
	if !ok {
		return nil, false
	}
	return t.clone(), true
}

// List returns deep copies of all tasks for a tenant, sorted by creation time (newest first).
func (s *JSONStore) List(tenantID string, limit int) []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []*Task
	for _, t := range s.tasks {
		if tenantID == "" || t.TenantID == tenantID {
			out = append(out, t.clone())
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// Update writes the task back to the in-memory map and saves to disk.
func (s *JSONStore) Update(t *Task) error {
	t.UpdatedAt = time.Now()
	s.mu.Lock()
	s.tasks[t.ID] = t
	// Serialize within the lock to prevent concurrent file writes for the same task
	err := s.save(t)
	s.mu.Unlock()
	return err
}

// Delete removes a task and its data directory.
func (s *JSONStore) Delete(id string) bool {
	s.mu.Lock()
	_, ok := s.tasks[id]
	if ok {
		delete(s.tasks, id)
	}
	s.mu.Unlock()
	if ok {
		os.RemoveAll(filepath.Join(s.baseDir, id))
	}
	return ok
}

// ArtifactDir returns the directory for a task's artifacts, creating if needed.
func (s *JSONStore) ArtifactDir(taskID string) (string, error) {
	dir := filepath.Join(s.baseDir, taskID, "artifacts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// ── persistence ──

func (s *JSONStore) save(t *Task) error {
	dir := filepath.Join(s.baseDir, t.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "task.json"), data, 0o644)
}

func (s *JSONStore) loadAll() {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return // dir doesn't exist yet, that's fine
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(s.baseDir, e.Name(), "task.json")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var t Task
		if err := json.Unmarshal(data, &t); err != nil {
			continue
		}
		s.tasks[t.ID] = &t
	}
}

// RecoverInterrupted marks any tasks in running/planning state as interrupted.
// Call this on startup to detect zombie tasks from a previous crash.
// Returns the number of recovered tasks.
func (s *JSONStore) RecoverInterrupted() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	now := time.Now()
	for _, t := range s.tasks {
		if t.Status == StatusRunning || t.Status == StatusPlanning {
			t.Status = StatusInterrupted
			t.Error = "process restarted while task was executing"
			t.FinishedAt = &now
			// Mark any running/retrying steps as pending so they can be retried on resume
			for i := range t.Steps {
				if t.Steps[i].Status == StepRunning || t.Steps[i].Status == StepRetrying {
					t.Steps[i].Status = StepPending
				}
			}
			_ = s.save(t)
			count++
		}
	}
	return count
}
