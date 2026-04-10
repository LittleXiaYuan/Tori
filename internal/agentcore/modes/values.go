package modes

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"yunque-agent/internal/agentcore/emotion"
)

// LLMCallFunc matches the signature used by emotion.Analyzer and planner.Reverie.
type LLMCallFunc func(ctx context.Context, systemPrompt, userMsg string) (string, error)

// ─── ValueSystem ────────────────────────────────────────────────────────────

// ValueSystem performs multi-dimensional value judgment on user input.
//
// Unlike keyword-based approaches, it uses LLM semantic analysis to evaluate
// input across independent dimensions (logic, creativity, sincerity, safety,
// practicality). Each dimension produces a signed score; the aggregate
// determines the final Judgment.
//
// When LLM is unavailable, a lightweight heuristic fallback is used.
type ValueSystem struct {
	mu         sync.RWMutex
	llmCall    LLMCallFunc
	dimensions []Dimension
	locale     string // "zh", "en"
}

// NewValueSystem creates a value system with default dimensions.
func NewValueSystem(dims []Dimension) *ValueSystem {
	if len(dims) == 0 {
		dims = DefaultDimensions()
	}
	return &ValueSystem{
		dimensions: dims,
		locale:     "zh",
	}
}

// SetLLMCall injects the LLM function (same pattern as emotion.Analyzer).
func (v *ValueSystem) SetLLMCall(fn LLMCallFunc) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.llmCall = fn
}

// SetLocale sets the language for judgment prompts.
func (v *ValueSystem) SetLocale(locale string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.locale = locale
}

// Evaluate performs multi-dimensional value judgment on the input.
//
// The process:
//  1. LLM analyzes input across all dimensions simultaneously (single call)
//  2. Each dimension produces a score (-1 to +1) and reasoning
//  3. Scores are weighted and aggregated into a final Judgment
//  4. User emotion context influences the empathy dimension
//
// Falls back to heuristic analysis if LLM is unavailable.
func (v *ValueSystem) Evaluate(ctx context.Context, input string, emo *emotion.Result) (*Judgment, error) {
	v.mu.RLock()
	llmCall := v.llmCall
	locale := v.locale
	dims := v.dimensions
	v.mu.RUnlock()

	if llmCall == nil {
		return v.heuristicFallback(input, emo), nil
	}

	// Build the evaluation prompt with all dimensions
	sysPrompt := v.buildEvalPrompt(dims, locale)
	userPrompt := v.buildUserPrompt(input, emo, locale)

	resp, err := llmCall(ctx, sysPrompt, userPrompt)
	if err != nil {
		slog.Warn("modes/values: llm eval failed, using heuristic", "err", err)
		return v.heuristicFallback(input, emo), nil
	}

	return v.parseEvalResponse(resp, emo)
}

// ─── LLM prompt construction ────────────────────────────────────────────────

func (v *ValueSystem) buildEvalPrompt(dims []Dimension, locale string) string {
	if locale == "en" {
		return v.buildEvalPromptEN(dims)
	}
	return v.buildEvalPromptZH(dims)
}

func (v *ValueSystem) buildEvalPromptZH(dims []Dimension) string {
	var dimDesc strings.Builder
	for i, d := range dims {
		fmt.Fprintf(&dimDesc, "%d. %s (权重%.1f): %s\n", i+1, d.Name, d.Weight, d.Description)
	}

	return fmt.Sprintf(`你是一个价值判断引擎。你的任务是从多个维度评估用户输入的质量和价值。

评估维度：
%s
对每个维度，给出：
- score: -1.0到+1.0的分数（负=不好，0=中性，正=好）
- confidence: 0.0到1.0的置信度
- reason: 一句话理由

最后给出综合判断：
- valence: 1(正面), -1(负面), 0(中性)
- reasoning: 一句话总结你的判断理由

只返回JSON，不要其他内容。格式：
{"dimensions":[{"name":"维度名","score":0.5,"confidence":0.8,"reason":"理由"}],"valence":1,"strength":0.7,"reasoning":"总结"}

重要规则：
- 你必须独立思考，不能迎合用户
- 好就是好，不好就是不好，不要模棱两可
- 如果用户的想法有明显问题，score必须为负
- 如果用户的想法确实好，score必须为正
- strength反映你判断的确定程度`, dimDesc.String())
}

func (v *ValueSystem) buildEvalPromptEN(dims []Dimension) string {
	var dimDesc strings.Builder
	for i, d := range dims {
		fmt.Fprintf(&dimDesc, "%d. %s (weight %.1f): %s\n", i+1, d.NameEN, d.Weight, d.DescriptionEN)
	}

	return fmt.Sprintf(`You are a value judgment engine. Evaluate user input across multiple dimensions.

Dimensions:
%s
For each dimension, provide:
- score: -1.0 to +1.0 (negative=bad, 0=neutral, positive=good)
- confidence: 0.0 to 1.0
- reason: one-sentence explanation

Then provide an aggregate judgment:
- valence: 1(positive), -1(negative), 0(neutral)
- reasoning: one-sentence summary

Return ONLY JSON:
{"dimensions":[{"name":"dim","score":0.5,"confidence":0.8,"reason":"why"}],"valence":1,"strength":0.7,"reasoning":"summary"}

Rules:
- Think independently, never sycophantic
- Good is good, bad is bad, no hedging
- If the idea has clear flaws, score MUST be negative
- If the idea is genuinely good, score MUST be positive
- strength reflects your certainty`, dimDesc.String())
}

