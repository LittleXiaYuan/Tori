package planner

import (
	"context"
	"testing"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/localbrain"
)

func TestRuntimeStrategyServiceModes(t *testing.T) {
	service := NewRuntimeStrategyService()
	if service.ReActMode() || service.LongHorizonMode() {
		t.Fatal("expected modes disabled by default")
	}
	service.SetReActMode(true)
	service.SetLongHorizonMode(true)
	if !service.ReActMode() || !service.LongHorizonMode() {
		t.Fatal("expected mode setters to enable runtime strategies")
	}
}

func TestRuntimeStrategyServiceLongHorizonCheckpointStore(t *testing.T) {
	store := NewFileLongHorizonCheckpointStore(t.TempDir() + "/checkpoints.jsonl")
	service := NewRuntimeStrategyService()
	service.SetLongHorizonCheckpointStore(store)
	if service.LongHorizonCheckpointStore() != store {
		t.Fatal("expected attached long-horizon checkpoint store")
	}
}

func TestRuntimeStrategyServiceLocalBrainContextFilter(t *testing.T) {
	brain := localbrain.New(nil, nil)
	service := NewRuntimeStrategyService()
	if service.HasContextFilter() {
		t.Fatal("expected no context filter before local brain is attached")
	}
	service.SetLocalBrain(brain)
	if !service.HasContextFilter() {
		t.Fatal("expected attached local brain to enable context filter")
	}
	filtered, err := service.FilterContext(context.Background(), "query", []RuntimeContextItem{
		{Source: "memory", Content: "a"},
		{Source: "graph", Content: "b"},
	}, 1)
	if err != nil {
		t.Fatalf("filter context: %v", err)
	}
	if filtered == nil || len(filtered.Items) != 1 || filtered.Filtered != 1 {
		t.Fatalf("unexpected filtered context: %#v", filtered)
	}
	if !service.ShouldClassify(PlanRequest{}) {
		t.Fatal("expected local brain to enable default classification")
	}
	if service.ShouldClassify(PlanRequest{ModelOverride: "expert"}) {
		t.Fatal("model override should skip classification")
	}
	if service.ShouldClassify(PlanRequest{DisableDelegation: true}) {
		t.Fatal("disabled delegation should skip classification")
	}
}

func TestRuntimeStrategyServiceNilRuntimeSetters(t *testing.T) {
	service := NewRuntimeStrategyService()
	service.SetLocalBrain(localbrain.New(nil, nil))
	if !service.ShouldClassify(PlanRequest{}) {
		t.Fatal("expected attached local brain to enable classification")
	}

	var nilBrain *localbrain.LocalBrain
	service.SetLocalBrain(nilBrain)
	if service.HasContextFilter() || service.ShouldClassify(PlanRequest{}) {
		t.Fatal("typed nil local brain should clear runtime strategy state")
	}

	var nilThinking *localbrain.AgenticThinking
	service.SetAgenticThinking(nilThinking)
	if got, err := service.Think(context.Background(), RuntimeThinkRequest{Query: "query"}); got != nil || err != nil {
		t.Fatalf("typed nil agentic thinking should be a no-op, got=%#v err=%v", got, err)
	}
}

func TestRuntimeStrategyServiceThinkingDTOConversion(t *testing.T) {
	req := RuntimeThinkRequest{
		TaskID:           "task-1",
		TenantID:         "tenant-1",
		Query:            "do work",
		PrevActionResult: "previous",
		StepIndex:        2,
		StepHistory: []RuntimeThinkStepSummary{
			{Action: "search", Result: "ok", Success: true},
			{Action: "write", Result: "failed", Success: false},
		},
	}
	converted := toLocalBrainThinkRequest(req)
	if converted.TaskID != req.TaskID || converted.TenantID != req.TenantID || converted.Query != req.Query || converted.PrevActionResult != req.PrevActionResult || converted.StepIndex != req.StepIndex {
		t.Fatalf("unexpected converted request: %#v", converted)
	}
	if len(converted.StepHistory) != 2 || converted.StepHistory[1].Action != "write" || converted.StepHistory[1].Success {
		t.Fatalf("unexpected converted history: %#v", converted.StepHistory)
	}

	result := fromLocalBrainThinkResult(&localbrain.ThinkResult{
		Level:      localbrain.ThinkDeep,
		Thought:    "consider alternatives",
		NextAction: "retry",
		Confidence: 0.8,
		ShouldStop: true,
	})
	if result == nil || result.Level != RuntimeThinkDeep || result.Thought != "consider alternatives" || result.NextAction != "retry" || result.Confidence != 0.8 || !result.ShouldStop {
		t.Fatalf("unexpected runtime think result: %#v", result)
	}
}

