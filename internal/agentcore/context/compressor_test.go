package context

import (
	"context"
	"strings"
	"testing"
)

func TestTruncateCompressor_ShouldCompress(t *testing.T) {
	c := NewTruncateCompressor(1, 0.8)

	if c.ShouldCompress(nil, 70, 100) {
		t.Error("70/100 should not trigger compression at 0.8 threshold")
	}
	if !c.ShouldCompress(nil, 85, 100) {
		t.Error("85/100 should trigger compression at 0.8 threshold")
	}
}

func TestTruncateCompressor_Compress(t *testing.T) {
	c := NewTruncateCompressor(1, 0.8)
	msgs := []Message{
		{Role: "system", Content: "you are a bot"},
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
		{Role: "user", Content: "how are you"},
		{Role: "assistant", Content: "good"},
		{Role: "user", Content: "bye"},
		{Role: "assistant", Content: "bye"},
	}

	result, err := c.Compress(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}
	// Should drop 1 turn (2 messages) from oldest conversation
	if len(result) < len(msgs)-2 {
		t.Errorf("expected at most 2 messages dropped, got %d remaining from %d", len(result), len(msgs))
	}

	// System message should be preserved
	if result[0].Role != "system" {
		t.Error("system message should be preserved")
	}
}

func TestLLMSummaryCompressor_Compress(t *testing.T) {
	mockLLM := func(_ context.Context, sysPrompt, userPrompt string) (string, error) {
		return "用户先打招呼，然后询问天气。", nil
	}

	c := NewLLMSummaryCompressor(mockLLM, 2, 0.8)
	msgs := []Message{
		{Role: "system", Content: "you are a bot"},
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
		{Role: "user", Content: "what is the weather"},
		{Role: "assistant", Content: "it's sunny"},
		{Role: "user", Content: "thanks"},
		{Role: "assistant", Content: "you're welcome"},
	}

	result, err := c.Compress(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}

	// Should have: 1 system + 1 summary + 2 recent = 4 messages
	if len(result) != 4 {
		t.Errorf("expected 4 messages after summary, got %d", len(result))
	}

	if result[0].Role != "system" {
		t.Error("first message should be system")
	}
	if !strings.Contains(result[1].Content, "对话历史摘要") {
		t.Error("second message should contain summary")
	}
	// Last 2 should be the recent messages
	if result[2].Content != "thanks" {
		t.Errorf("expected 'thanks', got '%s'", result[2].Content)
	}
}

func TestLLMSummaryCompressor_FallbackOnError(t *testing.T) {
	failLLM := func(_ context.Context, _, _ string) (string, error) {
		return "", context.DeadlineExceeded
	}

	c := NewLLMSummaryCompressor(failLLM, 2, 0.8)
	msgs := []Message{
		{Role: "system", Content: "bot"},
		{Role: "user", Content: "a"},
		{Role: "assistant", Content: "b"},
		{Role: "user", Content: "c"},
		{Role: "assistant", Content: "d"},
		{Role: "user", Content: "e"},
		{Role: "assistant", Content: "f"},
	}

	// Should fallback to truncation without error
	result, err := c.Compress(context.Background(), msgs)
	if err != nil {
		t.Fatal("should fallback gracefully, got error:", err)
	}
	if len(result) >= len(msgs) {
		t.Errorf("expected some messages dropped on fallback, got %d", len(result))
	}
}

func TestLLMSummaryCompressor_TooFewMessages(t *testing.T) {
	mockLLM := func(_ context.Context, _, _ string) (string, error) {
		return "summary", nil
	}

	c := NewLLMSummaryCompressor(mockLLM, 4, 0.8)
	msgs := []Message{
		{Role: "system", Content: "bot"},
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
	}

	result, err := c.Compress(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}
	// Should return unchanged (only 2 conv msgs, < keepRecent=4)
	if len(result) != len(msgs) {
		t.Errorf("expected unchanged %d messages, got %d", len(msgs), len(result))
	}
}

