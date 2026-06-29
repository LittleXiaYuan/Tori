package planner

import (
	"context"
	"time"

	"yunque-agent/pkg/skills"
)

type SkillMetricsFunc func(skillName string, duration time.Duration, err error)

// SkillIndexEntry is a lightweight slug+description pair for the L2 skill index.
type SkillIndexEntry struct {
	Slug        string
	Description string
}

type SkillIndexFunc func() []SkillIndexEntry

// RuntimeGradeFunc returns a runtime-grade context block that informs the LLM
// about the current execution environment's trust and capability posture (#4).
//
// This is the "runtime self-awareness" layer — it tells the model:
//   - Which skills are available (so it doesn't hallucinate tools that don't exist)
//   - The current trust gate tier (so it knows which operations need approval)
//   - The dynamic risk level (so it can calibrate caution for high-risk actions)
//
// Empty string when no runtime grade information is available (preserves prior
// behavior — the model operates without this awareness, as before #4).
type RuntimeGradeFunc func(tenantID, channel string) string

// CogniTool is an extra, runtime-resolved tool contributed by a Cogni that
// activated for the current turn (today: the tools exposed by that Cogni's
// connected MCP servers). Unlike skills it is NOT in the global skill registry,
// so the planner injects it into the per-turn tool list and routes any matching
// tool call back through Invoke. Parameters is a JSON-schema object; Invoke
// returns the tool output as a string (already stringified by the host).
type CogniTool struct {
	Name        string
	Description string
	Parameters  map[string]any
	Invoke      func(ctx context.Context, args map[string]any) (string, error)
}

// CogniRuntime is the planner-facing runtime boundary for declarative Cogni
// activation. Implementations own declaration evaluation, context rendering,
// tool-surface filtering, MCP tool resolution, and trace snapshot conversion.
// Planner only passes request data through this interface and consumes the
// rendered outputs.
type CogniRuntime interface {
	// BuildContext assembles the unified cogni layer for the current turn.
	// scope is the coarse conversation kind ("emotional", "technical", "")
	// derived from IntentHint by prompt_builder.intentToScope. It drives the
	// merged belief scope gate (#34) inside the runtime: a belief with
	// non-empty Scopes only activates when scope matches; empty scope = global
	// beliefs only. This single BuildContext is the sole belief/cogni injection
	// path — the former parallel belief-context func was removed.
	BuildContext(ctx context.Context, message, tenantID, channel, scope string) string
	FilterSkills(message, tenantID, channel string, in []skills.Skill) []skills.Skill
	Trace(message, tenantID, channel string) (CogniTraceDetail, bool)
	// Tools returns the extra tools contributed by the cognis activated for
	// this turn — currently the tools of their connected MCP servers. Returns
	// nil when no cogni activates or none declare MCP servers. Implementations
	// must lazily connect and degrade gracefully: a slow or broken MCP server
	// must never block the turn (skip its tools instead).
	Tools(ctx context.Context, message, tenantID, channel string) []CogniTool
	// SurfaceAuthoritative reports whether the cognis activated this turn applied
	// a non-identity ToolSurface. When true the planner treats the cogni-surfaced
	// capability set (skills ∪ MCP tools) as definitive and skips its own
	// per-message intent re-ranking and tool cap, so the tool block stays a
	// deterministic, prompt-cache-friendly prefix instead of a per-message
	// (cache-busting) one. This is what lets a Cogni own tool orchestration above
	// the flat skill/MCP layer rather than merely filtering it.
	SurfaceAuthoritative(message, tenantID, channel string) bool
	// RecordToolOutcome feeds a tool execution result back to the cognis active
	// this turn so a Cogni can learn which of its surfaced tools actually work and
	// self-tune its surface over time. Implementations attribute the outcome to
	// the owning cogni(s) and record asynchronously; this must be cheap and
	// no-op when experience is not configured.
	RecordToolOutcome(message, tenantID, channel, tool string, success bool)
}

type MemorySearchFunc func(ctx context.Context, tenantID, query string) string

type ReflectFunc func(ctx context.Context, intent, reply string) bool

// DynContextBudgetDefault signals "use the built-in default" (currently 4000 tokens).
// Callers should use this constant instead of bare 0 to express intent clearly.
const DynContextBudgetDefault = 0
