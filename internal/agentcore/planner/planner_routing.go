package planner

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/plan"
	"yunque-agent/internal/observe"
)

// ModelFallbackDetail is emitted with a fallback event so the UI can explain
// the switch without exposing low-level client errors as scary status text.
type ModelFallbackDetail struct {
	Model   string `json:"model,omitempty"`
	Attempt int    `json:"attempt"`
	Reason  string `json:"reason,omitempty"`
}

func modelFallbackSummary(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return "模型暂时没有回应，正在换一个可用模型继续。"
	}
	return fmt.Sprintf("模型暂时没有回应，正在换用 %s 继续。", model)
}

func (p *Planner) emitModelFallbackEvent(req PlanRequest, model string, attempt int, err error) {
	if req.StepCallback == nil {
		return
	}
	evt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventPlan, modelFallbackSummary(model))
	evt.Meta.TenantID = req.TenantID
	evt.Meta.TaskID = req.TaskID
	detail := ModelFallbackDetail{Model: model, Attempt: attempt}
	if err != nil {
		detail.Reason = truncate(plannerFriendlyFailureText(err.Error()), 160)
	}
	evt.Detail = detail
	req.StepCallback(evt)
}

func (p *Planner) isComplexTask(req PlanRequest) bool {
	goal := extractGoal(req)
	load := assessCognitiveLoad(req)
	if load.NeedsLongHorizon() {
		return true
	}
	return plan.NeedsPlan(goal)
}

// adaptiveRoute determines whether the request requires an elevated reasoning model.
func (p *Planner) adaptiveRoute(req PlanRequest) string {
	if req.ModelOverride != "" {
		return req.ModelOverride
	}
	if assessCognitiveLoad(req).NeedsLongHorizon() {
		slog.Info("planner: adaptive reasoning activated (high cognitive load), elevating to expert tier")
		return "expert"
	}
	lastMsg := ""
	if len(req.Messages) > 0 {
		lastMsg = req.Messages[len(req.Messages)-1].Content
	}

	runeLen := len([]rune(lastMsg))
	if runeLen > 500 {
		slog.Info("planner: adaptive reasoning activated (long query), elevating to expert tier")
		return "expert"
	}
	lower := strings.ToLower(lastMsg)
	expertIndicators := []string{
		"分析", "逻辑", "推理", "架构", "重构", "调研",
		"analyze", "reason", "architect", "refactor", "debug",
	}
	for _, ind := range expertIndicators {
		if strings.Contains(lower, ind) {
			slog.Info("planner: adaptive reasoning activated (complex intent), elevating to expert tier", "indicator", ind)
			return "expert"
		}
	}
	return "fast"
}

// selectClientWithCaps returns the best LLM client considering both tier and required capabilities.
func (p *Planner) selectClientWithCaps(req PlanRequest, messages []llm.Message) *llm.Client {
	var requiredCaps []llm.Capability
	for _, m := range messages {
		for _, part := range m.ContentParts {
			if part.Type == "image_url" || part.Type == "video_url" {
				requiredCaps = append(requiredCaps, llm.CapVision)
				break
			}
		}
		if len(requiredCaps) > 0 {
			break
		}
	}

	if len(requiredCaps) > 0 && p.runtimeStrategy != nil {
		if vp := p.runtimeStrategy.SelectProviderByCapability(requiredCaps...); vp != nil {
			slog.Info("planner: capability routing selected vision-capable provider",
				"provider", vp.Config.ID, "model", vp.Config.Model, "caps", requiredCaps)
			return vp.Client
		}
		slog.Warn("planner: no vision-capable provider found, falling back to default")
	}

	return p.LLMClientFor(p.adaptiveRoute(req))
}

// buildFallbackChain returns the ordered list of LLM clients to attempt for a
// request. A session ClientOverride pins to a single client (no tier fallback)
// so user-selected providers are honored as-is.
func (p *Planner) buildFallbackChain(req PlanRequest, messages []llm.Message) []*llm.Client {
	if req.ClientOverride != nil {
		return []*llm.Client{req.ClientOverride}
	}
	capClient := p.selectClientWithCaps(req, messages)
	targetModel := p.adaptiveRoute(req)
	var chain []*llm.Client
	if p.llmPool != nil {
		chain = p.llmPool.GetFallbackChain(targetModel)
	} else {
		chain = []*llm.Client{p.llm}
	}
	if capClient != nil && (len(chain) == 0 || capClient != chain[0]) {
		chain = append([]*llm.Client{capClient}, chain...)
	}
	return chain
}

