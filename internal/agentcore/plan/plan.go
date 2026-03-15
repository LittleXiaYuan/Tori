package plan

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ──────────────────────────────────────────────
// Step & Plan states
// ──────────────────────────────────────────────

type StepStatus string

const (
	StepPending    StepStatus = "pending"
	StepInProgress StepStatus = "in_progress"
	StepCompleted  StepStatus = "completed"
	StepFailed     StepStatus = "failed"
	StepSkipped    StepStatus = "skipped"
)

type PlanStatus string

const (
	PlanCreated   PlanStatus = "created"
	PlanRunning   PlanStatus = "running"
	PlanCompleted PlanStatus = "completed"
	PlanFailed    PlanStatus = "failed"
	PlanAborted   PlanStatus = "aborted"
)

// ──────────────────────────────────────────────
// PlanStep
// ──────────────────────────────────────────────

// PlanStep is a single step in a multi-step plan.
type PlanStep struct {
	Index       int           `json:"index"`
	Description string        `json:"description"`
	Status      StepStatus    `json:"status"`
	Output      string        `json:"output,omitempty"`
	Error       string        `json:"error,omitempty"`
	StartedAt   *time.Time    `json:"started_at,omitempty"`
	EndedAt     *time.Time    `json:"ended_at,omitempty"`
	Duration    time.Duration `json:"duration,omitempty"`
	ToolsUsed   []string      `json:"tools_used,omitempty"`
}

// ──────────────────────────────────────────────
// Plan
// ──────────────────────────────────────────────

// Plan represents a multi-step task decomposition.
type Plan struct {
	ID            string        `json:"id"`
	Task          string        `json:"task"` // original user request
	Status        PlanStatus    `json:"status"`
	Steps         []PlanStep    `json:"steps"`
	Summary       string        `json:"summary,omitempty"` // completion summary
	CreatedAt     time.Time     `json:"created_at"`
	CompletedAt   *time.Time    `json:"completed_at,omitempty"`
	TotalDuration time.Duration `json:"total_duration,omitempty"`
}

// CurrentStep returns the index of the first non-completed step, or -1.
func (p *Plan) CurrentStep() int {
	for i, s := range p.Steps {
		if s.Status == StepPending || s.Status == StepInProgress {
			return i
		}
	}
	return -1
}

// Progress returns (completed, total).
func (p *Plan) Progress() (int, int) {
	completed := 0
	for _, s := range p.Steps {
		if s.Status == StepCompleted || s.Status == StepSkipped {
			completed++
		}
	}
	return completed, len(p.Steps)
}

// IsComplete returns true if all steps are done.
func (p *Plan) IsComplete() bool {
	c, t := p.Progress()
	return c == t && t > 0
}

// ──────────────────────────────────────────────
// Callbacks
// ──────────────────────────────────────────────

// DecomposeFunc breaks a task into steps. Returns step descriptions.
type DecomposeFunc func(ctx context.Context, task string) ([]string, error)

// ExecuteStepFunc executes a single plan step. Returns output or error.
type ExecuteStepFunc func(ctx context.Context, plan *Plan, stepIndex int) (output string, toolsUsed []string, err error)

// SummarizeFunc produces a completion summary.
type SummarizeFunc func(ctx context.Context, plan *Plan) (string, error)

// OnStepUpdateFunc is called when a step status changes.
type OnStepUpdateFunc func(plan *Plan, stepIndex int, status StepStatus)

// ──────────────────────────────────────────────
// Manager
// ──────────────────────────────────────────────

// Manager creates, tracks, and executes plans.
type Manager struct {
	mu           sync.RWMutex
	plans        map[string]*Plan
	decompose    DecomposeFunc
	executeStep  ExecuteStepFunc
	summarize    SummarizeFunc
	onStepUpdate OnStepUpdateFunc
}

// NewManager creates a plan manager.
func NewManager(decompose DecomposeFunc, execute ExecuteStepFunc) *Manager {
	return &Manager{
		plans:       make(map[string]*Plan),
		decompose:   decompose,
		executeStep: execute,
	}
}

// SetSummarize sets the summarization callback.
func (m *Manager) SetSummarize(fn SummarizeFunc) { m.summarize = fn }

// SetOnStepUpdate sets the progress callback.
func (m *Manager) SetOnStepUpdate(fn OnStepUpdateFunc) { m.onStepUpdate = fn }

// ──────────────────────────────────────────────
// Plan lifecycle
// ──────────────────────────────────────────────

// Create decomposes a task into a plan.
func (m *Manager) Create(ctx context.Context, task string) (*Plan, error) {
	if m.decompose == nil {
		return nil, fmt.Errorf("plan: no decompose function set")
	}

	descriptions, err := m.decompose(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("plan: decompose failed: %w", err)
	}
	if len(descriptions) == 0 {
		return nil, fmt.Errorf("plan: decompose returned no steps")
	}

	steps := make([]PlanStep, len(descriptions))
	for i, desc := range descriptions {
		steps[i] = PlanStep{
			Index:       i,
			Description: desc,
			Status:      StepPending,
		}
	}

	plan := &Plan{
		ID:        uuid.New().String(),
		Task:      task,
		Status:    PlanCreated,
		Steps:     steps,
		CreatedAt: time.Now(),
	}

	m.mu.Lock()
	m.plans[plan.ID] = plan
	m.mu.Unlock()

	slog.Info("plan: created", "id", plan.ID, "steps", len(steps))
	return plan, nil
}

