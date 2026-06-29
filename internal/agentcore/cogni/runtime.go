package cogni

import (
	"sort"
	"strings"
)

// Runtime is the v2 Cogni orchestration layer that manages multiple Cognis
// (v1 and v2) and merges their decisions into a single CogniFinalDecision.
//
// This is a minimal interface that existing runtimes (like cmd/agent's
// plannerCogniRuntime) will implement. The actual Runtime struct lives in
// cmd/agent/module_cogni.go and holds the v1 hook, beliefAdapter, and v2 hooks.
type Runtime interface {
	// Decide calls all active Cognis' Analyze methods, merges their decisions
	// via weighted voting (intent) and union (resources), and returns the
	// final decision that drives prompt assembly.
	Decide(req CogniRequest) CogniFinalDecision
}

// MergeDecisions combines multiple CogniDecisions into a single CogniFinalDecision
// using the following merge strategy:
//   - Intent: weighted voting (Priority × Confidence), highest score wins
//   - ToolsNeeded: union of all requests (any Cogni can require a tool)
//   - SkillsNeeded: union of all requests
//   - MemoryScope: most permissive union (max limit, union categories/keywords)
//   - BehaviorText: concatenation in priority order (highest priority first)
//   - State: merged map, higher-priority Cognis overwrite lower-priority keys
//
// cognis is a slice of (decision, priority) pairs sorted by priority descending.
func MergeDecisions(cognis []CogniWithPriority) CogniFinalDecision {
	if len(cognis) == 0 {
		return CogniFinalDecision{}
	}

	// Sort by priority descending so higher-priority Cognis are processed first
	sort.Slice(cognis, func(i, j int) bool {
		return cognis[i].Priority > cognis[j].Priority
	})

	// Merge intent via weighted voting
	intent := mergeIntents(cognis)

	// Merge resources via union
	tools := mergeTools(cognis)
	denied := mergeDeniedTools(cognis)
	skills := mergeSkills(cognis)
	memory := mergeMemoryScope(cognis)

	// Merge behavior text via priority-ordered concatenation
	behavior := mergeBehaviorText(cognis)

	// Merge state maps (higher priority wins on key collision)
	state := mergeState(cognis)

	return CogniFinalDecision{
		Intent:       intent,
		ToolsNeeded:  tools,
		DeniedTools:  denied,
		SkillsNeeded: skills,
		MemoryScope:  memory,
		BehaviorText: behavior,
		State:        state,
	}
}

// CogniWithPriority pairs a CogniDecision with its source Cogni's priority.
type CogniWithPriority struct {
	Decision CogniDecision
	Priority int
}

// mergeIntents performs weighted voting: Priority × Confidence, highest score wins.
// Returns nil if no Cogni provided an intent classification.
func mergeIntents(cognis []CogniWithPriority) *Intent {
	var best *Intent
	var bestScore float64
	found := false

	for _, c := range cognis {
		if c.Decision.Intent == nil {
			continue
		}
		// Weighted score = Priority × Confidence
		// Priority is int, Confidence is 0-1, so normalize Priority to 0-100 range
		score := float64(c.Priority) * c.Decision.Confidence
		if !found || score > bestScore {
			bestScore = score
			best = c.Decision.Intent
			found = true
		}
	}

	return best
}

// mergeTools unions all ToolsNeeded lists, preserving wildcards and deduplicating.
func mergeTools(cognis []CogniWithPriority) []string {
	seen := make(map[string]bool)
	var result []string

	for _, c := range cognis {
		for _, tool := range c.Decision.ToolsNeeded {
			if tool != "" && !seen[tool] {
				seen[tool] = true
				result = append(result, tool)
			}
		}
	}

	return result
}

// mergeDeniedTools unions all DeniedTools globs, deduplicating. Denies are
// conservative: any Cogni can forbid a tool and the deny survives the merge,
// so a low-priority safety Cogni is never overruled by a high-priority intent.
func mergeDeniedTools(cognis []CogniWithPriority) []string {
	seen := make(map[string]bool)
	var result []string

	for _, c := range cognis {
		for _, tool := range c.Decision.DeniedTools {
			if tool != "" && !seen[tool] {
				seen[tool] = true
				result = append(result, tool)
			}
		}
	}

	return result
}

// mergeSkills unions all SkillsNeeded lists, deduplicating.
func mergeSkills(cognis []CogniWithPriority) []string {
	seen := make(map[string]bool)
	var result []string

	for _, c := range cognis {
		for _, skill := range c.Decision.SkillsNeeded {
			if skill != "" && !seen[skill] {
				seen[skill] = true
				result = append(result, skill)
			}
		}
	}

	return result
}

// mergeMemoryScope takes the most permissive union:
//   - Limit: max of all limits (0 = no limit specified, use orchestrator default)
//   - Categories: union of all category lists
//   - Keywords: union of all keyword lists
func mergeMemoryScope(cognis []CogniWithPriority) MemoryScope {
	var maxLimit int
	catSeen := make(map[string]bool)
	kwSeen := make(map[string]bool)
	var categories, keywords []string

	for _, c := range cognis {
		scope := c.Decision.MemoryScope

		// Take the maximum limit
		if scope.Limit > maxLimit {
			maxLimit = scope.Limit
		}

		// Union categories
		for _, cat := range scope.Categories {
			if cat != "" && !catSeen[cat] {
				catSeen[cat] = true
				categories = append(categories, cat)
			}
		}

		// Union keywords
		for _, kw := range scope.Keywords {
			if kw != "" && !kwSeen[kw] {
				kwSeen[kw] = true
				keywords = append(keywords, kw)
			}
		}
	}

	return MemoryScope{
		Limit:      maxLimit,
		Categories: categories,
		Keywords:   keywords,
	}
}

// mergeBehaviorText concatenates all non-empty BehaviorText strings in priority order,
// separated by blank lines. Higher-priority Cognis' text appears first.
func mergeBehaviorText(cognis []CogniWithPriority) string {
	var texts []string

	for _, c := range cognis {
		if c.Decision.BehaviorText != "" {
			texts = append(texts, c.Decision.BehaviorText)
		}
	}

	return strings.Join(texts, "\n\n")
}

// mergeState combines all Cognis' State maps. On key collision, higher-priority
// Cognis' values overwrite lower-priority ones (since cognis is sorted descending).
func mergeState(cognis []CogniWithPriority) map[string]any {
	result := make(map[string]any)

	// Iterate in reverse (lowest priority first) so higher-priority overwrites
	for i := len(cognis) - 1; i >= 0; i-- {
		for k, v := range cognis[i].Decision.State {
			result[k] = v
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}
