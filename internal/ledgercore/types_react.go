package ledger

import (
	"context"
	"time"
)

// ──────────────────────────────────────────────
// ReAct Protocol Types
// ──────────────────────────────────────────────

// ToolCall represents a tool/skill invocation requested by the reasoning engine.
type ToolCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args,omitempty"`
}

// ThinkResult is the structured output of one reasoning step.
type ThinkResult struct {
	Thought    string    // The LLM's chain-of-thought
	Action     *ToolCall // Tool to call (nil = reasoning is done)
	Answer     string    // Final answer (populated when Action is nil)
	Confidence float64   // [0,1] confidence in current direction
}

// ToolResult represents the output of executing a tool.
type ToolResult struct {
	Output string
	Error  string
}

// ReActStep represents one complete Observe→Think→Act cycle.
type ReActStep struct {
	StepNum     int         `json:"step_num"`
	Observation string      `json:"observation"`
	Thought     string      `json:"thought"`
	Action      *ToolCall   `json:"action,omitempty"`
	Result      *ToolResult `json:"result,omitempty"`
	Confidence  float64     `json:"confidence"`
	DurationMs  int64       `json:"duration_ms"`
}

// ReActConfig configures the ReAct loop behavior.
type ReActConfig struct {
	MaxSteps        int     // Maximum reasoning steps before forced stop (default 10)
	MinConfidence   float64 // Below this, emit a reflection event (default 0.3)
	BacktrackOnFail bool    // If true, failed tool calls trigger backtrack + rethink
	Actor           string  // Actor name for reasoning trace (default "react")
}

// Defaults fills zero-valued fields with sensible defaults.
func (c *ReActConfig) Defaults() {
	if c.MaxSteps <= 0 {
		c.MaxSteps = 10
	}
	if c.MinConfidence <= 0 {
		c.MinConfidence = 0.3
	}
	if c.Actor == "" {
		c.Actor = "react"
	}
}

// ReActResult is the final output of the ReAct loop.
type ReActResult struct {
	Answer     string      `json:"answer"`
	Steps      []ReActStep `json:"steps"`
	TotalSteps int         `json:"total_steps"`
	Backtracks int         `json:"backtracks"`
	Success    bool        `json:"success"`
	StopReason string      `json:"stop_reason"` // "answer" | "max_steps" | "error" | "cancelled"
}

// ThinkFunc is the LLM reasoning function. Given the step history,
// it produces the next thought and optional action.
type ThinkFunc func(ctx context.Context, history []ReActStep) (*ThinkResult, error)

// ActFunc executes a tool call and returns the result.
type ActFunc func(ctx context.Context, call ToolCall) (*ToolResult, error)

// ReActOnStep is an optional callback invoked after each step.
type ReActOnStep func(step ReActStep)

// ──────────────────────────────────────────────
// Plan-Execute-Reflect Protocol Types
// ──────────────────────────────────────────────

// PlanExecuteReflectConfig configures the Plan→Execute→Reflect loop.
type PlanExecuteReflectConfig struct {
	MaxAttempts int         // Max plan→execute→reflect cycles (default 3)
	ReActConfig ReActConfig // Config for the execution ReAct loop
	Actor       string      // Actor name for trace (default "per")
	AutoLearn   bool        // If true, write reflection as Memory experience
	TenantID    string      // Required when AutoLearn is true
}

// Defaults fills zero-valued fields with sensible defaults.
func (c *PlanExecuteReflectConfig) Defaults() {
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 3
	}
	if c.Actor == "" {
		c.Actor = "per"
	}
	c.ReActConfig.Defaults()
}

// PlanFunc generates a multi-step plan from the goal and context.
type PlanFunc func(ctx context.Context, goal string, attempt int, prevReflection string) ([]string, error)

// ReflectFunc evaluates execution results and returns structured reflection.
type ReflectFunc func(ctx context.Context, goal string, plan []string, execResult *ReActResult) (*Reflection, error)

// Reflection is the structured output of a reflection step.
type Reflection struct {
	Satisfied  bool     `json:"satisfied"`
	Score      float64  `json:"score"`
	Strengths  []string `json:"strengths"`
	Weaknesses []string `json:"weaknesses"`
	Suggestion string   `json:"suggestion"`
	Learnings  []string `json:"learnings"`
}

// PERResult is the output of the Plan-Execute-Reflect loop.
type PERResult struct {
	FinalAnswer string         `json:"final_answer"`
	Attempts    int            `json:"attempts"`
	Plans       [][]string     `json:"plans"`
	Reflections []Reflection   `json:"reflections"`
	ExecResults []*ReActResult `json:"exec_results"`
	Success     bool           `json:"success"`
	TotalSteps  int            `json:"total_steps"`
}

// ExperienceEntry represents a learned experience from task execution.
type ExperienceEntry struct {
	TaskID    string    `json:"task_id"`
	Goal      string    `json:"goal"`
	Outcome   string    `json:"outcome"` // "success" | "partial" | "failure"
	Learnings []string  `json:"learnings"`
	Score     float64   `json:"score"`
	Attempts  int       `json:"attempts"`
	CreatedAt time.Time `json:"created_at"`
}
