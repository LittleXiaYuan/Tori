package planner

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestPlannerRuntimeSettersAndFacades(t *testing.T) {
	t.Parallel()

	p := NewPlanner(nil, nil, 2)
	if p.maxPlanSteps() != 2 {
		t.Fatalf("expected constructor max steps, got %d", p.maxPlanSteps())
	}

	p.SetToolTimeout(2 * time.Second)
	if got := p.perToolTimeout(); got != 2*time.Second {
		t.Fatalf("expected tool timeout to be routed through execution runtime, got %s", got)
	}

	p.SetDynContextBudget(123)
	if got := p.dynamicContextBudget(); got != 123 {
		t.Fatalf("expected dyn context budget to be routed through execution runtime, got %d", got)
	}

	p.SetMemory(func(ctx context.Context, tenantID, query string) string {
		if tenantID != "tenant" || query != "query" {
			t.Fatalf("unexpected memory callback input: tenant=%q query=%q", tenantID, query)
		}
		return "remembered"
	})
	if got := p.ensureContextAssembly().Memory(context.Background(), "tenant", "query"); got != "remembered" {
		t.Fatalf("expected memory callback to be wired through context assembly, got %q", got)
	}

	p.SetReflect(func(ctx context.Context, intent, reply string) bool {
		return intent == "intent" && reply == "reply"
	})
	if p.reflect == nil || !p.reflect(context.Background(), "intent", "reply") {
		t.Fatal("expected reflect setter to store callback")
	}

	p.SetSkillMetrics(func(skillName string, duration time.Duration, err error) {})
	if p.skillMetrics == nil {
		t.Fatal("expected skill metrics setter to store callback")
	}

	var nilPlanner *Planner
	if got := nilPlanner.ModelRuntimeHealth(); got.Configured {
		t.Fatalf("expected nil planner health to be unconfigured, got %#v", got)
	}
	if got := nilPlanner.GenerateConversationTitle(context.Background(), "u", "a"); got != "" {
		t.Fatalf("expected nil planner title generation to be empty, got %q", got)
	}
	if _, err := nilPlanner.ParseMissionIntent(context.Background(), "mission"); err == nil || !strings.Contains(err.Error(), "planner or llm not configured") {
		t.Fatalf("expected nil planner mission parse error, got %v", err)
	}
}
