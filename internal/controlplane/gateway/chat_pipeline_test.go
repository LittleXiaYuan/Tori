package gateway

import (
	"context"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/pkg/skills"
)

func TestExecuteChatPipelineRejectsNilRequest(t *testing.T) {
	g := &Gateway{}
	if _, err := g.ExecuteChatPipeline(context.Background(), nil); err == nil {
		t.Fatal("expected nil request error")
	}
}

func TestExecuteChatPipelineRequiresPlanner(t *testing.T) {
	g := &Gateway{}
	_, err := g.ExecuteChatPipeline(context.Background(), &ChatRequest{
		TenantID: "t1",
		Messages: []llm.Message{{Role: "user", Content: "hello"}},
	})
	if err == nil || !strings.Contains(err.Error(), "planner not configured") {
		t.Fatalf("expected planner configuration error, got %v", err)
	}
}

func TestExecuteChatPipelineRequiresConversationStoreForSessions(t *testing.T) {
	llmClient := llm.NewClient("http://localhost:0", "test", "test")
	g := &Gateway{
		planner: planner.NewPlanner(llmClient, skills.NewRegistry(), 4),
	}
	_, err := g.ExecuteChatPipeline(context.Background(), &ChatRequest{
		TenantID:  "t1",
		SessionID: "s1",
		Messages:  []llm.Message{{Role: "user", Content: "hello"}},
	})
	if err == nil || !strings.Contains(err.Error(), "conversation store not configured") {
		t.Fatalf("expected conversation store configuration error, got %v", err)
	}
}
