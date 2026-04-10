package llm

import "strings"

// ModelConstraints defines API parameter constraints for a specific model family.
type ModelConstraints struct {
	FixedTemperature bool    // if true, do not send custom temperature
	DefaultTemp      float64 // if > 0, use this as default temperature when FixedTemperature is true
	NoResponseFormat bool    // if true, do not send response_format (e.g. json_object)
	NoTopP           bool    // if true, do not send top_p
	NoFreqPenalty    bool    // if true, do not send frequency_penalty
	NoPresencePen    bool    // if true, do not send presence_penalty
	MaxTokensDefault int     // if > 0, use as default max_tokens
	StreamOnly       bool    // if true, always use stream=true
	NoN              bool    // if true, n must be 1
	SupportsThinking bool    // if true, model supports a thinking/reasoning toggle
	ThinkingKey      string  // request body key for thinking toggle (e.g. "thinking")
}

// constraintsDB maps model name patterns to their constraints.
var constraintsDB = []struct {
	Pattern     string
	Constraints ModelConstraints
}{
	// Kimi K2.5: temperature=1.0 (thinking) or 0.6 (non-thinking), rejects custom values
	{Pattern: "kimi-k2", Constraints: ModelConstraints{
		FixedTemperature: true, NoResponseFormat: true, NoTopP: true,
		NoFreqPenalty: true, NoPresencePen: true, NoN: true,
		SupportsThinking: true, ThinkingKey: "thinking",
	}},
	{Pattern: "kimi-k1", Constraints: ModelConstraints{
		FixedTemperature: true, NoResponseFormat: true,
	}},
	// OpenAI reasoning models: temperature/top_p not allowed
	{Pattern: "o1", Constraints: ModelConstraints{
		FixedTemperature: true, NoTopP: true, NoResponseFormat: true,
	}},
	{Pattern: "o3", Constraints: ModelConstraints{
		FixedTemperature: true, NoTopP: true, NoResponseFormat: true,
	}},
	{Pattern: "o4", Constraints: ModelConstraints{
		FixedTemperature: true, NoTopP: true, NoResponseFormat: true,
	}},
	// DeepSeek Reasoner: supports temperature but with quirks
	{Pattern: "deepseek-reasoner", Constraints: ModelConstraints{
		NoResponseFormat: true,
	}},
	// Claude handled by dialect, but if accessed via OpenAI-compatible proxy
	{Pattern: "claude", Constraints: ModelConstraints{
		NoResponseFormat: true,
	}},
}

// GetConstraints returns the model constraints for a given model ID.
func GetConstraints(model string) ModelConstraints {
	m := strings.ToLower(model)
	for _, entry := range constraintsDB {
		if strings.Contains(m, strings.ToLower(entry.Pattern)) {
			return entry.Constraints
		}
	}
	return ModelConstraints{}
}

// SanitizeRequestBody removes or adjusts parameters that violate model constraints.
// Returns the list of parameters that were removed/adjusted for logging.
func SanitizeRequestBody(body map[string]any, model string) []string {
	c := GetConstraints(model)
	var adjusted []string

	if c.FixedTemperature {
		if _, ok := body["temperature"]; ok {
			delete(body, "temperature")
			adjusted = append(adjusted, "temperature")
		}
	}
	if c.NoResponseFormat {
		if _, ok := body["response_format"]; ok {
			delete(body, "response_format")
			adjusted = append(adjusted, "response_format")
		}
	}
	if c.NoTopP {
		if _, ok := body["top_p"]; ok {
			delete(body, "top_p")
			adjusted = append(adjusted, "top_p")
		}
	}
	if c.NoFreqPenalty {
		if _, ok := body["frequency_penalty"]; ok {
			delete(body, "frequency_penalty")
			adjusted = append(adjusted, "frequency_penalty")
		}
	}
	if c.NoPresencePen {
		if _, ok := body["presence_penalty"]; ok {
			delete(body, "presence_penalty")
			adjusted = append(adjusted, "presence_penalty")
		}
	}
	if c.NoN {
		if n, ok := body["n"]; ok {
			if nv, _ := n.(float64); nv > 1 {
				body["n"] = 1
				adjusted = append(adjusted, "n→1")
			}
		}
	}
	if c.MaxTokensDefault > 0 {
		if _, ok := body["max_tokens"]; !ok {
			body["max_tokens"] = c.MaxTokensDefault
		}
	}
	return adjusted
}

// InjectThinking adds the thinking toggle to the request body for models that support it.
// enabled: nil = skip (use model default), true/false = explicit toggle.
func InjectThinking(body map[string]any, model string, enabled *bool) {
	c := GetConstraints(model)
	if !c.SupportsThinking || c.ThinkingKey == "" {
		return
	}
	if enabled == nil {
		return
	}
	if *enabled {
		body[c.ThinkingKey] = map[string]string{"type": "enabled"}
	} else {
		body[c.ThinkingKey] = map[string]string{"type": "disabled"}
	}
}
