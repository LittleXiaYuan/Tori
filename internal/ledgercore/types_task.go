package ledger

import (
	"encoding/json"
	"time"
)

// ──────────────────────────────────────────────
// Task
// ──────────────────────────────────────────────

// TaskType classifies the execution model of a task.
type TaskType string

const (
	TaskTypeChat     TaskType = "chat"     // Single-turn conversation (lightest)
	TaskTypeGoal     TaskType = "goal"     // Goal-driven multi-step task
	TaskTypeWorkflow TaskType = "workflow" // Pre-defined workflow instance
	TaskTypeDaemon   TaskType = "daemon"   // Long-running background task
)

// TaskStatus represents the lifecycle state of a task.
type TaskStatus string

const (
	TaskCreated      TaskStatus = "created"
	TaskReady        TaskStatus = "ready"
	TaskRunning      TaskStatus = "running"
	TaskWaitingInput TaskStatus = "waiting_input"
	TaskBlocked      TaskStatus = "blocked"
	TaskRetrying     TaskStatus = "retrying"
	TaskCancelled    TaskStatus = "cancelled"
	TaskCompleted    TaskStatus = "completed"
	TaskFailed       TaskStatus = "failed"
)

// IsTerminal returns true if the status is a final state.
func (s TaskStatus) IsTerminal() bool {
	return s == TaskCompleted || s == TaskFailed || s == TaskCancelled
}

// Task is the primary execution unit managed by Ledger.
// Each task has an independent context, state, and result.
type Task struct {
	ID            string     `json:"id"              db:"id"`
	Type          TaskType   `json:"type"            db:"type"`
	Goal          string     `json:"goal"            db:"goal"`
	Status        TaskStatus `json:"status"          db:"status"`
	TenantID      string     `json:"tenant_id"       db:"tenant_id"`
	AgentID       string     `json:"agent_id"        db:"agent_id"`
	UserID        string     `json:"user_id"         db:"user_id"`
	ParentTaskID  *string    `json:"parent_task_id"  db:"parent_task_id"`
	Input         JSON       `json:"input"           db:"input"`
	Output        JSON       `json:"output"          db:"output"`
	Error         *string    `json:"error"           db:"error"`
	RetryCount    int        `json:"retry_count"     db:"retry_count"`
	MaxRetries    int        `json:"max_retries"     db:"max_retries"`
	CheckpointRef *string    `json:"checkpoint_ref"  db:"checkpoint_ref"`
	Priority      int        `json:"priority"        db:"priority"`
	Metadata      JSON       `json:"metadata"        db:"metadata"`
	CreatedAt     time.Time  `json:"created_at"      db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"      db:"updated_at"`
	StartedAt     *time.Time `json:"started_at"      db:"started_at"`
	FinishedAt    *time.Time `json:"finished_at"     db:"finished_at"`
	Version       int64      `json:"version"         db:"version"` // Optimistic lock
}

// JSON is a flexible container for structured data stored as raw JSON bytes.
type JSON = json.RawMessage

// TaskFilter specifies criteria for listing tasks.
type TaskFilter struct {
	TenantID     string       `json:"tenant_id,omitempty"`
	Status       []TaskStatus `json:"status,omitempty"`
	Type         *TaskType    `json:"type,omitempty"`
	ParentTaskID *string      `json:"parent_task_id,omitempty"`
	Limit        int          `json:"limit,omitempty"`
	Offset       int          `json:"offset,omitempty"`
}

// ──────────────────────────────────────────────
// Checkpoint
// ──────────────────────────────────────────────

// Checkpoint is a snapshot of task execution state for crash recovery.
type Checkpoint struct {
	ID         string    `json:"id"          db:"id"`
	TaskID     string    `json:"task_id"     db:"task_id"`
	EventSeq   int64     `json:"event_seq"   db:"event_seq"`
	StepIndex  int       `json:"step_index"  db:"step_index"`
	TaskState  JSON      `json:"task_state"  db:"task_state"`
	WorkingMem JSON      `json:"working_mem" db:"working_mem"`
	SizeBytes  int64     `json:"size_bytes"  db:"size_bytes"`
	Reason     string    `json:"reason"      db:"reason"` // "step_complete" | "pre_wait" | "pre_retry" | "periodic" | "manual"
	CreatedAt  time.Time `json:"created_at"  db:"created_at"`
}

// ──────────────────────────────────────────────
// TaskDependency
// ──────────────────────────────────────────────

// DepKind classifies the dependency relationship between tasks.
type DepKind string

const (
	DepBlocking DepKind = "blocking" // B waits for A to complete
	DepData     DepKind = "data"     // B needs an artifact from A
	DepWeak     DepKind = "weak"     // Soft dependency: A's result can improve B
)

// TaskDependency represents an explicit dependency between two tasks.
type TaskDependency struct {
	ID          string    `json:"id"            db:"id"`
	FromTaskID  string    `json:"from_task_id"  db:"from_task_id"`
	ToTaskID    string    `json:"to_task_id"    db:"to_task_id"`
	Kind        DepKind   `json:"kind"          db:"kind"`
	ArtifactRef *string   `json:"artifact_ref"  db:"artifact_ref"`
	Satisfied   bool      `json:"satisfied"     db:"satisfied"`
	CreatedAt   time.Time `json:"created_at"    db:"created_at"`
}

// ──────────────────────────────────────────────
// Artifact
// ──────────────────────────────────────────────

// Artifact represents a file or output produced by a task.
type Artifact struct {
	ID         string    `json:"id"          db:"id"`
	TaskID     string    `json:"task_id"     db:"task_id"`
	Name       string    `json:"name"        db:"name"`
	Kind       string    `json:"kind"        db:"kind"` // "file" | "text" | "code" | "image" | "data"
	MimeType   string    `json:"mime_type"   db:"mime_type"`
	SizeBytes  int64     `json:"size_bytes"  db:"size_bytes"`
	StorageRef string    `json:"storage_ref" db:"storage_ref"`
	Checksum   string    `json:"checksum"    db:"checksum"` // SHA256
	Metadata   JSON      `json:"metadata"    db:"metadata"`
	CreatedAt  time.Time `json:"created_at"  db:"created_at"`
}
