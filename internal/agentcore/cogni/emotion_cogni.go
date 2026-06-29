package cogni

import (
	"context"
	"strings"

	"yunque-agent/internal/agentcore/emotion"
)

// EmotionCogni detects the user's emotional state and adjusts resource allocation
// accordingly. For emotional conversations (sad, anxious), tools and skills are
// disabled to focus on empathetic dialogue.
//
// This is a v2 migration of the existing emotion detection system (internal/agentcore/emotion).
// It preserves the behavioral text output (tone guidance) while adding resource filtering.
type EmotionCogni struct {
	priority int
	analyzer *emotion.Analyzer
}

// NewEmotionCogni creates an EmotionCogni with the given priority and analyzer.
// If analyzer is nil, uses heuristic keyword-based detection.
// Recommended priority: 50 (medium, after IntentCogni but before auxiliary Cognis)
func NewEmotionCogni(analyzer *emotion.Analyzer) *EmotionCogni {
	return &EmotionCogni{
		priority: 50,
		analyzer: analyzer,
	}
}

// Analyze implements HookV2 by detecting emotion and adjusting resources.
func (c *EmotionCogni) Analyze(ctx context.Context, req CogniRequest) CogniDecision {
	var result emotion.Result

	if c.analyzer != nil && c.analyzer.Enabled() {
		// Use the full emotion analyzer if available
		resultPtr, err := c.analyzer.AnalyzeText(ctx, req.Message)
		if err == nil && resultPtr != nil {
			result = *resultPtr
		} else {
			// Fallback to heuristic on error
			result = detectEmotionHeuristic(req.Message)
		}
	} else {
		// Fallback to simple heuristic detection
		result = detectEmotionHeuristic(req.Message)
	}

	decision := CogniDecision{
		BehaviorText: result.ContextSnippet(), // Preserve v1 behavior guidance
		State: map[string]any{
			"emotion":    string(result.Emotion),
			"confidence": result.Confidence,
		},
	}

	// Resource adjustment based on emotion
	if result.IsNegative() {
		// Negative emotions (sad, angry, anxious) → emotional support mode
		// Disable tools and skills to focus on empathetic conversation
		decision.ToolsNeeded = []string{}  // Empty slice means "I want no tools"
		decision.SkillsNeeded = []string{} // Empty slice means "I want no skills"
		decision.MemoryScope = MemoryScope{
			Limit:      15,
			Categories: []string{"conversation", "identity"}, // Focus on conversation history
			Keywords:   []string{"情感", "心情", "感受"},
		}
	} else {
		// Positive, neutral, or unknown emotions → normal mode
		// Don't restrict resources (let IntentCogni decide)
		// nil means "I have no opinion, let other Cognis decide"
		decision.ToolsNeeded = nil
		decision.SkillsNeeded = nil
		decision.MemoryScope = MemoryScope{
			Limit: 0, // No specific limit (use default)
		}
	}

	return decision
}

// Priority returns this Cogni's priority in decision merging.
// EmotionCogni has medium priority (50), after IntentCogni (100).
func (c *EmotionCogni) Priority() int {
	return c.priority
}

// detectEmotionHeuristic performs simple keyword-based emotion detection.
// This is a fallback when the full emotion.Detector is not available.
func detectEmotionHeuristic(message string) emotion.Result {
	lower := strings.ToLower(message)

	// Sad
	if containsAnyEmotion(lower, []string{"难过", "伤心", "沮丧", "失落", "心情不好", "sad", "depressed", "down"}) {
		return emotion.Result{
			Emotion:    emotion.EmotionSad,
			Confidence: 0.7,
			Source:     "heuristic",
		}
	}

	// Angry
	if containsAnyEmotion(lower, []string{"生气", "愤怒", "烦死", "气死", "讨厌", "angry", "mad", "annoyed", "frustrated"}) {
		return emotion.Result{
			Emotion:    emotion.EmotionAngry,
			Confidence: 0.7,
			Source:     "heuristic",
		}
	}

	// Fearful/Anxious
	if containsAnyEmotion(lower, []string{"焦虑", "担心", "害怕", "紧张", "不安", "anxious", "worried", "scared", "nervous"}) {
		return emotion.Result{
			Emotion:    emotion.EmotionFearful,
			Confidence: 0.7,
			Source:     "heuristic",
		}
	}

	// Happy
	if containsAnyEmotion(lower, []string{"开心", "高兴", "快乐", "兴奋", "happy", "excited", "glad", "joyful"}) {
		return emotion.Result{
			Emotion:    emotion.EmotionHappy,
			Confidence: 0.7,
			Source:     "heuristic",
		}
	}

	// Default to neutral
	return emotion.Result{
		Emotion:    emotion.EmotionNeutral,
		Confidence: 0.5,
		Source:     "default",
	}
}

// containsAnyEmotion checks if the message contains any emotion keywords.
func containsAnyEmotion(s string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}
