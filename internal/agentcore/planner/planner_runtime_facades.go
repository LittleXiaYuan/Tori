package planner

import (
	"context"
	"fmt"
	"time"
)

func (p *Planner) maxPlanSteps() int {
	return p.ensureExecutionRuntime().MaxSteps()
}

func (p *Planner) perToolTimeout() time.Duration {
	return p.ensureExecutionRuntime().ToolTimeout()
}

func (p *Planner) dynamicContextBudget() int {
	return p.ensureExecutionRuntime().DynContextBudget()
}

// ModelIDForTier returns the configured model ID for a model tier without exposing the raw LLM client.
func (p *Planner) ModelIDForTier(tier string) string {
	if p == nil {
		return ""
	}
	return p.ensureModelRuntime().ModelIDForTier(tier)
}

// LLMResponseCacheStats returns default LLM response-cache stats without exposing the raw LLM client.
func (p *Planner) LLMResponseCacheStats() map[string]any {
	if p == nil {
		return nil
	}
	return p.ensureModelRuntime().DefaultResponseCacheStats()
}

// ModelRuntimeHealth returns model-runtime health without exposing the raw LLM breaker.
func (p *Planner) ModelRuntimeHealth() ModelRuntimeHealth {
	if p == nil {
		return ModelRuntimeHealth{Configured: false}
	}
	return p.ensureModelRuntime().Health()
}

// GenerateConversationTitle delegates small control-plane title generation to the model runtime.
func (p *Planner) GenerateConversationTitle(ctx context.Context, userMsg, assistReply string) string {
	if p == nil {
		return ""
	}
	return p.ensureModelRuntime().GenerateConversationTitle(ctx, userMsg, assistReply)
}

// ParseMissionIntent delegates mission intent parsing to the model runtime.
func (p *Planner) ParseMissionIntent(ctx context.Context, description string) (MissionParseResult, error) {
	if p == nil {
		return MissionParseResult{}, fmt.Errorf("planner or llm not configured")
	}
	return p.ensureModelRuntime().ParseMissionIntent(ctx, description)
}
