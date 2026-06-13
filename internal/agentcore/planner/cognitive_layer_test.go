package planner

import (
	"context"
	"testing"
)

// TestCognitiveLayerMasterGate verifies the runtime hot-toggle: when the master
// cognitive layer is off, BuildDynamicContext injects no cognitive context
// (only the active task's working memory survives), and the switch flips live
// via SetCognitiveLayerEnabled without a restart.
func TestCognitiveLayerMasterGate(t *testing.T) {
	if !CognitiveLayerEnabled() {
		t.Fatalf("cognitive layer should default to enabled")
	}

	SetCognitiveLayerEnabled(false)
	t.Cleanup(func() { SetCognitiveLayerEnabled(true) })

	if CognitiveLayerEnabled() {
		t.Fatalf("SetCognitiveLayerEnabled(false) should disable the layer")
	}

	// A zero-value PromptBuilder is sufficient: with the layer off,
	// BuildDynamicContext returns before touching any cognitive service.
	pb := &PromptBuilder{}

	// Layer off + a task context → only the task layer survives.
	got := pb.BuildDynamicContext(context.Background(), DynamicContextRequest{TaskContext: "TASK-CTX"})
	if got != "TASK-CTX" {
		t.Fatalf("layer off: want task context %q, got %q", "TASK-CTX", got)
	}
	if len(pb.LastIncludedLayers) != 1 || pb.LastIncludedLayers[0] != "task" {
		t.Fatalf("layer off with task: want [task], got %v", pb.LastIncludedLayers)
	}

	// Layer off + no task → no cognitive layers at all.
	got = pb.BuildDynamicContext(context.Background(), DynamicContextRequest{})
	if got != "" {
		t.Fatalf("layer off no task: want empty, got %q", got)
	}
	if len(pb.LastIncludedLayers) != 0 {
		t.Fatalf("layer off no task: want no layers, got %v", pb.LastIncludedLayers)
	}

	// Hot re-enable (no restart).
	SetCognitiveLayerEnabled(true)
	if !CognitiveLayerEnabled() {
		t.Fatalf("SetCognitiveLayerEnabled(true) should re-enable the layer")
	}
}
