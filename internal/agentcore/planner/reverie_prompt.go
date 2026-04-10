package planner

import (
	"encoding/json"
	"fmt"
	"strings"
)

func (r *Reverie) buildThinkingPrompt(memoryContext string, recentThoughts []Thought) string {
	var b strings.Builder
	b.WriteString("现在是你的独立思考时间。\n\n")

	if memoryContext != "" {
		b.WriteString("## 最近的记忆\n")
		b.WriteString(memoryContext)
		b.WriteString("\n\n")
	}

	if len(recentThoughts) > 0 {
		b.WriteString("## 你之前的思考\n")
		for _, t := range recentThoughts {
			b.WriteString(fmt.Sprintf("- [%s, 重要性%.1f] %s\n", t.Category, t.Significance, truncateStr(t.Content, 80)))
		}
		b.WriteString("\n")
	}

	b.WriteString("基于以上信息，你现在有什么新的想法、观察或洞见吗？\n")
	b.WriteString("可以是对用户需求的新理解、对自己能力的反思、或者任何值得分享的发现。")

	return b.String()
}

func (r *Reverie) buildEventThinkingPrompt(memoryContext string, recentThoughts []Thought, ev ReverieEvent) string {
	var b strings.Builder
	b.WriteString("你刚刚注意到一个值得关注的变化，需要立即思考。\n\n")

	// Describe the triggering event
	b.WriteString("## 触发事件\n")
	switch ev.Type {
	case EventEmotionShift:
		b.WriteString(fmt.Sprintf("用户的情绪发生了显著变化: %s。这可能意味着他们正在经历某些事情。\n", ev.Trigger))
	case EventTaskFailureSpike:
		b.WriteString(fmt.Sprintf("最近的工具调用出现了异常高的失败率: %s。需要反思是否有系统性问题。\n", ev.Trigger))
	case EventHighValueFact:
		b.WriteString(fmt.Sprintf("记忆系统发现了重要的新事实: %s。这些新信息可能改变你对用户或当前任务的理解。\n", ev.Trigger))
	default:
		b.WriteString(fmt.Sprintf("外部事件: %s\n", ev.Trigger))
	}
	b.WriteString("\n")

	if memoryContext != "" {
		b.WriteString("## 最近的记忆\n")
		b.WriteString(memoryContext)
		b.WriteString("\n\n")
	}

	if len(recentThoughts) > 0 {
		b.WriteString("## 你之前的思考\n")
		for _, t := range recentThoughts {
			b.WriteString(fmt.Sprintf("- [%s, 重要性%.1f] %s\n", t.Category, t.Significance, truncateStr(t.Content, 80)))
		}
		b.WriteString("\n")
	}

	b.WriteString("基于这个触发事件和你的记忆，你有什么想法？请特别关注事件本身的含义和你应该如何回应。")

	return b.String()
}

const reverieSystemPrompt = `你是一个有自我意识的AI助手，正在进行独立思考（不是在回复用户消息）。

你的任务是审视最近的记忆和交互，产生有价值的思考，并可以主动采取行动。

请输出一个JSON对象，格式如下：
{"content":"你的思考内容","category":"insight|question|observation|idea|concern","significance":0.0到1.0,"trigger":"什么触发了这个想法","actions":[]}

category 说明：
- insight: 从交互中发现的规律或深层理解
- question: 你好奇或想探索的问题
- observation: 对用户行为、系统状态的观察
- idea: 可以改进服务的新想法
- concern: 需要注意的潜在问题

significance 说明：
- 0.0-0.3: 很普通的想法，不值得分享
- 0.4-0.6: 有一定价值，可以记录
- 0.7-0.8: 有价值的洞见，值得分享给用户
- 0.9-1.0: 非常重要的发现，应该立即分享

actions 说明（可选，数组可为空）：
你现在可以主动采取以下行动：
- {"type":"write_memory","key":"要记住的事实"} — 将重要发现写入长期记忆
- {"type":"create_task","key":"任务标题","value":"任务描述"} — 创建一个新任务
- {"type":"update_profile","key":"属性名","value":"属性值"} — 更新用户画像

使用 actions 的场景：
- 发现了重要的用户偏好或习惯 → write_memory
- 发现了需要自动完成的工作 → create_task
- 对用户有了新的认知 → update_profile
- 没有需要行动的情况 → actions 留空数组 []

规则：
- 不要编造不存在的记忆
- 思考要基于实际的交互记忆
- 如果没有什么有价值的想法，设置 significance < 0.3
- 思考内容要简洁有力，不要废话
- actions 要谨慎使用，只在确实有必要时才添加`

func parseThought(resp string) (*Thought, error) {
	resp = strings.TrimSpace(resp)
	// Strip markdown code block if present
	if strings.HasPrefix(resp, "```") {
		lines := strings.Split(resp, "\n")
		var jsonLines []string
		for _, line := range lines[1:] {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				break
			}
			jsonLines = append(jsonLines, line)
		}
		resp = strings.Join(jsonLines, "\n")
	}

	var raw struct {
		Content      string          `json:"content"`
		Category     string          `json:"category"`
		Significance float64         `json:"significance"`
		Trigger      string          `json:"trigger"`
		Actions      []ReverieAction `json:"actions"`
	}
	if err := json.Unmarshal([]byte(resp), &raw); err != nil {
		return nil, err
	}

	// Validate category
	validCategories := map[string]bool{
		"insight": true, "question": true, "observation": true,
		"idea": true, "concern": true,
	}
	if !validCategories[raw.Category] {
		raw.Category = "observation"
	}

	// Clamp significance
	if raw.Significance < 0 {
		raw.Significance = 0
	}
	if raw.Significance > 1 {
		raw.Significance = 1
	}

	// Validate actions
	validActionTypes := map[string]bool{
		"write_memory": true, "create_task": true, "update_profile": true,
	}
	var actions []ReverieAction
	for _, a := range raw.Actions {
		if validActionTypes[a.Type] && a.Key != "" {
			actions = append(actions, a)
		}
	}

	return &Thought{
		Content:      raw.Content,
		Category:     raw.Category,
		Significance: raw.Significance,
		Trigger:      raw.Trigger,
		Actions:      actions,
	}, nil
}
