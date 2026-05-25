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

// CogniContextFunc returns the assembled Cogni context block for the current
// turn — empty string when no cogni activates. Implementations are expected
// to be cheap (just rule evaluation + string concat); heavy work belongs in
// the underlying Registry or its hook.
type CogniContextFunc func(ctx context.Context, message, tenantID, channel string) string

// BeliefContextFunc returns the assembled Cognition SDK context block for the
// current turn — empty string when no pack activates.
type BeliefContextFunc func(ctx context.Context, message, tenantID, channel string) string

// CogniSkillFilterFunc narrows the candidate skill list to the union of
// every activated cogni's ToolSurface. Implementations MUST return the
// input unchanged when no cogni activates — the planner relies on this
// "no-op default" so disabling cogni keeps existing behavior intact.
type CogniSkillFilterFunc func(message, tenantID, channel string, in []skills.Skill) []skills.Skill

// CogniTraceFunc returns the latest declarative Cogni decision snapshot for
// the current turn. It lets the planner surface "which Cogni activated and how
// it changed the tool surface" in the same SSE execution trace as normal
// thinking/tool events, instead of hiding it only behind admin endpoints.
type CogniTraceFunc func(message, tenantID, channel string) (CogniTraceDetail, bool)

type MemorySearchFunc func(ctx context.Context, tenantID, query string) string

type ReflectFunc func(ctx context.Context, intent, reply string) bool

// DynContextBudgetDefault signals "use the built-in default" (currently 4000 tokens).
// Callers should use this constant instead of bare 0 to express intent clearly.
const DynContextBudgetDefault = 0
