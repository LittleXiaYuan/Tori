package context

import (
	"strings"
	"testing"
)

func TestLayerAssemblerBasic(t *testing.T) {
	la := NewLayerAssembler(0) // unlimited budget

	layers := []Layer{
		{Name: "hints", Priority: LayerPriorityHints, Content: "## Hints\noptimize more"},
		{Name: "memory", Priority: LayerPriorityMemory, Content: "## Memory\nuser prefers dark theme"},
		{Name: "task", Priority: LayerPriorityTask, Content: "## Task\ngoal: analyze data"},
	}

	assembled, included := la.Assemble(layers)
	if len(included) != 3 {
		t.Fatalf("expected 3 layers included, got %d", len(included))
	}
	// Should be sorted by priority: task (10) → memory (20) → hints (50)
	if included[0] != "task" {
		t.Fatalf("expected 'task' first, got '%s'", included[0])
	}
	if included[1] != "memory" {
		t.Fatalf("expected 'memory' second, got '%s'", included[1])
	}
	if included[2] != "hints" {
		t.Fatalf("expected 'hints' third, got '%s'", included[2])
	}
	if !strings.Contains(assembled, "Task") {
		t.Fatal("assembled should contain task content")
	}
}

func TestLayerAssemblerBudget(t *testing.T) {
	// Very tight budget: only ~50 tokens
	la := NewLayerAssembler(50)

	layers := []Layer{
		{Name: "task", Priority: LayerPriorityTask, Content: "Task context", Tokens: 20},
		{Name: "memory", Priority: LayerPriorityMemory, Content: "Memory context", Tokens: 20},
		{Name: "hints", Priority: LayerPriorityHints, Content: "Skill hints with lots of extra text", Tokens: 30},
	}

	_, included := la.Assemble(layers)
	// task(20) + memory(20) = 40, fits. task(20) + memory(20) + hints(30) = 70, doesn't fit.
	if len(included) != 2 {
		t.Fatalf("expected 2 layers within budget, got %d: %v", len(included), included)
	}
	// Hints should be dropped (lowest priority)
	for _, name := range included {
		if name == "hints" {
			t.Fatal("hints should be dropped due to budget")
		}
	}
}

func TestLayerAssemblerEmptyLayers(t *testing.T) {
	la := NewLayerAssembler(0)

	layers := []Layer{
		{Name: "empty1", Priority: LayerPriorityTask, Content: ""},
		{Name: "empty2", Priority: LayerPriorityMemory, Content: ""},
		{Name: "valid", Priority: LayerPriorityCognition, Content: "Some content"},
	}

	assembled, included := la.Assemble(layers)
	if len(included) != 1 {
		t.Fatalf("expected 1 layer (empty filtered out), got %d", len(included))
	}
	if included[0] != "valid" {
		t.Fatalf("expected 'valid', got '%s'", included[0])
	}
	if assembled != "Some content" {
		t.Fatalf("unexpected assembled: %s", assembled)
	}
}

func TestLayerAssemblerAllEmpty(t *testing.T) {
	la := NewLayerAssembler(0)
	assembled, included := la.Assemble([]Layer{
		{Name: "a", Priority: LayerPriorityTask, Content: ""},
	})
	if assembled != "" {
		t.Fatal("expected empty assembled")
	}
	if included != nil {
		t.Fatal("expected nil included")
	}
}

func TestEstimateTokens(t *testing.T) {
	// English: ~4 chars/token → "hello world" (11 chars, ~3 tokens)
	tokens := estimateTokens("hello world")
	if tokens < 1 {
		t.Fatal("expected positive token count")
	}

	// Chinese: ~2 chars/token → "你好世界" (4 runes)
	tokensCN := estimateTokens("你好世界")
	if tokensCN < 1 {
		t.Fatal("expected positive token count for Chinese")
	}
}

func TestLayerAssemblerAutoEstimate(t *testing.T) {
	la := NewLayerAssembler(5) // Very tight: ~5 tokens

	layers := []Layer{
		{Name: "short", Priority: LayerPriorityTask, Content: "hi"},                                // ~1 token
		{Name: "long", Priority: LayerPriorityMemory, Content: strings.Repeat("这是一段很长的中文内容", 100)}, // many tokens
	}

	_, included := la.Assemble(layers)
	// "short" fits, "long" should be dropped
	if len(included) != 1 {
		t.Fatalf("expected 1 layer, got %d: %v", len(included), included)
	}
	if included[0] != "short" {
		t.Fatalf("expected 'short' included, got '%s'", included[0])
	}
}
