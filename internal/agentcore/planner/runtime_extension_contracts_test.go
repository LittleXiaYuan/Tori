package planner

import (
	"context"
	"testing"
	"time"

	"yunque-agent/pkg/skills"
)

func TestRuntimeExtensionContracts(t *testing.T) {
	t.Parallel()

	var metricsCalled bool
	metrics := SkillMetricsFunc(func(skillName string, duration time.Duration, err error) {
		metricsCalled = skillName == "search" && duration == time.Second && err == nil
	})
	metrics("search", time.Second, nil)
	if !metricsCalled {
		t.Fatal("expected skill metrics callback to receive invocation data")
	}

	index := SkillIndexFunc(func() []SkillIndexEntry {
		return []SkillIndexEntry{{Slug: "search", Description: "searches trusted sources"}}
	})()
	if len(index) != 1 || index[0].Slug != "search" {
		t.Fatalf("unexpected skill index entries: %#v", index)
	}

	memory := MemorySearchFunc(func(ctx context.Context, tenantID, query string) string {
		if ctx == nil || tenantID != "tenant" || query != "query" {
			t.Fatalf("unexpected memory callback input: tenant=%q query=%q", tenantID, query)
		}
		return "memory"
	})
	if got := memory(context.Background(), "tenant", "query"); got != "memory" {
		t.Fatalf("unexpected memory result: %q", got)
	}

	filter := CogniSkillFilterFunc(func(message, tenantID, channel string, in []skills.Skill) []skills.Skill {
		if message != "msg" || tenantID != "tenant" || channel != "chat" {
			t.Fatalf("unexpected filter input: %q %q %q", message, tenantID, channel)
		}
		return in
	})
	if out := filter("msg", "tenant", "chat", nil); out != nil {
		t.Fatalf("expected nil skill slice to round-trip, got %#v", out)
	}

	if DynContextBudgetDefault != 0 {
		t.Fatalf("expected default dynamic-context budget sentinel to remain 0, got %d", DynContextBudgetDefault)
	}
}