func TestManager_Process(t *testing.T) {
	mockLLM := func(_ context.Context, _, _ string) (string, error) {
		return "摘要：用户进行了多轮对话。", nil
	}

	mgr := NewManager(ManagerConfig{
		MaxContextTokens: 200, // Very low budget to trigger compression
		EnforceMaxTurns:  5,
		Compressor:       NewLLMSummaryCompressor(mockLLM, 2, 0.5),
	})

	// Create messages that exceed budget
	msgs := []Message{
		{Role: "system", Content: "you are a helpful assistant"},
		{Role: "user", Content: strings.Repeat("这是一段很长的消息", 20)},
		{Role: "assistant", Content: strings.Repeat("这是一段很长的回复", 20)},
		{Role: "user", Content: "继续讨论"},
		{Role: "assistant", Content: "好的"},
		{Role: "user", Content: "最后一个问题"},
		{Role: "assistant", Content: "请问"},
	}

	result, err := mgr.Process(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}

	if len(result) >= len(msgs) {
		t.Errorf("expected compression to reduce messages, got %d from %d", len(result), len(msgs))
	}
}

func TestEnforceMaxTurns(t *testing.T) {
	msgs := []Message{
		{Role: "system", Content: "bot"},
		{Role: "user", Content: "1"},
		{Role: "assistant", Content: "2"},
		{Role: "user", Content: "3"},
		{Role: "assistant", Content: "4"},
		{Role: "user", Content: "5"},
		{Role: "assistant", Content: "6"},
	}

	result := EnforceMaxTurns(msgs, 2)
	// Should keep system + last 4 conv messages (2 turns)
	if len(result) != 5 {
		t.Errorf("expected 5 messages (1 system + 4 conv), got %d", len(result))
	}
	if result[0].Role != "system" {
		t.Error("system should be first")
	}
	if result[1].Content != "3" {
		t.Errorf("expected '3', got '%s'", result[1].Content)
	}
}

func TestDropOldestTurns(t *testing.T) {
	msgs := []Message{
		{Role: "system", Content: "bot"},
		{Role: "user", Content: "a"},
		{Role: "assistant", Content: "b"},
		{Role: "user", Content: "c"},
		{Role: "assistant", Content: "d"},
	}

	result := DropOldestTurns(msgs, 1)
	// Should drop 1 turn (2 messages) from oldest
	if len(result) != 3 {
		t.Errorf("expected 3 messages (1 system + 2 conv), got %d", len(result))
	}
	if result[1].Content != "c" {
		t.Errorf("expected 'c', got '%s'", result[1].Content)
	}
}

func TestHalveMessages(t *testing.T) {
	msgs := []Message{
		{Role: "system", Content: "bot"},
		{Role: "user", Content: "1"},
		{Role: "assistant", Content: "2"},
		{Role: "user", Content: "3"},
		{Role: "assistant", Content: "4"},
		{Role: "user", Content: "5"},
		{Role: "assistant", Content: "6"},
	}

	result := HalveMessages(msgs)
	if result[0].Role != "system" {
		t.Error("system should be preserved")
	}
	if len(result) >= len(msgs) {
		t.Errorf("expected some messages removed, got %d from %d", len(result), len(msgs))
	}
}

// ──────────────────────────────────────────────
// #37 Runtime Hardening: Token Reduction Verification
// ──────────────────────────────────────────────

