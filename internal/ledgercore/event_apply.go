package ledger

import "encoding/json"

// ApplyEvent updates a Task's in-memory state based on an event.
// This is the core projection function: events → state.
//
// INV-1: S(T,t) = fold(ApplyEvent, S₀, [E₁…Eₙ])
func ApplyEvent(t *Task, e *Event) {
	var p eventPayload
	_ = json.Unmarshal(e.Payload, &p)

	switch e.Kind {
	// ── Task lifecycle ──

	case EventTaskCreated:
		t.ID = e.TaskID
		t.Status = TaskCreated
		// Mirror CreateTask defaults so replay matches the materialized row
		// even for events that predate richer payloads.
		t.AgentID = "default"
		t.Input = JSON("{}")
		t.Output = JSON("{}")
		t.Metadata = JSON("{}")
		t.MaxRetries = 2
		if p.Goal != "" {
			t.Goal = p.Goal
		}
		if p.Type != "" {
			t.Type = TaskType(p.Type)
		}
		if p.TenantID != "" {
			t.TenantID = p.TenantID
		}
		if p.AgentID != "" {
			t.AgentID = p.AgentID
		}
		if p.UserID != "" {
			t.UserID = p.UserID
		}
		if len(p.Input) > 0 {
			t.Input = JSON(p.Input)
		}
		if len(p.Metadata) > 0 {
			t.Metadata = JSON(p.Metadata)
		}
		if p.Priority != nil {
			t.Priority = *p.Priority
		}
		if p.MaxRetries != nil {
			t.MaxRetries = *p.MaxRetries
		}
		if p.ParentTaskID != "" {
			pid := p.ParentTaskID
			t.ParentTaskID = &pid
		}
		t.CreatedAt = e.CreatedAt

	case EventTaskReady:
		t.Status = TaskReady

	case EventTaskStarted:
		t.Status = TaskRunning
		// First start wins, matching the materialized-view update in
		// TaskManager.Transition (retry re-entry emits task.retry_succeeded,
		// which keeps this original StartedAt).
		if t.StartedAt == nil {
			t.StartedAt = &e.CreatedAt
		}

	case EventTaskCompleted:
		t.Status = TaskCompleted
		t.FinishedAt = &e.CreatedAt
		if len(p.Output) > 0 {
			t.Output = JSON(p.Output)
		}

	case EventTaskFailed:
		t.Status = TaskFailed
		t.FinishedAt = &e.CreatedAt
		if p.Error != "" {
			t.Error = &p.Error
		}

	case EventTaskCancelled:
		t.Status = TaskCancelled
		t.FinishedAt = &e.CreatedAt

	case EventTaskRetrying:
		t.Status = TaskRetrying
		t.RetryCount++

	case EventTaskBlocked:
		t.Status = TaskBlocked

	case EventTaskWaitingInput:
		t.Status = TaskWaitingInput

	case EventTaskInputReceived:
		t.Status = TaskRunning

	case EventTaskResumed:
		// Task is being restarted/unblocked ???go back to ready
		t.Status = TaskReady
		// Clear terminal fields
		t.Error = nil
		t.FinishedAt = nil

	case EventTaskRetrySucceeded:
		// A retry re-entered execution. StartedAt is preserved (first start
		// wins, set on the original task.started); only clear any stale error.
		t.Status = TaskRunning
		t.Error = nil

	// ── Checkpoint ──

	case EventCheckpointCreated:
		if p.CheckpointID != "" {
			t.CheckpointRef = &p.CheckpointID
		}

	default:
		// Step/reasoning/infra events never touch the materialized task row,
		// so they must not advance its version or timestamp during replay.
		return
	}

	t.UpdatedAt = e.CreatedAt
	// task.created corresponds to the INSERT (version 0); every other
	// projected kind corresponds to exactly one UpdateTask (+1). Keeping the
	// counts aligned lets replayed tasks participate in optimistic locking.
	if e.Kind != EventTaskCreated {
		t.Version++
	}
}

// eventPayload is the union of all possible payload fields.
// Fields are omitempty so only relevant fields are present per event kind.
type eventPayload struct {
	// Task fields
	Goal     string          `json:"goal,omitempty"`
	Type     string          `json:"type,omitempty"`
	TenantID string          `json:"tenant_id,omitempty"`
	AgentID  string          `json:"agent_id,omitempty"`
	UserID   string          `json:"user_id,omitempty"`
	Input    json.RawMessage `json:"input,omitempty"`
	Output   json.RawMessage `json:"output,omitempty"`
	Error    string          `json:"error,omitempty"`

	// Step fields
	StepIndex *int   `json:"step_index,omitempty"`
	Action    string `json:"action,omitempty"`
	SkillName string `json:"skill_name,omitempty"`
	Result    string `json:"result,omitempty"`
	Retryable *bool  `json:"retryable,omitempty"`

	// Reasoning trace fields
	Thought     string   `json:"thought,omitempty"`     // The reasoning text
	Observation string   `json:"observation,omitempty"` // What was observed
	Decision    string   `json:"decision,omitempty"`    // What was decided
	Alternative string   `json:"alternative,omitempty"` // Alternative considered / backtrack target
	Reason      string   `json:"reason,omitempty"`      // Why (for decisions, backtracks)
	Confidence  *float64 `json:"confidence,omitempty"`  // Confidence level [0,1]
	PlanSteps   []string `json:"plan_steps,omitempty"`  // Steps in a generated plan
	Depth       *int     `json:"depth,omitempty"`       // Reasoning depth (for nested thought trees)
	ParentStep  *int     `json:"parent_step,omitempty"` // Links to parent reasoning step

	// Creation fields (task.created)
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	Priority     *int            `json:"priority,omitempty"`
	MaxRetries   *int            `json:"max_retries,omitempty"`
	ParentTaskID string          `json:"parent_task_id,omitempty"`

	// Infrastructure
	CheckpointID string `json:"checkpoint_id,omitempty"`
	ArtifactID   string `json:"artifact_id,omitempty"`
	MemoryKey    string `json:"memory_key,omitempty"`

	// Timing
	DurationMs int64 `json:"duration_ms,omitempty"`
}

// MakePayload is defined in types.go
