package context

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// Compressor 上下文压缩策略接口
type Compressor interface {
	// ShouldCompress 判断是否需要压缩
	ShouldCompress(messages []Message, currentTokens, maxTokens int) bool
	// Compress 压缩消息列表，返回压缩后的消息
	Compress(ctx context.Context, messages []Message) ([]Message, error)
}

// ────────────────────────────────────────
// TruncateCompressor — 按轮数截断(丢弃最旧的对话轮次)
// ────────────────────────────────────────

// TruncateCompressor drops oldest conversation turns when over budget.
type TruncateCompressor struct {
	TruncateTurns        int     // 每次丢弃几轮（默认1）
	CompressionThreshold float64 // 触发压缩的token使用率（默认0.82）
}

// NewTruncateCompressor creates a truncation compressor.
func NewTruncateCompressor(truncateTurns int, threshold float64) *TruncateCompressor {
	if truncateTurns <= 0 {
		truncateTurns = 1
	}
	if threshold <= 0 || threshold > 1 {
		threshold = 0.82
	}
	return &TruncateCompressor{TruncateTurns: truncateTurns, CompressionThreshold: threshold}
}

func (c *TruncateCompressor) ShouldCompress(messages []Message, currentTokens, maxTokens int) bool {
	if maxTokens <= 0 {
		return false
	}
	return float64(currentTokens)/float64(maxTokens) > c.CompressionThreshold
}

func (c *TruncateCompressor) Compress(_ context.Context, messages []Message) ([]Message, error) {
	return DropOldestTurns(messages, c.TruncateTurns), nil
}

// ────────────────────────────────────────
// LLMSummaryCompressor — LLM摘要压缩
// ────────────────────────────────────────

// LLMCaller 用于调用LLM生成摘要
type LLMCaller func(ctx context.Context, systemPrompt, userPrompt string) (string, error)

// LLMSummaryCompressor uses an LLM to summarize old messages.
type LLMSummaryCompressor struct {
	Call                 LLMCaller
	KeepRecent           int     // 保留最近N条消息（默认4）
	Instruction          string  // 摘要指令（可自定义）
	CompressionThreshold float64 // 触发压缩的token使用率
}

// NewLLMSummaryCompressor creates an LLM-based summary compressor.
func NewLLMSummaryCompressor(caller LLMCaller, keepRecent int, threshold float64) *LLMSummaryCompressor {
	if keepRecent <= 0 {
		keepRecent = 4
	}
	if threshold <= 0 || threshold > 1 {
		threshold = 0.82
	}
	return &LLMSummaryCompressor{
		Call:                 caller,
		KeepRecent:           keepRecent,
		CompressionThreshold: threshold,
		Instruction: "请将以下对话历史浓缩为一段简洁的摘要，保留所有关键信息、用户偏好和重要结论。" +
			"要求：1) 保留核心事实和决策 2) 保留用户提到的具体需求 3) 忽略寒暄和重复内容 4) 使用第三人称描述。",
	}
}

func (c *LLMSummaryCompressor) ShouldCompress(messages []Message, currentTokens, maxTokens int) bool {
	if maxTokens <= 0 {
		return false
	}
	return float64(currentTokens)/float64(maxTokens) > c.CompressionThreshold
}

func (c *LLMSummaryCompressor) Compress(ctx context.Context, messages []Message) ([]Message, error) {
	if c.Call == nil {
		// Fallback to truncation if no LLM caller
		return DropOldestTurns(messages, 2), nil
	}

	// Split: [system messages] | [old messages to summarize] | [recent messages to keep]
	var systemMsgs []Message
	var convMsgs []Message
	for _, m := range messages {
		if m.Role == "system" {
			systemMsgs = append(systemMsgs, m)
		} else {
			convMsgs = append(convMsgs, m)
		}
	}

	if len(convMsgs) <= c.KeepRecent {
		return messages, nil // 不够多，无需压缩
	}

	// Separate old (to summarize) and recent (to keep)
	oldMsgs := convMsgs[:len(convMsgs)-c.KeepRecent]
	recentMsgs := convMsgs[len(convMsgs)-c.KeepRecent:]

	// Build the conversation text for summarization
	var sb strings.Builder
	for _, m := range oldMsgs {
		sb.WriteString(fmt.Sprintf("[%s]: %s\n", m.Role, m.Content))
	}

	instruction := c.Instruction
	if instruction == "" {
		instruction = "请将以下对话历史浓缩为简洁的摘要，保留所有关键信息。"
	}

	summary, err := c.Call(ctx, instruction, sb.String())
	if err != nil {
		// LLM失败时降级为截断
		return DropOldestTurns(messages, 2), nil
	}

	// Rebuild: system messages + summary + recent messages
	var result []Message
	result = append(result, systemMsgs...)
	result = append(result, Message{
		Role:    "user",
		Content: "[对话历史摘要]\n" + summary,
	})
	result = append(result, recentMsgs...)

	return result, nil
}