func TestRuntimeStrategyServiceDecisionDTOConversion(t *testing.T) {
	decision := fromLocalBrainDecision(&localbrain.Decision{
		Handler: "smart",
		Intent: localbrain.Intent{
			Category:   "code",
			Complexity: "hard",
			Confidence: 0.77,
			NeedTools:  true,
		},
		Reason:     "needs tools",
		LocalReply: "local answer",
	})
	if decision == nil || decision.Handler != "smart" || decision.Intent.Category != "code" || decision.Intent.Complexity != "hard" || decision.Intent.Confidence != 0.77 || !decision.Intent.NeedTools || decision.Reason != "needs tools" || decision.LocalReply != "local answer" {
		t.Fatalf("unexpected runtime decision: %#v", decision)
	}
	if got := fromLocalBrainDecision(nil); got != nil {
		t.Fatalf("nil LocalBrain decision should convert to nil, got %#v", got)
	}
}

func TestRuntimeStrategyServiceClassifyRequestAppliesDecision(t *testing.T) {
	service := NewRuntimeStrategyService()
	service.SetLocalBrain(localbrain.New(nil, nil))
	result, err := service.ClassifyRequest(context.Background(), PlanRequest{TenantID: "tenant-1"}, "hi")
	if err != nil {
		t.Fatalf("classify request: %v", err)
	}
	if result == nil || result.Decision == nil {
		t.Fatal("expected classification result for greeting")
	}
	if result.Request.ModelOverride != "fast" {
		t.Fatalf("expected fast model override for greeting, got %q", result.Request.ModelOverride)
	}
	if !result.ToolFree {
		t.Fatal("expected greeting classification to use tool-free path")
	}
	if result.LogHandler != "fast" || result.LogIntent != "chat" || result.LogNeedTools || result.LogReason == "" {
		t.Fatalf("unexpected log fields: %#v", result)
	}
	if result.TraceHandler != "fast" || result.TraceReason == "" || result.TraceScore != 1.0 {
		t.Fatalf("unexpected trace fields: %#v", result)
	}
	if result.TraceMeta["category"] != "chat" || result.TraceMeta["complexity"] != "simple" || result.TraceMeta["need_tools"] != false {
		t.Fatalf("unexpected trace metadata: %#v", result.TraceMeta)
	}

	skipped, err := service.ClassifyRequest(context.Background(), PlanRequest{ModelOverride: "expert"}, "hi")
	if err != nil || skipped != nil {
		t.Fatalf("model override should skip classification, got=%#v err=%v", skipped, err)
	}
}

func TestRuntimeStrategyServiceSelectExecutionMode(t *testing.T) {
	service := NewRuntimeStrategyService()
	if got := service.SelectExecutionMode(PlanExecutionModeRequest{NativeFC: true, LedgerEnabled: true}); got.Mode != PlanExecutionNativeFC {
		t.Fatalf("expected native-fc when no runtime strategy mode is enabled, got %s", got.Mode)
	}

	service.SetReActMode(true)
	if got := service.SelectExecutionMode(PlanExecutionModeRequest{NativeFC: true, LedgerEnabled: true}); got.Mode != PlanExecutionReAct {
		t.Fatalf("expected ReAct to outrank native-fc, got %s", got.Mode)
	}
	if got := service.SelectExecutionMode(PlanExecutionModeRequest{NativeFC: true, LedgerEnabled: false}); got.Mode != PlanExecutionNativeFC {
		t.Fatalf("expected native-fc when ReAct has no ledger, got %s", got.Mode)
	}

	service.SetLongHorizonMode(true)
	load := CognitiveLoadAssessment{Level: CognitiveLoadHigh, Score: 6}
	got := service.SelectExecutionMode(PlanExecutionModeRequest{
		NativeFC:             true,
		LedgerEnabled:        true,
		ComplexTask:          true,
		CognitiveLoad:        load,
		CognitiveLoadEnabled: true,
	})
	if got.Mode != PlanExecutionLongHorizon || got.CognitiveLoad.Score != 6 {
		t.Fatalf("expected long-horizon with cognitive load detail, got %#v", got)
	}
}

