package workflow

import (
	"time"
)

// ──────────────────────────────────────────────
// Workflow Engine — DAG-based deterministic execution
//
// Unlike the Planner (AI-driven autonomous planning), the Workflow Engine
// executes human-defined Directed Acyclic Graphs (DAGs). Each node is a
// concrete action (skill call, LLM prompt, conditional branch, or sub-workflow).
//
// Key differentiator: Workflows are reproducible and auditable.
// ──────────────────────────────────────────────

// ── Node types ──

// NodeType identifies what kind of action a node performs.
type NodeType string

const (
	NodeSkill     NodeType = "skill"     // Execute a registered skill
	NodeLLM       NodeType = "llm"       // Free-form LLM call with a prompt template
	NodeCondition NodeType = "condition" // Branch based on expression evaluation
	NodeParallel  NodeType = "parallel"  // Fan-out to multiple branches
	NodeJoin      NodeType = "join"      // Fan-in: wait for all incoming edges
	NodeSubflow   NodeType = "subflow"   // Invoke another workflow definition
	NodeInput     NodeType = "input"     // Wait for user/external input
	NodeTransform NodeType = "transform" // Transform data (template / jq / JSONPath)
	NodeBrowser   NodeType = "browser"   // Browser automation action
	NodeCode      NodeType = "code"      // Execute sandboxed code
	NodeKnowledge NodeType = "knowledge" // Knowledge base retrieval (RAG)
	NodeStart     NodeType = "start"     // Workflow entry point (pass-through)
	NodeEnd       NodeType = "end"       // Workflow exit point (pass-through)
)

// Node is a single step in a workflow DAG.
type Node struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Type        NodeType       `json:"type"`
	Config      map[string]any `json:"config,omitempty"`  // type-specific config
	Position    Position       `json:"position"`          // visual editor coordinates
	Timeout     *Duration      `json:"timeout,omitempty"` // per-node timeout
	RetryPolicy *RetryPolicy   `json:"retry_policy,omitempty"`
}

// Edge connects two nodes in the DAG.
type Edge struct {
	ID        string `json:"id"`
	FromNode  string `json:"from_node"`
	ToNode    string `json:"to_node"`
	Condition string `json:"condition,omitempty"` // expression for conditional edges
	Label     string `json:"label,omitempty"`     // display label in editor
}

// Position holds visual editor coordinates.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Duration wraps time.Duration for JSON marshaling.
type Duration struct {
	time.Duration
}

// MarshalJSON encodes duration as a string like "30s", "5m".
func (d Duration) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.Duration.String() + `"`), nil
}

// UnmarshalJSON decodes a duration string.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if len(b) > 2 && b[0] == '"' {
		s = string(b[1 : len(b)-1])
	} else {
		s = string(b)
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = dur
	return nil
}

// RetryPolicy defines retry behavior for a node.
type RetryPolicy struct {
	MaxRetries int     `json:"max_retries"` // 0 = no retry
	BackoffMs  int     `json:"backoff_ms"`  // base backoff in ms
	Multiplier float64 `json:"multiplier"`  // backoff multiplier (default 2.0)
}

// ── Workflow Definition ──

// Definition is a complete workflow template.
type Definition struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Version     int        `json:"version"`
	Nodes       []Node     `json:"nodes"`
	Edges       []Edge     `json:"edges"`
	Variables   []Variable `json:"variables,omitempty"` // input/output schema
	TenantID    string     `json:"tenant_id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Variable defines an input or output parameter of a workflow.
type Variable struct {
	Name         string `json:"name"`
	Type         string `json:"type"` // "string", "number", "boolean", "json"
	Required     bool   `json:"required"`
	DefaultValue any    `json:"default_value,omitempty"`
	Description  string `json:"description,omitempty"`
}

// ── Workflow Instance (execution) ──

// InstanceStatus is the state of a running workflow instance.
type InstanceStatus string

const (
	InstancePending   InstanceStatus = "pending"
	InstanceRunning   InstanceStatus = "running"
	InstancePaused    InstanceStatus = "paused"
	InstanceCompleted InstanceStatus = "completed"
	InstanceFailed    InstanceStatus = "failed"
	InstanceCancelled InstanceStatus = "cancelled"
)

// Instance is a single execution of a workflow definition.
type Instance struct {
	ID           string                `json:"id"`
	DefinitionID string                `json:"definition_id"`
	Version      int                   `json:"version"` // definition version at start
	Status       InstanceStatus        `json:"status"`
	Variables    map[string]any        `json:"variables"` // runtime variable state
	NodeStates   map[string]*NodeState `json:"node_states"`
	Error        string                `json:"error,omitempty"`
	TenantID     string                `json:"tenant_id"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
	StartedAt    *time.Time            `json:"started_at,omitempty"`
	FinishedAt   *time.Time            `json:"finished_at,omitempty"`
}

// NodeState tracks the execution state of a single node within an instance.
type NodeState struct {
	NodeID     string     `json:"node_id"`
	Status     NodeStatus `json:"status"`
	Input      any        `json:"input,omitempty"`
	Output     any        `json:"output,omitempty"`
	Error      string     `json:"error,omitempty"`
	RetryCount int        `json:"retry_count"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// NodeStatus is the execution state of a node.
type NodeStatus string

const (
	NodePending NodeStatus = "pending"
	NodeRunning NodeStatus = "running"
	NodeDone    NodeStatus = "done"
	NodeFailed  NodeStatus = "failed"
	NodeSkipped NodeStatus = "skipped"
	NodeWaiting NodeStatus = "waiting" // waiting for input or join
)
