package planner

import (
	"context"
	"log/slog"
	"reflect"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/localbrain"
)

// RuntimeContextItem is the planner/runtime-facing DTO for context filtering.
// It keeps PromptBuilder and other runtime callers independent from LocalBrain's
// package-level filter structs.
type RuntimeContextItem struct {
	Source     string
	Content    string
	Importance int
	Score      float64
}

// RuntimeContextFilterResult is the planner/runtime-facing output for context
// filtering. RuntimeStrategyService owns conversion to/from LocalBrain DTOs.
type RuntimeContextFilterResult struct {
	Items    []RuntimeContextItem
	Summary  string
	Filtered int
	Elapsed  time.Duration
}

type RuntimeThinkLevel string

const (
	RuntimeThinkNone   RuntimeThinkLevel = "none"
	RuntimeThinkQuick  RuntimeThinkLevel = "quick"
	RuntimeThinkNormal RuntimeThinkLevel = "normal"
	RuntimeThinkDeep   RuntimeThinkLevel = "deep"
)

// RuntimeThinkStepSummary is the planner/runtime-facing summary for adaptive
// thinking history. RuntimeStrategyService owns conversion to LocalBrain.
type RuntimeThinkStepSummary struct {
	Action  string
	Result  string
	Success bool
}

// RuntimeThinkRequest is the planner/runtime-facing adaptive-thinking request.
type RuntimeThinkRequest struct {
	TaskID           string
	TenantID         string
	Query            string
	PrevActionResult string
	StepIndex        int
	StepHistory      []RuntimeThinkStepSummary
}

// RuntimeThinkResult is the planner/runtime-facing adaptive-thinking result.
type RuntimeThinkResult struct {
	Level      RuntimeThinkLevel
	Thought    string
	NextAction string
	Confidence float64
	ShouldStop bool
}

// RuntimeIntent is the planner/runtime-facing LocalBrain intent snapshot.
type RuntimeIntent struct {
	Category   string
	Complexity string
	Confidence float64
	NeedTools  bool
}

// RuntimeDecision is the planner/runtime-facing LocalBrain routing decision.
type RuntimeDecision struct {
	Handler    string
	Intent     RuntimeIntent
	Reason     string
	LocalReply string
}

// RuntimeClassificationResult is the planner-facing outcome of applying a
// LocalBrain routing decision to a PlanRequest. It keeps Planner from knowing
// how LocalBrain handlers, intent snapshots, and trace metadata are interpreted.
type RuntimeClassificationResult struct {
	Decision     *RuntimeDecision
	Request      PlanRequest
	ToolFree     bool
	LogHandler   string
	LogIntent    string
	LogNeedTools bool
	LogReason    string
	TraceHandler string
	TraceReason  string
	TraceScore   float64
	TraceMeta    map[string]interface{}
}

type PlanExecutionMode string

const (
	PlanExecutionTextBased   PlanExecutionMode = "text-based"
	PlanExecutionLongHorizon PlanExecutionMode = "long-horizon"
	PlanExecutionReAct       PlanExecutionMode = "react"
	PlanExecutionNativeFC    PlanExecutionMode = "native-fc"
)

type PlanExecutionModeRequest struct {
	Request              PlanRequest
	NativeFC             bool
	LedgerEnabled        bool
	ComplexTask          bool
	CognitiveLoad        CognitiveLoadAssessment
	CognitiveLoadEnabled bool
}

type PlanExecutionModeDecision struct {
	Mode          PlanExecutionMode
	CognitiveLoad CognitiveLoadAssessment
}

// LocalBrainRuntime is the narrow runtime-strategy contract for LocalBrain.
// Keeping the concrete *localbrain.LocalBrain behind this interface lets
// Planner expose runtime wiring without importing the LocalBrain package.
type LocalBrainRuntime interface {
	Classify(ctx context.Context, query, tenantID string) (*localbrain.Decision, error)
	FilterContext(ctx context.Context, query string, items []localbrain.ContextItem, maxItems int) (*localbrain.FilterResult, error)
}

// AgenticThinkerRuntime is the narrow runtime-strategy contract for adaptive
// thinking. RuntimeStrategyService still owns LocalBrain DTO conversion; callers
// outside this file should use RuntimeThinkRequest/RuntimeThinkResult.
type AgenticThinkerRuntime interface {
	Think(ctx context.Context, req localbrain.ThinkRequest) (*localbrain.ThinkResult, error)
}

// RuntimeStrategyService owns Planner runtime strategy switches and
// capability-aware provider routing. Model pool lookup and fallback chains live
// in ModelRuntimeService; this service keeps higher-level execution mode state.
type RuntimeStrategyService struct {
	reactMode        bool
	longHorizonMode  bool
	longHorizonStore LongHorizonCheckpointStore
	localBrain       LocalBrainRuntime
	agenticThinking  AgenticThinkerRuntime
	providerReg      *llm.ProviderRegistry
}

