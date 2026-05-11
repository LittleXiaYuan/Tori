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
	StepReady      StepStatus = "ready"
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
	PlanRevising  PlanStatus = "revising"
	PlanFailed    PlanStatus = "failed"
	PlanAborted   PlanStatus = "aborted"
)

type Budget struct {
	MaxSteps      int           `json:"max_steps"`
	MaxRevisions  int           `json:"max_revisions"`
	MaxDuration   time.Duration `json:"max_duration"`
	StepsUsed     int           `json:"steps_used"`
	RevisionsUsed int           `json:"revisions_used"`
}

func DefaultBudget() Budget {
	return Budget{MaxSteps: 20, MaxRevisions: 3, MaxDuration: 5 * time.Minute}
}

func (b *Budget) CanStep() bool   { return b.MaxSteps <= 0 || b.StepsUsed < b.MaxSteps }
func (b *Budget) CanRevise() bool { return b.MaxRevisions <= 0 || b.RevisionsUsed < b.MaxRevisions }

// ──────────────────────────────────────────────
// PlanStep
// ──────────────────────────────────────────────

// PlanStep is a single step in a multi-step plan.
type PlanStep struct {
	Index       int            `json:"index"`
	Description string         `json:"description"`
	Skill       string         `json:"skill,omitempty"`
	Args        map[string]any `json:"args,omitempty"`
	DependsOn   []int          `json:"depends_on"`
	Status      StepStatus     `json:"status"`
	Output      string         `json:"output,omitempty"`
	Error       string         `json:"error,omitempty"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	EndedAt     *time.Time     `json:"ended_at,omitempty"`
	Duration    time.Duration  `json:"duration,omitempty"`
	ToolsUsed   []string       `json:"tools_used,omitempty"`
	RetryCount  int            `json:"retry_count,omitempty"`
}

// ──────────────────────────────────────────────
// Plan
// ──────────────────────────────────────────────

// Plan represents a multi-step task decomposition.
type Plan struct {
	ID            string        `json:"id"`
	Task          string        `json:"task"`
	Status        PlanStatus    `json:"status"`
	Steps         []PlanStep    `json:"steps"`
	Summary       string        `json:"summary,omitempty"`
	Budget        Budget        `json:"budget"`
	Revisions     int           `json:"revisions"`
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

func (p *Plan) ReadySteps() []int {
	var ready []int
	for i, step := range p.Steps {
		if step.Status != StepPending && step.Status != StepReady {
			continue
		}
		allMet := true
		for _, dep := range step.DependsOn {
			if dep < 0 || dep >= len(p.Steps) {
				allMet = false
				break
			}
			ds := p.Steps[dep].Status
			if ds != StepCompleted && ds != StepSkipped {
				allMet = false
				break
			}
		}
		if allMet {
			ready = append(ready, i)
		}
	}
	return ready
}

func (p *Plan) FailedSteps() []int {
	var failed []int
	for i, s := range p.Steps {
		if s.Status == StepFailed {
			failed = append(failed, i)
		}
	}
	return failed
}

func (p *Plan) StepSummary() string {
	var b []byte
	for _, s := range p.Steps {
		line := fmt.Sprintf("[%d] %s — %s: %s", s.Index, s.Status, s.Description, s.Skill)
		if s.Output != "" {
			out := s.Output
			if len([]rune(out)) > 100 {
				out = string([]rune(out)[:100]) + "..."
			}
			line += " → " + out
		}
		if s.Error != "" {
			line += " ✗ " + s.Error
		}
		b = append(b, []byte(line+"\n")...)
	}
	return string(b)
}

// ──────────────────────────────────────────────
// Callbacks
// ──────────────────────────────────────────────

// DecomposeFunc breaks a task into steps. Returns step descriptions.
type DecomposeFunc func(ctx context.Context, task string) ([]string, error)

type DecomposeDAGFunc func(ctx context.Context, task string) ([]PlanStep, error)

type ReviseFunc func(ctx context.Context, goal string, current *Plan, failedStep int) ([]PlanStep, error)

type ExecuteStepFunc func(ctx context.Context, plan *Plan, stepIndex int) (output string, toolsUsed []string, err error)

type SummarizeFunc func(ctx context.Context, plan *Plan) (string, error)

type OnStepUpdateFunc func(plan *Plan, stepIndex int, status StepStatus)

// Manager creates, tracks, and executes plans.
type Manager struct {
	mu           sync.RWMutex
	plans        map[string]*Plan
	decompose    DecomposeFunc
	decomposeDAG DecomposeDAGFunc
	revise       ReviseFunc
	executeStep  ExecuteStepFunc
	summarize    SummarizeFunc
	onStepUpdate OnStepUpdateFunc
}

func (m *Manager) SetDecomposeDAG(fn DecomposeDAGFunc) { m.decomposeDAG = fn }
func (m *Manager) SetRevise(fn ReviseFunc)             { m.revise = fn }

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

func (m *Manager) CreateDAG(ctx context.Context, task string, budget Budget) (*Plan, error) {
	if m.decomposeDAG == nil {
		return nil, fmt.Errorf("plan: no DAG decompose function set")
	}
	steps, err := m.decomposeDAG(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("plan: DAG decompose: %w", err)
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("plan: no steps")
	}
	for i := range steps {
		steps[i].Index = i
		steps[i].Status = StepPending
	}
	p := &Plan{ID: uuid.New().String(), Task: task, Status: PlanCreated, Steps: steps, Budget: budget, CreatedAt: time.Now()}
	m.mu.Lock()
	m.plans[p.ID] = p
	m.mu.Unlock()
	slog.Info("plan: created DAG", "id", p.ID, "steps", len(steps))
	return p, nil
}

func (m *Manager) ExecuteDAG(ctx context.Context, planID string) error {
	m.mu.RLock()
	p, ok := m.plans[planID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("plan: %q not found", planID)
	}
	if p.Budget.MaxDuration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, time.Now().Add(p.Budget.MaxDuration))
		defer cancel()
	}
	p.Status = PlanRunning
	for {
		if ctx.Err() != nil {
			p.Status = PlanAborted
			return ctx.Err()
		}
		if p.IsComplete() {
			break
		}
		ready := p.ReadySteps()
		if len(ready) == 0 {
			failed := p.FailedSteps()
			if len(failed) > 0 && m.revise != nil && p.Budget.CanRevise() {
				p.Budget.RevisionsUsed++
				p.Revisions++
				p.Status = PlanRevising
				newSteps, err := m.revise(ctx, p.Task, p, failed[0])
				if err != nil || len(newSteps) == 0 {
					p.Status = PlanFailed
					return fmt.Errorf("plan: revision failed")
				}
				var kept []PlanStep
				for _, s := range p.Steps {
					if s.Status == StepCompleted || s.Status == StepSkipped {
						kept = append(kept, s)
					}
				}
				base := len(kept)
				for i, ns := range newSteps {
					ns.Index = base + i
					ns.Status = StepPending
					kept = append(kept, ns)
				}
				p.Steps = kept
				p.Status = PlanRunning
				continue
			}
			p.Status = PlanFailed
			return fmt.Errorf("plan: no ready steps")
		}
		if !p.Budget.CanStep() {
			p.Status = PlanFailed
			return fmt.Errorf("plan: budget exhausted")
		}
		if p.Budget.MaxSteps > 0 {
			remaining := p.Budget.MaxSteps - p.Budget.StepsUsed
			if remaining <= 0 {
				p.Status = PlanFailed
				return fmt.Errorf("plan: budget exhausted")
			}
			if len(ready) > remaining {
				ready = ready[:remaining]
			}
		}
		type result struct {
			idx   int
			out   string
			tools []string
			err   error
		}
		ch := make(chan result, len(ready))
		for _, idx := range ready {
			p.Steps[idx].Status = StepInProgress
			now := time.Now()
			p.Steps[idx].StartedAt = &now
			m.notifyStep(p, idx, StepInProgress)
			go func(i int) {
				out, tools, err := m.executeStep(ctx, p, i)
				ch <- result{i, out, tools, err}
			}(idx)
		}
		for range ready {
			r := <-ch
			p.Budget.StepsUsed++
			step := &p.Steps[r.idx]
			end := time.Now()
			step.EndedAt = &end
			if step.StartedAt != nil {
				step.Duration = end.Sub(*step.StartedAt)
			}
			step.ToolsUsed = r.tools
			if r.err != nil {
				step.Status = StepFailed
				step.Error = r.err.Error()
				step.RetryCount++
				m.notifyStep(p, r.idx, StepFailed)
			} else {
				step.Status = StepCompleted
				step.Output = r.out
				m.notifyStep(p, r.idx, StepCompleted)
			}
		}
	}
	now := time.Now()
	p.CompletedAt = &now
	p.TotalDuration = now.Sub(p.CreatedAt)
	p.Status = PlanCompleted
	if m.summarize != nil {
		if s, err := m.summarize(ctx, p); err == nil {
			p.Summary = s
		}
	}
	return nil
}
