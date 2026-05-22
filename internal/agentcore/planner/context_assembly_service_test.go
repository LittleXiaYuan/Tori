package planner

import (
	"context"
	"testing"

	"yunque-agent/pkg/skills"
)

func TestContextAssemblyServiceMemoryAndGraph(t *testing.T) {
	service := NewContextAssemblyService()
	service.SetMemory(func(_ context.Context, tenantID, query string) string {
		return tenantID + ":" + query
	})
	service.SetGraphContext(func(query string) string {
		return "graph:" + query
	})

	if got := service.Memory(context.Background(), "tenant", "query"); got != "tenant:query" {
		t.Fatalf("unexpected memory context: %q", got)
	}
	graph := service.GraphContext()
	if graph == nil || graph("q") != "graph:q" {
		t.Fatalf("unexpected graph context")
	}
}

func TestContextAssemblyServiceCogniBoundary(t *testing.T) {
	service := NewContextAssemblyService()
	service.SetCogniContext(func(_ context.Context, message, _, _ string) string {
		return "ctx:" + message
	})
	service.SetCogniSkillFilter(func(_ string, _, _ string, in []skills.Skill) []skills.Skill {
		return in[:1]
	})
	service.SetCogniTrace(func(_ string, _, _ string) (CogniTraceDetail, bool) {
		return CogniTraceDetail{Activated: []string{"demo"}, ContextBytes: 3}, true
	})

	if service.cogniService == nil {
		t.Fatal("expected cogni service to be created")
	}
	if got := service.cogniService.Context(context.Background(), "hello", "tenant", "web"); got != "ctx:hello" {
		t.Fatalf("unexpected cogni context: %q", got)
	}
	filtered := service.FilterCogniSkills("hello", "tenant", "web", []skills.Skill{dummyPlannerSkill("a"), dummyPlannerSkill("b")})
	if len(filtered) != 1 || filtered[0].Name() != "a" {
		t.Fatalf("unexpected filtered skills: %#v", filtered)
	}
	if !service.HasCogniTrace() {
		t.Fatal("expected trace to be enabled")
	}
	trace, ok := service.CogniTrace("hello", "tenant", "web")
	if !ok || len(trace.Activated) != 1 || trace.Activated[0] != "demo" {
		t.Fatalf("unexpected trace: %#v ok=%v", trace, ok)
	}
}

func TestNilContextAssemblyServiceIsNoop(t *testing.T) {
	var service *ContextAssemblyService
	if got := service.Memory(context.Background(), "tenant", "query"); got != "" {
		t.Fatalf("nil service should return empty memory, got %q", got)
	}
	if got := service.GraphContext(); got != nil {
		t.Fatalf("nil service should have no graph context")
	}
	in := []skills.Skill{dummyPlannerSkill("a")}
	if got := service.FilterCogniSkills("msg", "tenant", "web", in); len(got) != 1 || got[0].Name() != "a" {
		t.Fatalf("nil service should keep skill input unchanged: %#v", got)
	}
}
