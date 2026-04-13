package main

import (
	"context"

	"yunque-agent/internal/agentcore/llm"
)

// llmChatFunc returns a standard LLM chat function with the given temperature.
// This eliminates the 15+ identical anonymous function patterns.
func llmChatFunc(client *llm.Client, temp float64) func(ctx context.Context, system, user string) (string, error) {
	return func(ctx context.Context, system, user string) (string, error) {
		msgs := []llm.Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		}
		return client.Chat(ctx, msgs, temp)
	}
}

// llmChatFuncFromPool returns a standard LLM chat function using a pool tier.
// Falls back to the primary client if the requested tier is unavailable.
func llmChatFuncFromPool(pool *llm.Pool, tier string, temp float64) func(ctx context.Context, system, user string) (string, error) {
	return func(ctx context.Context, system, user string) (string, error) {
		client := pool.Get(tier)
		if client == nil {
			client = pool.Primary()
		}
		msgs := []llm.Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		}
		return client.Chat(ctx, msgs, temp)
	}
}

// llmBreakerChatFunc wraps a circuit breaker call into the standard chat function signature.
func llmBreakerChatFunc(call func(ctx context.Context, system, user string) (string, error)) func(ctx context.Context, system, user string) (string, error) {
	return call
}
