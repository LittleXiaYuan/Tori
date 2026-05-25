package planner

import (
	"context"
	"strings"
	"testing"

	ctxwindow "yunque-agent/internal/agentcore/context"
	"yunque-agent/internal/agentcore/llm"
)

func TestContextWindowRuntimeServicePrunesToolResults(t *testing.T) {
	service := NewContextWindowRuntimeService()
	msgs := []llm.Message{{Role: "tool", Content: strings.Repeat("a", 200)}}
	service.PruneToolResults(msgs, 80)
	if len(msgs[0].Content) >= 200 {
		t.Fatal("expected tool output to be pruned")
	}
	if !strings.Contains(msgs[0].Content, "pruned") {
		t.Fatalf("expected pruning marker, got %q", msgs[0].Content)
	}
}

func TestContextWindowRuntimeServiceCompressesAndTrims(t *testing.T) {
	service := NewContextWindowRuntimeService()
	service.SetManager(ctxwindow.NewManager(ctxwindow.ManagerConfig{
		MaxContextTokens: 1,
		EnforceMaxTurns:  1,
		Compressor:       ctxwindow.NewTruncateCompressor(1, 0.1),
	}))
	service.SetWindowConfig(ctxwindow.WindowConfig{
		MaxTokens:     64,
		SystemReserve: 8,
		ReplyReserve:  8,
		MaxMessages:   4,
		PreserveFirst: 1,
		PreserveLast:  1,
	})
	msgs := []llm.Message{
		{Role: "system", Content: "system"},
		{Role: "user", Content: strings.Repeat("历史消息", 80)},
		{Role: "assistant", Content: strings.Repeat("旧回答", 80)},
		{Role: "user", Content: strings.Repeat("当前问题", 80)},
	}
	got := service.CompressAndTrim(context.Background(), msgs, nil)
	if len(got) >= len(msgs) {
		t.Fatalf("expected compression or trimming to reduce messages, got %d >= %d", len(got), len(msgs))
	}
	if got[0].Role != "system" {
		t.Fatalf("expected system message preserved, got %#v", got[0])
	}
}

func TestContextWindowRuntimeServiceFitsMessagesForRequest(t *testing.T) {
	service := NewContextWindowRuntimeService()
	service.SetWindowConfig(ctxwindow.WindowConfig{
		MaxTokens:     64,
		SystemReserve: 8,
		ReplyReserve:  8,
		MaxMessages:   3,
		PreserveFirst: 1,
		PreserveLast:  1,
	})
	msgs := []llm.Message{
		{Role: "system", Content: "system"},
		{Role: "tool", Content: strings.Repeat("tool-output", 900)},
		{Role: "assistant", Content: strings.Repeat("old", 300)},
		{Role: "user", Content: strings.Repeat("current", 300)},
	}

	got := service.FitMessagesForRequest(context.Background(), msgs, nil)
	for _, msg := range got {
		if msg.Role == "tool" && len(msg.Content) >= len(strings.Repeat("tool-output", 900)) {
			t.Fatalf("expected tool output to be pruned, got %d bytes", len(msg.Content))
		}
	}
	if len(got) > 3 {
		t.Fatalf("expected window trim to enforce max messages, got %d", len(got))
	}
}

func TestNilContextWindowRuntimeServiceIsNoop(t *testing.T) {
	var service *ContextWindowRuntimeService
	msgs := []llm.Message{{Role: "user", Content: "hello"}}
	got := service.CompressAndTrim(context.Background(), msgs, nil)
	if len(got) != 1 || got[0].Content != "hello" {
		t.Fatalf("nil service should not change messages, got %#v", got)
	}
	service.PruneToolResults(msgs, 1)
	if got := service.WindowConfig(); got != nil {
		t.Fatalf("nil service should have no window config, got %#v", got)
	}
	if got := service.Manager(); got != nil {
		t.Fatalf("nil service should have no manager, got %#v", got)
	}
}
