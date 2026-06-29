package cogni

import (
	"context"
	"testing"

	"yunque-agent/pkg/cogni"
)

// mockV1Hook is a minimal v1 Hook implementation for testing the compat adapter.
type mockV1Hook struct {
	contextText string
}

func (m *mockV1Hook) BuildContext(req cogni.ContextRequest) string {
	return m.contextText
}

func (m *mockV1Hook) FilterSkills(req cogni.ContextRequest, in []interface{}) []interface{} {
	// Not used by compat adapter (handled separately in prompt builder)
	return in
}

func TestV1CompatAdapter_Analyze(t *testing.T) {
	v1 := &mockV1Hook{
		contextText: "Test behavioral guidance from v1 Cogni",
	}

	adapter := NewV1CompatAdapter(v1)

	req := CogniRequest{
		Message:  "test message",
		TenantID: "test-tenant",
		Channel:  "web",
	}

	decision := adapter.Analyze(context.Background(), req)

	// v1 compat adapter should only populate BehaviorText
	if decision.BehaviorText != v1.contextText {
		t.Errorf("expected BehaviorText=%q, got %q", v1.contextText, decision.BehaviorText)
	}

	// v1 should not participate in intent detection
	if decision.Intent != nil {
		t.Errorf("expected nil Intent, got %v", decision.Intent)
	}

	// v1 should not filter tools
	if len(decision.ToolsNeeded) != 0 {
		t.Errorf("expected empty ToolsNeeded, got %v", decision.ToolsNeeded)
	}

	// v1 should not filter skills
	if len(decision.SkillsNeeded) != 0 {
		t.Errorf("expected empty SkillsNeeded, got %v", decision.SkillsNeeded)
	}

	// v1 should not constrain memory
	if decision.MemoryScope.Limit != 0 || len(decision.MemoryScope.Categories) != 0 {
		t.Errorf("expected empty MemoryScope, got %+v", decision.MemoryScope)
	}

	// v1 should not expose structured state
	if decision.State != nil {
		t.Errorf("expected nil State, got %v", decision.State)
	}
}

func TestV1CompatAdapter_Priority(t *testing.T) {
	v1 := &mockV1Hook{contextText: "test"}
	adapter := NewV1CompatAdapter(v1)

	// v1 compat adapters should have priority 0 (lowest)
	if adapter.Priority() != 0 {
		t.Errorf("expected Priority=0, got %d", adapter.Priority())
	}
}

func TestV1CompatAdapter_NilHook(t *testing.T) {
	adapter := NewV1CompatAdapter(nil)

	req := CogniRequest{
		Message:  "test",
		TenantID: "test",
		Channel:  "web",
	}

	decision := adapter.Analyze(context.Background(), req)

	// Should return zero decision without crashing
	if decision.BehaviorText != "" {
		t.Errorf("expected empty decision for nil hook, got %+v", decision)
	}
}

func TestV1CompatAdapter_InMerge(t *testing.T) {
	// Test that v1 compat adapter works correctly in MergeDecisions

	v1 := &mockV1Hook{
		contextText: "V1 behavioral text",
	}
	adapter := NewV1CompatAdapter(v1)

	v2Decision := CogniDecision{
		Intent: &Intent{Type: "search", Confidence: 0.9},
		ToolsNeeded: []string{"browser_search"},
		SkillsNeeded: []string{"research"},
		BehaviorText: "V2 behavioral text",
	}

	cognis := []CogniWithPriority{
		{
			Decision: v2Decision,
			Priority: 100, // v2 has higher priority
		},
		{
			Decision: adapter.Analyze(context.Background(), CogniRequest{}),
			Priority: adapter.Priority(), // v1 compat has priority 0
		},
	}

	result := MergeDecisions(cognis)

	// Intent should come from v2 (v1 doesn't provide intent)
	if result.Intent == nil || result.Intent.Type != "search" {
		t.Errorf("expected intent=search from v2, got %v", result.Intent)
	}

	// Tools should come from v2 (v1 doesn't provide tools)
	if len(result.ToolsNeeded) != 1 || result.ToolsNeeded[0] != "browser_search" {
		t.Errorf("expected tools from v2, got %v", result.ToolsNeeded)
	}

	// BehaviorText should have v2 first (higher priority), then v1
	expectedBehavior := "V2 behavioral text\n\nV1 behavioral text"
	if result.BehaviorText != expectedBehavior {
		t.Errorf("expected:\n%s\n\ngot:\n%s", expectedBehavior, result.BehaviorText)
	}
}