// CreateFromSteps creates a plan from pre-defined step descriptions.
func (m *Manager) CreateFromSteps(task string, descriptions []string) *Plan {
	steps := make([]PlanStep, len(descriptions))
	for i, desc := range descriptions {
		steps[i] = PlanStep{Index: i, Description: desc, Status: StepPending}
	}
	plan := &Plan{
		ID:        uuid.New().String(),
		Task:      task,
		Status:    PlanCreated,
		Steps:     steps,
		CreatedAt: time.Now(),
	}
	m.mu.Lock()
	m.plans[plan.ID] = plan
	m.mu.Unlock()
	return plan
}

// Execute runs all steps in a plan sequentially.
func (m *Manager) Execute(ctx context.Context, planID string) error {
	m.mu.RLock()
	plan, ok := m.plans[planID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("plan: %q not found", planID)
	}

	plan.Status = PlanRunning
	slog.Info("plan: executing", "id", planID, "steps", len(plan.Steps))

	for i := range plan.Steps {
		select {
		case <-ctx.Done():
			plan.Status = PlanAborted
			return ctx.Err()
		default:
		}

		step := &plan.Steps[i]
		if step.Status == StepCompleted || step.Status == StepSkipped {
			continue
		}

		step.Status = StepInProgress
		now := time.Now()
		step.StartedAt = &now
		m.notifyStep(plan, i, StepInProgress)

		output, tools, err := m.executeStep(ctx, plan, i)
		end := time.Now()
		step.EndedAt = &end
		step.Duration = end.Sub(now)
		step.ToolsUsed = tools

		if err != nil {
			step.Status = StepFailed
			step.Error = err.Error()
			m.notifyStep(plan, i, StepFailed)

			plan.Status = PlanFailed
			slog.Warn("plan: step failed", "plan", planID, "step", i, "err", err)
			return fmt.Errorf("step %d failed: %w", i, err)
		}

		step.Status = StepCompleted
		step.Output = output
		m.notifyStep(plan, i, StepCompleted)
	}

	now := time.Now()
	plan.CompletedAt = &now
	plan.TotalDuration = now.Sub(plan.CreatedAt)
	plan.Status = PlanCompleted

	// Generate summary
	if m.summarize != nil {
		summary, err := m.summarize(ctx, plan)
		if err == nil {
			plan.Summary = summary
		}
	}

	slog.Info("plan: completed", "id", planID, "duration", plan.TotalDuration)
	return nil
}

// UpdateStep manually updates a step's status.
func (m *Manager) UpdateStep(planID string, stepIndex int, status StepStatus, output string) error {
	m.mu.RLock()
	plan, ok := m.plans[planID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("plan: %q not found", planID)
	}
	if stepIndex < 0 || stepIndex >= len(plan.Steps) {
		return fmt.Errorf("plan: step %d out of range", stepIndex)
	}
	plan.Steps[stepIndex].Status = status
	plan.Steps[stepIndex].Output = output
	m.notifyStep(plan, stepIndex, status)
	return nil
}

// SkipStep marks a step as skipped.
func (m *Manager) SkipStep(planID string, stepIndex int) error {
	return m.UpdateStep(planID, stepIndex, StepSkipped, "skipped by user")
}

func (m *Manager) notifyStep(plan *Plan, index int, status StepStatus) {
	if m.onStepUpdate != nil {
		m.onStepUpdate(plan, index, status)
	}
}

// ──────────────────────────────────────────────
// Query
// ──────────────────────────────────────────────

// Get returns a plan by ID.
func (m *Manager) Get(id string) (*Plan, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.plans[id]
	return p, ok
}

// List returns all plans.
func (m *Manager) List() []*Plan {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Plan, 0, len(m.plans))
	for _, p := range m.plans {
		out = append(out, p)
	}
	return out
}

// ActivePlans returns plans that are currently running.
func (m *Manager) ActivePlans() []*Plan {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []*Plan
	for _, p := range m.plans {
		if p.Status == PlanRunning || p.Status == PlanCreated {
			out = append(out, p)
		}
	}
	return out
}

// Remove deletes a plan.
func (m *Manager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.plans, id)
}

// ──────────────────────────────────────────────
// Detection helper
// ──────────────────────────────────────────────

// NeedsPlan heuristically checks if a task should use Plan Mode.
func NeedsPlan(task string) bool {
	asciiKeywords := []string{"then", "next", "after", "first", "finally", "step", "plan"}
	cjkKeywords := []string{"然后", "接下来", "之后", "首先", "最后", "步骤", "计划"}
	lower := toLower(task)
	for _, kw := range asciiKeywords {
		if containsStr(lower, kw) {
			return true
		}
	}
	for _, kw := range cjkKeywords {
		if containsStr(task, kw) {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
