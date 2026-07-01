package cognisdk

import (
	"context"

	"yunque-agent/pkg/belief"
)

// HostAdapter bridges host requests into the experimental cognition SDK.
//
// It owns two belief engines side by side:
//   - engine is the cognisdk pack engine (Perception / Disposition / PackIDs /
//     feedback / bundle export). It is the source of pack-level signals.
//   - beliefEngine is the pkg/belief graph engine with the scope gate (#34).
//     It is seeded from the cognisdk pack BeliefSeeds at construction time and
//     drives InnerState.ActiveBeliefs so that scope filtering actually applies
//     at runtime. nil when no packs declare beliefs (preserves prior behavior).
type HostAdapter struct {
	engine       *Engine
	beliefEngine *belief.Engine
}

// NewHostAdapter creates an adapter from a direct config.
func NewHostAdapter(config Config) *HostAdapter {
	engine := NewEngine(config)
	return &HostAdapter{
		engine:       engine,
		beliefEngine: seedBeliefEngine(engine),
	}
}

// NewHostAdapterFromDir loads local packs from a directory and builds an adapter.
func NewHostAdapterFromDir(dir string) (*HostAdapter, []PackLoadError, error) {
	pm, errs, err := NewPackManagerFromDir(dir)
	if err != nil {
		return nil, errs, err
	}
	engine := &Engine{manager: pm}
	return &HostAdapter{
		engine:       engine,
		beliefEngine: seedBeliefEngine(engine),
	}, errs, nil
}

// NewHostAdapterFromBundle restores a host adapter from a portable pack bundle.
func NewHostAdapterFromBundle(bundle PackBundle) (*HostAdapter, error) {
	pm, err := NewPackManagerFromBundle(bundle)
	if err != nil {
		return nil, err
	}
	engine := &Engine{manager: pm}
	return &HostAdapter{
		engine:       engine,
		beliefEngine: seedBeliefEngine(engine),
	}, nil
}

// seedBeliefEngine builds a pkg/belief graph from the cognisdk pack BeliefSeeds
// and wraps it in a belief.Engine. This is the bridge that lets the scope gate
// (#34) actually filter beliefs at runtime: cognisdk owns pack enablement and
// disposition rendering, pkg/belief owns the scope-gated activation graph.
// Returns nil when no packs declare beliefs (preserves prior behavior — the
// host then renders the cognisdk平面 BeliefSeeds unchanged).
func seedBeliefEngine(engine *Engine) *belief.Engine {
	if engine == nil || engine.manager == nil {
		return nil
	}
	merged := engine.manager.Merge()
	if len(merged.BeliefSeeds) == 0 {
		return nil
	}
	graph := belief.NewBeliefGraph()
	for _, seed := range merged.BeliefSeeds {
		node := cognisdkSeedToBeliefNode(seed)
		if err := graph.Add(node); err != nil {
			// Skip invalid seeds rather than failing construction — a bad
			// pack shouldn't take down the whole belief subsystem.
			continue
		}
	}
	if graph.Size() == 0 {
		return nil
	}
	return belief.NewEngine(graph)
}

// cognisdkSeedToBeliefNode converts a cognisdk pack seed into a pkg/belief graph
// node. Field mapping:
//   - ID/Statement/Kind map directly (cognisdk and belief share Kind names:
//     root/value/relational/boundary/preference).
//   - Confidence maps directly.
//   - Scopes maps directly (#34 scope gate).
//   - Source is set to SourceCogni (belief tracks provenance differently than
//     cognisdk's SourcePack string — the pack id stays on the cognisdk side).
//   - Strength/Valence/Stability/Plasticity get default lifecycle values
//     because cognisdk seeds don't carry them: durable kinds (root/value/
//     boundary) get high strength + high stability (they're declared policy,
//     not learned hypotheses); preference gets mid strength + low stability
//     (preferences drift). Valence defaults neutral.
func cognisdkSeedToBeliefNode(seed BeliefNode) *belief.BeliefNode {
	node := &belief.BeliefNode{
		ID:        seed.ID,
		Statement: seed.Statement,
		Kind:      belief.BeliefKind(seed.Kind),
		Confidence: seed.Confidence,
		Source:    belief.SourceCogni,
		Scopes:    append([]string(nil), seed.Scopes...),
	}
	switch seed.Kind {
	case BeliefRoot, BeliefValue, BeliefBoundary:
		node.Strength = 1.0
		node.Stability = 0.95
		node.Plasticity = 0.05
	case BeliefRelational:
		node.Strength = 0.8
		node.Stability = 0.7
		node.Plasticity = 0.2
	case BeliefPreference:
		node.Strength = 0.5
		node.Stability = 0.3
		node.Plasticity = 0.6
	default:
		node.Strength = 0.7
		node.Stability = 0.5
		node.Plasticity = 0.3
	}
	node.Valence = 0.0
	return node
}

