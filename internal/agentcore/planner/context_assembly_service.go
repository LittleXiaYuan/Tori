package planner

import (
	"context"
	"log/slog"
	"strings"

	agentcogni "yunque-agent/internal/agentcore/cogni"
	"yunque-agent/internal/agentcore/emotion"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/observe"
	"yunque-agent/pkg/skills"
)

// ContextAssemblyService owns Planner's dynamic context assembly callbacks.
//
// It groups retrieval, strategy, Cogni, CognitivePlugin, and belief context
// sources behind one boundary. Planner remains responsible for orchestration;
// PromptBuilder consumes a snapshot of this service instead of many scattered
// Planner fields.
type ContextAssemblyService struct {
	memory             MemorySearchFunc
	graphContext       func(query string) string
	graphContextTenant func(ctx context.Context, tenantID, query string) string
	codeContext        func(query string) string
	stateContext       func() string
	strategyContext    func() string
	strategyContextFor func(query string) string
	cognitiveContext   CognitiveContextFunc
	runtimeGrade       RuntimeGradeFunc // #4: trust gate tier + skill list + risk_level
	cogniService       *CogniContextService
	// packContext lets enabled capability packs (Tier 0 microkernel) inject
	// context into the prompt, so a Pack's enablement actually shows up in the
	// agent's reasoning flow — not just as an HTTP route. nil = no pack context.
	packContext func(ctx context.Context, tenantID, query string) string
}

// SetPackContext attaches a source that aggregates context from enabled packs.
func (s *ContextAssemblyService) SetPackContext(fn func(ctx context.Context, tenantID, query string) string) {
	if s != nil {
		s.packContext = fn
	}
}

type DynamicContextAssemblyRequest struct {
	LastMessage string
	TenantID    string
	Channel     string
	TaskContext string
	EmotionHint *emotion.Result
	IntentHint  string // LocalBrain intent category; see PlanRequest.IntentHint
}

type DynamicContextAssemblyResult struct {
	Content        string
	IncludedLayers []string
}

func NewContextAssemblyService() *ContextAssemblyService {
	return &ContextAssemblyService{}
}

func (s *ContextAssemblyService) SetMemory(fn MemorySearchFunc) {
	if s != nil {
		s.memory = fn
	}
}

func (s *ContextAssemblyService) Memory(ctx context.Context, tenantID, query string) string {
	if s == nil || s.memory == nil {
		return ""
	}
	return s.memory(ctx, tenantID, query)
}

func (s *ContextAssemblyService) SetGraphContext(fn func(query string) string) {
	if s != nil {
		s.graphContext = fn
	}
}

func (s *ContextAssemblyService) AppendGraphContext(fn func(query string) string) {
	if s == nil || fn == nil {
		return
	}
	prev := s.graphContext
	if prev == nil {
		s.graphContext = fn
		return
	}
	s.graphContext = func(query string) string {
		return JoinContextSections(prev(query), fn(query))
	}
}

func (s *ContextAssemblyService) GraphContextFor(query string) string {
	if s == nil || s.graphContext == nil {
		return ""
	}
	return s.graphContext(query)
}

// SetGraphContextForTenant attaches a tenant-aware graph/recall context source.
// Preferred over SetGraphContext when the source needs the per-request tenant
// (for example Ledger recall, which scopes memories by tenant).
func (s *ContextAssemblyService) SetGraphContextForTenant(fn func(ctx context.Context, tenantID, query string) string) {
	if s != nil {
		s.graphContextTenant = fn
	}
}

// HasGraphContext reports whether any graph context source (tenant-aware or
// query-only) is wired.
func (s *ContextAssemblyService) HasGraphContext() bool {
	return s != nil && (s.graphContext != nil || s.graphContextTenant != nil)
}

