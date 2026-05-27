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

// BeliefContextFunc returns the assembled Cognition SDK context block for the
// current turn — empty string when no pack activates.
type BeliefContextFunc func(ctx context.Context, message, tenantID, channel string) string

// CogniRuntime is the planner-facing runtime boundary for declarative Cogni
// activation. Implementations own declaration evaluation, context rendering,
// tool-surface filtering, and trace snapshot conversion. Planner only passes
// request data through this interface and consumes the rendered outputs.
type CogniRuntime interface {
	BuildContext(ctx context.Context, message, tenantID, channel string) string
	FilterSkills(message, tenantID, channel string, in []skills.Skill) []skills.Skill
	Trace(message, tenantID, channel string) (CogniTraceDetail, bool)
}

type MemorySearchFunc func(ctx context.Context, tenantID, query string) string

type ReflectFunc func(ctx context.Context, intent, reply string) bool

// DynContextBudgetDefault signals "use the built-in default" (currently 4000 tokens).
// Callers should use this constant instead of bare 0 to express intent clearly.
const DynContextBudgetDefault = 0
