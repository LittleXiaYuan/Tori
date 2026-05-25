package planner

import (
	"context"
	"time"

	"yunque-agent/internal/agentcore/llm"
)

// BuildMessages constructs the full message list using Manus-style context engineering.
//
// Layout: [stable_prefix] [dynamic_context?] [history...] [goal_recitation?] [last_user_msg+timestamp]
//
// Key principles:
//   - Stable prefix (persona+skills+domain) is a single system message — enables LLM KV-cache reuse
//   - Dynamic context (memory+graph) is a SEPARATE system message — prefix cache survives per-query changes
//   - Timestamp injected into last user message, NOT system prompt — avoids cache invalidation
//   - Goal recitation inserted before last user message in multi-turn — keeps model focused
//   - Errors preserved (append-only context) — model learns from failures
func (p *Planner) BuildMessages(ctx context.Context, req PlanRequest) ([]llm.Message, []string) {
	stablePrefix := p.ensurePromptRuntime().BuildStablePrefix(req.DisableDelegation, req.GroupSystemPrompt, p.buildSystemPrompt, p.buildSubagentSystemPrompt)
	msgs := []llm.Message{{Role: "system", Content: stablePrefix}}

	var includedLayers []string
	if len(req.Messages) > 0 {
		msgs, includedLayers = p.ensureContextAssembly().AppendDynamicContextMessage(ctx, msgs, DynamicContextAssemblyRequest{
			LastMessage: req.Messages[len(req.Messages)-1].Content,
			TenantID:    req.TenantID,
			Channel:     req.ChannelType,
			TaskContext: req.TaskContext,
			EmotionHint: req.EmotionHint,
		}, NewPromptBuilder(p))
	}

	msgs = append(msgs, p.ensurePromptRuntime().PrepareConversationMessages(req.Messages, time.Now())...)
	msgs = p.ensureContextWindowRuntime().FitMessagesForRequest(ctx, msgs, p.ensureModelRuntime().ClientForRequest(req))
	return msgs, includedLayers
}