// GraphContextForRequest assembles graph context from both the tenant-aware and
// query-only sources (whichever are wired), joined as sections.
func (s *ContextAssemblyService) GraphContextForRequest(ctx context.Context, tenantID, query string) string {
	if s == nil {
		return ""
	}
	var sections []string
	if s.graphContextTenant != nil {
		sections = append(sections, s.graphContextTenant(ctx, tenantID, query))
	}
	if s.graphContext != nil {
		sections = append(sections, s.graphContext(query))
	}
	return JoinContextSections(sections...)
}

func JoinContextSections(sections ...string) string {
	parts := make([]string, 0, len(sections))
	for _, section := range sections {
		section = strings.TrimSpace(section)
		if section != "" {
			parts = append(parts, section)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n---\n")
}

func (s *ContextAssemblyService) SetCodeContext(fn func(query string) string) {
	if s != nil {
		s.codeContext = fn
	}
}

func (s *ContextAssemblyService) SetStateContext(fn func() string) {
	if s != nil {
		s.stateContext = fn
	}
}

func (s *ContextAssemblyService) SetStrategyContext(fn func() string) {
	if s != nil {
		s.strategyContext = fn
	}
}

func (s *ContextAssemblyService) SetStrategyContextFor(fn func(query string) string) {
	if s != nil {
		s.strategyContextFor = fn
	}
}

func (s *ContextAssemblyService) SetCognitiveContext(fn CognitiveContextFunc) {
	if s != nil {
		s.cognitiveContext = fn
	}
}

// SetRuntimeGrade attaches the runtime-grade context source (#4): trust gate
// tier, available skill list, and dynamic risk level. When wired, this is
// injected as a dedicated context layer so the LLM knows its own capability
// boundary and doesn't hallucinate tools or underestimate risk.
func (s *ContextAssemblyService) SetRuntimeGrade(fn RuntimeGradeFunc) {
	if s != nil {
		s.runtimeGrade = fn
	}
}

func (s *ContextAssemblyService) ensureCogniService() *CogniContextService {
	if s.cogniService == nil {
		s.cogniService = NewCogniContextService()
	}
	return s.cogniService
}

func (s *ContextAssemblyService) SetCogniRuntime(runtime CogniRuntime) {
	if s != nil {
		s.ensureCogniService().SetRuntime(runtime)
	}
}

func (s *ContextAssemblyService) CogniContext(ctx context.Context, message, tenantID, channel, scope string) string {
	if s == nil || s.cogniService == nil {
		return ""
	}
	return s.cogniService.Context(ctx, message, tenantID, channel, scope)
}

// CogniTools resolves the extra tools (e.g. MCP-backed) contributed by the
// cognis that activate for this turn. Returns nil when no cogni runtime is
// wired or none activates. The planner merges these into the per-turn tool list
// and routes their calls back through each tool's Invoke.
func (s *ContextAssemblyService) CogniTools(ctx context.Context, message, tenantID, channel string) []CogniTool {
	if s == nil || s.cogniService == nil {
		return nil
	}
	return s.cogniService.Tools(ctx, message, tenantID, channel)
}

// CogniSurfaceAuthoritative reports whether an activated cogni applied a
// non-identity ToolSurface this turn. The FC tool builder uses it to keep the
// cogni-declared surface definitive (skip per-message intent ranking/cap) for a
// deterministic, cache-stable tool prefix.
func (s *ContextAssemblyService) CogniSurfaceAuthoritative(message, tenantID, channel string) bool {
	if s == nil || s.cogniService == nil {
		return false
	}
	return s.cogniService.SurfaceAuthoritative(message, tenantID, channel)
}

// RecordCogniToolOutcome feeds a tool execution result back to the active
// cognis so a Cogni can self-tune its surface from accumulated outcomes. No-op
// when no cogni runtime is wired.
func (s *ContextAssemblyService) RecordCogniToolOutcome(message, tenantID, channel, tool string, success bool) {
	if s == nil || s.cogniService == nil {
		return
	}
	s.cogniService.RecordToolOutcome(message, tenantID, channel, tool, success)
}

// CogniDecide calls the v2 Cogni runtime's Decide() method to get the merged
// decision from all active Cognis (IntentCogni, RiskCogni, EmotionCogni, v1 compat).
// Returns a CogniFinalDecision with intent, tools/skills needed, memory scope, and
// behavioral guidance. This is the v2 entry point for intelligent resource allocation.
func (s *ContextAssemblyService) CogniDecide(ctx context.Context, message, tenantID, channel string) agentcogni.CogniFinalDecision {
	if s == nil || s.cogniService == nil || s.cogniService.runtime == nil {
		// No cogni runtime wired, return empty decision
		return agentcogni.CogniFinalDecision{}
	}

	// Call the v2 Decide() method on the cogni runtime
	// (wired in cmd/agent/module_cogni.go as plannerCogniRuntime)
	return s.cogniService.runtime.Decide(ctx, message, tenantID, channel)
}

func (s *ContextAssemblyService) ApplyCogniSkillFilter(message, tenantID, channel string, in []skills.Skill) []skills.Skill {
	if s == nil || s.cogniService == nil || !s.cogniService.HasSkillFilter() {
		return in
	}
	before := len(in)
	out := s.cogniService.FilterSkills(message, tenantID, channel, in)
	if after := len(out); after != before {
		slog.Info("buildFunctionDefs: cogni surface filter applied",
			"before", before, "after", after, "msg_prefix", truncate(message, 50))
	}
	return out
}

func (s *ContextAssemblyService) EmitCogniTrace(message, tenantID, channel, traceID, sessionID, taskID string, callback func(observe.AgentEvent)) {
	if s == nil || callback == nil || s.cogniService == nil || !s.cogniService.HasTrace() {
		return
	}
	detail, ok := s.cogniService.Trace(message, tenantID, channel)
	if !ok || !detail.hasVisibleEffect() {
		return
	}
	evt := observe.NewEvent(traceID, observe.DomainPlanner, observe.EventPlan, detail.summary())
	evt.Meta.TenantID = tenantID
	evt.Meta.SessionID = sessionID
	evt.Meta.TaskID = taskID
	evt.Detail = detail
	callback(evt)
}

func (s *ContextAssemblyService) EmitCogniTraceForRequest(req PlanRequest) {
	if s == nil {
		return
	}
	s.EmitCogniTrace(extractUserMessage(req), req.TenantID, req.ChannelType, req.TraceID, req.SessionID, req.TaskID, req.StepCallback)
}

func (s *ContextAssemblyService) BuildDynamicContext(ctx context.Context, req DynamicContextAssemblyRequest, builder *PromptBuilder) DynamicContextAssemblyResult {
	if builder == nil {
		return DynamicContextAssemblyResult{}
	}
	content := builder.BuildDynamicContext(ctx, DynamicContextRequest{
		LastMessage: req.LastMessage,
		TenantID:    req.TenantID,
		Channel:     req.Channel,
		TaskContext: req.TaskContext,
		EmotionHint: req.EmotionHint,
		IntentHint:  req.IntentHint,
	})
	layers := append([]string(nil), builder.LastIncludedLayers...)
	return DynamicContextAssemblyResult{Content: content, IncludedLayers: layers}
}

func (s *ContextAssemblyService) AppendDynamicContextMessage(ctx context.Context, messages []llm.Message, req DynamicContextAssemblyRequest, builder *PromptBuilder) ([]llm.Message, []string) {
	if req.LastMessage == "" {
		return messages, nil
	}
	dynamic := s.BuildDynamicContext(ctx, req, builder)
	if dynamic.Content != "" {
		messages = append(messages, llm.Message{
			Role:    "system",
			Content: "[动态上下文]\n" + dynamic.Content,
		})
	}
	return messages, dynamic.IncludedLayers
}