func TestRuntimeStrategyServiceSelectLongHorizonReasoningTier(t *testing.T) {
	var nilService *RuntimeStrategyService
	if got := nilService.SelectLongHorizonReasoningTier(context.Background(), LongHorizonReasoningTierRequest{
		Request: PlanRequest{ModelOverride: "expert"},
	}); got != "expert" {
		t.Fatalf("model override should win even without runtime strategy, got %q", got)
	}
	if got := nilService.SelectLongHorizonReasoningTier(context.Background(), LongHorizonReasoningTierRequest{}); got != "" {
		t.Fatalf("nil runtime strategy should select no tier, got %q", got)
	}

	service := NewRuntimeStrategyService()
	if got := service.SelectLongHorizonReasoningTier(context.Background(), LongHorizonReasoningTierRequest{
		Request: PlanRequest{TenantID: "tenant-1"},
		PlanID:  "plan-1",
		Query:   "reason about evidence",
	}); got != "" {
		t.Fatalf("runtime strategy without agentic thinking should select no tier, got %q", got)
	}

	thinker := &stubAgenticThinker{result: &localbrain.ThinkResult{Level: localbrain.ThinkDeep}}
	service.SetAgenticThinking(thinker)
	if got := service.SelectLongHorizonReasoningTier(context.Background(), LongHorizonReasoningTierRequest{
		Request:   PlanRequest{TenantID: "tenant-1"},
		PlanID:    "plan-1",
		Query:     "reason about evidence",
		StepIndex: 3,
	}); got != "expert" {
		t.Fatalf("expected deep thinking to select expert tier, got %q", got)
	}
	if len(thinker.calls) != 1 {
		t.Fatalf("expected one thinking call, got %d", len(thinker.calls))
	}
	call := thinker.calls[0]
	if call.TaskID != "plan-1" || call.TenantID != "tenant-1" || call.Query != "reason about evidence" || call.StepIndex != 3 {
		t.Fatalf("unexpected thinking request: %#v", call)
	}

	if got := service.SelectLongHorizonReasoningTier(context.Background(), LongHorizonReasoningTierRequest{
		Request: PlanRequest{ModelOverride: "fast"},
		Query:   "override should bypass thinking",
	}); got != "fast" {
		t.Fatalf("model override should bypass agentic thinking, got %q", got)
	}
	if len(thinker.calls) != 1 {
		t.Fatalf("override should not call agentic thinking, got %d calls", len(thinker.calls))
	}
}

func TestRuntimeStrategyServiceSelectProviderByCapability(t *testing.T) {
	reg := llm.NewProviderRegistry(llm.NewPool())
	if err := reg.Register(llm.ProviderConfig{
		ID:           "vision-provider",
		Type:         llm.ProviderTypeChat,
		BaseURL:      "http://example.invalid",
		Model:        "vision-model",
		Enabled:      true,
		Capabilities: []llm.Capability{llm.CapChat, llm.CapVision},
		Priority:     1,
	}); err != nil {
		t.Fatalf("register provider: %v", err)
	}

	service := NewRuntimeStrategyService()
	service.SetProviderRegistry(reg)
	got := service.SelectProviderByCapability(llm.CapVision)
	if got == nil || got.Config.ID != "vision-provider" {
		t.Fatalf("expected vision provider, got %#v", got)
	}
}

func TestNilRuntimeStrategyServiceIsNoop(t *testing.T) {
	var service *RuntimeStrategyService
	if service.ReActMode() || service.LongHorizonMode() {
		t.Fatal("nil service should report disabled modes")
	}
	if service.HasContextFilter() {
		t.Fatal("nil service should have no context filter")
	}
	if got, err := service.FilterContext(context.Background(), "query", nil, 1); got != nil || err != nil {
		t.Fatalf("nil service should not filter context, got=%#v err=%v", got, err)
	}
	if service.ShouldClassify(PlanRequest{}) {
		t.Fatal("nil service should not classify")
	}
	if got, err := service.ClassifyRequest(context.Background(), PlanRequest{}, "hi"); got != nil || err != nil {
		t.Fatalf("nil service should not classify request, got=%#v err=%v", got, err)
	}
	if got, err := service.Think(context.Background(), RuntimeThinkRequest{Query: "query"}); got != nil || err != nil {
		t.Fatalf("nil service should not run agentic thinking, got=%#v err=%v", got, err)
	}
	if got := service.SelectProviderByCapability(llm.CapVision); got != nil {
		t.Fatalf("nil service should select no provider, got %#v", got)
	}
}

type stubAgenticThinker struct {
	result *localbrain.ThinkResult
	err    error
	calls  []localbrain.ThinkRequest
}

func (s *stubAgenticThinker) Think(_ context.Context, req localbrain.ThinkRequest) (*localbrain.ThinkResult, error) {
	s.calls = append(s.calls, req)
	return s.result, s.err
}