func (p *Planner) chatFallback(ctx context.Context, req PlanRequest, messages []llm.Message) (string, error) {
	chain := p.buildFallbackChain(req, messages)

	var lastErr error
	for i, client := range chain {
		if i > 0 {
			slog.Warn("planner: degrading LLM client", "fallback_to", client.Model(), "err", lastErr)
			p.emitModelFallbackEvent(req, client.Model(), i+1, lastErr)
		}
		reply, err := client.Chat(ctx, messages, 0.7)
		if err == nil {
			return reply, nil
		}
		if ctx.Err() != nil {
			return "", err
		}
		lastErr = err
	}
	return "", fmt.Errorf("all fallback LLM clients failed: %w", lastErr)
}

// chatFallbackFull is like chatFallback but returns ChatResult with reasoning_content.
func (p *Planner) chatFallbackFull(ctx context.Context, req PlanRequest, messages []llm.Message, onDelta ...llm.StreamDeltaFunc) (llm.ChatResult, error) {
	chain := p.buildFallbackChain(req, messages)

	var lastErr error
	for i, client := range chain {
		if i > 0 {
			slog.Warn("planner: degrading LLM client (full)", "fallback_to", client.Model(), "err", lastErr)
			p.emitModelFallbackEvent(req, client.Model(), i+1, lastErr)
		}
		result, err := client.ChatFull(ctx, messages, 0.7, onDelta...)
		if err == nil {
			return result, nil
		}
		if ctx.Err() != nil {
			return llm.ChatResult{}, err
		}
		lastErr = err
	}
	return llm.ChatResult{}, fmt.Errorf("all fallback LLM clients failed: %w", lastErr)
}

// chatWithToolsFallback wraps native FC chat calls with a graceful degradation retry loop.
func (p *Planner) chatWithToolsFallback(ctx context.Context, req PlanRequest, messages []llm.Message, tools []llm.FunctionDef) (string, []llm.ToolCall, string, error) {
	chain := p.buildFallbackChain(req, messages)

	thinkingFlag := req.ThinkingEnabled
	if thinkingFlag == nil {
		if shouldAutoThink(req.Messages) {
			t := true
			thinkingFlag = &t
			slog.Info("planner: auto-thinking enabled (complex query detected)")
		}
	}

	var lastErr error
	thinkingRetried := false
	for i, client := range chain {
		if i > 0 {
			slog.Warn("planner: degrading LLM client (FC)", "fallback_to", client.Model(), "err", lastErr)
			p.emitModelFallbackEvent(req, client.Model(), i+1, lastErr)
		}
		var lastReasoning string
		fcOpts := &llm.ChatWithToolsOpts{ThinkingEnabled: thinkingFlag, LastReasoningOut: &lastReasoning}
		if req.StepCallback != nil {
			fcOpts.OnReasoningDelta = func(delta string) {
				evt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventThinking, delta)
				evt.Meta.TenantID = req.TenantID
				evt.Detail = map[string]any{"stream_type": "thinking_delta"}
				req.StepCallback(evt)
			}
			fcOpts.OnReasoning = func(reasoning string) {
				evt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventThinking, reasoning)
				evt.Meta.TenantID = req.TenantID
				evt.Detail = map[string]any{"stream_type": "reasoning_batch"}
				req.StepCallback(evt)
			}
		}
		reply, toolCalls, err := client.ChatWithToolsEx(ctx, messages, tools, 0.7, fcOpts)
		if err == nil {
			return reply, toolCalls, lastReasoning, nil
		}
		if !thinkingRetried && thinkingFlag != nil && *thinkingFlag && strings.Contains(err.Error(), "status 400") {
			slog.Warn("planner: thinking caused 400, retrying without thinking", "model", client.Model())
			f := false
			thinkingFlag = &f
			fcOpts.ThinkingEnabled = thinkingFlag
			reply, toolCalls, err = client.ChatWithToolsEx(ctx, messages, tools, 0.7, fcOpts)
			thinkingRetried = true
			if err == nil {
				return reply, toolCalls, lastReasoning, nil
			}
		}
		if ctx.Err() != nil {
			return "", nil, "", err
		}
		lastErr = err
	}
	return "", nil, "", fmt.Errorf("all fallback LLM clients failed (FC): %w", lastErr)
}

// shouldAutoThink heuristically determines if a query is complex enough to warrant thinking.
func shouldAutoThink(messages []llm.Message) bool {
	if len(messages) == 0 {
		return false
	}
	last := messages[len(messages)-1].Content
	runes := []rune(last)
	if len(runes) > 200 {
		return true
	}
	complexIndicators := []string{
		"分析", "论文", "编写", "设计", "调研", "对比",
		"推理", "计算", "重构", "架构",
		"compare", "analyze", "implement", "design",
		"optimize", "debug", "review", "refactor",
	}
	lower := strings.ToLower(last)
	for _, ind := range complexIndicators {
		if strings.Contains(lower, ind) {
			return true
		}
	}
	return false
}
