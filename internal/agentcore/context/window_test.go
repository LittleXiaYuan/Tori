package context

import (
	"strings"
	"testing"
)

func TestEstimateTokensEmpty(t *testing.T) {
	if EstimateTokens("") != 0 {
		t.Fatal("empty string should be 0 tokens")
	}
}

func TestEstimateTokensLatin(t *testing.T) {
	tokens := EstimateTokens("Hello world, this is a test message.")
	if tokens < 5 || tokens > 20 {
		t.Fatalf("unexpected token count for latin: %d", tokens)
	}
}

func TestEstimateTokensCJK(t *testing.T) {
	tokens := EstimateTokens("你好世界，这是一条测试消息。")
	if tokens < 5 {
		t.Fatalf("unexpected token count for CJK: %d", tokens)
	}
}

func TestTrimToFitUnderBudget(t *testing.T) {
	msgs := []Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hi"},
		{Role: "assistant", Content: "Hello!"},
	}
	result := TrimToFit(msgs, DefaultConfig())
	if result.DroppedCount != 0 {
		t.Fatalf("should not drop: dropped %d", result.DroppedCount)
	}
	if len(result.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result.Messages))
	}
}

func TestTrimToFitOverBudget(t *testing.T) {
	msgs := make([]Message, 0, 50)
	msgs = append(msgs, Message{Role: "system", Content: "System prompt."})
	for i := 0; i < 48; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		msgs = append(msgs, Message{Role: role, Content: strings.Repeat("word ", 200)})
	}
	msgs = append(msgs, Message{Role: "user", Content: "Latest question?"})

	cfg := WindowConfig{
		MaxTokens:     2000,
		SystemReserve: 100,
		ReplyReserve:  100,
		MaxMessages:   100,
		PreserveFirst: 1,
		PreserveLast:  1,
	}
	result := TrimToFit(msgs, cfg)
	if result.DroppedCount == 0 {
		t.Fatal("should have dropped messages")
	}
	if result.Messages[0].Role != "system" {
		t.Fatal("first message should be system")
	}
	if result.Messages[len(result.Messages)-1].Content != "Latest question?" {
		t.Fatal("last message should be preserved")
	}
	if result.TotalTokens > 1800 {
		t.Fatalf("total tokens over budget: %d", result.TotalTokens)
	}
}

func TestTrimToFitMaxMessages(t *testing.T) {
	msgs := make([]Message, 20)
	for i := range msgs {
		msgs[i] = Message{Role: "user", Content: "msg"}
	}
	cfg := DefaultConfig()
	cfg.MaxMessages = 5
	result := TrimToFit(msgs, cfg)
	if len(result.Messages) > 5 {
		t.Fatalf("expected max 5, got %d", len(result.Messages))
	}
}

func TestTrimToFitPreservation(t *testing.T) {
	msgs := []Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: strings.Repeat("a", 5000)},
		{Role: "assistant", Content: strings.Repeat("b", 5000)},
		{Role: "user", Content: "latest"},
	}
	cfg := WindowConfig{
		MaxTokens:     500,
		SystemReserve: 50,
		ReplyReserve:  50,
		MaxMessages:   100,
		PreserveFirst: 1,
		PreserveLast:  1,
	}
	result := TrimToFit(msgs, cfg)
	if result.Messages[0].Content != "sys" {
		t.Fatal("first should be preserved")
	}
	if result.Messages[len(result.Messages)-1].Content != "latest" {
		t.Fatal("last should be preserved")
	}
}

func TestPruneToolOutputShort(t *testing.T) {
	text := "short output"
	pruned := PruneToolOutput(text, 1000)
	if pruned != text {
		t.Fatal("short text should not be pruned")
	}
}

func TestPruneToolOutputLong(t *testing.T) {
	text := strings.Repeat("line of output\n", 500)
	pruned := PruneToolOutput(text, 1000)
	if len(pruned) >= len(text) {
		t.Fatal("long text should be pruned")
	}
	if !strings.Contains(pruned, "pruned") {
		t.Fatal("should contain pruned marker")
	}
}