// ────────────────────────────────────────
// ContextManager — 多阶段压缩管理器
// ────────────────────────────────────────

// ManagerConfig 上下文管理器配置
type ManagerConfig struct {
	MaxContextTokens int        // 最大上下文token数
	EnforceMaxTurns  int        // 强制最大对话轮数（在压缩前执行）
	Compressor       Compressor // 压缩策略
}

// Manager manages context window compression with multi-stage pipeline.
type Manager struct {
	cfg ManagerConfig
}

// NewManager creates a context manager.
func NewManager(cfg ManagerConfig) *Manager {
	if cfg.MaxContextTokens <= 0 {
		cfg.MaxContextTokens = 128000
	}
	if cfg.EnforceMaxTurns <= 0 {
		cfg.EnforceMaxTurns = 50
	}
	return &Manager{cfg: cfg}
}

// Process applies multi-stage context compression:
// Stage 1: Enforce max turns (hard truncation)
// Stage 2: Apply compression strategy if still over budget
// Stage 3: Emergency halving if still over budget
func (m *Manager) Process(ctx context.Context, messages []Message) ([]Message, error) {
	if len(messages) == 0 {
		return messages, nil
	}

	result := make([]Message, len(messages))
	copy(result, messages)

	tokensBefore := countTokensAll(result)
	msgsBefore := len(result)

	// Stage 1: Enforce max turns
	result = EnforceMaxTurns(result, m.cfg.EnforceMaxTurns)

	// Stage 2: Check token budget and compress if needed
	tokens := countTokensAll(result)
	if m.cfg.Compressor != nil && m.cfg.Compressor.ShouldCompress(result, tokens, m.cfg.MaxContextTokens) {
		var err error
		result, err = m.cfg.Compressor.Compress(ctx, result)
		if err != nil {
			return result, err
		}
	}

	// Stage 3: Emergency halving if still over budget
	tokens = countTokensAll(result)
	if tokens > m.cfg.MaxContextTokens {
		result = HalveMessages(result)
	}

	tokensAfter := countTokensAll(result)
	if tokensAfter < tokensBefore {
		slog.Info("context compression",
			"tokens_before", tokensBefore,
			"tokens_after", tokensAfter,
			"token_reduction", tokensBefore-tokensAfter,
			"msgs_before", msgsBefore,
			"msgs_after", len(result),
		)
	}

	return result, nil
}

// ────────────────────────────────────────
// Helper functions
// ────────────────────────────────────────

// DropOldestTurns removes the oldest N conversation turns (user+assistant pairs),
// preserving system messages.
func DropOldestTurns(messages []Message, turns int) []Message {
	var systemMsgs []Message
	var convMsgs []Message
	for _, m := range messages {
		if m.Role == "system" {
			systemMsgs = append(systemMsgs, m)
		} else {
			convMsgs = append(convMsgs, m)
		}
	}

	// Drop 'turns' pairs (2 messages per turn) from the start
	drop := turns * 2
	if drop >= len(convMsgs) {
		drop = len(convMsgs) - 2 // Keep at least 1 turn
		if drop < 0 {
			drop = 0
		}
	}

	// Align to user message boundary
	for drop < len(convMsgs) && convMsgs[drop].Role != "user" {
		drop++
	}

	var result []Message
	result = append(result, systemMsgs...)
	result = append(result, convMsgs[drop:]...)
	return result
}

// EnforceMaxTurns keeps only the most recent N turns, preserving system messages.
func EnforceMaxTurns(messages []Message, maxTurns int) []Message {
	var systemMsgs []Message
	var convMsgs []Message
	for _, m := range messages {
		if m.Role == "system" {
			systemMsgs = append(systemMsgs, m)
		} else {
			convMsgs = append(convMsgs, m)
		}
	}

	maxMsgs := maxTurns * 2
	if len(convMsgs) <= maxMsgs {
		return messages
	}

	// Keep only the last maxMsgs conversation messages
	kept := convMsgs[len(convMsgs)-maxMsgs:]
	var result []Message
	result = append(result, systemMsgs...)
	result = append(result, kept...)
	return result
}

// HalveMessages drops 50% of non-system messages (emergency fallback).
func HalveMessages(messages []Message) []Message {
	var result []Message
	convIdx := 0
	for _, m := range messages {
		if m.Role == "system" {
			result = append(result, m)
			continue
		}
		convIdx++
		// Keep every other message, always keep the last 2
		remaining := countNonSystem(messages) - convIdx
		if convIdx%2 == 0 || remaining < 2 {
			result = append(result, m)
		}
	}
	return result
}

func countTokensAll(messages []Message) int {
	total := 0
	for _, m := range messages {
		total += EstimateTokens(m.Content)
	}
	return total
}

func countNonSystem(messages []Message) int {
	count := 0
	for _, m := range messages {
		if m.Role != "system" {
			count++
		}
	}
	return count
}
