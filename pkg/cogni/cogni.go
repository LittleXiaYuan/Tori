// Package cogni defines the Cogni — the AI-cognition shell paired with a
// Capsule. A Cogni separates the "AI-layer" concerns (activation, context
// injection, tool surface, memory policy) from the "engineering-layer"
// concerns (install, run, isolate) owned by the Capsule.
//
// A Cogni is a declarative value (Declaration), not a piece of code. The
// Evaluator (evaluator.go) reads a Declaration plus a Session and produces
// an ActivationResult: "should this capsule be engaged for this turn? which
// tools should we expose? what context should we inject?".
//
// This separation means:
//   - Capsule authors write a JSON/YAML Declaration; they don't write
//     `ShouldHandle`/`DynamicContext` hooks in Go.
//   - The host can apply cross-capsule policies (e.g. "at most 3 cognis
//     activated per turn", "prioritize by tenant preference") without
//     modifying any capsule.
//   - UIs can visualize & edit activation rules without reading source code.
//
// Status (2026-04): this package is consumed by the running agent through
// cmd/agent/module_cogni.go. The host uses declarations for activation,
// context injection, tool surface filtering, traces, verification, workflows,
// experience, evolution, federation administration, and per-cogni economics.
// pkg/capsule remains the longer-term unification layer for install/runtime
// isolation.
package cogni

import (
	"regexp"
	"strings"
)

// Declaration is the serializable Cogni configuration. It lives either
// inside a Capsule (Exports().Cogni) or on disk (JSON/YAML).
type Declaration struct {
	// ID uniquely identifies this Cogni.
	ID string `json:"id"`

	// DisplayName is the user-facing label.
	DisplayName string `json:"display_name,omitempty"`

	// Description is a short summary of what the Cogni activates.
	Description string `json:"description,omitempty"`

	// Capsule is the Capsule ID this Cogni binds to. Empty means the Cogni
	// is free-standing (e.g. a cross-capsule routing policy).
	Capsule string `json:"capsule,omitempty"`

	// Activation declares when this Cogni engages.
	Activation ActivationRules `json:"activation,omitempty"`

	// Surface declares what tools/capabilities get exposed once activated.
	Surface ToolSurface `json:"surface,omitempty"`

	// Context declares what supplemental text is injected into the planner
	// system prompt when activated.
	Context ContextInjection `json:"context,omitempty"`

	// MCP declares per-Cogni MCP server connections and tool filters.
	MCP MCPConfig `json:"mcp,omitempty"`

	// Workflows declares multi-step workflows this Cogni can execute.
	Workflows []WorkflowDef `json:"workflows,omitempty"`

	// Experience declares the experience engine configuration.
	Experience ExperienceConfig `json:"experience,omitempty"`

	// Economics declares resource constraints for this Cogni.
	Economics EconomicsConfig `json:"economics,omitempty"`

	// Memory declares how this Cogni transforms memory extraction.
	Memory MemoryPolicy `json:"memory,omitempty"`

	// Priority is used to tie-break multiple activated Cognis
	// (lower number = higher priority). Default 100.
	Priority int `json:"priority,omitempty"`

	// Exclusive declares that when this Cogni activates, no other Cogni
	// with a matching Exclusive group may be active in the same turn.
	Exclusive string `json:"exclusive,omitempty"`

	// Checks are optional self-tests used by cogni.VerifyDeclaration (and
	// by Registry reload) to confirm that activation rules behave as the
	// author intended. Think of each check as a one-line unit test — it
	// supplies an input Message/Tenant/Channel and asserts on the
	// resulting Activation. This is how "CI for declarative agents"
	// enters the codebase: bad configs can no longer silently land in
	// production, they fail the reload path and surface as alerts.
	Checks []ActivationCheck `json:"checks,omitempty"`
}

// ActivationCheck is one declarative self-test for an activation rule.
// At least one of ExpectActive / ExpectScoreAtLeast / ExpectReasonContains
// must be set; unset fields are ignored so authors can assert narrowly.
type ActivationCheck struct {
	// Name is a human-readable label surfaced in failure reports.
	Name string `json:"name,omitempty"`

	// Message is the user-visible text sent to the evaluator.
	Message string `json:"message"`

	// Tenant / Channel narrow the Session the check runs in.
	Tenant  string `json:"tenant,omitempty"`
	Channel string `json:"channel,omitempty"`

	// PriorHandover simulates tags emitted by earlier turns in the same
	// conversation.
	PriorHandover []string `json:"prior_handover,omitempty"`

	// ExpectActive asserts the post-exclusivity Activated flag.
	// Use a pointer so the zero value is distinguishable from "unset".
	ExpectActive *bool `json:"expect_active,omitempty"`

	// ExpectScoreAtLeast asserts Activation.Score >= value.
	ExpectScoreAtLeast float64 `json:"expect_score_at_least,omitempty"`

	// ExpectReasonContains asserts that at least one reason string
	// contains every substring in this list (case-sensitive).
	ExpectReasonContains []string `json:"expect_reason_contains,omitempty"`
}

