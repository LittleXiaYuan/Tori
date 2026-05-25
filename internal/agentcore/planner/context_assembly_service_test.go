package planner

import (
	"context"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/observe"
	"yunque-agent/pkg/skills"
)

type stubCogniRuntime struct {
	context string
	trace   CogniTraceDetail
}

func (s stubCogniRuntime) BuildContext(_ context.Context, message, _, _ string) string {
	return s.context + ":" + message
}

func (s stubCogniRuntime) FilterSkills(_ string, _ string, _ string, in []skills.Skill) []skills.Skill {
	return in[:1]
}

func (s stubCogniRuntime) Trace(_ string, _ string, _ string) (CogniTraceDetail, bool) {
	return s.trace, true
}

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
	if got := service.GraphContextFor("q"); got != "graph:q" {
		t.Fatalf("unexpected graph context: %q", got)
	}
}

func TestContextAssemblyServiceAppendGraphContext(t *testing.T) {
	service := NewContextAssemblyService()
	service.AppendGraphContext(func(query string) string { return "first:" + query })
	service.AppendGraphContext(func(query string) string { return "second:" + query })

	got := service.GraphContextFor("q")
	want := "first:q\n---\nsecond:q"
	if got != want {
		t.Fatalf("unexpected appended graph context: got %q want %q", got, want)
	}

	service.AppendGraphContext(func(query string) string { return "  " })
	if got := service.GraphContextFor("q"); got != want {
		t.Fatalf("empty appended context should be ignored: %q", got)
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

	if got := service.CogniContext(context.Background(), "hello", "tenant", "web"); got != "ctx:hello" {
		t.Fatalf("unexpected cogni context: %q", got)
	}
	filtered := service.ApplyCogniSkillFilter("hello", "tenant", "web", []skills.Skill{dummyPlannerSkill("a"), dummyPlannerSkill("b")})
	if len(filtered) != 1 || filtered[0].Name() != "a" {
		t.Fatalf("unexpected filtered skills: %#v", filtered)
	}
	var emitted observe.AgentEvent
	service.EmitCogniTrace("hello", "tenant", "web", "trace-id", "task-id", func(evt observe.AgentEvent) {
		emitted = evt
	})
	if emitted.Summary == "" || emitted.Meta.TenantID != "tenant" || emitted.Meta.TaskID != "task-id" {
		t.Fatalf("expected cogni trace event, got %#v", emitted)
	}
	detail, ok := emitted.Detail.(CogniTraceDetail)
	if !ok || len(detail.Activated) != 1 || detail.Activated[0] != "demo" {
		t.Fatalf("unexpected trace detail: %#v", emitted.Detail)
	}
}

func TestContextAssemblyServiceCogniRuntimeBoundary(t *testing.T) {
	service := NewContextAssemblyService()
	service.SetCogniRuntime(stubCogniRuntime{
		context: "runtime",
		trace:   CogniTraceDetail{Activated: []string{"runtime-cogni"}, ContextBytes: 8},
	})

	if got := service.CogniContext(context.Background(), "hello", "tenant", "web"); got != "runtime:hello" {
		t.Fatalf("unexpected cogni runtime context: %q", got)
	}
	filtered := service.ApplyCogniSkillFilter("hello", "tenant", "web", []skills.Skill{dummyPlannerSkill("a"), dummyPlannerSkill("b")})
	if len(filtered) != 1 || filtered[0].Name() != "a" {
		t.Fatalf("unexpected runtime-filtered skills: %#v", filtered)
	}
	var emitted observe.AgentEvent
	service.EmitCogniTraceForRequest(PlanRequest{
		Messages:    []llm.Message{{Role: "user", Content: "hello"}},
		TenantID:    "tenant",
		ChannelType: "web",
		TraceID:     "trace-id",
		TaskID:      "task-id",
		StepCallback: func(evt observe.AgentEvent) {
			emitted = evt
		},
	})
	detail, ok := emitted.Detail.(CogniTraceDetail)
	if !ok || detail.ContextBytes != 8 || len(detail.Activated) != 1 || detail.Activated[0] != "runtime-cogni" {
		t.Fatalf("unexpected runtime trace detail: %#v", emitted.Detail)
	}
}

func TestNilContextAssemblyServiceIsNoop(t *testing.T) {
	var service *ContextAssemblyService
	if got := service.Memory(context.Background(), "tenant", "query"); got != "" {
		t.Fatalf("nil service should return empty memory, got %q", got)
	}
	if got := service.GraphContextFor("query"); got != "" {
		t.Fatalf("nil service should have no graph context, got %q", got)
	}
	in := []skills.Skill{dummyPlannerSkill("a")}
	if got := service.ApplyCogniSkillFilter("msg", "tenant", "web", in); len(got) != 1 || got[0].Name() != "a" {
		t.Fatalf("nil service should keep skill input unchanged: %#v", got)
	}
}

func TestContextAssemblyServiceBuildDynamicContext(t *testing.T) {
	service := NewContextAssemblyService()
	service.SetMemory(func(_ context.Context, tenantID, query string) string {
		return "memory:" + tenantID + ":" + query
	})
	builder := &PromptBuilder{
		contextAssembly: service,
		dynBudget:       1000,
	}

	got := service.BuildDynamicContext(context.Background(), DynamicContextAssemblyRequest{
		LastMessage: "需要读取长期任务",
		TenantID:    "tenant",
		Channel:     "web",
		TaskContext: "task context",
	}, builder)

	if got.Content == "" {
		t.Fatal("expected dynamic context content")
	}
	if len(got.IncludedLayers) == 0 {
		t.Fatalf("expected included layers, got %#v", got)
	}
	if builder.LastIncludedLayers == nil || len(builder.LastIncludedLayers) == 0 {
		t.Fatal("expected builder to record included layers")
	}

	msgs, layers := service.AppendDynamicContextMessage(context.Background(), []llm.Message{{Role: "system", Content: "stable"}}, DynamicContextAssemblyRequest{
		LastMessage: "需要读取长期任务",
		TenantID:    "tenant",
		Channel:     "web",
	}, builder)
	if len(msgs) != 2 || !strings.HasPrefix(msgs[1].Content, "[动态上下文]\n") {
		t.Fatalf("expected dynamic context system message, got %#v", msgs)
	}
	if len(layers) == 0 {
		t.Fatalf("expected layers from append helper")
	}
}
