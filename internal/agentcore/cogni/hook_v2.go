package cogni

import "context"

// HookV2 is the v2 Cogni interface that replaces v1's BuildContext + FilterSkills
// with a unified Analyze method. Instead of generating text and filtering skills
// separately, v2 Cognis return a structured CogniDecision that drives resource
// allocation (tools, skills, memory) and prompt assembly in one shot.
//
// V2 Cognis participate in intelligent context routing: the runtime calls all
// active Cognis' Analyze methods, merges their decisions via weighted voting and
// union, then passes the result to the prompt builder for filtered injection.
//
// V1 Cognis (pkg/cogni.Hook with BuildContext) remain supported through the
// V1CompatAdapter wrapper, so existing Cognis continue working without modification.
type HookV2 interface {
	// Analyze inspects the request and returns this Cogni's decision about:
	//   - Task intent (search / code / chat / browser / file / complex)
	//   - Required resources (tools, skills, memory scope)
	//   - Behavioral guidance text (injected into prompt P4 layer)
	//   - Internal state (risk level, emotion detection, etc.)
	//
	// Multiple Cognis' decisions are merged by CogniRuntime.Decide():
	//   - Intent: weighted voting (Priority × Confidence)
	//   - Resources: union (any Cogni can request a tool/skill)
	//   - Memory: most permissive union (max limit, union categories/keywords)
	//   - Behavior: concatenation in priority order
	//
	// Return a zero CogniDecision if this Cogni has no opinion for this request
	// (e.g. an emotion Cogni during a technical code task). The runtime will
	// skip it during merging.
	Analyze(ctx context.Context, req CogniRequest) CogniDecision

	// Priority returns this Cogni's precedence in decision merging.
	// Higher priority wins during intent conflicts and appears first in
	// concatenated BehaviorText.
	//
	// Suggested values:
	//   100 — core intent detection (IntentCogni, task classifier)
	//   50  — domain-specific策略 (EmotionCogni, RiskCogni)
	//   10  — auxiliary/experimental Cognis
	//   0   — v1 compat adapters (no priority opinion)
	Priority() int
}

// CogniRequest is the input to HookV2.Analyze, carrying the user's message,
// tenant context, and conversation metadata needed for intent detection and
// resource allocation.
//
// This is a placeholder; the actual structure will be defined based on what
// the existing v1 ContextRequest contains. For now, keep it compatible.
type CogniRequest struct {
	// Message is the latest user input or a summary of the conversation turn.
	Message string

	// TenantID identifies the user/workspace for memory recall and personalization.
	TenantID string

	// Channel identifies the interaction surface (web, desktop, API, etc.)
	// for context-aware behavior adjustment.
	Channel string

	// ConversationHistory holds recent turns for intent detection that requires
	// multi-turn context (e.g. follow-up questions, implicit references).
	// May be nil for single-turn requests.
	ConversationHistory []HistoryMessage

	// Metadata holds additional context from the planner (intent hint from
	// LocalBrain, trust tier, session state, etc.). Keys are runtime-specific.
	Metadata map[string]any

	// ForceCogniIDs lists Cogni IDs the user pinned for this turn (chat
	// `/智能体` pick). They are force-activated regardless of score so their
	// behavior, tool surface and MCP tools engage. Empty = score-driven only.
	ForceCogniIDs []string
}

// HistoryMessage represents one turn in the conversation history.
type HistoryMessage struct {
	Role    string // "user" / "assistant"
	Content string
}
