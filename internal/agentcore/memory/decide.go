package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Action represents a memory mutation decision.
type Action struct {
	Op        string `json:"event"`         // ADD, UPDATE, DELETE, NONE
	ID        string `json:"id,omitempty"`   // target memory ID (for UPDATE/DELETE)
	Text      string `json:"text"`          // new or updated content
	OldMemory string `json:"old_memory,omitempty"`
}

// DecideResult holds the list of actions to apply against the memory store.
type DecideResult struct {
	Actions []Action `json:"actions"`
}

// Candidate is an existing memory entry for comparison.
type Candidate struct {
	ID        string `json:"id"`
	Content   string `json:"text"`
	CreatedAt string `json:"created_at,omitempty"`
}

// Decider uses LLM to determine how new facts should affect existing memories.
type Decider struct {
	chatFn ChatFunc
}

// NewDecider creates a memory decision engine.
func NewDecider(chatFn ChatFunc) *Decider {
	return &Decider{chatFn: chatFn}
}

// Decide compares new facts against existing memories and produces actions.
func (d *Decider) Decide(ctx context.Context, facts []string, candidates []Candidate) (*DecideResult, error) {
	if len(facts) == 0 {
		return &DecideResult{}, nil
	}

	prompt := buildDecisionPrompt(candidates, facts)
	reply, err := d.chatFn(ctx, []ChatMessage{
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return nil, fmt.Errorf("decide memory: %w", err)
	}

	actions, err := parseDecisionResponse(reply)
	if err != nil {
		// Fallback: treat all facts as ADD if parsing fails
		fallback := make([]Action, 0, len(facts))
		for _, f := range facts {
			fallback = append(fallback, Action{Op: "ADD", Text: f})
		}
		return &DecideResult{Actions: fallback}, nil
	}
	return &DecideResult{Actions: actions}, nil
}

func buildDecisionPrompt(existing []Candidate, newFacts []string) string {
	existingJSON, _ := json.Marshal(existing)
	factsJSON, _ := json.Marshal(newFacts)

	return fmt.Sprintf(`你是一个记忆管理器，负责维护系统的长期记忆。

你可以执行四种操作：
1. ADD — 新信息，记忆中不存在
2. UPDATE — 已有相关记忆但信息有变化，需要更新（保留原ID）
3. DELETE — 新信息与已有记忆矛盾，需要删除旧记忆
4. NONE — 信息已存在且无变化，不操作

判断规则：
- 新增：记忆中没有的新信息
- 更新：同一主题但内容不同（如偏好改变）
- 删除：直接矛盾的信息
- 无变化：完全重复或已包含的信息

当前记忆：
%s

新提取的事实：
%s

请返回JSON数组，每个元素包含：
- event: "ADD" | "UPDATE" | "DELETE" | "NONE"
- id: 目标记忆ID（UPDATE/DELETE时必填，ADD时留空）
- text: 内容文本
- old_memory: 被替换的旧内容（UPDATE时填写）

格式：[{"event":"ADD","text":"..."},{"event":"UPDATE","id":"xxx","text":"...","old_memory":"..."}]
仅返回JSON数组，不要其他文字。`, string(existingJSON), string(factsJSON))
}

func parseDecisionResponse(reply string) ([]Action, error) {
	cleaned := stripCodeFences(reply)

	// Try parsing as array first
	var actions []Action
	if err := json.Unmarshal([]byte(cleaned), &actions); err == nil {
		return filterValidActions(actions), nil
	}

	// Try as object with nested array
	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal([]byte(cleaned), &wrapper); err != nil {
		return nil, fmt.Errorf("parse decision: %w", err)
	}

	// Look for any array-typed field
	for _, v := range wrapper {
		if err := json.Unmarshal(v, &actions); err == nil {
			return filterValidActions(actions), nil
		}
	}
	return nil, fmt.Errorf("no valid actions found in response")
}

func filterValidActions(actions []Action) []Action {
	valid := actions[:0]
	for _, a := range actions {
		a.Op = strings.ToUpper(strings.TrimSpace(a.Op))
		a.Text = strings.TrimSpace(a.Text)
		if a.Op == "NONE" || a.Text == "" {
			continue
		}
		if a.Op != "ADD" && a.Op != "UPDATE" && a.Op != "DELETE" {
			continue
		}
		valid = append(valid, a)
	}
	return valid
}
