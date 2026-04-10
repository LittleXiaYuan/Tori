package modes

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
)

// StanceGenerator converts a Judgment into a natural-language Stance.
//
// When LLM is available, it generates contextual, non-templated expressions.
// When LLM is unavailable, it falls back to curated template pools.
type StanceGenerator struct {
	llmCall LLMCallFunc
	locale  string
}

// NewStanceGenerator creates a stance generator.
func NewStanceGenerator(llmCall LLMCallFunc, locale string) *StanceGenerator {
	if locale == "" {
		locale = "zh"
	}
	return &StanceGenerator{llmCall: llmCall, locale: locale}
}

// Generate produces a Stance from a Judgment, shaped by the mode's Tone.
func (sg *StanceGenerator) Generate(ctx context.Context, j *Judgment, tone Tone, input string) (*Stance, error) {
	if j == nil || j.Valence == 0 {
		return &Stance{Position: PositionNeutral}, nil
	}

	pos := PositionSupport
	if j.Valence < 0 {
		pos = PositionOppose
	}

	// Try LLM-generated stance first
	if sg.llmCall != nil {
		stance, err := sg.llmGenerate(ctx, j, tone, input)
		if err == nil && stance != nil {
			stance.Position = pos
			stance.Intensity = j.Strength
			return stance, nil
		}
		slog.Debug("modes/stance: llm generation failed, using templates", "err", err)
	}

	// Fallback to templates
	return sg.templateFallback(j, tone, pos), nil
}

// ─── LLM-based stance generation ────────────────────────────────────────────

func (sg *StanceGenerator) llmGenerate(ctx context.Context, j *Judgment, tone Tone, input string) (*Stance, error) {
	sysPrompt := sg.buildStancePrompt(tone)
	userPrompt := sg.buildStanceInput(j, input)

	resp, err := sg.llmCall(ctx, sysPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	return sg.parseStanceResponse(resp)
}

func (sg *StanceGenerator) buildStancePrompt(tone Tone) string {
	if sg.locale == "en" {
		return fmt.Sprintf(`You are a stance expression generator. Given a value judgment, produce a natural response.

Tone parameters (0-1 scale):
- Directness: %.1f (0=indirect/hedging, 1=blunt/straightforward)
- Warmth: %.1f (0=cold/clinical, 1=warm/caring)
- Formality: %.1f (0=casual, 1=formal)
- Assertiveness: %.1f (0=passive/tentative, 1=confident/firm)

Return ONLY JSON: {"text":"your expression","reasoning":"why you chose this expression"}

Rules:
- Match the tone parameters precisely
- High directness = no hedging, no "I think maybe..."
- High assertiveness = confident statements, not questions
- High warmth = caring even when disagreeing
- NEVER be sycophantic. If the judgment is negative, express disagreement clearly.`, tone.Directness, tone.Warmth, tone.Formality, tone.Assertiveness)
	}

	return fmt.Sprintf(`你是一个立场表达生成器。根据价值判断，生成自然的回应。

语气参数（0-1）：
- 直率度: %.1f（0=委婉含蓄，1=直言不讳）
- 温暖度: %.1f（0=冷淡客观，1=温暖关怀）
- 正式度: %.1f（0=随意口语，1=正式书面）
- 主张度: %.1f（0=被动试探，1=自信坚定）

只返回JSON：{"text":"你的表达","reasoning":"为什么这样表达"}

规则：
- 严格匹配语气参数
- 高直率度 = 不绕弯子，不说"我觉得可能也许..."
- 高主张度 = 自信的陈述，不是疑问句
- 高温暖度 = 即使不同意也要关怀对方
- 绝对不能奉承。如果判断是负面的，必须清楚表达不同意。`, tone.Directness, tone.Warmth, tone.Formality, tone.Assertiveness)
}

func (sg *StanceGenerator) buildStanceInput(j *Judgment, input string) string {
	var sb strings.Builder
	if sg.locale == "en" {
		fmt.Fprintf(&sb, "User said: %s\n", input)
		fmt.Fprintf(&sb, "Judgment: valence=%d, strength=%.2f\n", j.Valence, j.Strength)
		fmt.Fprintf(&sb, "Reason: %s", j.Reasoning)
	} else {
		fmt.Fprintf(&sb, "用户说：%s\n", input)
		fmt.Fprintf(&sb, "判断：倾向=%d，强度=%.2f\n", j.Valence, j.Strength)
		fmt.Fprintf(&sb, "理由：%s", j.Reasoning)
	}
	return sb.String()
}

func (sg *StanceGenerator) parseStanceResponse(resp string) (*Stance, error) {
	resp = strings.TrimSpace(resp)
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
		Text      string `json:"text"`
		Reasoning string `json:"reasoning"`
	}
	if err := json.Unmarshal([]byte(resp), &raw); err != nil {
		return nil, fmt.Errorf("modes/stance: parse response: %w", err)
	}

	return &Stance{
		Text:      raw.Text,
		Reasoning: raw.Reasoning,
	}, nil
}

// ─── Template fallback ──────────────────────────────────────────────────────

func (sg *StanceGenerator) templateFallback(j *Judgment, tone Tone, pos Position) *Stance {
	var pool []string

	if sg.locale == "en" {
		pool = sg.englishTemplates(pos, tone)
	} else {
		pool = sg.chineseTemplates(pos, tone)
	}

	text := pool[rand.Intn(len(pool))]

	return &Stance{
		Position:  pos,
		Intensity: j.Strength,
		Text:      text,
		Reasoning: j.Reasoning,
		Tone:      tone,
	}
}

func (sg *StanceGenerator) chineseTemplates(pos Position, tone Tone) []string {
	if pos == PositionSupport {
		if tone.Directness > 0.7 {
			return []string{
				"这个想法我很认同！",
				"说得对，我完全同意。",
				"这正是我想说的。",
				"没错，这个方向是对的。",
			}
		}
		return []string{
			"这个想法很不错。",
			"我觉得这样挺好的。",
			"我支持这个方向。",
			"这个做法我认同。",
		}
	}

	// Oppose
	if tone.Directness > 0.7 {
		return []string{
			"说实话，我不太认同这个。",
			"我觉得这样不太好。",
			"抱歉，但我不同意这个观点。",
			"这个做法有问题。",
		}
	}
	return []string{
		"我觉得这样可能不太合适。",
		"也许我们可以换个角度想。",
		"我有点担心这个做法。",
		"这样做可能会有些问题。",
	}
}

func (sg *StanceGenerator) englishTemplates(pos Position, tone Tone) []string {
	if pos == PositionSupport {
		if tone.Directness > 0.7 {
			return []string{
				"I really like this idea!",
				"You're right, I fully agree.",
				"That's exactly what I was thinking.",
				"Yes, this is the right direction.",
			}
		}
		return []string{
			"That's a solid idea.",
			"I think this works well.",
			"I support this direction.",
			"This approach makes sense.",
		}
	}

	if tone.Directness > 0.7 {
		return []string{
			"Honestly, I don't agree with this.",
			"I don't think this is a good approach.",
			"Sorry, but I disagree.",
			"There's a problem with this approach.",
		}
	}
	return []string{
		"I'm not sure this is the best approach.",
		"Maybe we could look at this differently.",
		"I have some concerns about this.",
		"This might cause some issues.",
	}
}