func NewRuntimeStrategyService() *RuntimeStrategyService {
	return &RuntimeStrategyService{}
}

func (s *RuntimeStrategyService) SetReActMode(enabled bool) {
	if s == nil {
		return
	}
	s.reactMode = enabled
}

func (s *RuntimeStrategyService) ReActMode() bool {
	return s != nil && s.reactMode
}

func (s *RuntimeStrategyService) SetLongHorizonMode(enabled bool) {
	if s == nil {
		return
	}
	s.longHorizonMode = enabled
}

func (s *RuntimeStrategyService) LongHorizonMode() bool {
	return s != nil && s.longHorizonMode
}

func (s *RuntimeStrategyService) SetLongHorizonCheckpointStore(store LongHorizonCheckpointStore) {
	if s == nil {
		return
	}
	s.longHorizonStore = store
}

func (s *RuntimeStrategyService) LongHorizonCheckpointStore() LongHorizonCheckpointStore {
	if s == nil {
		return nil
	}
	return s.longHorizonStore
}

func (s *RuntimeStrategyService) SetLocalBrain(brain LocalBrainRuntime) {
	if s == nil {
		return
	}
	if isNilRuntime(brain) {
		s.localBrain = nil
		return
	}
	s.localBrain = brain
}

func (s *RuntimeStrategyService) HasContextFilter() bool {
	return s != nil && s.localBrain != nil
}

func (s *RuntimeStrategyService) FilterContext(ctx context.Context, query string, items []RuntimeContextItem, maxItems int) (*RuntimeContextFilterResult, error) {
	if s == nil || s.localBrain == nil {
		return nil, nil
	}
	filtered, err := s.localBrain.FilterContext(ctx, query, toLocalBrainContextItems(items), maxItems)
	if err != nil {
		return nil, err
	}
	return fromLocalBrainFilterResult(filtered), nil
}

func toLocalBrainContextItems(items []RuntimeContextItem) []localbrain.ContextItem {
	if len(items) == 0 {
		return nil
	}
	converted := make([]localbrain.ContextItem, 0, len(items))
	for _, item := range items {
		converted = append(converted, localbrain.ContextItem{
			Source:     item.Source,
			Content:    item.Content,
			Importance: item.Importance,
			Score:      item.Score,
		})
	}
	return converted
}

func fromLocalBrainFilterResult(result *localbrain.FilterResult) *RuntimeContextFilterResult {
	if result == nil {
		return nil
	}
	converted := make([]RuntimeContextItem, 0, len(result.Items))
	for _, item := range result.Items {
		converted = append(converted, RuntimeContextItem{
			Source:     item.Source,
			Content:    item.Content,
			Importance: item.Importance,
			Score:      item.Score,
		})
	}
	return &RuntimeContextFilterResult{
		Items:    converted,
		Summary:  result.Summary,
		Filtered: result.Filtered,
		Elapsed:  result.Elapsed,
	}
}

func (s *RuntimeStrategyService) SetAgenticThinking(thinking AgenticThinkerRuntime) {
	if s == nil {
		return
	}
	if isNilRuntime(thinking) {
		s.agenticThinking = nil
		return
	}
	s.agenticThinking = thinking
}

func (s *RuntimeStrategyService) SetProviderRegistry(reg *llm.ProviderRegistry) {
	if s == nil {
		return
	}
	s.providerReg = reg
}

func (s *RuntimeStrategyService) SelectProviderByCapability(required ...llm.Capability) *llm.ProviderInstance {
	if s == nil || s.providerReg == nil || len(required) == 0 {
		return nil
	}
	return s.providerReg.SelectByCapability(required...)
}

