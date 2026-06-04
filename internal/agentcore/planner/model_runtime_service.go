package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"yunque-agent/internal/agentcore/llm"
)

// ModelRuntimeService owns Planner's default LLM client and tier pool. It is a
// small boundary for model lookup/fallback so Planner does not directly carry
// both the default client and pool-selection rules as separate concerns.
type ModelRuntimeService struct {
	defaultClient *llm.Client
	pool          *llm.Pool
}

// ModelRuntimeHealth is a narrow, JSON-ready snapshot for control-plane health
// surfaces. It intentionally exposes breaker status values without returning
// the raw llm.CircuitBreaker to callers outside the model runtime boundary.
type ModelRuntimeHealth struct {
	BreakerState string `json:"breaker_state,omitempty"`
	Failures     int    `json:"failures,omitempty"`
	Configured   bool   `json:"configured"`
}

type ModelFallbackEventFunc func(model string, attempt int, err error)

type CapabilityProviderSelector func(required ...llm.Capability) *llm.ProviderInstance

type ModelReasoningEventFunc func(summary string, detail map[string]any)

func NewModelRuntimeService(defaultClient *llm.Client) *ModelRuntimeService {
	return &ModelRuntimeService{defaultClient: defaultClient}
}

func (s *ModelRuntimeService) SetDefaultClient(client *llm.Client) {
	if s == nil {
		return
	}
	s.defaultClient = client
}

func (s *ModelRuntimeService) DefaultClient() *llm.Client {
	if s == nil {
		return nil
	}
	return s.defaultClient
}

func (s *ModelRuntimeService) SetPool(pool *llm.Pool) {
	if s == nil {
		return
	}
	s.pool = pool
}

func (s *ModelRuntimeService) Pool() *llm.Pool {
	if s == nil {
		return nil
	}
	return s.pool
}

func (s *ModelRuntimeService) ClientFor(modelOverride string) *llm.Client {
	if s == nil {
		return nil
	}
	if modelOverride != "" && s.pool != nil {
		if c := s.pool.GetOrFallback(modelOverride); c != nil {
			return c
		}
	}
	return s.defaultClient
}

func (s *ModelRuntimeService) ClientForRequest(req PlanRequest) *llm.Client {
	if req.ClientOverride != nil {
		return req.ClientOverride
	}
	return s.ClientFor(req.ModelOverride)
}

func (s *ModelRuntimeService) ClientForRequestTier(req PlanRequest, tier string) *llm.Client {
	if req.ClientOverride != nil {
		return req.ClientOverride
	}
	return s.ClientFor(tier)
}

func (s *ModelRuntimeService) ModelIDForTier(tier string) string {
	client := s.ClientFor(tier)
	if client == nil {
		return ""
	}
	return client.Model()
}

func (s *ModelRuntimeService) DefaultResponseCacheStats() map[string]any {
	if s == nil || s.defaultClient == nil || s.defaultClient.Cache() == nil {
		return nil
	}
	return s.defaultClient.Cache().Stats()
}

func (s *ModelRuntimeService) Health() ModelRuntimeHealth {
	if s == nil || s.defaultClient == nil || s.defaultClient.Breaker() == nil {
		return ModelRuntimeHealth{Configured: false}
	}
	breaker := s.defaultClient.Breaker()
	return ModelRuntimeHealth{
		BreakerState: breaker.State(),
		Failures:     breaker.Failures(),
		Configured:   true,
	}
}

func (s *ModelRuntimeService) ChatForRequest(ctx context.Context, req PlanRequest, messages []llm.Message, temperature float64) (string, error) {
	client := s.ClientForRequest(req)
	if client == nil {
		return "", fmt.Errorf("planner LLM client not configured")
	}
	return client.Chat(ctx, messages, temperature)
}

func (s *ModelRuntimeService) ChatForRequestTier(ctx context.Context, req PlanRequest, tier string, messages []llm.Message, temperature float64) (string, error) {
	client := s.ClientForRequestTier(req, tier)
	if client == nil {
		return "", fmt.Errorf("planner LLM client not configured")
	}
	return client.Chat(ctx, messages, temperature)
}

