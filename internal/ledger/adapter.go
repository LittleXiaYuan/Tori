// Package ledger provides adapters that connect yunque-agent's existing
// task and memory systems to the standalone Ledger module.
package ledger

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/LittleXiaYuan/ledger"
	lsqlite "github.com/LittleXiaYuan/ledger/backend/sqlite"

	agtask "yunque-agent/internal/agentcore/task"
)

// InitLedger creates and returns a Ledger instance configured for yunque-agent.
//
// Configuration priority:
//  1. LEDGER_DB_PATH env → custom SQLite path
//  2. Default → ./data/ledger/ledger.db
func InitLedger() (*ledger.Ledger, error) {
	dbPath := os.Getenv("LEDGER_DB_PATH")
	if dbPath == "" {
		dbPath = filepath.Join(".", "data", "ledger", "ledger.db")
	}
	return InitLedgerAt(dbPath)
}

func InitLedgerAt(dbPath string) (*ledger.Ledger, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("ledger db path must not be empty")
	}
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create ledger data dir: %w", err)
	}

	backend, err := lsqlite.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("create ledger backend: %w", err)
	}

	ldg, err := ledger.Open(backend)
	if err != nil {
		backend.Close()
		return nil, fmt.Errorf("open ledger: %w", err)
	}

	return ldg, nil
}

// LedgerStore adapts the standalone Ledger module to the yunque-agent
// agtask.Store interface. The gateway handlers continue to call
// Create/Get/List/Update/Delete without knowing about Ledger.
type LedgerStore struct {
	ldg     *ledger.Ledger
	baseDir string // for artifact file storage
	mu      sync.RWMutex
}

// NewLedgerStore creates a store backed by Ledger.
func NewLedgerStore(ldg *ledger.Ledger, artifactBaseDir string) *LedgerStore {
	return &LedgerStore{
		ldg:     ldg,
		baseDir: artifactBaseDir,
	}
}

// Ledger returns the underlying Ledger instance.
func (s *LedgerStore) Ledger() *ledger.Ledger {
	return s.ldg
}

// Create builds a new task in Ledger and returns a yunque-agent Task.
func (s *LedgerStore) Create(req agtask.CreateRequest) (*agtask.Task, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	ctx := context.Background()
	title := req.Title
	if title == "" {
		r := []rune(req.Description)
		if len(r) > 50 {
			title = string(r[:50]) + "..."
		} else {
			title = req.Description
		}
	}

	lt, err := s.ldg.Tasks.CreateTask(ctx, req.Description, ledger.TaskTypeGoal, req.TenantID,
		ledger.WithMetadata(mustJSON(map[string]string{"title": title})),
	)
	if err != nil {
		return nil, fmt.Errorf("ledger create: %w", err)
	}

	return ledgerToAgentTask(lt, title), nil
}

// Get returns a deep copy of a task by ID.
func (s *LedgerStore) Get(id string) (*agtask.Task, bool) {
	ctx := context.Background()
	lt, err := s.ldg.Tasks.GetTask(ctx, id)
	if err != nil {
		return nil, false
	}
	return ledgerToAgentTask(lt, extractTitle(lt)), true
}

