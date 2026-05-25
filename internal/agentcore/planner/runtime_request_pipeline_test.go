package planner

import (
	"context"
	"testing"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/localbrain"
	"yunque-agent/pkg/skills"
)

func TestRuntimeRequestPipelineDisableToolsUsesToolFreeChat(t *testing.T) {
	calls := 0
	client := mockLLMServer(t, func(_ []llm.Message) string {
		calls++
		return "纯聊天回复。\n---NEXT---\n- 继续阅读代码\n- 运行针对性测试"
	})
	p := NewPlanner(client, skills.NewRegistry(), 3)

	result, err := p.runInner(context.Background(), PlanRequest{
		Messages:     []llm.Message{{Role: "user", Content: "你好"}},
		TenantID:     "tenant-pipeline",
		DisableTools: true,
	})
	if err != nil {
		t.Fatalf("run inner: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected one tool-free chat call, got %d", calls)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Steps != 0 {
		t.Fatalf("DisableTools path should report zero tool steps, got %d", result.Steps)
	}
	if result.Reply != "纯聊天回复。" {
		t.Fatalf("unexpected cleaned reply: %q", result.Reply)
	}
	if len(result.Suggestions) != 2 || result.Suggestions[0] != "继续阅读代码" || result.Suggestions[1] != "运行针对性测试" {
		t.Fatalf("unexpected suggestions: %#v", result.Suggestions)
	}
}

func TestRuntimeRequestPipelineLocalBrainToolFreeUsesToolFreeChat(t *testing.T) {
	calls := 0
	client := mockLLMServer(t, func(_ []llm.Message) string {
		calls++
		return "LocalBrain 仍只走聊天。\n---NEXT---\n1. 保持无工具路径"
	})
	p := NewPlanner(client, skills.NewRegistry(), 3)
	p.SetLocalBrain(localbrain.New(nil, nil))

	result, err := p.runInner(context.Background(), PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
		TenantID: "tenant-localbrain",
	})
	if err != nil {
		t.Fatalf("run inner: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected one LocalBrain tool-free chat call, got %d", calls)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Steps != 1 {
		t.Fatalf("LocalBrain tool-free path should keep one classification step, got %d", result.Steps)
	}
	if result.Reply != "LocalBrain 仍只走聊天。" {
		t.Fatalf("unexpected cleaned reply: %q", result.Reply)
	}
	if len(result.Suggestions) != 1 || result.Suggestions[0] != "保持无工具路径" {
		t.Fatalf("unexpected suggestions: %#v", result.Suggestions)
	}
}

func TestRuntimeRequestLifecycleRunInvokesLearningSidecarAfterRun(t *testing.T) {
	client := mockLLMServer(t, func(_ []llm.Message) string {
		return "生命周期完成。"
	})
	p := NewPlanner(client, skills.NewRegistry(), 3)
	meta := &fakeMetaCogSidecar{summary: "metacog[task-lifecycle]: ok"}
	p.ensureLearningSidecar().SetMetaCogSidecar(meta)

	result, err := p.Run(context.Background(), PlanRequest{
		Messages:     []llm.Message{{Role: "user", Content: "你好"}},
		TenantID:     "tenant-lifecycle",
		TaskID:       "task-lifecycle",
		DisableTools: true,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result == nil || result.Reply != "生命周期完成。" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if meta.cleared != "task-lifecycle" {
		t.Fatalf("expected Run lifecycle to invoke LearningSidecar.AfterRun, cleared=%q", meta.cleared)
	}
}