func (s *ModelRuntimeService) ChatWithToolsForRequest(ctx context.Context, req PlanRequest, messages []llm.Message, tools []llm.FunctionDef, temperature float64, toolChoice ...string) (string, []llm.ToolCall, error) {
	client := s.ClientForRequest(req)
	if client == nil {
		return "", nil, fmt.Errorf("planner LLM client not configured")
	}
	return client.ChatWithTools(ctx, messages, tools, temperature, toolChoice...)
}

func (s *ModelRuntimeService) AnalyzeUploadedFile(ctx context.Context, filename, textSnippet string) (*UploadAnalysis, error) {
	snippet := textSnippet
	if len([]rune(snippet)) > 8000 {
		snippet = string([]rune(snippet)[:8000])
	}
	system := `你是文件分析助手。只输出一段合法 JSON，不要 markdown 代码块。格式：
{"file_kind":"xlsx|docx|csv|pdf|txt|other","is_template":true/false,"summary":"一句话说明文件用途与结构","suggestions":["可选的后续动作短句"]}
is_template：是否为表单/模板/需用户填写的范式文件。`
	user := fmt.Sprintf("文件名: %s\n\n内容预览:\n%s", filename, snippet)
	raw, err := s.ChatForRequest(ctx, PlanRequest{}, []llm.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}, 0.2)
	if err != nil {
		return nil, err
	}
	raw = strings.TrimSpace(raw)
	if i := strings.Index(raw, "{"); i >= 0 {
		raw = raw[i:]
	}
	if j := strings.LastIndex(raw, "}"); j >= 0 {
		raw = raw[:j+1]
	}
	var analysis UploadAnalysis
	if err := json.Unmarshal([]byte(raw), &analysis); err != nil {
		analysis = UploadAnalysis{
			FileKind:   "unknown",
			Summary:    "无法自动解析元数据，可直接描述你的目标。",
			IsTemplate: strings.Contains(strings.ToLower(filename), "模板"),
		}
	}

	// Detect {{placeholder}} patterns in file content — works regardless of LLM analysis.
	analysis.Placeholders = detectPlaceholders(snippet)
	if len(analysis.Placeholders) > 0 {
		analysis.IsTemplate = true
	}
	return &analysis, nil
}

func (s *ModelRuntimeService) FallbackChain(targetModel string) []*llm.Client {
	if s == nil {
		return nil
	}
	if s.pool != nil {
		return s.pool.GetFallbackChain(targetModel)
	}
	if s.defaultClient != nil {
		return []*llm.Client{s.defaultClient}
	}
	return nil
}

func (s *ModelRuntimeService) AdaptiveRoute(req PlanRequest) string {
	if req.ModelOverride != "" {
		return req.ModelOverride
	}
	if assessCognitiveLoad(req).NeedsLongHorizon() {
		slog.Info("model runtime: adaptive reasoning activated (high cognitive load), elevating to expert tier")
		return "expert"
	}
	lastMsg := ""
	if len(req.Messages) > 0 {
		lastMsg = req.Messages[len(req.Messages)-1].Content
	}

	if len([]rune(lastMsg)) > 500 {
		slog.Info("model runtime: adaptive reasoning activated (long query), elevating to expert tier")
		return "expert"
	}
	lower := strings.ToLower(lastMsg)
	expertIndicators := []string{
		"分析", "逻辑", "推理", "架构", "重构", "调研",
		"analyze", "reason", "architect", "refactor", "debug",
	}
	for _, indicator := range expertIndicators {
		if strings.Contains(lower, indicator) {
			slog.Info("model runtime: adaptive reasoning activated (complex intent), elevating to expert tier", "indicator", indicator)
			return "expert"
		}
	}
	return "fast"
}