// List returns tasks for a tenant, sorted by creation time (newest first).
func (s *LedgerStore) List(tenantID string, limit int) []*agtask.Task {
	ctx := context.Background()
	filter := ledger.TaskFilter{
		TenantID: tenantID,
		Limit:    limit,
	}
	lts, err := s.ldg.Tasks.ListTasks(ctx, filter)
	if err != nil {
		return nil
	}

	out := make([]*agtask.Task, 0, len(lts))
	for _, lt := range lts {
		out = append(out, ledgerToAgentTask(lt, extractTitle(lt)))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// Update writes the task back to Ledger.
func (s *LedgerStore) Update(t *agtask.Task) error {
	ctx := context.Background()
	lt, err := s.ldg.Tasks.GetTask(ctx, t.ID)
	if err != nil {
		return err
	}

	targetStatus := agentStatusToLedger(t.Status)
	if targetStatus != lt.Status {
		if err := s.transitionTaskStatus(ctx, t.ID, lt.Status, targetStatus); err != nil {
			return fmt.Errorf("ledger transition %s -> %s: %w", lt.Status, targetStatus, err)
		}
	}

	lt, _ = s.ldg.Tasks.GetTask(ctx, t.ID)
	lt.Goal = t.Description
	lt.UpdatedAt = time.Now()

	meta := map[string]interface{}{
		"title":     t.Title,
		"steps":     t.Steps,
		"artifacts": t.Artifacts,
	}
	lt.Metadata = mustJSON(meta)

	if t.Error != "" {
		lt.Error = &t.Error
	}
	lt.StartedAt = t.StartedAt
	lt.FinishedAt = t.FinishedAt

	return s.ldg.Backend().UpdateTask(ctx, lt)
}

func (s *LedgerStore) transitionTaskStatus(ctx context.Context, taskID string, from, to ledger.TaskStatus) error {
	path, ok := ledgerTransitionPath(from, to)
	if !ok {
		return fmt.Errorf("unsupported transition path")
	}
	for _, next := range path {
		actor := string(ledger.TransitionActorFor(from, next))
		if err := s.ldg.Tasks.Transition(ctx, taskID, next, actor, nil); err != nil {
			return err
		}
		from = next
	}
	return nil
}

func ledgerTransitionPath(from, to ledger.TaskStatus) ([]ledger.TaskStatus, bool) {
	if from == to {
		return nil, true
	}
	switch from {
	case ledger.TaskCreated:
		switch to {
		case ledger.TaskReady:
			return []ledger.TaskStatus{ledger.TaskReady}, true
		case ledger.TaskRunning:
			return []ledger.TaskStatus{ledger.TaskReady, ledger.TaskRunning}, true
		case ledger.TaskCompleted, ledger.TaskFailed, ledger.TaskWaitingInput, ledger.TaskBlocked, ledger.TaskRetrying:
			return []ledger.TaskStatus{ledger.TaskReady, ledger.TaskRunning, to}, true
		case ledger.TaskCancelled:
			return []ledger.TaskStatus{ledger.TaskCancelled}, true
		}
	case ledger.TaskReady:
		switch to {
		case ledger.TaskRunning:
			return []ledger.TaskStatus{ledger.TaskRunning}, true
		case ledger.TaskCompleted, ledger.TaskFailed, ledger.TaskWaitingInput, ledger.TaskBlocked, ledger.TaskRetrying:
			return []ledger.TaskStatus{ledger.TaskRunning, to}, true
		case ledger.TaskCancelled:
			return []ledger.TaskStatus{ledger.TaskCancelled}, true
		}
	case ledger.TaskRunning:
		switch to {
		case ledger.TaskCompleted, ledger.TaskFailed, ledger.TaskWaitingInput, ledger.TaskBlocked, ledger.TaskRetrying, ledger.TaskCancelled:
			return []ledger.TaskStatus{to}, true
		}
	case ledger.TaskWaitingInput:
		switch to {
		case ledger.TaskRunning:
			return []ledger.TaskStatus{ledger.TaskRunning}, true
		case ledger.TaskCompleted, ledger.TaskFailed, ledger.TaskBlocked, ledger.TaskRetrying:
			return []ledger.TaskStatus{ledger.TaskRunning, to}, true
		case ledger.TaskCancelled:
			return []ledger.TaskStatus{ledger.TaskCancelled}, true
		}
	case ledger.TaskBlocked:
		switch to {
		case ledger.TaskReady:
			return []ledger.TaskStatus{ledger.TaskReady}, true
		case ledger.TaskRunning:
			return []ledger.TaskStatus{ledger.TaskReady, ledger.TaskRunning}, true
		case ledger.TaskCompleted, ledger.TaskFailed, ledger.TaskWaitingInput, ledger.TaskRetrying:
			return []ledger.TaskStatus{ledger.TaskReady, ledger.TaskRunning, to}, true
		case ledger.TaskCancelled:
			return []ledger.TaskStatus{ledger.TaskCancelled}, true
		}
	case ledger.TaskRetrying:
		switch to {
		case ledger.TaskRunning:
			return []ledger.TaskStatus{ledger.TaskRunning}, true
		case ledger.TaskCompleted, ledger.TaskWaitingInput, ledger.TaskBlocked:
			return []ledger.TaskStatus{ledger.TaskRunning, to}, true
		case ledger.TaskFailed:
			return []ledger.TaskStatus{ledger.TaskFailed}, true
		}
	case ledger.TaskFailed, ledger.TaskCancelled:
		switch to {
		case ledger.TaskReady:
			return []ledger.TaskStatus{ledger.TaskReady}, true
		case ledger.TaskRunning:
			return []ledger.TaskStatus{ledger.TaskReady, ledger.TaskRunning}, true
		case ledger.TaskCompleted, ledger.TaskFailed, ledger.TaskWaitingInput, ledger.TaskBlocked, ledger.TaskRetrying:
			return []ledger.TaskStatus{ledger.TaskReady, ledger.TaskRunning, to}, true
		case ledger.TaskCancelled:
			if from == ledger.TaskCancelled {
				return nil, true
			}
		}
	}
	return nil, false
}

// Delete removes a task.
func (s *LedgerStore) Delete(id string) bool {
	ctx := context.Background()
	lt, err := s.ldg.Tasks.GetTask(ctx, id)
	if err != nil {
		return false
	}
	if !lt.Status.IsTerminal() {
		s.ldg.Tasks.Cancel(ctx, id, "deleted by user")
	}

	if s.baseDir != "" {
		os.RemoveAll(filepath.Join(s.baseDir, id))
	}
	return true
}

// ArtifactDir returns the directory for a task's artifacts.
func (s *LedgerStore) ArtifactDir(taskID string) (string, error) {
	dir := filepath.Join(s.baseDir, taskID, "artifacts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// RecoverInterrupted marks any running tasks as failed on startup.
func (s *LedgerStore) RecoverInterrupted() int {
	ctx := context.Background()
	running, err := s.ldg.Tasks.ListTasks(ctx, ledger.TaskFilter{
		Status: []ledger.TaskStatus{ledger.TaskRunning},
	})
	if err != nil {
		return 0
	}

	count := 0
	for _, lt := range running {
		if err := s.ldg.Tasks.Fail(ctx, lt.ID, "process restarted while task was executing"); err == nil {
			count++
		}
	}
	return count
}

// ── conversion helpers ──

func ledgerToAgentTask(lt *ledger.Task, title string) *agtask.Task {
	t := &agtask.Task{
		ID:          lt.ID,
		Title:       title,
		Description: lt.Goal,
		Status:      ledgerStatusToAgent(lt.Status),
		TenantID:    lt.TenantID,
		CreatedAt:   lt.CreatedAt,
		UpdatedAt:   lt.UpdatedAt,
		StartedAt:   lt.StartedAt,
		FinishedAt:  lt.FinishedAt,
	}
	if lt.Error != nil {
		t.Error = *lt.Error
	}

	if len(lt.Metadata) > 0 {
		var meta struct {
			Title     string            `json:"title"`
			Steps     []agtask.Step     `json:"steps"`
			Artifacts []agtask.Artifact `json:"artifacts"`
		}
		if json.Unmarshal(lt.Metadata, &meta) == nil {
			if len(meta.Steps) > 0 {
				t.Steps = meta.Steps
			}
			if len(meta.Artifacts) > 0 {
				t.Artifacts = meta.Artifacts
			}
		}
	}

	return t
}

func extractTitle(lt *ledger.Task) string {
	if len(lt.Metadata) > 0 {
		var meta struct {
			Title string `json:"title"`
		}
		if json.Unmarshal(lt.Metadata, &meta) == nil && meta.Title != "" {
			return meta.Title
		}
	}
	r := []rune(lt.Goal)
	if len(r) > 50 {
		return string(r[:50]) + "..."
	}
	return lt.Goal
}

func ledgerStatusToAgent(s ledger.TaskStatus) agtask.Status {
	switch s {
	case ledger.TaskCreated, ledger.TaskReady:
		return agtask.StatusPending
	case ledger.TaskRunning:
		return agtask.StatusRunning
	case ledger.TaskWaitingInput:
		return agtask.StatusPaused
	case ledger.TaskBlocked:
		return agtask.StatusPaused
	case ledger.TaskRetrying:
		return agtask.StatusRunning
	case ledger.TaskCompleted:
		return agtask.StatusCompleted
	case ledger.TaskFailed:
		return agtask.StatusFailed
	case ledger.TaskCancelled:
		return agtask.StatusCancelled
	default:
		return agtask.StatusPending
	}
}

func agentStatusToLedger(s agtask.Status) ledger.TaskStatus {
	switch s {
	case agtask.StatusPending:
		return ledger.TaskCreated
	case agtask.StatusPlanning:
		return ledger.TaskRunning
	case agtask.StatusRunning:
		return ledger.TaskRunning
	case agtask.StatusPaused:
		return ledger.TaskWaitingInput
	case agtask.StatusCompleted:
		return ledger.TaskCompleted
	case agtask.StatusFailed:
		return ledger.TaskFailed
	case agtask.StatusCancelled:
		return ledger.TaskCancelled
	case agtask.StatusInterrupted:
		return ledger.TaskFailed
	default:
		return ledger.TaskCreated
	}
}

func mustJSON(v interface{}) ledger.JSON {
	b, _ := json.Marshal(v)
	return b
}
