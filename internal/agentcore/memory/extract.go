package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Extractor uses LLM to extract structured facts from conversations.
type Extractor struct {
	chatFn ChatFunc
}

// ChatFunc is a function that calls the LLM with messages and returns the response.
type ChatFunc func(ctx context.Context, messages []ChatMessage) (string, error)

// ChatMessage is a simplified message for internal LLM calls.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ExtractResult holds the facts extracted from a conversation.
type ExtractResult struct {
	Facts []string `json:"facts"`
}

// NewExtractor creates a fact extractor with the given LLM chat function.
func NewExtractor(chatFn ChatFunc) *Extractor {
	return &Extractor{chatFn: chatFn}
}

// Extract analyzes conversation messages and returns structured facts.
func (e *Extractor) Extract(ctx context.Context, messages []ChatMessage) (*ExtractResult, error) {
	if len(messages) == 0 {
		return &ExtractResult{}, nil
	}

	formatted := formatConversation(messages)
	sysPrompt, userPrompt := buildExtractionPrompts(formatted)

	reply, err := e.chatFn(ctx, []ChatMessage{
		{Role: "system", Content: sysPrompt},
		{Role: "user", Content: userPrompt},
	})
	if err != nil {
		return nil, fmt.Errorf("extract facts: %w", err)
	}

	var result ExtractResult
	cleaned := stripCodeFences(reply)
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("parse extraction result: %w", err)
	}

	// Filter out empty facts
	filtered := result.Facts[:0]
	for _, f := range result.Facts {
		if strings.TrimSpace(f) != "" {
			filtered = append(filtered, strings.TrimSpace(f))
		}
	}
	result.Facts = filtered
	return &result, nil
}

func formatConversation(messages []ChatMessage) string {
	var sb strings.Builder
	for _, m := range messages {
		if m.Role == "system" {
			continue
		}
		sb.WriteString(m.Role)
		sb.WriteString(": ")
		sb.WriteString(m.Content)
		sb.WriteString("\n")
	}
	return sb.String()
}

func buildExtractionPrompts(conversation string) (string, string) {
	sysPrompt := fmt.Sprintf(`你是一个信息整理专家，擅长从对话中精确提取关键事实和用户偏好。

需要关注的信息类型：
1. 个人偏好：喜好、厌恶、习惯
2. 重要细节：姓名、关系、重要日期
3. 计划意图：即将发生的事件、目标
4. 专业信息：职业、技能、工作相关
5. 客观事实：明确的陈述和知识点

提取规则：
- 每条事实必须是独立、完整的陈述
- 多条信息拆分为独立事实
- 保持原文语言（中文输入则中文输出）
- 忽略寒暄和无实质信息的对话
- 今天是 %s

输出JSON格式：{"facts": ["事实1", "事实2"]}
如果没有可提取的信息，返回：{"facts": []}
仅返回JSON，不要添加任何其他文字。`, time.Now().Format("2006-01-02"))

	userPrompt := fmt.Sprintf("请从以下对话中提取关键事实：\n\n%s", conversation)
	return sysPrompt, userPrompt
}

func stripCodeFences(s string) string {
	s = strings.ReplaceAll(s, "```json", "")
	s = strings.ReplaceAll(s, "```", "")
	return strings.TrimSpace(s)
}
