package localbrain

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ExperienceDistiller 将大模型的成功经验蒸馏为小模型可用的知识。
//
// 林俊旸文章核心观点：agent 的"经验"不只是对话历史，而是"什么决策在什么场景下有效"。
// 这个模块实现：
//   - 大模型处理请求成功 → 记录 (scenario, decision, outcome)
//   - 积累到阈值 → 用 Fast 模型从经验中提炼规则
//   - 规则用于小模型的快速决策（不用每次都调大模型）
type ExperienceDistiller struct {
	mu         sync.Mutex
	experiences map[string][]Experience // category → experiences
	rules       map[string][]Rule       // category → distilled rules
	ruleLimit   int
}

// Experience 记录一次成功的大模型决策经验。
type Experience struct {
	Query      string    `json:"query"`
	Category   string    `json:"category"`
	Decision   string    `json:"decision"`   // 选择了什么 handler
	Outcome    string    `json:"outcome"`     // 结果摘要
	Successful bool      `json:"successful"`
	Timestamp  time.Time `json:"timestamp"`
}

// Rule 是从经验中蒸馏出的规则。
type Rule struct {
	Pattern   string  `json:"pattern"`   // 场景模式描述
	Action    string  `json:"action"`    // 建议的决策
	Confidence float64 `json:"confidence"`
	Source    int     `json:"source"`    // 基于多少条经验蒸馏
}

// NewExperienceDistiller 创建经验蒸馏器。
func NewExperienceDistiller() *ExperienceDistiller {
	return &ExperienceDistiller{
		experiences: make(map[string][]Experience),
		rules:       make(map[string][]Rule),
		ruleLimit:   50, // 每类别最多 50 条规则
	}
}

// Record 记录一条经验。当同类别经验累积到 10 条时，触发规则蒸馏。
func (ed *ExperienceDistiller) Record(exp Experience) {
	ed.mu.Lock()
	defer ed.mu.Unlock()

	ed.experiences[exp.Category] = append(ed.experiences[exp.Category], exp)

	// 滑动窗口：只保留最近 500 条
	if exps := ed.experiences[exp.Category]; len(exps) > 500 {
		ed.experiences[exp.Category] = exps[len(exps)-500:]
	}
}

// DistillRules 从积累的经验中提炼决策规则。
// llmCall 是 Fast 模型的调用函数（蒸馏用低成本模型即可）。
func (ed *ExperienceDistiller) DistillRules(ctx context.Context, category string, llmCall func(ctx context.Context, system, user string) (string, error)) error {
	ed.mu.Lock()
	exps := ed.experiences[category]
	if len(exps) < 5 {
		ed.mu.Unlock()
		return fmt.Errorf("not enough experiences (%d < 5) for category %s", len(exps), category)
	}

	// 只用成功的经验
	var successful []Experience
	for _, e := range exps {
		if e.Successful {
			successful = append(successful, e)
		}
	}
	ed.mu.Unlock()

	if len(successful) < 3 {
		return fmt.Errorf("not enough successful experiences for category %s", category)
	}

	// 构建蒸馏 prompt
	var expSummary string
	for i, e := range successful {
		if i >= 20 { // 最多用 20 条
			break
		}
		expSummary += fmt.Sprintf("- Query: %s | Decision: %s | Outcome: %s\n",
			truncate(e.Query, 100), e.Decision, truncate(e.Outcome, 100))
	}

	system := `You are a pattern extraction engine. Extract reusable decision rules from the following successful experiences.
Output JSON array: [{"pattern":"when user asks about X","action":"route to Y","confidence":0.8}]
Rules: max 5 rules, focus on the most common patterns, confidence 0.0-1.0.`
	user := fmt.Sprintf("Category: %s\nExperiences:\n%s", category, expSummary)

	reply, err := llmCall(ctx, system, user)
	if err != nil {
		return fmt.Errorf("llm call: %w", err)
	}

	var rules []Rule
	if err := extractJSON(reply, &rules); err != nil {
		slog.Warn("distill: parse rules failed", "err", err, "category", category)
		return fmt.Errorf("parse rules: %w", err)
	}

	// 标记来源
	for i := range rules {
		rules[i].Source = len(successful)
	}

	ed.mu.Lock()
	ed.rules[category] = rules
	// 限制规则总数
	if len(ed.rules[category]) > ed.ruleLimit {
		ed.rules[category] = ed.rules[category][len(ed.rules[category])-ed.ruleLimit:]
	}
	ed.mu.Unlock()

	slog.Info("distill: rules extracted", "category", category, "count", len(rules))
	return nil
}

// MatchRule 查找匹配当前查询的规则。
// 返回最佳匹配的规则和置信度，如果无匹配返回 nil。
func (ed *ExperienceDistiller) MatchRule(category string) *Rule {
	ed.mu.Lock()
	defer ed.mu.Unlock()

	rules := ed.rules[category]
	if len(rules) == 0 {
		return nil
	}

	// 找置信度最高的规则
	var best *Rule
	for i := range rules {
		if best == nil || rules[i].Confidence > best.Confidence {
			best = &rules[i]
		}
	}
	return best
}

// RuleCount 返回指定类别的规则数量。
func (ed *ExperienceDistiller) RuleCount(category string) int {
	ed.mu.Lock()
	defer ed.mu.Unlock()
	return len(ed.rules[category])
}

// AllRules 返回所有类别的规则。
func (ed *ExperienceDistiller) AllRules() map[string][]Rule {
	ed.mu.Lock()
	defer ed.mu.Unlock()
	result := make(map[string][]Rule, len(ed.rules))
	for k, v := range ed.rules {
		cp := make([]Rule, len(v))
		copy(cp, v)
		result[k] = cp
	}
	return result
}
