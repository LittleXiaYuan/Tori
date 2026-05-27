package task

import (
	"fmt"
	"time"

	"yunque-agent/pkg/risk"
)

// ──────────────────────────────────────────────
// Task — first-class work unit for the Agent Runtime
//
// A Task represents a concrete piece of work the agent accepts, plans,
// executes step-by-step, and delivers artifacts. Unlike a chat turn
// (message→reply→forget), a Task is persistent, trackable, resumable.
// ──────────────────────────────────────────────

// Status is the lifecycle state of a task.
type Status string

const (
	StatusPending     Status = "pending"     // created, not started
	StatusPlanning    Status = "planning"    // LLM generating execution plan
	StatusRunning     Status = "running"     // executing steps
	StatusPaused      Status = "paused"      // paused by user, can resume
	StatusCompleted   Status = "completed"   // all steps done, artifacts ready
	StatusFailed      Status = "failed"      // execution failed
	StatusCancelled   Status = "cancelled"   // cancelled by user
	StatusInterrupted Status = "interrupted" // process crashed while running, recoverable
)

// StepStatus is the state of a single step within a task.
type StepStatus string

const (
	StepPending  StepStatus = "pending"
	StepRunning  StepStatus = "running"
	StepDone     StepStatus = "done"
	StepFailed   StepStatus = "failed"
	StepSkipped  StepStatus = "skipped"
	StepRetrying StepStatus = "retrying"
)

const DefaultMaxRetries = 2

// Task is a persistent, trackable work unit.
type Task struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      Status     `json:"status"`
	Steps       []Step     `json:"steps"`
	Artifacts   []Artifact `json:"artifacts,omitempty"`
	Error       string     `json:"error,omitempty"` // failure reason
	TenantID    string     `json:"tenant_id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`

	// ── Constraints (execution guardrails) ──
	Constraints *TaskConstraints `json:"constraints,omitempty"`
}

// RiskLevel controls review behavior for a task.
type RiskLevel = risk.Level

const (
	RiskLow    RiskLevel = risk.Low    // async/sidecar review, don't block completion
	RiskMedium RiskLevel = risk.Medium // standard blocking review (default)
	RiskHigh   RiskLevel = risk.High   // blocking review + require human approval
)

// TaskConstraints defines execution guardrails for a task.
type TaskConstraints struct {
	MaxSteps        int            `json:"max_steps,omitempty"`        // 0 = use default (8)
	TimeoutSec      int            `json:"timeout_sec,omitempty"`      // 0 = no global timeout
	MaxCostUSD      float64        `json:"max_cost_usd,omitempty"`     // 0 = no cost limit
	SuccessCriteria string         `json:"success_criteria,omitempty"` // natural-language acceptance condition
	TestCommand     string         `json:"test_command,omitempty"`     // shell command to verify result (exit 0 = pass)
	Priority        string         `json:"priority,omitempty"`         // low / medium / high
	RiskLevel       RiskLevel      `json:"risk_level,omitempty"`       // low/medium/high — controls review mode
	AutoApprove     bool           `json:"auto_approve,omitempty"`     // skip human approval for medium-risk ops
	Tags            []string       `json:"tags,omitempty"`
	Extra           map[string]any `json:"extra,omitempty"` // extensible metadata
}

// Step is one unit of execution within a task.
type Step struct {
	ID         int            `json:"id"`
	Action     string         `json:"action"`     // human-readable description
	SkillName  string         `json:"skill_name"` // skill to execute (empty = LLM-only step)
	Args       map[string]any `json:"args,omitempty"`
	Status     StepStatus     `json:"status"`
	Result     string         `json:"result,omitempty"`
	Error      string         `json:"error,omitempty"`
	Input      string         `json:"input,omitempty"`       // chained from previous step result
	RetryCount int            `json:"retry_count,omitempty"` // how many retries attempted
	MaxRetries int            `json:"max_retries,omitempty"` // max allowed retries (default 2)
	GapType    string         `json:"gap_type,omitempty"`    // capability gap classification if failed
	Group      int            `json:"group,omitempty"`       // parallel group: steps with same group run concurrently
	DependsOn  []int          `json:"depends_on,omitempty"`  // task-local prerequisite step IDs
	Metadata   map[string]any `json:"metadata,omitempty"`    // extensible step provenance and planner state
	StartedAt  *time.Time     `json:"started_at,omitempty"`
	DoneAt     *time.Time     `json:"done_at,omitempty"`
}

// Artifact is a file or output produced by the task.
type Artifact struct {
	Name     string `json:"name"`
	Path     string `json:"path"` // relative path under data/tasks/{id}/
	Type     string `json:"type"` // "file", "text", "image", "code"
	Size     int64  `json:"size,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
}

// CreateRequest is the input for creating a new task.
type CreateRequest struct {
	Title       string           `json:"title"`
	Description string           `json:"description"`
	TenantID    string           `json:"-"` // injected from auth
	Constraints *TaskConstraints `json:"constraints,omitempty"`
}

// Validate checks the create request.
func (r *CreateRequest) Validate() error {
	if r.Description == "" {
		return fmt.Errorf("description is required")
	}
	return nil
}

// IsTerminal returns true if the task is in a final state.
func (t *Task) IsTerminal() bool {
	return t.Status == StatusCompleted || t.Status == StatusFailed || t.Status == StatusCancelled
}

// IsResumable returns true if the task can be resumed (paused, interrupted, or failed).
func (t *Task) IsResumable() bool {
	return t.Status == StatusPaused || t.Status == StatusInterrupted || t.Status == StatusFailed
}

// CurrentStep returns the first non-done step, or nil if all done.
func (t *Task) CurrentStep() *Step {
	for i := range t.Steps {
		if t.Steps[i].Status == StepPending || t.Steps[i].Status == StepRunning {
			return &t.Steps[i]
		}
	}
	return nil
}

// Progress returns (completed, total) step counts.
func (t *Task) Progress() (int, int) {
	done := 0
	for _, s := range t.Steps {
		if s.Status == StepDone || s.Status == StepSkipped {
			done++
		}
	}
	return done, len(t.Steps)
}

// clone returns a deep copy of the Task so callers cannot mutate internal store state.
func (t *Task) clone() *Task {
	cp := *t
	if len(t.Steps) > 0 {
		cp.Steps = make([]Step, len(t.Steps))
		for i, s := range t.Steps {
			cp.Steps[i] = s
			if s.Args != nil {
				cp.Steps[i].Args = make(map[string]any, len(s.Args))
				for k, v := range s.Args {
					cp.Steps[i].Args[k] = v
				}
			}
			if s.DependsOn != nil {
				cp.Steps[i].DependsOn = append([]int(nil), s.DependsOn...)
			}
			if s.Metadata != nil {
				cp.Steps[i].Metadata = make(map[string]any, len(s.Metadata))
				for k, v := range s.Metadata {
					cp.Steps[i].Metadata[k] = v
				}
			}
		}
	}
	if len(t.Artifacts) > 0 {
		cp.Artifacts = make([]Artifact, len(t.Artifacts))
		copy(cp.Artifacts, t.Artifacts)
	}
	if t.StartedAt != nil {
		sa := *t.StartedAt
		cp.StartedAt = &sa
	}
	if t.FinishedAt != nil {
		fa := *t.FinishedAt
		cp.FinishedAt = &fa
	}
	return &cp
}
