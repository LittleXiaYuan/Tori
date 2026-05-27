package iterate

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// Perspective defines a discussion participant's viewpoint.
type Perspective struct {
	Role   string // e.g. "安全审查员"
	Prompt string // system prompt for this participant
}

// Message represents a single message in the discussion.
type Message struct {
	From    string `json:"from"`
	Content string `json:"content"`
	Round   int    `json:"round"`
}

// Conclusion is the final result of a multi-agent discussion.
type Conclusion struct {
	Approved   bool      `json:"approved"`
	Summary    string    `json:"summary"`
	Messages   []Message `json:"messages"`
	TokensUsed int       `json:"tokens_used"`
}

// Discusser orchestrates multi-agent discussions for proposal review.
type Discusser struct {
	llmCall   LLMCallFunc // from engine, uses Fast tier
	maxRounds int
}

// NewDiscusser creates a discussion orchestrator.
func NewDiscusser(llmCall LLMCallFunc) *Discusser {
	return &Discusser{
		llmCall:   llmCall,
		maxRounds: 3,
	}
}

// SetMaxRounds overrides the default round limit.
func (d *Discusser) SetMaxRounds(n int) {
	if n > 0 && n <= 5 {
		d.maxRounds = n
	}
}

// builtinPerspectives maps role names to system prompts.
var builtinPerspectives = map[string]Perspective{
	"安全审查员": {
		Role: "安全审查员",
		Prompt: "你是云雀Agent的安全审查员。你的职责是评估提案的安全风险。\n" +
			"关注：权限提升、数据泄露、恶意代码注入、不可逆操作。\n" +
			"回答格式：先给出\"安全/有风险\"判断，再说明理由（1-2句话）。",
	},
	"性能优化师": {
		Role: "性能优化师",
		Prompt: "你是云雀Agent的性能优化师。你的职责是评估提案对性能的影响。\n" +
			"关注：Token消耗、延迟影响、资源占用、缓存效率。\n" +
			"回答格式：先给出\"可行/需优化\"判断，再说明理由（1-2句话）。",
	},
	"用户体验师": {
		Role: "用户体验师",
		Prompt: "你是云雀Agent的用户体验师。你的职责是评估提案对用户体验的影响。\n" +
			"关注：响应质量、交互流畅度、功能实用性、是否符合用户期望。\n" +
			"回答格式：先给出\"推荐/不推荐\"判断，再说明理由（1-2句话）。",
	},
}

// RunDiscussion orchestrates a multi-round discussion among the given participants.
// Returns a Conclusion with the consensus and all messages exchanged.
func (d *Discusser) RunDiscussion(ctx context.Context, topic string, participants []string) (*Conclusion, error) {
	if d.llmCall == nil {
		return nil, fmt.Errorf("no LLM call function configured")
	}
	if len(participants) == 0 {
		return nil, fmt.Errorf("no participants specified")
	}

	// Resolve perspectives
	perspectives := make([]Perspective, 0, len(participants))
	for _, name := range participants {
		if p, ok := builtinPerspectives[name]; ok {
			perspectives = append(perspectives, p)
		} else {
			// Create ad-hoc perspective
			perspectives = append(perspectives, Perspective{
				Role:   name,
				Prompt: fmt.Sprintf("你是%s。请从你的专业角度评审以下提案。回答格式：判断 + 1-2句理由。", name),
			})
		}
	}

	var allMessages []Message
	totalTokens := 0

	// Each round: all participants respond to the topic + prior messages
	for round := 1; round <= d.maxRounds; round++ {
		roundMessages := make([]Message, 0, len(perspectives))

		for _, p := range perspectives {
			select {
			case <-ctx.Done():
				return &Conclusion{
					Summary:    "讨论被取消",
					Messages:   allMessages,
					TokensUsed: totalTokens,
				}, ctx.Err()
			default:
			}

			// Build context for this participant
			userPrompt := d.buildParticipantPrompt(topic, allMessages, p.Role, round)

			reply, tokens, err := d.llmCall(ctx, p.Prompt, userPrompt)
			if err != nil {
				slog.Warn("discuss: participant failed", "role", p.Role, "round", round, "err", err)
				continue
			}
			totalTokens += tokens

			msg := Message{From: p.Role, Content: reply, Round: round}
			roundMessages = append(roundMessages, msg)
		}

		allMessages = append(allMessages, roundMessages...)

		// Check early convergence after round 1
		if round >= 1 && d.hasConsensus(roundMessages) {
			break
		}
	}

	// Synthesize conclusion
	conclusion := d.synthesize(allMessages, totalTokens)
	return conclusion, nil
}

// buildParticipantPrompt constructs what a participant sees on their turn.
func (d *Discusser) buildParticipantPrompt(topic string, history []Message, myRole string, round int) string {
	var b strings.Builder
	b.WriteString("## 讨论主题\n")
	b.WriteString(topic)
	b.WriteString("\n\n")

	if len(history) > 0 {
		b.WriteString("## 此前讨论记录\n")
		for _, m := range history {
			fmt.Fprintf(&b, "[%s] (第%d轮): %s\n", m.From, m.Round, m.Content)
		}
		b.WriteString("\n")
	}

	if round == 1 {
		b.WriteString("请从你的专业角度给出初步评审意见。")
	} else {
		b.WriteString("请考虑其他参与者的意见，给出你的更新看法。如果你同意共识，可以简短回复\"同意\"加理由。")
	}
	return b.String()
}

// hasConsensus checks if all participants in a round reached the same verdict.
func (d *Discusser) hasConsensus(msgs []Message) bool {
	if len(msgs) < 2 {
		return false
	}
	approvals := 0
	for _, m := range msgs {
		lower := strings.ToLower(m.Content)
		if containsAny(lower, "安全", "可行", "推荐", "同意", "approved") {
			approvals++
		}
	}
	// Consensus if all approve or all reject
	return approvals == len(msgs) || approvals == 0
}

// synthesize produces the final conclusion from all messages.
func (d *Discusser) synthesize(msgs []Message, tokensUsed int) *Conclusion {
	if len(msgs) == 0 {
		return &Conclusion{
			Summary:    "无讨论结果",
			TokensUsed: tokensUsed,
		}
	}

	// Count approval signals across all messages
	approvals, rejections := 0, 0
	var summaryParts []string

	for _, m := range msgs {
		lower := strings.ToLower(m.Content)
		if containsAny(lower, "安全", "可行", "推荐", "同意", "approved") {
			approvals++
		}
		if containsAny(lower, "有风险", "不推荐", "rejected", "需优化", "反对") {
			rejections++
		}
		// Use last-round messages for summary
		if m.Round >= d.lastRound(msgs) {
			summaryParts = append(summaryParts, fmt.Sprintf("[%s] %s", m.From, truncate(m.Content, 100)))
		}
	}

	approved := approvals > rejections
	summary := strings.Join(summaryParts, "; ")

	return &Conclusion{
		Approved:   approved,
		Summary:    summary,
		Messages:   msgs,
		TokensUsed: tokensUsed,
	}
}

// lastRound finds the maximum round number in the messages.
func (d *Discusser) lastRound(msgs []Message) int {
	max := 0
	for _, m := range msgs {
		if m.Round > max {
			max = m.Round
		}
	}
	return max
}

// ── Helpers ──

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func truncate(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen]) + "..."
}