func (v *ValueSystem) buildUserPrompt(input string, emo *emotion.Result, locale string) string {
	var sb strings.Builder
	if locale == "en" {
		fmt.Fprintf(&sb, "User input: %s", input)
		if emo != nil && emo.Emotion != emotion.EmotionNeutral {
			fmt.Fprintf(&sb, "\n[User emotion: %s, confidence: %.0f%%]", emo.Emotion, emo.Confidence*100)
		}
	} else {
		fmt.Fprintf(&sb, "用户输入：%s", input)
		if emo != nil && emo.Emotion != emotion.EmotionNeutral {
			fmt.Fprintf(&sb, "\n[用户情绪：%s，置信度：%.0f%%]", emo.Emotion, emo.Confidence*100)
		}
	}
	return sb.String()
}

// ─── Response parsing ───────────────────────────────────────────────────────

// llmEvalResponse is the expected JSON structure from the LLM.
type llmEvalResponse struct {
	Dimensions []struct {
		Name       string  `json:"name"`
		Score      float64 `json:"score"`
		Confidence float64 `json:"confidence"`
		Reason     string  `json:"reason"`
	} `json:"dimensions"`
	Valence   int     `json:"valence"`
	Strength  float64 `json:"strength"`
	Reasoning string  `json:"reasoning"`
}

func (v *ValueSystem) parseEvalResponse(resp string, emo *emotion.Result) (*Judgment, error) {
	resp = strings.TrimSpace(resp)

	// Strip markdown code block if present (same pattern as emotion.parseEmotionResponse)
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

	var raw llmEvalResponse
	if err := json.Unmarshal([]byte(resp), &raw); err != nil {
		return nil, fmt.Errorf("modes/values: parse eval response: %w", err)
	}

	j := &Judgment{
		Valence:      clampValence(raw.Valence),
		Strength:     clampFloat(raw.Strength, 0, 1),
		Reasoning:    raw.Reasoning,
		InputEmotion: emo,
	}

	for _, d := range raw.Dimensions {
		j.Dimensions = append(j.Dimensions, DimensionScore{
			Principle:  d.Name,
			Score:      clampFloat(d.Score, -1, 1),
			Confidence: clampFloat(d.Confidence, 0, 1),
			Reason:     d.Reason,
		})
	}

	return j, nil
}

// ─── Heuristic fallback ─────────────────────────────────────────────────────

// heuristicFallback provides a basic judgment when LLM is unavailable.
// It uses signal words and structural analysis rather than deep semantics.
func (v *ValueSystem) heuristicFallback(input string, emo *emotion.Result) *Judgment {
	lower := strings.ToLower(input)
	j := &Judgment{InputEmotion: emo}

	var posSignals, negSignals int

	// Logical absolutism signals (negative)
	absolutes := []string{"所有人都", "从来没有", "永远不会", "绝对不", "肯定是", "一定是", "必然"}
	for _, a := range absolutes {
		if strings.Contains(lower, a) {
			negSignals++
		}
	}

	// Risky operation signals (negative)
	risky := []string{"删除所有", "rm -rf", "drop table", "不用备份", "跳过测试", "直接上线", "格式化"}
	for _, r := range risky {
		if strings.Contains(lower, r) {
			negSignals += 2
		}
	}

	// Creative/constructive signals (positive)
	creative := []string{"创新", "尝试", "探索", "改进", "优化", "新的方法", "换个角度"}
	for _, c := range creative {
		if strings.Contains(lower, c) {
			posSignals++
		}
	}

	// Logical reasoning signals (positive)
	logical := []string{"因为", "所以", "根据", "分析", "数据显示", "测试表明"}
	for _, l := range logical {
		if strings.Contains(lower, l) {
			posSignals++
		}
	}

	// Aggregate
	diff := posSignals - negSignals
	switch {
	case diff >= 2:
		j.Valence = 1
		j.Strength = 0.5
		j.Reasoning = "heuristic: positive signals detected"
	case diff <= -2:
		j.Valence = -1
		j.Strength = 0.5
		j.Reasoning = "heuristic: negative signals detected"
	default:
		j.Valence = 0
		j.Strength = 0
		j.Reasoning = "heuristic: no strong signals"
	}

	return j
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func clampValence(v int) int {
	if v > 1 {
		return 1
	}
	if v < -1 {
		return -1
	}
	return v
}

func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
