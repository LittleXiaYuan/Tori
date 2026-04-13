package emotion

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Emotion represents a detected emotional state.
type Emotion string

const (
	EmotionHappy     Emotion = "happy"
	EmotionSad       Emotion = "sad"
	EmotionAngry     Emotion = "angry"
	EmotionNeutral   Emotion = "neutral"
	EmotionFearful   Emotion = "fearful"
	EmotionDisgusted Emotion = "disgusted"
	EmotionSurprised Emotion = "surprised"
	EmotionUnknown   Emotion = "unknown"
)

// AllEmotions lists all recognized emotion categories.
var AllEmotions = []Emotion{
	EmotionHappy, EmotionSad, EmotionAngry, EmotionNeutral,
	EmotionFearful, EmotionDisgusted, EmotionSurprised,
}

// Result holds the outcome of emotion analysis.
type Result struct {
	Emotion    Emotion `json:"emotion"`
	Confidence float64 `json:"confidence"` // 0.0 - 1.0
	Source     string  `json:"source"`     // "text", "audio", "default", "fallback"
}

// IsPositive returns true for positive emotions.
func (r Result) IsPositive() bool {
	return r.Emotion == EmotionHappy || r.Emotion == EmotionSurprised
}

// IsNegative returns true for negative emotions.
func (r Result) IsNegative() bool {
	return r.Emotion == EmotionSad || r.Emotion == EmotionAngry ||
		r.Emotion == EmotionFearful || r.Emotion == EmotionDisgusted
}

// ContextSnippet returns a system prompt snippet with the emotion context.
// Returns empty string for neutral/unknown emotions.
func (r *Result) ContextSnippet() string {
	return r.ContextSnippetLocale("zh")
}

// ContextSnippetLocale returns a locale-aware system prompt snippet.
func (r *Result) ContextSnippetLocale(locale string) string {
	if r == nil || r.Emotion == EmotionNeutral || r.Emotion == EmotionUnknown {
		return ""
	}
	label := emotionLabelLocale(r.Emotion, locale)
	conf := r.Confidence * 100
	switch locale {
	case "en":
		return fmt.Sprintf("[User emotion: %s (confidence: %.0f%%)] Please adjust your tone and care level based on the user's current emotion.",
			label, conf)
	case "ja":
		return fmt.Sprintf("[ユーザーの感情: %s (確信度: %.0f%%)] ユーザーの感情に合わせて、返答のトーンと配慮を調整してください。",
			label, conf)
	default:
		return fmt.Sprintf("[用户情绪: %s (置信度: %.0f%%)] 请根据用户当前情绪适当调整回复的语气和关怀程度。",
			label, conf)
	}
}

// emotionLabel returns a Chinese description for the emotion.
func emotionLabel(e Emotion) string {
	return emotionLabelLocale(e, "zh")
}

// emotionLabelLocale returns a locale-aware description for the emotion.
func emotionLabelLocale(e Emotion, locale string) string {
	switch locale {
	case "en":
		switch e {
		case EmotionHappy:
			return "happy 😊"
		case EmotionSad:
			return "sad 😢"
		case EmotionAngry:
			return "angry 😠"
		case EmotionFearful:
			return "anxious 😰"
		case EmotionDisgusted:
			return "disgusted 😒"
		case EmotionSurprised:
			return "surprised 😮"
		case EmotionNeutral:
			return "neutral"
		default:
			return string(e)
		}
	case "ja":
		switch e {
		case EmotionHappy:
			return "嬉しい 😊"
		case EmotionSad:
			return "悲しい 😢"
		case EmotionAngry:
			return "怒り 😠"
		case EmotionFearful:
			return "不安 😰"
		case EmotionDisgusted:
			return "嫌悪 😒"
		case EmotionSurprised:
			return "驚き 😮"
		case EmotionNeutral:
			return "穏やか"
		default:
			return string(e)
		}
	default:
		switch e {
		case EmotionHappy:
			return "开心 😊"
		case EmotionSad:
			return "悲伤 😢"
		case EmotionAngry:
			return "愤怒 😠"
		case EmotionFearful:
			return "焦虑 😰"
		case EmotionDisgusted:
			return "反感 😒"
		case EmotionSurprised:
			return "惊讶 😮"
		case EmotionNeutral:
			return "平静"
		default:
			return string(e)
		}
	}
}

// ────────────────────────────────────────
// LLM-based text emotion Analyzer
// ────────────────────────────────────────

// LLMCallFunc calls an LLM with system + user prompt, returns the response text.
type LLMCallFunc func(ctx context.Context, systemPrompt, userMsg string) (string, error)

// Analyzer detects emotion from text via LLM.
type Analyzer struct {
	mu      sync.RWMutex
	llmCall LLMCallFunc
	enabled bool
	locale  string // "zh", "en", "ja" etc. defaults to "zh"
}

