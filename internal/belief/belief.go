// Package belief implements a sustainable belief structure for maintaining
// self-consistency, relationship memory, and value-driven selection.
//
// Belief is not a "soul module" — it is a value/relationship/self-continuity
// kernel. It does not replace the LLM; it orients, constrains, and sediments
// what matters across time.
//
// The design follows these principles:
//   - Prompt is performance layer (what to say this turn)
//   - Belief is selection layer (why this response, not that one)
//   - Confidence and strength are separate — guesses are not facts
//   - Updates are gradual, audited, and bounded by root values
package belief

import "time"

// BeliefKind classifies a belief node's role in the network.
type BeliefKind string

const (
	KindRoot       BeliefKind = "root"        // from Cogni declaration; stable, low-frequency update, audited
	KindValue      BeliefKind = "value"       // e.g. "freedom > obedience", "sincerity > flattery"
	KindRelational BeliefKind = "relational"  // e.g. "夏鸢 cares about being taken seriously"
	KindPreference BeliefKind = "preference"  // e.g. "夏鸢 likes lighter tone"; fast-updating
	KindBoundary   BeliefKind = "boundary"    // e.g. "I will not lie to please"
	KindTension    BeliefKind = "tension"     // conflict between two beliefs; this IS personality
)

// RelationType describes how one belief relates to another.
type RelationType string

const (
	RelationSupports  RelationType = "supports"
	RelationConflicts RelationType = "conflicts"
	RelationCauses    RelationType = "causes"
	RelationProtects  RelationType = "protects"
)

// Source indicates where a belief originated.
type Source string

const (
	SourceCogni         Source = "cogni"          // from Cogni declaration (root)
	SourceInteraction   Source = "interaction"    // derived from user interaction
	SourceReflection    Source = "reflection"     // from self-reflection / Reverie
	SourceUserExplicit  Source = "user_explicit"  // explicitly stated by user
	SourceSystem        Source = "system"         // system-defined default
)

// BeliefNode is a single belief in the network.
//
// Fields are deliberately separated into:
//   - Identity: id, statement, kind
//   - Strength: strength, confidence, valence, stability, plasticity
//   - Provenance: source, evidence, related
//   - Lifecycle: lastUpdatedAt
type BeliefNode struct {
	// ID uniquely identifies this belief.
	ID string `json:"id"`

	// Statement is the natural-language expression of this belief.
	// e.g. "小羽想让夏鸢开心"
	Statement string `json:"statement"`

	// Kind classifies the role of this belief.
	Kind BeliefKind `json:"kind"`

	// ── Strength dimensions ──

	// Strength is how strongly this belief is held (0..1).
	Strength float64 `json:"strength"`

	// Confidence is how sure we are that this belief is true.
	// MUST be separated from strength: we can hold a belief strongly
	// while being uncertain about its accuracy.
	Confidence float64 `json:"confidence"`

	// Valence is the emotional direction (-1..1).
	// Negative = painful/cautious, positive = joyful/trusting.
	Valence float64 `json:"valence"`

	// Stability is resistance to change (0..1).
	// Root beliefs have high stability; preferences have low stability.
	Stability float64 `json:"stability"`

	// Plasticity is the maximum per-update delta (0..1).
	// Controls how much this belief can change in a single update.
	Plasticity float64 `json:"plasticity"`

	// ── Provenance ──

	// Source indicates where this belief originated.
	Source Source `json:"source"`

	// Evidence is a list of experiences or reasons supporting this belief.
	Evidence []string `json:"evidence,omitempty"`

	// Related connects this belief to others in the graph.
	Related []BeliefEdge `json:"related,omitempty"`

	// ── Lifecycle ──

	// LastUpdatedAt is the timestamp of the most recent update.
	LastUpdatedAt time.Time `json:"last_updated_at"`

	// CreatedAt is the timestamp when this belief was first created.
	CreatedAt time.Time `json:"created_at"`
}

// BeliefEdge represents a directed relationship between two beliefs.
type BeliefEdge struct {
	TargetID string       `json:"target_id"`
	Relation RelationType `json:"relation"`
}

// ActivateResult captures the output of belief engine evaluation for a single turn.
// It describes the "inner state" that the LLM expression layer should use.
type ActivateResult struct {
	// ActiveBeliefs are the IDs of beliefs activated by the current interaction.
	ActiveBeliefs []string `json:"active_beliefs"`

	// PrimaryTension, if non-empty, is the ID of a tension node that is
	// currently active. This is where personality lives — the choice between
	// conflicting beliefs.
	PrimaryTension string `json:"primary_tension,omitempty"`

	// Tendency is a summary direction derived from the active belief set.
	// e.g. "protective", "curious", "cautious", "warm"
	Tendency string `json:"tendency,omitempty"`

	// Boundaries is the set of boundary beliefs that should NOT be crossed.
	Boundaries []string `json:"boundaries,omitempty"`

	// LowConfidenceInferences lists beliefs with high strength but low confidence —
	// things we act on but should not treat as facts.
	LowConfidenceInferences []string `json:"low_confidence_inferences,omitempty"`
}

// Validate checks basic integrity of a BeliefNode.
func (b *BeliefNode) Validate() error {
	if b.ID == "" {
		return &ErrInvalidBelief{Field: "id", Reason: "id is required"}
	}
	if b.Statement == "" {
		return &ErrInvalidBelief{Field: "statement", Reason: "statement is required"}
	}
	if b.Strength < 0 || b.Strength > 1 {
		return &ErrInvalidBelief{Field: "strength", Reason: "must be in [0, 1]"}
	}
	if b.Confidence < 0 || b.Confidence > 1 {
		return &ErrInvalidBelief{Field: "confidence", Reason: "must be in [0, 1]"}
	}
	if b.Valence < -1 || b.Valence > 1 {
		return &ErrInvalidBelief{Field: "valence", Reason: "must be in [-1, 1]"}
	}
	if b.Stability < 0 || b.Stability > 1 {
		return &ErrInvalidBelief{Field: "stability", Reason: "must be in [0, 1]"}
	}
	if b.Plasticity < 0 || b.Plasticity > 1 {
		return &ErrInvalidBelief{Field: "plasticity", Reason: "must be in [0, 1]"}
	}
	if b.Kind == "" {
		return &ErrInvalidBelief{Field: "kind", Reason: "kind is required"}
	}
	return nil
}

// IsRoot returns true if this is a root belief (from Cogni).
func (b *BeliefNode) IsRoot() bool { return b.Kind == KindRoot }

// IsTension returns true if this belief represents an internal conflict.
func (b *BeliefNode) IsTension() bool { return b.Kind == KindTension }

// QuickSummary returns a one-line summary of the belief state.
func (b *BeliefNode) QuickSummary() string {
	prefix := string(b.Kind)
	return "[" + prefix + "] " + b.Statement + " (💪" + f64(b.Strength) + " 🎯" + f64(b.Confidence) + ")"
}

func f64(v float64) string {
	if v >= 0.95 {
		return "▰▰▰"
	}
	if v >= 0.7 {
		return "▰▰▱"
	}
	if v >= 0.4 {
		return "▰▱▱"
	}
	return "▱▱▱"
}

// ErrInvalidBelief is returned when a BeliefNode fails validation.
type ErrInvalidBelief struct {
	Field  string
	Reason string
}

func (e *ErrInvalidBelief) Error() string {
	return "belief: invalid " + e.Field + ": " + e.Reason
}
