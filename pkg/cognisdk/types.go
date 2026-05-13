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

// PackChange describes a changed pack between two portable bundle snapshots.
type PackChange struct {
	ID          string `json:"id" yaml:"id"`
	FromVersion string `json:"from_version,omitempty" yaml:"from_version,omitempty"`
	ToVersion   string `json:"to_version,omitempty" yaml:"to_version,omitempty"`
	Reason      string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

// PackBundleDiff is a reviewable, non-mutating delta between two bundle
// snapshots. It is useful for plugin UIs, CI checks, and rollback notes.
type PackBundleDiff struct {
	FromID        string       `json:"from_id" yaml:"from_id"`
	ToID          string       `json:"to_id" yaml:"to_id"`
	AddedPacks    []PackStatus `json:"added_packs,omitempty" yaml:"added_packs,omitempty"`
	RemovedPacks  []PackStatus `json:"removed_packs,omitempty" yaml:"removed_packs,omitempty"`
	ChangedPacks  []PackChange `json:"changed_packs,omitempty" yaml:"changed_packs,omitempty"`
	EnabledPacks  []string     `json:"enabled_packs,omitempty" yaml:"enabled_packs,omitempty"`
	DisabledPacks []string     `json:"disabled_packs,omitempty" yaml:"disabled_packs,omitempty"`
}

// PackBundleReviewOutcome is the SDK's coarse recommendation for a candidate bundle.
type PackBundleReviewOutcome string

const (
	PackBundleReviewReady   PackBundleReviewOutcome = "ready"
	PackBundleReviewReview  PackBundleReviewOutcome = "review"
	PackBundleReviewBlocked PackBundleReviewOutcome = "blocked"
)

// PackBundleReview combines diff, golden-test gate, and rollback metadata for
// a candidate bundle. It never applies the candidate to a host.
type PackBundleReview struct {
	FromID           string                  `json:"from_id" yaml:"from_id"`
	CandidateID      string                  `json:"candidate_id" yaml:"candidate_id"`
	Outcome          PackBundleReviewOutcome `json:"outcome" yaml:"outcome"`
	Reason           string                  `json:"reason" yaml:"reason"`
	RollbackBundleID string                  `json:"rollback_bundle_id,omitempty" yaml:"rollback_bundle_id,omitempty"`
	Diff             PackBundleDiff          `json:"diff" yaml:"diff"`
	GoldenTests      GoldenTestSummary       `json:"golden_tests" yaml:"golden_tests"`
}

// PackBundle is a portable collection of declarative packs. It is still data
// only: loading a bundle validates manifests but never executes code.
type PackBundle struct {
	Version      int               `json:"version" yaml:"version"`
	ID           string            `json:"id" yaml:"id"`
	CreatedAt    time.Time         `json:"created_at,omitempty" yaml:"created_at,omitempty"`
	Packs        []PackManifest    `json:"packs" yaml:"packs"`
	EnabledPacks []string          `json:"enabled_packs,omitempty" yaml:"enabled_packs,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
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

// FeedbackKind classifies audit feedback from a user, reviewer, or host.
type FeedbackKind string

const (
	FeedbackCorrection        FeedbackKind = "correction"
	FeedbackPreference        FeedbackKind = "preference"
	FeedbackBoundaryViolation FeedbackKind = "boundary_violation"
	FeedbackPraise            FeedbackKind = "praise"
	FeedbackRejection         FeedbackKind = "rejection"
)

// FeedbackSeverity is intentionally coarse so hosts can map UI controls,
// audit events, or CLI flags without importing platform internals.
type FeedbackSeverity string

const (
	FeedbackSeverityLow    FeedbackSeverity = "low"
	FeedbackSeverityMedium FeedbackSeverity = "medium"
	FeedbackSeverityHigh   FeedbackSeverity = "high"
)

// AuditFeedback is the SDK-level learning signal after a turn.
// It is observation only: the SDK may produce proposals, but never writes
// memory, ledger rows, packs, or durable beliefs by itself.
type AuditFeedback struct {
	ID              string           `json:"id,omitempty" yaml:"id,omitempty"`
	Time            time.Time        `json:"time,omitempty" yaml:"time,omitempty"`
	Kind            FeedbackKind     `json:"kind" yaml:"kind"`
	Severity        FeedbackSeverity `json:"severity,omitempty" yaml:"severity,omitempty"`
	UserID          string           `json:"user_id,omitempty" yaml:"user_id,omitempty"`
	Channel         string           `json:"channel,omitempty" yaml:"channel,omitempty"`
	Message         string           `json:"message" yaml:"message"`
	Evidence        []string         `json:"evidence,omitempty" yaml:"evidence,omitempty"`
	TargetBeliefIDs []string         `json:"target_belief_ids,omitempty" yaml:"target_belief_ids,omitempty"`
	Tags            []string         `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// BeliefUpdateAction describes the proposed host-side operation.
type BeliefUpdateAction string

const (
	BeliefUpdateAddPreference BeliefUpdateAction = "add_preference"
	BeliefUpdateReinforce     BeliefUpdateAction = "reinforce"
	BeliefUpdateWeaken        BeliefUpdateAction = "weaken"
	BeliefUpdateReviewOnly    BeliefUpdateAction = "review_only"
)

// FeedbackOutcome gives callers a stable summary without inspecting every
// proposal. ReviewRequired means a human or host policy must decide whether
// anything becomes durable.
type FeedbackOutcome string

const (
	FeedbackOutcomeNoAction       FeedbackOutcome = "no_action"
	FeedbackOutcomeProposed       FeedbackOutcome = "proposed"
	FeedbackOutcomeReviewRequired FeedbackOutcome = "review_required"
)

// BeliefUpdateProposal is a non-mutating change request for the host's
// Memory/Ledger/Audit layer. Root, value, and boundary beliefs should normally
// remain read-only; the SDK marks such proposals review-only.
type BeliefUpdateProposal struct {
	ID               string             `json:"id" yaml:"id"`
	Action           BeliefUpdateAction `json:"action" yaml:"action"`
	BeliefID         string             `json:"belief_id,omitempty" yaml:"belief_id,omitempty"`
	Kind             BeliefKind         `json:"kind,omitempty" yaml:"kind,omitempty"`
	Statement        string             `json:"statement,omitempty" yaml:"statement,omitempty"`
	ConfidenceDelta  float64            `json:"confidence_delta,omitempty" yaml:"confidence_delta,omitempty"`
	Reason           string             `json:"reason" yaml:"reason"`
	RequiresReview   bool               `json:"requires_review" yaml:"requires_review"`
	ReadOnlyTarget   bool               `json:"read_only_target,omitempty" yaml:"read_only_target,omitempty"`
	SourceFeedbackID string             `json:"source_feedback_id,omitempty" yaml:"source_feedback_id,omitempty"`
	Evidence         []string           `json:"evidence,omitempty" yaml:"evidence,omitempty"`
}

// FeedbackProposal is the full SDK output for an audit feedback event.
type FeedbackProposal struct {
	ID          string                 `json:"id" yaml:"id"`
	Time        time.Time              `json:"time" yaml:"time"`
	Outcome     FeedbackOutcome        `json:"outcome" yaml:"outcome"`
	Summary     string                 `json:"summary" yaml:"summary"`
	Proposals   []BeliefUpdateProposal `json:"proposals,omitempty" yaml:"proposals,omitempty"`
	AuditEvents []AuditEvent           `json:"audit_events,omitempty" yaml:"audit_events,omitempty"`
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
