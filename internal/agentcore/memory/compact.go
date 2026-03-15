package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Compactor consolidates redundant memories into a smaller, concise set.
type Compactor struct {
	chatFn ChatFunc
}

// NewCompactor creates a memory compactor.
func NewCompactor(chatFn ChatFunc) *Compactor {
	return &Compactor{chatFn: chatFn}
}

// CompactInput describes memories to be consolidated.
type CompactInput struct {
	Memories    []Candidate
	TargetCount int
	DecayDays   int // memories older than this many days are low priority
}

// CompactOutput holds the consolidated facts.
type CompactOutput struct {
	Facts       []string `json:"facts"`
	BeforeCount int      `json:"before_count"`
	AfterCount  int      `json:"after_count"`
}

// Compact merges and deduplicates a set of memories.
func (c *Compactor) Compact(ctx context.Context, input CompactInput) (*CompactOutput, error) {
	if len(input.Memories) == 0 {
		return &CompactOutput{}, nil
	}
	if input.TargetCount <= 0 {
		input.TargetCount = len(input.Memories) / 2
		if input.TargetCount < 1 {
			input.TargetCount = 1
		}
	}

	sysPrompt, userPrompt := buildCompactPrompts(input)
	reply, err := c.chatFn(ctx, []ChatMessage{
		{Role: "system", Content: sysPrompt},
		{Role: "user", Content: userPrompt},
	})
	if err != nil {
		return nil, fmt.Errorf("compact memories: %w", err)
	}

	var parsed struct {
		Facts []string `json:"facts"`
	}
	if err := json.Unmarshal([]byte(stripCodeFences(reply)), &parsed); err != nil {
		return nil, fmt.Errorf("parse compact result: %w", err)
	}

	return &CompactOutput{
		Facts:       parsed.Facts,
		BeforeCount: len(input.Memories),
		AfterCount:  len(parsed.Facts),
	}, nil
}

func buildCompactPrompts(input CompactInput) (string, string) {
	decayNote := ""
	if input.DecayDays > 0 {
		decayNote = fmt.Sprintf(`
时间衰减规则：今天是 %s，超过 %d 天的记忆优先级较低。
- 相似信息保留较新的版本
- 仅当旧记忆包含唯一且仍然相关的信息时才保留`,
			time.Now().Format("2006-01-02"), input.DecayDays)
	}

	sysPrompt := fmt.Sprintf(`你是一个记忆整理专家，负责将多条记忆合并为更精简的集合。

整理原则：
1. 合并相似或重复的条目为一条简洁事实
2. 矛盾信息保留更具体或更新的版本
3. 保留所有不重复的关键信息，不要遗漏
4. 每条输出必须是独立、完整的陈述
5. 目标约 %d 条（可以更少，但不能超过输入数量）
6. 保持原文语言
7. 仅返回JSON：{"facts": ["...", "..."]}%s`, input.TargetCount, decayNote)

	memoriesJSON, _ := json.Marshal(input.Memories)
	userPrompt := fmt.Sprintf("请将以下 %d 条记忆整理为约 %d 条：\n\n%s",
		len(input.Memories), input.TargetCount, string(memoriesJSON))

	return sysPrompt, userPrompt
}
