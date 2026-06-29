package cogni

import (
	"testing"
)

func TestMergeIntents_WeightedVoting(t *testing.T) {
	cognis := []CogniWithPriority{
		{
			Decision: CogniDecision{
				Intent: &Intent{Type: "search", Confidence: 0.9},
			},
			Priority: 100,
		},
		{
			Decision: CogniDecision{
				Intent: &Intent{Type: "code", Confidence: 0.7},
			},
			Priority: 50,
		},
		{
			Decision: CogniDecision{
				Intent: &Intent{Type: "search", Confidence: 0.6},
			},
			Priority: 10,
		},
	}

	result := MergeDecisions(cognis)

	// Priority 100 × Confidence 0.9 = 90 (wins over Priority 50 × 0.7 = 35)
	if result.Intent == nil || result.Intent.Type != "search" {
		t.Errorf("expected intent=search, got %v", result.Intent)
	}
}

func TestMergeIntents_NoIntents(t *testing.T) {
	cognis := []CogniWithPriority{
		{
			Decision: CogniDecision{Intent: nil},
			Priority: 100,
		},
	}

	result := MergeDecisions(cognis)

	if result.Intent != nil {
		t.Errorf("expected nil intent, got %v", result.Intent)
	}
}

func TestMergeTools_Union(t *testing.T) {
	cognis := []CogniWithPriority{
		{
			Decision: CogniDecision{
				ToolsNeeded: []string{"file_read", "file_write"},
			},
			Priority: 100,
		},
		{
			Decision: CogniDecision{
				ToolsNeeded: []string{"browser_search", "file_read"}, // duplicate file_read
			},
			Priority: 50,
		},
	}

	result := MergeDecisions(cognis)

	// Should have 3 tools (file_read deduplicated)
	if len(result.ToolsNeeded) != 3 {
		t.Errorf("expected 3 tools, got %d: %v", len(result.ToolsNeeded), result.ToolsNeeded)
	}

	// Check all expected tools are present
	toolSet := make(map[string]bool)
	for _, tool := range result.ToolsNeeded {
		toolSet[tool] = true
	}
	expected := []string{"file_read", "file_write", "browser_search"}
	for _, exp := range expected {
		if !toolSet[exp] {
			t.Errorf("expected tool %s not found in result", exp)
		}
	}
}

func TestMergeSkills_Union(t *testing.T) {
	cognis := []CogniWithPriority{
		{
			Decision: CogniDecision{
				SkillsNeeded: []string{"code", "research"},
			},
			Priority: 100,
		},
		{
			Decision: CogniDecision{
				SkillsNeeded: []string{"research", "chat"}, // duplicate research
			},
			Priority: 50,
		},
	}

	result := MergeDecisions(cognis)

	if len(result.SkillsNeeded) != 3 {
		t.Errorf("expected 3 skills, got %d: %v", len(result.SkillsNeeded), result.SkillsNeeded)
	}
}

func TestMergeMemoryScope_MostPermissive(t *testing.T) {
	cognis := []CogniWithPriority{
		{
			Decision: CogniDecision{
				MemoryScope: MemoryScope{
					Limit:      5,
					Categories: []string{"identity", "project"},
					Keywords:   []string{"search"},
				},
			},
			Priority: 100,
		},
		{
			Decision: CogniDecision{
				MemoryScope: MemoryScope{
					Limit:      10, // higher limit wins
					Categories: []string{"project", "conversation"}, // union
					Keywords:   []string{"code"}, // union
				},
			},
			Priority: 50,
		},
	}

	result := MergeDecisions(cognis)

	// Limit should be max (10)
	if result.MemoryScope.Limit != 10 {
		t.Errorf("expected limit=10, got %d", result.MemoryScope.Limit)
	}

	// Categories should be union (identity, project, conversation)
	if len(result.MemoryScope.Categories) != 3 {
		t.Errorf("expected 3 categories, got %d: %v", len(result.MemoryScope.Categories), result.MemoryScope.Categories)
	}

	// Keywords should be union (search, code)
	if len(result.MemoryScope.Keywords) != 2 {
		t.Errorf("expected 2 keywords, got %d: %v", len(result.MemoryScope.Keywords), result.MemoryScope.Keywords)
	}
}

func TestMergeBehaviorText_PriorityOrder(t *testing.T) {
	cognis := []CogniWithPriority{
		{
			Decision: CogniDecision{
				BehaviorText: "High priority text",
			},
			Priority: 100,
		},
		{
			Decision: CogniDecision{
				BehaviorText: "Low priority text",
			},
			Priority: 10,
		},
		{
			Decision: CogniDecision{
				BehaviorText: "Medium priority text",
			},
			Priority: 50,
		},
	}

	result := MergeDecisions(cognis)

	// Should be concatenated in priority order (100, 50, 10)
	expected := "High priority text\n\nMedium priority text\n\nLow priority text"
	if result.BehaviorText != expected {
		t.Errorf("expected:\n%s\n\ngot:\n%s", expected, result.BehaviorText)
	}
}

func TestMergeState_HighPriorityWins(t *testing.T) {
	cognis := []CogniWithPriority{
		{
			Decision: CogniDecision{
				State: map[string]any{
					"risk":  "high",
					"mode":  "analytical",
				},
			},
			Priority: 100,
		},
		{
			Decision: CogniDecision{
				State: map[string]any{
					"risk":    "low", // overridden by high-priority
					"emotion": "happy",
				},
			},
			Priority: 50,
		},
	}

	result := MergeDecisions(cognis)

	// risk should be "high" (from priority 100)
	if result.State["risk"] != "high" {
		t.Errorf("expected risk=high, got %v", result.State["risk"])
	}

	// mode should be "analytical" (from priority 100)
	if result.State["mode"] != "analytical" {
		t.Errorf("expected mode=analytical, got %v", result.State["mode"])
	}

	// emotion should be "happy" (only in priority 50)
	if result.State["emotion"] != "happy" {
		t.Errorf("expected emotion=happy, got %v", result.State["emotion"])
	}
}

func TestMergeDecisions_Empty(t *testing.T) {
	result := MergeDecisions(nil)

	if result.Intent != nil {
		t.Errorf("expected nil intent for empty input")
	}
	if len(result.ToolsNeeded) != 0 {
		t.Errorf("expected empty tools for empty input")
	}
	if len(result.SkillsNeeded) != 0 {
		t.Errorf("expected empty skills for empty input")
	}
	if result.BehaviorText != "" {
		t.Errorf("expected empty behavior text for empty input")
	}
}
