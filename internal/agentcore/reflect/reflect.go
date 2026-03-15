package reflect

import (
	"context"
	"encoding/json"
	"fmt"

	"yunque-agent/internal/agentcore/llm"
)

// Engine evaluates execution results and suggests improvements.
type Engine struct {
	llm *llm.Client
}

// NewEngine creates a reflection engine.
func NewEngine(llmClient *llm.Client) *Engine {
	return &Engine{llm: llmClient}
}

// Evaluation is the result of a reflection.
type Evaluation struct {
	Satisfied   bool     `json:"satisfied"`
	Quality     int      `json:"quality"`     // 1-10
	Issues      []string `json:"issues"`
	Suggestions []string `json:"suggestions"`
	MemoryUpdates []MemoryUpdate `json:"memory_updates,omitempty"`
}

// MemoryUpdate is a suggested change to the memory store.
type MemoryUpdate struct {
	Action string `json:"action"` // "add", "update", "delete"
	Key    string `json:"key"`
	Value  string `json:"value"`
}

// Evaluate assesses whether the agent's response satisfies the user's intent.
func (e *Engine) Evaluate(ctx context.Context, userIntent, agentReply string, skillResults []string) (*Evaluation, error) {
	prompt := fmt.Sprintf(`你是一个AI执行质量评估器。请评估以下AI回复是否满足用户意图。

## 用户意图
%s

## AI回复
%s

## 技能执行结果
%v

请以JSON格式输出评估结果：
{
  "satisfied": true/false,
  "quality": 1-10,
  "issues": ["问题1", ...],
  "suggestions": ["建议1", ...],
  "memory_updates": [{"action": "add/update/delete", "key": "...", "value": "..."}]
}

只输出JSON，不要其他内容。`, userIntent, agentReply, skillResults)

	reply, err := e.llm.Chat(ctx, []llm.Message{
		{Role: "system", Content: "你是质量评估器，只输出JSON。"},
		{Role: "user", Content: prompt},
	}, 0.1)
	if err != nil {
		return nil, fmt.Errorf("reflect: %w", err)
	}

	var eval Evaluation
	if err := json.Unmarshal([]byte(extractJSON(reply)), &eval); err != nil {
		// Default to satisfied if parsing fails
		return &Evaluation{Satisfied: true, Quality: 7}, nil
	}
	return &eval, nil
}

// ShouldRetry returns true if the evaluation suggests retrying.
func (e *Evaluation) ShouldRetry() bool {
	return !e.Satisfied && e.Quality < 5
}

func extractJSON(s string) string {
	start := -1
	for i, c := range s {
		if c == '{' {
			start = i
			break
		}
	}
	if start < 0 {
		return "{}"
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return "{}"
}