// ActivationRules describes the declarative activation condition.
// The final activation score is the sum of all matching sub-rules,
// clamped to [0, 1]. Any score ≥ MinScore triggers activation.
type ActivationRules struct {
	// MinScore is the threshold for activation (default 0.5).
	MinScore float64 `json:"min_score,omitempty"`

	// Keywords are simple substring matches on the user message (case-insensitive).
	// Each hit contributes `KeywordWeight` to the score. Default weight 0.3.
	Keywords       []string `json:"keywords,omitempty"`
	KeywordWeight  float64  `json:"keyword_weight,omitempty"`

	// Regex patterns that match against the user message (case-insensitive
	// unless the pattern embeds its own flags). Each match contributes
	// `RegexWeight` (default 0.5).
	Regex       []string `json:"regex,omitempty"`
	RegexWeight float64  `json:"regex_weight,omitempty"`

	// Channels restricts activation to specific channel types (e.g. "webchat",
	// "telegram"). Empty = any channel.
	Channels []string `json:"channels,omitempty"`

	// Tenants restricts activation to specific tenant IDs. Empty = any tenant.
	Tenants []string `json:"tenants,omitempty"`

	// AlwaysOn bypasses all other rules — the Cogni is always activated.
	// Intended for infrastructure Cognis (guardrails, persona, logging).
	AlwaysOn bool `json:"always_on,omitempty"`

	// HandoverOn declares activation when a previous Cogni hands off control
	// by emitting one of these tags via its Handle result.
	HandoverOn []string `json:"handover_on,omitempty"`

	// Perception declares multi-modal activation signals beyond keywords/regex.
	Perception []PerceptionRule `json:"perception,omitempty"`

	// Semantic enables embedding-based activation: the user message is matched
	// against Examples by cosine similarity, adding to the keyword/regex score.
	// Requires an embedder wired into the Hook (SetEmbedder); inert otherwise,
	// so existing keyword-only Cognis are unaffected.
	Semantic *SemanticActivation `json:"semantic,omitempty"`
}

// SemanticActivation declares embedding-based activation signals. The example
// phrases are embedded once (centroid, cached) and compared to the user message
// vector by cosine similarity. The contribution to the activation score is
//
//	Weight * max(0, (sim - Floor) / (1 - Floor))
//
// so only a meaningfully-similar message contributes, and a strong match can
// cross MinScore on its own — making activation robust to paraphrases that the
// literal keyword list would miss.
type SemanticActivation struct {
	// Examples are representative user utterances for this Cogni.
	Examples []string `json:"examples,omitempty"`
	// Weight scales the semantic contribution (default 0.5).
	Weight float64 `json:"weight,omitempty"`
	// Floor is the minimum cosine similarity that contributes (default 0.55).
	Floor float64 `json:"floor,omitempty"`
}

// ToolSurface describes which skills get exposed to the planner once this
// Cogni activates. The final set is computed as:
//
//	base = host.AllSkills()
//	if Only: base = filter(base, name ∈ Only)
//	base = base - Exclude
//	base = base + Include
//	if FromCapsules: base = base ∩ capsuleSkills(FromCapsules)
//
// In other words, Only/Exclude/Include work in lockstep; FromCapsules narrows
// to a specific set of owning capsules (the usual case: a Cogni exposes only
// its own capsule's skills).
type ToolSurface struct {
	// Only restricts the surface to exactly these skill names. If non-empty,
	// Include/Exclude are applied on top.
	Only []string `json:"only,omitempty"`

	// Include adds skill names even if they would otherwise be filtered out.
	Include []string `json:"include,omitempty"`

	// Exclude removes skill names from the surface.
	Exclude []string `json:"exclude,omitempty"`

	// FromCapsules restricts the surface to skills owned by these capsules.
	FromCapsules []string `json:"from_capsules,omitempty"`

	// MaxTools caps the number of exposed tools (0 = unlimited).
	// Useful for keeping prompt size in check.
	MaxTools int `json:"max_tools,omitempty"`
}

// ContextInjection declares text to be inserted into the planner system
// prompt while this Cogni is active.
type ContextInjection struct {
	// Static is a fixed text block (Markdown supported).
	Static string `json:"static,omitempty"`

	// MemoryQuery, if set, is used to fetch recent memories and inject them
	// as a "## 相关记忆" block. The placeholder "{message}" is replaced by
	// the user message.
	MemoryQuery string `json:"memory_query,omitempty"`
	MemoryTopK  int    `json:"memory_top_k,omitempty"`

	// Template is a Go text/template string evaluated against
	// {"Message":..., "Tenant":..., "Channel":...}.
	// If both Static and Template are set, Static is used as fallback on
	// template error.
	Template string `json:"template,omitempty"`
}

// MemoryPolicy controls how facts extracted during a turn handled by this
// Cogni are stored.
type MemoryPolicy struct {
	// DropAll silently discards all extracted facts (useful for privacy-
	// sensitive capsules).
	DropAll bool `json:"drop_all,omitempty"`

	// DropKeys drops facts whose key matches one of these literals.
	DropKeys []string `json:"drop_keys,omitempty"`

	// TagAll adds these tags to every extracted fact.
	TagAll map[string]string `json:"tag_all,omitempty"`

	// Namespace scopes facts under a dedicated memory namespace so they
	// don't contaminate the global memory. Empty = global.
	Namespace string `json:"namespace,omitempty"`
}

// Validate ensures the declaration has the minimum required fields and
// reasonable values.
func (d *Declaration) Validate() error {
	if d == nil {
		return errRequired("cogni: declaration is nil")
	}
	if strings.TrimSpace(d.ID) == "" {
		return errRequired("cogni.id is required")
	}
	for _, p := range d.Activation.Regex {
		if _, err := regexp.Compile(p); err != nil {
			return errBadField("cogni.activation.regex", p, err)
		}
	}
	if d.Activation.MinScore < 0 || d.Activation.MinScore > 1 {
		return errBadField("cogni.activation.min_score", d.Activation.MinScore, nil)
	}
	return nil
}
