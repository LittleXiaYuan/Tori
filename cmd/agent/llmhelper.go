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