func (s *ModelRuntimeService) FallbackChainForRequest(req PlanRequest, messages []llm.Message, selectProvider CapabilityProviderSelector) []*llm.Client {
	if req.ClientOverride != nil {
		return []*llm.Client{req.ClientOverride}
	}
	route := s.AdaptiveRoute(req)
	capClient := s.clientWithCapabilities(messages, route, selectProvider)
	chain := s.FallbackChain(route)
	if capClient != nil && (len(chain) == 0 || capClient != chain[0]) {
		chain = append([]*llm.Client{capClient}, chain...)
	}
	return chain
}

func (s *ModelRuntimeService) FallbackChainForPlannerRequest(req PlanRequest, messages []llm.Message, strategy *RuntimeStrategyService) []*llm.Client {
	var selectProvider CapabilityProviderSelector
	if strategy != nil {
		selectProvider = strategy.SelectProviderByCapability
	}
	return s.FallbackChainForRequest(req, messages, selectProvider)
}

func (s *ModelRuntimeService) ChatFallbackForRequest(ctx context.Context, req PlanRequest, messages []llm.Message, strategy *RuntimeStrategyService, onFallback ModelFallbackEventFunc) (string, error) {
	return s.ChatFallback(ctx, s.FallbackChainForPlannerRequest(req, messages, strategy), messages, onFallback)
}

func (s *ModelRuntimeService) ChatFallbackFullForRequest(ctx context.Context, req PlanRequest, messages []llm.Message, strategy *RuntimeStrategyService, onFallback ModelFallbackEventFunc, onDelta ...llm.StreamDeltaFunc) (llm.ChatResult, error) {
	return s.ChatFallbackFull(ctx, s.FallbackChainForPlannerRequest(req, messages, strategy), messages, onFallback, onDelta...)
}

func (s *ModelRuntimeService) ChatWithToolsFallbackForRequest(ctx context.Context, req PlanRequest, messages []llm.Message, tools []llm.FunctionDef, strategy *RuntimeStrategyService, reasoningEvents ModelReasoningEventFunc, onFallback ModelFallbackEventFunc) (string, []llm.ToolCall, string, error) {
	return s.ChatWithToolsFallback(
		ctx,
		s.FallbackChainForPlannerRequest(req, messages, strategy),
		messages,
		tools,
		s.ThinkingFlagForRequest(req),
		s.streamConfigurator(reasoningEvents, req.OnReplyDelta),
		onFallback,
	)
}

func (s *ModelRuntimeService) ThinkingFlagForRequest(req PlanRequest) *bool {
	thinkingFlag := req.ThinkingEnabled
	if thinkingFlag == nil && shouldAutoThink(req.Messages) {
		t := true
		thinkingFlag = &t
		slog.Info("model runtime: auto-thinking enabled (complex query detected)")
	}
	return thinkingFlag
}

func (s *ModelRuntimeService) ReasoningCallbacks(emit ModelReasoningEventFunc) func(*llm.ChatWithToolsOpts) {
	if emit == nil {
		return nil
	}
	return func(fcOpts *llm.ChatWithToolsOpts) {
		fcOpts.OnReasoningDelta = func(delta string) {
			emit(delta, map[string]any{"stream_type": "thinking_delta"})
		}
		fcOpts.OnReasoning = func(reasoning string) {
			emit(reasoning, map[string]any{"stream_type": "reasoning_batch"})
		}
	}
}

// streamConfigurator composes the reasoning-delta callbacks with an optional
// live content-delta callback, so the planner can stream the final answer text
// token-by-token (true streaming) on top of the existing thinking deltas. When
// both inputs are absent it returns nil, preserving the non-streaming path.
func (s *ModelRuntimeService) streamConfigurator(emit ModelReasoningEventFunc, onContent func(string)) func(*llm.ChatWithToolsOpts) {
	base := s.ReasoningCallbacks(emit)
	if base == nil && onContent == nil {
		return nil
	}
	return func(fcOpts *llm.ChatWithToolsOpts) {
		if base != nil {
			base(fcOpts)
		}
		if onContent != nil {
			fcOpts.OnContentDelta = onContent
		}
	}
}