func (s *RuntimeStrategyService) Classify(ctx context.Context, query, tenantID string) (*RuntimeDecision, error) {
	if s == nil || s.localBrain == nil {
		return nil, nil
	}
	decision, err := s.localBrain.Classify(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	return fromLocalBrainDecision(decision), nil
}

func (s *RuntimeStrategyService) ClassifyRequest(ctx context.Context, req PlanRequest, query string) (*RuntimeClassificationResult, error) {
	if !s.ShouldClassify(req) {
		return nil, nil
	}
	decision, err := s.Classify(ctx, query, req.TenantID)
	if err != nil || decision == nil {
		return nil, err
	}
	classifiedReq := req
	if decision.Handler != "local" {
		classifiedReq.ModelOverride = decision.Handler
	}
	return &RuntimeClassificationResult{
		Decision:     decision,
		Request:      classifiedReq,
		ToolFree:     !decision.IntentNeedTools(),
		LogHandler:   decision.Handler,
		LogIntent:    decision.IntentCategory(),
		LogNeedTools: decision.IntentNeedTools(),
		LogReason:    decision.Reason,
		TraceHandler: decision.Handler,
		TraceReason:  decision.Reason,
		TraceScore:   decision.IntentConfidence(),
		TraceMeta: map[string]interface{}{
			"category":   decision.IntentCategory(),
			"complexity": decision.IntentComplexity(),
			"need_tools": decision.IntentNeedTools(),
		},
	}, nil
}

func fromLocalBrainDecision(decision *localbrain.Decision) *RuntimeDecision {
	if decision == nil {
		return nil
	}
	return &RuntimeDecision{
		Handler: decision.Handler,
		Intent: RuntimeIntent{
			Category:   decision.Intent.Category,
			Complexity: decision.Intent.Complexity,
			Confidence: decision.Intent.Confidence,
			NeedTools:  decision.Intent.NeedTools,
		},
		Reason:     decision.Reason,
		LocalReply: decision.LocalReply,
	}
}

func (d *RuntimeDecision) IntentCategory() string {
	if d == nil {
		return ""
	}
	return d.Intent.Category
}

func (d *RuntimeDecision) IntentComplexity() string {
	if d == nil {
		return ""
	}
	return d.Intent.Complexity
}

func (d *RuntimeDecision) IntentConfidence() float64 {
	if d == nil {
		return 0
	}
	return d.Intent.Confidence
}

func (d *RuntimeDecision) IntentNeedTools() bool {
	if d == nil {
		return false
	}
	return d.Intent.NeedTools
}

func (s *RuntimeStrategyService) ShouldClassify(req PlanRequest) bool {
	return s != nil && s.localBrain != nil && req.ModelOverride == "" && !req.DisableDelegation
}

func (s *RuntimeStrategyService) SelectExecutionMode(req PlanExecutionModeRequest) PlanExecutionModeDecision {
	decision := PlanExecutionModeDecision{Mode: PlanExecutionTextBased}
	if s != nil && s.LongHorizonMode() && req.ComplexTask {
		decision.Mode = PlanExecutionLongHorizon
		if req.CognitiveLoadEnabled {
			decision.CognitiveLoad = req.CognitiveLoad
		}
		return decision
	}
	if s != nil && s.ReActMode() && req.LedgerEnabled {
		decision.Mode = PlanExecutionReAct
		return decision
	}
	if req.NativeFC {
		decision.Mode = PlanExecutionNativeFC
	}
	return decision
}

func (s *RuntimeStrategyService) Think(ctx context.Context, req RuntimeThinkRequest) (*RuntimeThinkResult, error) {
	if s == nil || s.agenticThinking == nil {
		return nil, nil
	}
	result, err := s.agenticThinking.Think(ctx, toLocalBrainThinkRequest(req))
	if err != nil {
		return nil, err
	}
	return fromLocalBrainThinkResult(result), nil
}

func (s *RuntimeStrategyService) SelectTierFromThinking(ctx context.Context, req RuntimeThinkRequest) (tier string, stop bool, result *RuntimeThinkResult) {
	result, err := s.Think(ctx, req)
	if err != nil || result == nil {
		if err != nil {
			slog.Debug("runtime strategy: agentic thinking skipped", "err", err)
		}
		return "", false, nil
	}
	if result.ShouldStop {
		return "", true, result
	}
	switch result.Level {
	case RuntimeThinkQuick:
		return "fast", false, result
	case RuntimeThinkDeep:
		return "expert", false, result
	default:
		return "smart", false, result
	}
}

func toLocalBrainThinkRequest(req RuntimeThinkRequest) localbrain.ThinkRequest {
	return localbrain.ThinkRequest{
		TaskID:           req.TaskID,
		TenantID:         req.TenantID,
		Query:            req.Query,
		PrevActionResult: req.PrevActionResult,
		StepIndex:        req.StepIndex,
		StepHistory:      toLocalBrainStepSummaries(req.StepHistory),
	}
}

func toLocalBrainStepSummaries(steps []RuntimeThinkStepSummary) []localbrain.StepSummary {
	if len(steps) == 0 {
		return nil
	}
	converted := make([]localbrain.StepSummary, 0, len(steps))
	for _, step := range steps {
		converted = append(converted, localbrain.StepSummary{
			Action:  step.Action,
			Result:  step.Result,
			Success: step.Success,
		})
	}
	return converted
}

func fromLocalBrainThinkResult(result *localbrain.ThinkResult) *RuntimeThinkResult {
	if result == nil {
		return nil
	}
	return &RuntimeThinkResult{
		Level:      fromLocalBrainThinkLevel(result.Level),
		Thought:    result.Thought,
		NextAction: result.NextAction,
		Confidence: result.Confidence,
		ShouldStop: result.ShouldStop,
	}
}

func fromLocalBrainThinkLevel(level localbrain.ThinkLevel) RuntimeThinkLevel {
	switch level {
	case localbrain.ThinkNone:
		return RuntimeThinkNone
	case localbrain.ThinkQuick:
		return RuntimeThinkQuick
	case localbrain.ThinkDeep:
		return RuntimeThinkDeep
	default:
		return RuntimeThinkNormal
	}
}

func isNilRuntime(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}
