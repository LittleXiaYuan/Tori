package planner

import (
	"context"
	"fmt"
	"strings"

	"yunque-agent/internal/agentcore/emotion"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/observe"
)

// PlanRequest is the input to the planner.
type PlanRequest struct {
	Messages          []llm.Message
	ClassID           string
	TeacherID         string
	StudentID         string
	TenantID          string
	ModelOverride     string          // pool key (e.g. "fast","smart","expert") — EXPLICIT caller override; suppresses LocalBrain classification
	RoutedTier        string          // pool key chosen by the gateway smart router / thinking level — AUTOMATIC routing hint; lower precedence than ModelOverride and does NOT suppress classification (so the tool-free fast path can still fire)
	EmotionHint       *emotion.Result // optional emotion detected from user input (STT or text analysis)
	TaskID            string          // if set, this request is part of a task thread
	TaskContext       string          // pre-rendered task working memory (injected by gateway)
	IsGroup           bool            // true if this request comes from a group chat
	GroupSystemPrompt string          // extra system prompt for group context
	ChannelType       string          // source channel type (e.g. "telegram", "feishu")
	ChatType          string          // chat type ("group", "private", etc.)
	InboxContext      string          // buffered group inbox messages for context
	StepCallback      StepCallback    // optional: called for each intermediate step (thinking, tool call, etc.)
	OnReplyDelta      func(string)    // optional: called per-chunk to stream the final answer text live (true token streaming)
	TraceID           string          // trace context ID for unified event protocol
	ThinkingEnabled   *bool           // nil = model default; true/false = explicit override
	DisableDelegation bool            // when true, buildFunctionDefs exposes direct skills instead of handoff tools
	DisableTools      bool            // when true, skip all tools — pure chat mode
	ClientOverride    *llm.Client     // if set, bypass pool and use this client directly (session-level provider override)
	AllowedSkills     []string        // if non-empty, buildFunctionDefs restricts to exactly these skill names (user-picked tool whitelist)
	WorkspacePaths    []string        // extra host dirs the conversation opened; merged into read-only file skills' allowed roots
}

// EffectiveModelTier resolves the pool key to use for model selection: an
// explicit ModelOverride wins; otherwise the gateway's auto-routed RoutedTier.
// Tier-resolution call sites use this instead of ModelOverride directly so the
// smart-router tier still applies even though it no longer suppresses LocalBrain
// chat-vs-tools classification.
func (r PlanRequest) EffectiveModelTier() string {
	if r.ModelOverride != "" {
		return r.ModelOverride
	}
	return r.RoutedTier
}

// StepEventType classifies the kind of intermediate step event.
type StepEventType string

const (
	StepEventThinking   StepEventType = "thinking"    // agent is reasoning
	StepEventToolStart  StepEventType = "tool_start"  // about to call a skill
	StepEventToolResult StepEventType = "tool_result" // skill returned
	StepEventReflect    StepEventType = "reflect"     // self-reflection
	StepEventPlan       StepEventType = "plan"        // decomposed plan
)

// StepEvent is an intermediate step notification during planning.
type StepEvent struct {
	Type      StepEventType  `json:"type"`
	Step      int            `json:"step"`
	Message   string         `json:"message"` // human-readable description
	SkillName string         `json:"skill_name,omitempty"`
	Args      map[string]any `json:"args,omitempty"`
	Result    string         `json:"result,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// StepCallback is called for each intermediate step during planning.
// Uses the unified AgentEvent protocol from observe package.
// If nil, no intermediate notifications are sent.
type StepCallback func(event observe.AgentEvent)

type ctxKeyStepCB struct{}

// WithStepCallback attaches a StepCallback to the context so exec agents can emit SSE events.
func WithStepCallback(ctx context.Context, cb StepCallback) context.Context {
	return context.WithValue(ctx, ctxKeyStepCB{}, cb)
}

// StepCallbackFromCtx retrieves the StepCallback attached to the context.
func StepCallbackFromCtx(ctx context.Context) StepCallback {
	cb, _ := ctx.Value(ctxKeyStepCB{}).(StepCallback)
	return cb
}

// StepStatus tracks the state of a plan step.
type StepStatus string

const (
	StepPending StepStatus = "pending"
	StepRunning StepStatus = "running"
	StepDone    StepStatus = "done"
	StepFailed  StepStatus = "failed"
	StepSkipped StepStatus = "skipped"
)

// PlanStep represents one step in a multi-step plan.
type PlanStep struct {
	ID        int            `json:"id"`
	Action    string         `json:"action"` // what to do
	Skill     string         `json:"skill"`  // skill to call (empty = LLM reasoning)
	Args      map[string]any `json:"args,omitempty"`
	DependsOn []int          `json:"depends_on"` // IDs of steps this depends on
	Status    StepStatus     `json:"status"`
	Result    string         `json:"result,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// PlanResult is the output of the planner.
type PlanResult struct {
	Reply            string        `json:"reply"`
	ReasoningContent string        `json:"reasoning_content,omitempty"`
	Actions          []AgentAction `json:"actions,omitempty"`
	SkillsUsed       []string      `json:"skills_used"`
	Steps            int           `json:"steps"`
	Plan             []PlanStep    `json:"plan,omitempty"`
	ContextLayers    []string      `json:"context_layers,omitempty"`
	Suggestions      []string      `json:"suggestions,omitempty"`
}

// ExecutionSummary builds a concise summary of skill executions for session persistence.
// This allows the next conversation turn to see what tools were called and their results,
// enabling multi-turn task continuity.
// Returns empty string if no skills were used.
func (r *PlanResult) ExecutionSummary() string {
	if r == nil || len(r.Plan) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("[执行记录] ")
	for i, step := range r.Plan {
		if i > 0 {
			b.WriteString(" → ")
		}
		if step.Status == StepFailed {
			b.WriteString(fmt.Sprintf("%s(失败: %s)", step.Skill, truncate(step.Error, 80)))
		} else {
			b.WriteString(fmt.Sprintf("%s(✓ %s)", step.Skill, truncate(step.Result, 120)))
		}
	}
	return b.String()
}