func shouldAutoThink(messages []llm.Message) bool {
	if len(messages) == 0 {
		return false
	}
	last := messages[len(messages)-1].Content
	if len([]rune(last)) > 200 {
		return true
	}
	complexIndicators := []string{
		"分析", "论文", "编写", "设计", "调研", "对比",
		"推理", "计算", "重构", "架构",
		"compare", "analyze", "implement", "design",
		"optimize", "debug", "review", "refactor",
	}
	lower := strings.ToLower(last)
	for _, indicator := range complexIndicators {
		if strings.Contains(lower, indicator) {
			return true
		}
	}
	return false
}

func (s *ModelRuntimeService) clientWithCapabilities(messages []llm.Message, fallbackTier string, selectProvider CapabilityProviderSelector) *llm.Client {
	requiredCaps := requiredCapabilitiesForMessages(messages)
	if len(requiredCaps) > 0 && selectProvider != nil {
		if vp := selectProvider(requiredCaps...); vp != nil {
			slog.Info("model runtime: capability routing selected provider",
				"provider", vp.Config.ID, "model", vp.Config.Model, "caps", requiredCaps)
			return vp.Client
		}
		slog.Warn("model runtime: no provider found for required capabilities", "caps", requiredCaps)
	}
	return s.ClientFor(fallbackTier)
}

func requiredCapabilitiesForMessages(messages []llm.Message) []llm.Capability {
	var required []llm.Capability
	for _, m := range messages {
		for _, part := range m.ContentParts {
			if part.Type == "image_url" || part.Type == "video_url" {
				return []llm.Capability{llm.CapVision}
			}
		}
	}
	return required
}

func (s *ModelRuntimeService) ChatFallback(ctx context.Context, chain []*llm.Client, messages []llm.Message, onFallback ModelFallbackEventFunc) (string, error) {
	var lastErr error
	for i, client := range chain {
		if client == nil {
			continue
		}
		if i > 0 {
			slog.Warn("model runtime: degrading LLM client", "fallback_to", client.Model(), "err", lastErr)
			if onFallback != nil {
				onFallback(client.Model(), i+1, lastErr)
			}
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

func (s *ModelRuntimeService) ChatFallbackFull(ctx context.Context, chain []*llm.Client, messages []llm.Message, onFallback ModelFallbackEventFunc, onDelta ...llm.StreamDeltaFunc) (llm.ChatResult, error) {
	var lastErr error
	for i, client := range chain {
		if client == nil {
			continue
		}
		if i > 0 {
			slog.Warn("model runtime: degrading LLM client (full)", "fallback_to", client.Model(), "err", lastErr)
			if onFallback != nil {
				onFallback(client.Model(), i+1, lastErr)
			}
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

func (s *ModelRuntimeService) ChatWithToolsFallback(ctx context.Context, chain []*llm.Client, messages []llm.Message, tools []llm.FunctionDef, thinkingFlag *bool, opts func(*llm.ChatWithToolsOpts), onFallback ModelFallbackEventFunc) (string, []llm.ToolCall, string, error) {
	var lastErr error
	thinkingRetried := false
	for i, client := range chain {
		if client == nil {
			continue
		}
		if i > 0 {
			slog.Warn("model runtime: degrading LLM client (FC)", "fallback_to", client.Model(), "err", lastErr)
			if onFallback != nil {
				onFallback(client.Model(), i+1, lastErr)
			}
		}
		var lastReasoning string
		fcOpts := &llm.ChatWithToolsOpts{ThinkingEnabled: thinkingFlag, LastReasoningOut: &lastReasoning}
		if opts != nil {
			opts(fcOpts)
		}
		reply, toolCalls, err := client.ChatWithToolsEx(ctx, messages, tools, 0.7, fcOpts)
		if err == nil {
			return reply, toolCalls, lastReasoning, nil
		}
		if !thinkingRetried && thinkingFlag != nil && *thinkingFlag && strings.Contains(err.Error(), "status 400") {
			slog.Warn("model runtime: thinking caused 400, retrying without thinking", "model", client.Model())
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
