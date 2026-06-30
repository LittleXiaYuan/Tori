package planner

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	agentcogni "yunque-agent/internal/agentcore/cogni"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/observe"
	"yunque-agent/pkg/skills"
)

type stubCogniRuntime struct {
	context string
	trace   CogniTraceDetail
}

// Decide satisfies the v2 CogniRuntime entry point. The stub returns an empty
// decision; tests exercise the legacy BuildContext/FilterSkills/Trace path.
func (s stubCogniRuntime) Decide(_ context.Context, _, _, _, _ string) agentcogni.CogniFinalDecision {
	return agentcogni.CogniFinalDecision{}
}

func (s stubCogniRuntime) BuildContext(_ context.Context, message, _, _, _ string) string {
	return s.context + ":" + message
}

func (s stubCogniRuntime) FilterSkills(_ string, _ string, _ string, in []skills.Skill) []skills.Skill {
	return in[:1]
}

func (s stubCogniRuntime) Trace(_ string, _ string, _ string) (CogniTraceDetail, bool) {
	return s.trace, true
}

func (s stubCogniRuntime) Tools(_ context.Context, _, _, _ string) []CogniTool {
	return nil
}

func (s stubCogniRuntime) SurfaceAuthoritative(_ string, _ string, _ string) bool {
	return false
}

func (s stubCogniRuntime) RecordToolOutcome(_ string, _ string, _ string, _ string, _ bool) {}

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

func TestContextAssemblyServiceCogniRuntimeBoundary(t *testing.T) {
	service := NewContextAssemblyService()
	service.SetCogniRuntime(stubCogniRuntime{
		context: "runtime",
		trace:   CogniTraceDetail{Activated: []string{"runtime-cogni"}, ContextBytes: 8},
	})

	if got := service.CogniContext(context.Background(), "hello", "tenant", "web", ""); got != "runtime:hello" {
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
		SessionID:   "session-id",
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
	if emitted.Meta.SessionID != "session-id" {
		t.Fatalf("expected cogni trace to carry session metadata, got %#v", emitted.Meta)
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

// Casual-chat turns (LocalBrain IntentHint == "chat", short message) must keep
// the memory layer but skip graph/ledger recall and code retrieval — both carry
// network hops with near-zero relevance for small talk. Regression guard for
// the IntentHint plumbing: without it casualChat can never fire.
func TestBuildDynamicContextCasualChatSkipsGraphAndCode(t *testing.T) {
	type calls struct {
		memory, graph, code atomic.Bool
	}

	run := func(intentHint string) *calls {
		c := &calls{}
		service := NewContextAssemblyService()
		service.SetMemory(func(_ context.Context, _, _ string) string {
			c.memory.Store(true)
			return "memory context"
		})
		service.SetGraphContext(func(_ string) string {
			c.graph.Store(true)
			return "graph context"
		})
		service.SetCodeContext(func(_ string) string {
			c.code.Store(true)
			return "code context"
		})
		builder := &PromptBuilder{contextAssembly: service, dynBudget: 1000}
		service.BuildDynamicContext(context.Background(), DynamicContextAssemblyRequest{
			LastMessage: "你好呀，今天过得怎么样", // > 6 runes (not skipRetrieval), < 24 (casual when chat)
			TenantID:    "tenant",
			Channel:     "web",
			IntentHint:  intentHint,
		}, builder)
		return c
	}

	chat := run("chat")
	if !chat.memory.Load() {
		t.Fatal("casual chat must still query the memory layer")
	}
	if chat.graph.Load() || chat.code.Load() {
		t.Fatalf("casual chat must skip graph/code retrieval, got graph=%v code=%v", chat.graph.Load(), chat.code.Load())
	}

	unclassified := run("")
	if !unclassified.memory.Load() || !unclassified.graph.Load() || !unclassified.code.Load() {
		t.Fatalf("without intent hint all retrieval layers must run, got memory=%v graph=%v code=%v",
			unclassified.memory.Load(), unclassified.graph.Load(), unclassified.code.Load())
	}
}
