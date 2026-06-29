package cogni

import (
	"context"

	"yunque-agent/pkg/cogni"
)

// V1Hook is the interface that v1 Cognis implement (subset of pkg/cogni.Hook).
// This allows testing with mocks instead of requiring a full Hook instance.
type V1Hook interface {
	BuildContext(req cogni.ContextRequest) string
}

// V1CompatAdapter wraps a v1 Cogni (pkg/cogni.Hook with BuildContext/FilterSkills)
// to work in the v2 CogniRuntime without modification. It implements HookV2 by:
//   - Calling BuildContext and returning the text as BehaviorText
//   - Leaving Intent/ToolsNeeded/SkillsNeeded/MemoryScope empty (no opinion)
//   - Defaulting Priority to 0
//
// This allows gradual migration: v1 Cognis continue working via the compat layer
// while new v2 Cognis can participate in resource allocation. The v1 Cogni still
// contributes behavioral text, just doesn't influence tool/skill/memory filtering.
//
// Note: v1's FilterSkills is handled separately in the prompt builder's skill
// filtering pipeline, not through CogniDecision. The compat adapter does NOT
// call FilterSkills — that remains the prompt builder's responsibility.
type V1CompatAdapter struct {
	v1Hook V1Hook
}

// NewV1CompatAdapter wraps a v1 Hook for use in v2 CogniRuntime.
func NewV1CompatAdapter(v1 V1Hook) *V1CompatAdapter {
	return &V1CompatAdapter{v1Hook: v1}
}

// Analyze implements HookV2 by calling the v1 Hook's BuildContext and wrapping
// the result in a minimal CogniDecision.
func (a *V1CompatAdapter) Analyze(ctx context.Context, req CogniRequest) CogniDecision {
	if a.v1Hook == nil {
		return CogniDecision{}
	}

	// Convert CogniRequest to v1's ContextRequest format.
	// The v1 ContextRequest structure is in pkg/cogni and expects specific fields.
	// We'll construct it based on what's available in CogniRequest.
	v1Req := cogni.ContextRequest{
		Message:  req.Message,
		TenantID: req.TenantID,
		Channel:  req.Channel,
		// v1 may expect other fields (intent hint, etc.) — wire as needed
	}

	// Call v1's BuildContext to get the behavioral text
	behaviorText := a.v1Hook.BuildContext(v1Req)

	// Return a CogniDecision with only BehaviorText populated.
	// v1 Cognis do not participate in intent detection or resource allocation.
	return CogniDecision{
		Intent:       nil, // v1 doesn't classify intent
		ToolsNeeded:  nil, // v1 doesn't filter tools
		SkillsNeeded: nil, // v1 FilterSkills handled separately
		MemoryScope:  MemoryScope{}, // v1 doesn't constrain memory
		BehaviorText: behaviorText,
		State:        nil, // v1 doesn't expose structured state
	}
}

// Priority returns 0 for v1 compat adapters, placing them last in decision merging.
// v1 Cognis' BehaviorText will be concatenated after all v2 Cognis' text.
func (a *V1CompatAdapter) Priority() int {
	return 0
}
