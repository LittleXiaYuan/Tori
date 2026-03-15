package reflect

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"yunque-agent/internal/agentcore/llm"
)

// LearningLoop implements continuous self-improvement.
// After each interaction, it evaluates quality, extracts lessons, and updates strategy.
type LearningLoop struct {
	llm      *llm.Client
	onUpdate func(key, value string) // callback to update memory
	engine   *Engine
}

// NewLearningLoop creates a learning loop.
func NewLearningLoop(llmClient *llm.Client, onUpdate func(key, value string)) *LearningLoop {
	return &LearningLoop{llm: llmClient, onUpdate: onUpdate, engine: NewEngine(llmClient)}
}

// Reflect returns the underlying reflect engine.
func (l *LearningLoop) Reflect() *Engine { return l.engine }

// Lesson is a learned insight from an interaction.
type Lesson struct {
	Category string `json:"category"` // "skill_usage", "user_preference", "domain_knowledge", "error_pattern"
	Insight  string `json:"insight"`
	Action   string `json:"action"` // "remember", "adjust_strategy", "add_skill", "flag_for_review"
}

// AfterInteraction runs after each agent interaction to extract lessons.
func (l *LearningLoop) AfterInteraction(ctx context.Context, userMsg, agentReply string, skillsUsed []string, quality int) {
	if quality >= 8 {
		// High quality — just note what worked
		if l.onUpdate != nil {
			l.onUpdate("strategy:success:"+time.Now().Format("20060102_150405"),
				"User asked: "+truncateStr(userMsg, 100)+" → Skills: "+joinStr(skillsUsed)+" → Quality: high")
		}
		return
	}

	// Lower quality — ask LLM to extract lessons
	go func() {
		lessons := l.extractLessons(ctx, userMsg, agentReply, skillsUsed, quality)
		for _, lesson := range lessons {
			slog.Info("learning loop", "category", lesson.Category, "insight", lesson.Insight, "action", lesson.Action)
			if l.onUpdate != nil {
				l.onUpdate("lesson:"+lesson.Category+":"+time.Now().Format("20060102_150405"), lesson.Insight)
			}
		}
	}()
}

func (l *LearningLoop) extractLessons(ctx context.Context, userMsg, agentReply string, skillsUsed []string, quality int) []Lesson {
	prompt := `分析以下交互，提取可学习的经验。

用户: ` + truncateStr(userMsg, 200) + `
助手回复: ` + truncateStr(agentReply, 300) + `
使用技能: ` + joinStr(skillsUsed) + `
质量评分: ` + intToStr(quality) + `/10

请输出JSON数组，每个元素包含:
- category: skill_usage/user_preference/domain_knowledge/error_pattern
- insight: 学到的经验
- action: remember/adjust_strategy/add_skill/flag_for_review

只输出JSON数组。`

	reply, err := l.llm.Chat(ctx, []llm.Message{
		{Role: "system", Content: "你是经验提取器，只输出JSON数组。"},
		{Role: "user", Content: prompt},
	}, 0.1)
	if err != nil {
		return nil
	}

	// Parse JSON array of lessons
	var lessons []Lesson
	// Find JSON array in reply
	start := -1
	for i, c := range reply {
		if c == '[' {
			start = i
			break
		}
	}
	if start < 0 {
		return nil
	}
	end := -1
	depth := 0
	for i := start; i < len(reply); i++ {
		switch reply[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return nil
	}
	if err := json.Unmarshal([]byte(reply[start:end+1]), &lessons); err != nil {
		slog.Warn("learning loop: parse lessons failed", "err", err)
		return nil
	}
	return lessons
}

func truncateStr(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "..."
}

func joinStr(ss []string) string {
	if len(ss) == 0 {
		return "none"
	}
	result := ss[0]
	for _, s := range ss[1:] {
		result += ", " + s
	}
	return result
}

func intToStr(n int) string {
	if n < 0 {
		return "-" + intToStr(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return intToStr(n/10) + string(rune('0'+n%10))
}