// NewAnalyzer creates an emotion analyzer (enabled by default).
func NewAnalyzer() *Analyzer {
	return &Analyzer{enabled: true, locale: "zh"}
}

// SetLLMCall sets the LLM function for text-based emotion analysis.
func (a *Analyzer) SetLLMCall(fn LLMCallFunc) { a.llmCall = fn }

// SetEnabled enables or disables the analyzer.
func (a *Analyzer) SetEnabled(v bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.enabled = v
}

// Enabled returns whether the analyzer is active.
func (a *Analyzer) Enabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.enabled
}

// SetLocale sets the language for emotion prompts ("zh", "en", "ja").
func (a *Analyzer) SetLocale(locale string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.locale = locale
}

// Locale returns the current locale.
func (a *Analyzer) Locale() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.locale == "" {
		return "zh"
	}
	return a.locale
}

const emotionSystemPrompt = `你是一个情绪分析专家。分析用户消息中表达的情绪，只返回一个JSON对象，不要其他内容。
格式：{"emotion":"<类别>","confidence":<0-1的数值>}

情绪类别(只能选一个)：
- happy: 开心、愉快、兴奋、感激
- sad: 悲伤、失落、沮丧、遗憾
- angry: 愤怒、烦躁、不满、生气
- neutral: 平静、客观、陈述事实
- fearful: 害怕、担忧、焦虑、不安
- disgusted: 厌恶、反感、鄙视
- surprised: 惊讶、意外、震惊`

// emotionSystemPrompts maps locale to the appropriate system prompt.
var emotionSystemPrompts = map[string]string{
	"zh": emotionSystemPrompt,
	"en": `You are an emotion analysis expert. Analyze the user's message for emotional content. Return ONLY a JSON object, nothing else.
Format: {"emotion":"<category>","confidence":<0-1 number>}

Emotion categories (pick exactly one):
- happy: joyful, excited, grateful, pleased
- sad: sorrowful, disappointed, depressed, regretful
- angry: furious, irritated, dissatisfied, enraged
- neutral: calm, objective, factual
- fearful: afraid, worried, anxious, uneasy
- disgusted: repulsed, contemptuous, averse
- surprised: astonished, shocked, unexpected`,
	"ja": `あなたは感情分析の専門家です。ユーザーのメッセージから感情を分析し、JSONオブジェクトのみを返してください。
形式: {"emotion":"<カテゴリ>","confidence":<0-1の数値>}

感情カテゴリ（1つだけ選択）:
- happy: 嬉しい、楽しい、興奮、感謝
- sad: 悲しい、失望、落胆、後悔
- angry: 怒り、苛立ち、不満
- neutral: 穏やか、客観的、事実の陳述
- fearful: 恐怖、心配、不安
- disgusted: 嫌悪、反感、軽蔑
- surprised: 驚き、意外、衝撃`,
}

// getEmotionPrompt returns the system prompt for the analyzer's locale.
func (a *Analyzer) getEmotionPrompt() string {
	locale := a.Locale()
	if p, ok := emotionSystemPrompts[locale]; ok {
		return p
	}
	return emotionSystemPrompts["zh"]
}

// AnalyzeText detects emotion from text using LLM.
// Returns a neutral result if the analyzer is disabled or LLM is unavailable.
func (a *Analyzer) AnalyzeText(ctx context.Context, text string) (*Result, error) {
	if !a.Enabled() || a.llmCall == nil {
		return &Result{Emotion: EmotionNeutral, Source: "default"}, nil
	}
	if strings.TrimSpace(text) == "" {
		return &Result{Emotion: EmotionNeutral, Source: "default"}, nil
	}

	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	resp, err := a.llmCall(callCtx, a.getEmotionPrompt(), text)
	if err != nil {
		slog.Warn("emotion: llm call failed", "err", err)
		return &Result{Emotion: EmotionNeutral, Source: "fallback"}, nil
	}
	return parseEmotionResponse(resp)
}

// parseEmotionResponse parses the LLM JSON response into a Result.
func parseEmotionResponse(resp string) (*Result, error) {
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
		Emotion    string  `json:"emotion"`
		Confidence float64 `json:"confidence"`
	}
	if err := json.Unmarshal([]byte(resp), &raw); err != nil {
		return nil, fmt.Errorf("emotion: parse response: %w", err)
	}

	emotion := Emotion(strings.ToLower(raw.Emotion))
	valid := false
	for _, e := range AllEmotions {
		if e == emotion {
			valid = true
			break
		}
	}
	if !valid {
		emotion = EmotionUnknown
	}

	return &Result{
		Emotion:    emotion,
		Confidence: raw.Confidence,
		Source:     "text",
	}, nil
}
