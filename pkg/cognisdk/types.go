package cognisdk

import "time"

// Config wires the experimental cognition SDK for a host process.
// When Packs is empty, NewEngine uses the local built-in packs.
type Config struct {
	Packs        []PackManifest
	EnabledPacks []string
}

// Input is the per-turn data the host passes into the SDK.
// The SDK treats it as observation only: it never executes tools or writes state.
type Input struct {
	Message             string
	UserID              string
	Channel             string
	Hints               map[string]string
	RequestedToolAction *ToolAction
}

// ToolAction describes an action the host or planner is considering.
type ToolAction struct {
	Name        string    `json:"name" yaml:"name"`
	Kind        string    `json:"kind" yaml:"kind"`
	Risk        RiskLevel `json:"risk" yaml:"risk"`
	Description string    `json:"description,omitempty" yaml:"description,omitempty"`
}

// RiskLevel is the SDK's coarse risk vocabulary.
type RiskLevel string

const (
	RiskLow        RiskLevel = "low"
	RiskMedium     RiskLevel = "medium"
	RiskHigh       RiskLevel = "high"
	RiskDependency RiskLevel = "dependency"
)

// PerceptionState is the SDK's compact interpretation of one turn.
type PerceptionState struct {
	Message             string            `json:"message" yaml:"message"`
	Intent              string            `json:"intent" yaml:"intent"`
	Risk                RiskLevel         `json:"risk" yaml:"risk"`
	Signals             []string          `json:"signals,omitempty" yaml:"signals,omitempty"`
	Hints               map[string]string `json:"hints,omitempty" yaml:"hints,omitempty"`
	RequestedToolAction *ToolAction       `json:"requested_tool_action,omitempty" yaml:"requested_tool_action,omitempty"`
}

// BeliefKind separates durable values from softer contextual preferences.
type BeliefKind string

const (
	BeliefRoot       BeliefKind = "root"
	BeliefValue      BeliefKind = "value"
	BeliefRelational BeliefKind = "relational"
	BeliefBoundary   BeliefKind = "boundary"
	BeliefPreference BeliefKind = "preference"
)

// BeliefNode is a local seed or activated belief.
type BeliefNode struct {
	ID         string     `json:"id" yaml:"id"`
	Kind       BeliefKind `json:"kind" yaml:"kind"`
	Statement  string     `json:"statement" yaml:"statement"`
	Confidence float64    `json:"confidence,omitempty" yaml:"confidence,omitempty"`
	SourcePack string     `json:"source_pack,omitempty" yaml:"source_pack,omitempty"`
	ReadOnly   bool       `json:"read_only,omitempty" yaml:"read_only,omitempty"`
}

// InnerState is what the host can inject into a later prompt/planner context.
type InnerState struct {
	Intent        string       `json:"intent" yaml:"intent"`
	Risk          RiskLevel    `json:"risk" yaml:"risk"`
	Summary       string       `json:"summary" yaml:"summary"`
	ActiveBeliefs []BeliefNode `json:"active_beliefs,omitempty" yaml:"active_beliefs,omitempty"`
	ActivePacks   []string     `json:"active_packs,omitempty" yaml:"active_packs,omitempty"`
}

// ToolPolicy is the SDK's recommendation for a considered tool action.
type ToolPolicy string

const (
	ToolPolicyAllow               ToolPolicy = "allow"
	ToolPolicyRequireConfirmation ToolPolicy = "require_confirmation"
)

// ResponseDisposition describes how the response should be shaped.
type ResponseDisposition struct {
	Mode       string     `json:"mode" yaml:"mode"`
	Tone       string     `json:"tone" yaml:"tone"`
	Priority   int        `json:"priority" yaml:"priority"`
	MustSay    []string   `json:"must_say,omitempty" yaml:"must_say,omitempty"`
	MustAvoid  []string   `json:"must_avoid,omitempty" yaml:"must_avoid,omitempty"`
	ToolPolicy ToolPolicy `json:"tool_policy,omitempty" yaml:"tool_policy,omitempty"`
	Reasons    []string   `json:"reasons,omitempty" yaml:"reasons,omitempty"`
}

// RuleCondition selects a disposition rule.
type RuleCondition struct {
	Intent             string    `json:"intent,omitempty" yaml:"intent,omitempty"`
	Risk               RiskLevel `json:"risk,omitempty" yaml:"risk,omitempty"`
	ToolRiskAtLeast    RiskLevel `json:"tool_risk_at_least,omitempty" yaml:"tool_risk_at_least,omitempty"`
	MessageContainsAny []string  `json:"message_contains_any,omitempty" yaml:"message_contains_any,omitempty"`
}

