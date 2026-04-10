package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// SkillSuggestion represents a suggested skill that could be created from conversation patterns.
type SkillSuggestion struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Trigger     string `json:"trigger"`
	Confidence  int    `json:"confidence"` // 1-10
}

// SkillSuggestResult holds skill suggestions extracted from a conversation.
type SkillSuggestResult struct {
	Suggestions []SkillSuggestion `json:"suggestions"`
}

// SkillSuggester analyzes conversations for repeatable patterns that could become skills.
type SkillSuggester struct {
	chatFn ChatFunc
}

func NewSkillSuggester(chatFn ChatFunc) *SkillSuggester {
	return &SkillSuggester{chatFn: chatFn}
}

// Analyze checks if the conversation contains patterns worth turning into a skill.
func (s *SkillSuggester) Analyze(ctx context.Context, userMsg, assistReply string, skillsUsed []string) (*SkillSuggestResult, error) {
	if userMsg == "" || assistReply == "" {
		return &SkillSuggestResult{}, nil
	}
	if len(assistReply) < 100 {
		return &SkillSuggestResult{}, nil
	}

	usedSkills := "无"
	if len(skillsUsed) > 0 {
		usedSkills = strings.Join(skillsUsed, ", ")
	}

	prompt := fmt.Sprintf(`分析以下对话，判断是否包含可复用的工作流或操作模式，值得保存为一个"技能"（Skill）。

技能是可以被 Agent 在未来对话中复用的自动化操作模板。

判断标准：
- 用户请求了多步骤的复杂操作（如数据处理流水线、代码生成模板、文档转换等）
- 涉及特定领域的专业操作流程
- 用户可能在未来重复类似请求
- 当前已使用的技能不能覆盖这个需求
- 简单问答、闲聊、查询天气等不需要技能

已使用的技能：%s

用户：%s

助手回复（前500字）：%s

如果发现值得保存的模式，返回JSON：
{"suggestions": [{"name": "技能名", "description": "一句话描述", "trigger": "触发条件", "confidence": 8}]}

如果没有值得保存的模式，返回：
{"suggestions": []}

仅返回JSON。`, usedSkills, userMsg, clipText(assistReply, 500))

	reply, err := s.chatFn(ctx, []ChatMessage{
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return nil, fmt.Errorf("skill suggest: %w", err)
	}

	var result SkillSuggestResult
	cleaned := stripCodeFences(reply)
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return &SkillSuggestResult{}, nil
	}

	filtered := result.Suggestions[:0]
	for _, s := range result.Suggestions {
		if s.Name != "" && s.Confidence >= 6 {
			filtered = append(filtered, s)
		}
	}
	result.Suggestions = filtered
	return &result, nil
}

func clipText(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}
