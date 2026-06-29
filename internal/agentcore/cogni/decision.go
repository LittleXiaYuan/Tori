package cogni

// CogniDecision represents a single Cogni's analysis result for a given request.
// It encapsulates the Cogni's understanding of the task intent, the resources
// needed to handle it (tools, skills, memory), behavioral guidance, and any
// internal state it wants to expose.
//
// Multiple CogniDecisions from different Cognis are merged by CogniRuntime.Decide()
// into a single CogniFinalDecision that drives prompt assembly and resource injection.
type CogniDecision struct {
	// Intent is this Cogni's classification of the user's task type.
	// nil if this Cogni does not perform intent detection or is uncertain.
	Intent *Intent

	// Confidence is a 0-1 score indicating how certain this Cogni is about
	// its intent classification. Only meaningful when Intent is non-nil.
	// Used during intent voting in mergeIntents (weighted by Priority × Confidence).
	Confidence float64

	// ToolsNeeded lists the tools required for this task. Supports wildcards:
	//   "file_*"     matches file_read, file_write, file_search, etc.
	//   "browser_*"  matches browser_search, browser_navigate, etc.
	//   "github_*"   matches all tools from the GitHub MCP server
	// Empty slice means this Cogni has no opinion on tools (does not restrict).
	ToolsNeeded []string

	// SkillsNeeded lists skill categories or names to include in the prompt.
	// Categories like "code", "research", "chat" filter the full skill registry.
	// Empty slice means this Cogni has no opinion on skills (does not restrict).
	SkillsNeeded []string

	// DeniedTools lists tool-name globs this Cogni wants REMOVED from the surface,
	// regardless of what other Cognis allow. Supports the same trailing-"*"
	// wildcards as ToolsNeeded ("file_write", "shell_*"). This is a restrictive
	// (deny) signal, applied as a final subtractive pass AFTER the additive
	// ToolsNeeded/SkillsNeeded union — so a safety Cogni (RiskCogni) can strip
	// destructive tools even when an intent Cogni broadly allowed "file_*".
	// Denies from all Cognis accumulate (union): safety is conservative.
	DeniedTools []string

	// MemoryScope defines memory recall constraints for this task.
	// Multiple Cognis' scopes are merged into the most permissive union
	// (max limit, union of categories/keywords) so no Cogni blocks another's needs.
	MemoryScope MemoryScope

	// BehaviorText is the textual guidance injected into the prompt to adjust
	// the model's output style, tone, or risk posture for this task.
	// This is the equivalent of v1's BuildContext() output.
	// Multiple Cognis' BehaviorText are concatenated in priority order.
	BehaviorText string

	// State holds Cogni-specific internal state (e.g. Inner State, risk level,
	// emotion detection results) that downstream systems can inspect.
	// Keys and semantics are Cogni-specific. Not directly injected into prompt.
	State map[string]any
}

// Intent represents a Cogni's classification of the user's task type.
type Intent struct {
	// Type is the intent category: "search", "code", "chat", "browser", "file", "complex".
	Type string

	// Confidence is a 0-1 score for this intent classification.
	// Redundant with CogniDecision.Confidence but kept here for self-documentation.
	Confidence float64
}

// MemoryScope defines memory recall filtering constraints for a task.
type MemoryScope struct {
	// Limit caps the number of recalled memories. 0 means no explicit limit
	// (falls back to the orchestrator's default, typically 20).
	Limit int

	// Categories filters memories by their classification:
	//   "identity"      — core identity facts (who the user is, preferences)
	//   "project"       — active project structure, goals, constraints
	//   "conversation"  — recent dialogue history, emotional context
	// Empty slice means no category filtering (all categories included).
	Categories []string

	// Keywords are content-based filters passed to the memory orchestrator.
	// Memories matching any keyword receive a boost in the recall ranking.
	Keywords []string
}

// CogniFinalDecision is the merged result from all active Cognis, produced by
// CogniRuntime.Decide(). It drives the prompt assembly pipeline:
//   - ToolsNeeded filters the tool registry (only inject matching tools)
//   - SkillsNeeded filters the skill registry (only inject matching skills)
//   - MemoryScope constrains memory recall (limit, categories, keywords)
//   - BehaviorText is injected into the P4 Cognition layer of the system prompt
//   - Intent is surfaced for telemetry and can influence downstream logic
type CogniFinalDecision struct {
	// Intent is the consensus task type from all Cognis, determined by
	// weighted voting (Priority × Confidence). nil if no Cogni classified intent.
	Intent *Intent

	// ToolsNeeded is the union of all Cognis' tool requirements.
	// Wildcards are preserved; expansion happens in the tool registry filter.
	ToolsNeeded []string

	// SkillsNeeded is the union of all Cognis' skill requirements.
	SkillsNeeded []string

	// DeniedTools is the union of all Cognis' tool-deny globs, applied as a final
	// subtractive pass after the ToolsNeeded/SkillsNeeded allow-list is resolved.
	// A tool matching any glob here is removed even if an intent Cogni allowed it.
	DeniedTools []string

	// MemoryScope is the most permissive union of all Cognis' scopes:
	//   - Limit = max of all limits
	//   - Categories = union of all categories
	//   - Keywords = union of all keywords
	MemoryScope MemoryScope

	// BehaviorText is the concatenation of all Cognis' BehaviorText,
	// ordered by Cogni priority (high-priority Cognis' text appears first).
	BehaviorText string

	// State aggregates all Cognis' State maps. Keys from lower-priority Cognis
	// are overwritten by higher-priority ones in case of collision.
	State map[string]any
}