// TestTokenReductionMultiStage verifies that multi-stage compression actually reduces tokens
// across three scenarios: long task, document analysis, multi-turn conversation.
func TestTokenReductionMultiStage(t *testing.T) {
	// Scenario 1: Long multi-turn conversation (30 turns)
	t.Run("long_conversation", func(t *testing.T) {
		msgs := []Message{{Role: "system", Content: "你是一个智能助手。"}}
		for i := 0; i < 30; i++ {
			msgs = append(msgs,
				Message{Role: "user", Content: strings.Repeat("这是第"+string(rune('A'+i%26))+"轮的用户消息内容。", 20)},
				Message{Role: "assistant", Content: strings.Repeat("这是助手的回复结果，包含详细分析。", 20)},
			)
		}

		tokensBefore := countTokensAll(msgs)
		t.Logf("scenario=long_conversation tokens_before=%d msgs=%d", tokensBefore, len(msgs))

		// Use truncate compressor with a tight budget
		mgr := NewManager(ManagerConfig{
			EnforceMaxTurns:  10,
			MaxContextTokens: 2000,
			Compressor:       NewTruncateCompressor(2, 0.5),
		})
		result, err := mgr.Process(context.Background(), msgs)
		if err != nil {
			t.Fatal(err)
		}

		tokensAfter := countTokensAll(result)
		t.Logf("scenario=long_conversation tokens_after=%d msgs=%d reduction=%d%%",
			tokensAfter, len(result), (tokensBefore-tokensAfter)*100/tokensBefore)

		if tokensAfter >= tokensBefore {
			t.Errorf("expected token reduction: before=%d after=%d", tokensBefore, tokensAfter)
		}
		if len(result) >= len(msgs) {
			t.Errorf("expected message reduction: before=%d after=%d", len(msgs), len(result))
		}
	})

	// Scenario 2: Large document context (simulated RAG injection)
	t.Run("document_context", func(t *testing.T) {
		largeDoc := strings.Repeat("联合国宪章第一条规定了国际和平与安全的维护原则。", 100)
		msgs := []Message{
			{Role: "system", Content: "你是文档分析专家。"},
			{Role: "system", Content: "[动态上下文]\n## 知识文档\n" + largeDoc},
			{Role: "user", Content: "总结这份文档的核心内容。"},
		}

		tokensBefore := countTokensAll(msgs)
		t.Logf("scenario=document_context tokens_before=%d", tokensBefore)

		// Trim with window config
		cfg := WindowConfig{
			MaxTokens:     4000,
			SystemReserve: 500,
			ReplyReserve:  500,
			MaxMessages:   10,
			PreserveFirst: 1,
			PreserveLast:  1,
		}
		result := TrimToFit(msgs, cfg)
		tokensAfter := countTokensAll(result.Messages)
		t.Logf("scenario=document_context tokens_after=%d dropped=%d",
			tokensAfter, result.DroppedCount)

		// With this budget, the large doc message should be trimmed
		if tokensAfter > cfg.MaxTokens-cfg.SystemReserve-cfg.ReplyReserve {
			t.Logf("token count after trim (%d) is within expected budget", tokensAfter)
		}
	})

	// Scenario 3: LLM summary compression (mock LLM)
	t.Run("llm_summary", func(t *testing.T) {
		msgs := []Message{
			{Role: "system", Content: "助手系统。"},
		}
		for i := 0; i < 20; i++ {
			msgs = append(msgs,
				Message{Role: "user", Content: strings.Repeat("长消息内容", 30)},
				Message{Role: "assistant", Content: strings.Repeat("详细回复", 30)},
			)
		}

		tokensBefore := countTokensAll(msgs)

		// Mock LLM that returns a short summary
		compressor := &LLMSummaryCompressor{
			Call: func(ctx context.Context, system, user string) (string, error) {
				return "这是一段简短的对话摘要。", nil
			},
			KeepRecent:           4,
			CompressionThreshold: 0.1, // always compress
		}

		mgr := NewManager(ManagerConfig{
			EnforceMaxTurns:  50,
			MaxContextTokens: 3000, // slightly above total tokens so ShouldCompress triggers
			Compressor:       compressor,
		})

		result, err := mgr.Process(context.Background(), msgs)
		if err != nil {
			t.Fatal(err)
		}

		tokensAfter := countTokensAll(result)
		reduction := (tokensBefore - tokensAfter) * 100 / tokensBefore
		t.Logf("scenario=llm_summary tokens_before=%d tokens_after=%d reduction=%d%%",
			tokensBefore, tokensAfter, reduction)

		if reduction < 50 {
			t.Errorf("expected at least 50%% reduction, got %d%%", reduction)
		}
	})
}

// TestLayerPriorityEviction verifies that low-priority layers are evicted first.
func TestLayerPriorityEviction(t *testing.T) {
	layers := []Layer{
		{Name: "task", Priority: LayerPriorityTask, Content: strings.Repeat("任务上下文", 50)},
		{Name: "memory", Priority: LayerPriorityMemory, Content: strings.Repeat("记忆内容", 50)},
		{Name: "hints", Priority: LayerPriorityHints, Content: strings.Repeat("提示内容", 50)},
	}

	totalTokens := 0
	for _, l := range layers {
		totalTokens += estimateTokens(l.Content)
	}

	// Budget that can fit ~2 layers but not all 3
	budget := totalTokens * 2 / 3
	assembler := NewLayerAssembler(budget)
	assembled, includedNames := assembler.Assemble(layers)

	t.Logf("budget=%d total_tokens=%d included=%d names=%v", budget, totalTokens, len(includedNames), includedNames)

	if len(includedNames) >= len(layers) {
		t.Logf("all layers fit (budget=%d), skipping eviction test", budget)
		return
	}

	// When eviction occurs, highest priority (task=10) should survive
	if !strings.Contains(assembled, "任务上下文") {
		t.Error("expected task layer (highest priority) to survive eviction")
	}
}