// Engine exposes the underlying engine for inspection.
func (a *HostAdapter) Engine() *Engine {
	if a == nil {
		return nil
	}
	return a.engine
}

// BuildContext evaluates one turn and returns planner-ready markdown.
//
// scope is the coarse conversation kind ("emotional", "technical", ""),
// passed straight through to pkg/belief's scope gate. When the adapter has a
// beliefEngine (i.e. some pack declared BeliefSeeds), the scope-gated
// ActivateResult overrides cognisdk's平面 ActiveBeliefs — this is where #34
// actually bites at runtime. Empty scope preserves prior behavior (only global
// beliefs activate; scoped beliefs stay dormant).
func (a *HostAdapter) BuildContext(ctx context.Context, message, tenantID, channel, scope string) string {
	if a == nil || a.engine == nil {
		return ""
	}
	result := a.engine.Evaluate(ctx, Input{
		Message: message,
		UserID:  tenantID,
		Channel: channel,
		Scope:   scope,
	})
	// Scope-gate override: when pkg/belief is wired, its ActivateResult is the
	// source of truth for ActiveBeliefs. cognisdk's pack-level disposition /
	// packIDs / perception stay — only the belief list is upgraded so scope
	// filtering actually applies.
	if a.beliefEngine != nil {
		activated, err := a.beliefEngine.EvaluateInteraction(message, nil, scope)
		if err == nil && activated != nil {
			result.InnerState.ActiveBeliefs = activateResultToCognisdkBeliefs(a, activated)
		}
	}
	return RenderMarkdown(result)
}

// activateResultToCognisdkBeliefs renders a pkg/belief ActivateResult back into
// cognisdk's BeliefNode shape so RenderMarkdown can consume it unchanged.
// Only beliefs that the scope gate let through appear here — that's the whole
// point of #34. We pull the full node from the belief graph to preserve the
// Statement, then attach the SourcePack the seed originally came from (looked
// up by belief ID in the cognisdk merged seeds) so the rendered context still
// shows provenance.
func activateResultToCognisdkBeliefs(a *HostAdapter, res *belief.ActivateResult) []BeliefNode {
	if res == nil || len(res.ActiveBeliefs) == 0 {
		return nil
	}
	seedByID := map[string]BeliefNode{}
	if a.engine != nil && a.engine.manager != nil {
		for _, seed := range a.engine.manager.Merge().BeliefSeeds {
			seedByID[seed.ID] = seed
		}
	}
	out := make([]BeliefNode, 0, len(res.ActiveBeliefs))
	graph := a.beliefEngine.Graph()
	for _, id := range res.ActiveBeliefs {
		node := graph.Get(id)
		if node == nil {
			continue
		}
		seed := seedByID[id]
		bn := BeliefNode{
			ID:        node.ID,
			Kind:      BeliefKind(node.Kind),
			Statement: node.Statement,
			Scopes:    append([]string(nil), node.Scopes...),
		}
		if seed.SourcePack != "" {
			bn.SourcePack = seed.SourcePack
		}
		if seed.ReadOnly {
			bn.ReadOnly = true
		}
		out = append(out, bn)
	}
	return out
}

// Evaluate returns the structured cognition result for a turn.
func (a *HostAdapter) Evaluate(ctx context.Context, input Input) Result {
	if a == nil || a.engine == nil {
		return Result{}
	}
	return a.engine.Evaluate(ctx, input)
}

// ProposeUpdates converts audit feedback into non-mutating belief update
// proposals through the underlying engine.
func (a *HostAdapter) ProposeUpdates(ctx context.Context, result Result, feedback AuditFeedback) FeedbackProposal {
	if a == nil || a.engine == nil {
		return BuildFeedbackProposal(result, feedback)
	}
	return a.engine.ProposeUpdates(ctx, result, feedback)
}

// PackManager exposes the runtime pack manager.
func (a *HostAdapter) PackManager() *PackManager {
	if a == nil || a.engine == nil {
		return nil
	}
	return a.engine.PackManager()
}