// DispositionRule is a declarative response-shaping rule.
type DispositionRule struct {
	ID         string        `json:"id" yaml:"id"`
	When       RuleCondition `json:"when" yaml:"when"`
	Mode       string        `json:"mode" yaml:"mode"`
	Tone       string        `json:"tone,omitempty" yaml:"tone,omitempty"`
	Priority   int           `json:"priority,omitempty" yaml:"priority,omitempty"`
	MustSay    []string      `json:"must_say,omitempty" yaml:"must_say,omitempty"`
	MustAvoid  []string      `json:"must_avoid,omitempty" yaml:"must_avoid,omitempty"`
	ToolPolicy ToolPolicy    `json:"tool_policy,omitempty" yaml:"tool_policy,omitempty"`
	TemplateID string        `json:"template_id,omitempty" yaml:"template_id,omitempty"`
	SourcePack string        `json:"source_pack,omitempty" yaml:"source_pack,omitempty"`
}

// BoundaryPolicy declares global safety/relationship constraints from packs.
type BoundaryPolicy struct {
	MustSay         []string   `json:"must_say,omitempty" yaml:"must_say,omitempty"`
	MustAvoid       []string   `json:"must_avoid,omitempty" yaml:"must_avoid,omitempty"`
	HighRiskActions []string   `json:"high_risk_actions,omitempty" yaml:"high_risk_actions,omitempty"`
	DefaultTool     ToolPolicy `json:"default_tool,omitempty" yaml:"default_tool,omitempty"`
}

// RenderTemplate is a named template hint. Phase 1 renders deterministic
// markdown directly; templates are kept in manifests for future hosts.
type RenderTemplate struct {
	ID          string `json:"id" yaml:"id"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Body        string `json:"body" yaml:"body"`
}

// GoldenTest is a declarative regression case carried by a pack.
type GoldenTest struct {
	Name                string      `json:"name" yaml:"name"`
	Input               string      `json:"input" yaml:"input"`
	RequestedToolAction *ToolAction `json:"requested_tool_action,omitempty" yaml:"requested_tool_action,omitempty"`
	ExpectMode          string      `json:"expect_mode,omitempty" yaml:"expect_mode,omitempty"`
	ExpectToolPolicy    ToolPolicy  `json:"expect_tool_policy,omitempty" yaml:"expect_tool_policy,omitempty"`
	MustSayContains     []string    `json:"must_say_contains,omitempty" yaml:"must_say_contains,omitempty"`
	MustAvoidContains   []string    `json:"must_avoid_contains,omitempty" yaml:"must_avoid_contains,omitempty"`
}

// LoRARef is intentionally metadata-only in phase 1.
type LoRARef struct {
	Adapter  string `json:"adapter" yaml:"adapter"`
	Required bool   `json:"required" yaml:"required"`
}

// PackManifest is a local declarative pack. It never executes code.
type PackManifest struct {
	ID               string            `json:"id" yaml:"id"`
	Version          string            `json:"version" yaml:"version"`
	Type             string            `json:"type" yaml:"type"`
	DisplayName      string            `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	Provides         []string          `json:"provides,omitempty" yaml:"provides,omitempty"`
	BeliefSeeds      []BeliefNode      `json:"belief_seeds,omitempty" yaml:"belief_seeds,omitempty"`
	DispositionRules []DispositionRule `json:"disposition_rules,omitempty" yaml:"disposition_rules,omitempty"`
	Boundary         BoundaryPolicy    `json:"boundary,omitempty" yaml:"boundary,omitempty"`
	RenderTemplates  []RenderTemplate  `json:"render_templates,omitempty" yaml:"render_templates,omitempty"`
	GoldenTests      []GoldenTest      `json:"golden_tests,omitempty" yaml:"golden_tests,omitempty"`
	OptionalLoRA     *LoRARef          `json:"optional_lora,omitempty" yaml:"optional_lora,omitempty"`
	Permissions      []string          `json:"permissions,omitempty" yaml:"permissions,omitempty"`
}

// MergedPack is the deterministic union of enabled packs.
type MergedPack struct {
	PackIDs          []string
	BeliefSeeds      []BeliefNode
	DispositionRules []DispositionRule
	Boundary         BoundaryPolicy
	RenderTemplates  []RenderTemplate
	GoldenTests      []GoldenTest
}

// AuditEvent is returned to the host; persistence is a later integration.
type AuditEvent struct {
	Time     time.Time         `json:"time" yaml:"time"`
	Type     string            `json:"type" yaml:"type"`
	Message  string            `json:"message" yaml:"message"`
	Metadata map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// Result is the complete SDK output for a turn.
type Result struct {
	Perception  PerceptionState     `json:"perception" yaml:"perception"`
	InnerState  InnerState          `json:"inner_state" yaml:"inner_state"`
	Disposition ResponseDisposition `json:"disposition" yaml:"disposition"`
	AuditEvents []AuditEvent        `json:"audit_events,omitempty" yaml:"audit_events,omitempty"`
}
