package planner

import (
	"context"
	"fmt"
	"time"
)

func (p *Planner) maxPlanSteps() int {
	executionRuntime := p.ensureExecutionRuntime()
	return executionRuntime.MaxSteps()
}

func (p *Planner) perToolTimeout() time.Duration {
	executionRuntime := p.ensureExecutionRuntime()
	return executionRuntime.ToolTimeout()
}

func (p *Planner) dynamicContextBudget() int {
	executionRuntime := p.ensureExecutionRuntime()
	return executionRuntime.DynContextBudget()
}

// ModelIDForTier returns the configured model ID for a model tier without exposing the raw LLM client.
func (p *Planner) ModelIDForTier(tier string) string {
	if p == nil {
		return ""
	}
	modelRuntime := p.ensureModelRuntime()
	return modelRuntime.ModelIDForTier(tier)
}

// LLMResponseCacheStats returns default LLM response-cache stats without exposing the raw LLM client.
func (p *Planner) LLMResponseCacheStats() map[string]any {
	if p == nil {
		return nil
	}
	modelRuntime := p.ensureModelRuntime()
	return modelRuntime.DefaultResponseCacheStats()
}

// ModelRuntimeHealth returns model-runtime health without exposing the raw LLM breaker.
func (p *Planner) ModelRuntimeHealth() ModelRuntimeHealth {
	if p == nil {
		return ModelRuntimeHealth{Configured: false}
	}
	modelRuntime := p.ensureModelRuntime()
	return modelRuntime.Health()
}

// GenerateConversationTitle delegates small control-plane title generation to the model runtime.
func (p *Planner) GenerateConversationTitle(ctx context.Context, userMsg, assistReply string) string {
	if p == nil {
		return ""
	}
	modelRuntime := p.ensureModelRuntime()
	return modelRuntime.GenerateConversationTitle(ctx, userMsg, assistReply)
}

// GenerateStarterSuggestions delegates personalized empty-screen chat openers to the model runtime.
func (p *Planner) GenerateStarterSuggestions(ctx context.Context, profile string) ([]StarterSuggestion, error) {
	if p == nil {
		return nil, fmt.Errorf("planner or llm not configured")
	}
	modelRuntime := p.ensureModelRuntime()
	return modelRuntime.GenerateStarterSuggestions(ctx, profile)
}

// ParseMissionIntent delegates mission intent parsing to the model runtime.
func (p *Planner) ParseMissionIntent(ctx context.Context, description string) (MissionParseResult, error) {
	if p == nil {
		return MissionParseResult{}, fmt.Errorf("planner or llm not configured")
	}
	modelRuntime := p.ensureModelRuntime()
	return modelRuntime.ParseMissionIntent(ctx, description)
}
