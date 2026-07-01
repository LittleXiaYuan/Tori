package planner

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"yunque-agent/internal/agentcore/subagent"
	"yunque-agent/internal/observe"
	"yunque-agent/pkg/opp"
)

var ErrHandoffRegistryNotConfigured = fmt.Errorf("handoff registry not configured")

// FederationBridge abstracts the OPP federation layer so the planner
// can delegate tasks to remote agents without importing the federation package.
type FederationBridge interface {
	// Delegate sends a task to the best matching remote agent via model-aware routing.
	Delegate(ctx context.Context, dp opp.DelegatePayload, timeout time.Duration) (*opp.DelegateResultPayload, error)
	// LocalCaps returns the local agent's capabilities.
	LocalCaps() opp.CapabilitiesPayload
}

// DelegationRuntimeService owns Planner's delegation boundaries: local
// handoff agents and OPP federation. Keeping them together makes delegation a
// runtime capability instead of two more top-level Planner fields.
type DelegationRuntimeService struct {
	handoffReg *subagent.HandoffRegistry
	fedBridge  FederationBridge
}

type HandoffExecutionHooks struct {
	Metrics                SkillMetricsFunc
	RecordExecutionFailure func(failed bool) bool
}

// HandoffExecutionHooksProvider is the narrow Planner-side contract required
// to build request-level handoff side-effect hooks. Executors should ask the
// delegation runtime for hooks through this interface instead of repeatedly
// reaching into Planner metrics and proactive cognition internals.
type HandoffExecutionHooksProvider interface {
	HandoffMetricsHook() SkillMetricsFunc
	HandoffFailureHook() func(failed bool) bool
}

type HandoffExecutionResult struct {
	Handled       bool
	ToolName      string
	AgentName     string
	Input         string
	Reply         string
	PartialResult string // #33: subagent's recoverable work on timeout/cancel
	Duration      time.Duration
	Err           error
}

func NewDelegationRuntimeService() *DelegationRuntimeService {
	return &DelegationRuntimeService{}
}

func (s *DelegationRuntimeService) SetHandoffRegistry(reg *subagent.HandoffRegistry) {
	if s == nil {
		return
	}
	s.handoffReg = reg
}

func (s *DelegationRuntimeService) HandoffRegistry() *subagent.HandoffRegistry {
	if s == nil {
		return nil
	}
	return s.handoffReg
}

func (s *DelegationRuntimeService) SetFederationBridge(bridge FederationBridge) {
	if s == nil {
		return
	}
	s.fedBridge = bridge
}

func (s *DelegationRuntimeService) FederationBridge() FederationBridge {
	if s == nil {
		return nil
	}
	return s.fedBridge
}

func (s *DelegationRuntimeService) IsHandoffCall(name string) (string, bool) {
	if s == nil || s.handoffReg == nil {
		return "", false
	}
	return s.handoffReg.IsHandoffCall(name)
}

func (s *DelegationRuntimeService) HandoffTimeoutForTool(name string, fallback time.Duration) time.Duration {
	if _, ok := s.IsHandoffCall(name); ok {
		// Subagents do real multi-step work — research_exec runs several
		// searches, file_exec generates whole documents (PPT/Word) in a single
		// long LLM turn. With a slow reasoning model 90s truncated them into
		// partial results before the deliverable existed. 240s gives a document
		// generation turn room to finish. 360s ≥ the 300s single-call ceiling so
		// a slow reasoning-model document turn isn't cut by the handoff itself.
		return 360 * time.Second
	}
	return fallback
}

func (s *DelegationRuntimeService) HandoffHooks(provider HandoffExecutionHooksProvider) HandoffExecutionHooks {
	if provider == nil {
		return HandoffExecutionHooks{}
	}
	return HandoffExecutionHooks{
		Metrics:                provider.HandoffMetricsHook(),
		RecordExecutionFailure: provider.HandoffFailureHook(),
	}
}

func (s *DelegationRuntimeService) HasHandoffAgents(min int) bool {
	if s == nil || s.handoffReg == nil {
		return false
	}
	return len(s.handoffReg.List()) >= min
}

func (s *DelegationRuntimeService) HandoffToolDefinitions() []map[string]any {
	if s == nil || s.handoffReg == nil {
		return nil
	}
	return s.handoffReg.ToolDefinitions()
}

func (s *DelegationRuntimeService) ExecuteHandoff(ctx context.Context, tenantID, agentName, input, providerOverride string) (*subagent.HandoffResult, error) {
	if s == nil || s.handoffReg == nil {
		return nil, ErrHandoffRegistryNotConfigured
	}
	return s.handoffReg.Execute(ctx, tenantID, agentName, input, providerOverride)
}

func (s *DelegationRuntimeService) ExecuteHandoffForRequest(ctx context.Context, req PlanRequest, toolName string, args map[string]any, source string, step int, hooks HandoffExecutionHooks) HandoffExecutionResult {
	agentName, ok := s.IsHandoffCall(toolName)
	if !ok {
		return HandoffExecutionResult{Handled: false, ToolName: toolName}
	}

	input := handoffInputFromArgs(args)
	slog.Info("planner: handoff delegation", "source", source, "agent", agentName, "step", step)
	emitHandoffStart(req, agentName, input)

	cbCtx := ctx
	if req.StepCallback != nil {
		cbCtx = WithStepCallback(ctx, req.StepCallback)
	}

	startedAt := time.Now()
	hr, err := s.ExecuteHandoff(cbCtx, req.TenantID, agentName, input, req.EffectiveModelTier())
	duration := time.Since(startedAt)
	if hooks.Metrics != nil {
		hooks.Metrics(toolName, duration, err)
	}
	if hooks.RecordExecutionFailure != nil {
		hooks.RecordExecutionFailure(err != nil)
	}

	reply := ""
	partial := ""
	if hr != nil {
		reply = hr.Reply
		partial = hr.PartialResult
	}
	emitHandoffDone(req, agentName, reply, duration, err)

	return HandoffExecutionResult{
		Handled:       true,
		ToolName:      toolName,
		AgentName:     agentName,
		Input:         input,
		Reply:         reply,
		PartialResult: partial,
		Duration:      duration,
		Err:           err,
	}
}

func handoffInputFromArgs(args map[string]any) string {
	if args == nil {
		return ""
	}
	if input, _ := args["input"].(string); input != "" {
		return input
	}
	for _, value := range args {
		if input, ok := value.(string); ok && input != "" {
			return input
		}
	}
	return ""
}

func emitHandoffStart(req PlanRequest, agentName, input string) {
	if req.StepCallback == nil {
		return
	}
	evt := observe.NewEvent(req.TraceID, observe.DomainAgent, observe.EventHandoffStart,
		fmt.Sprintf("🤖 委派 [%s]：%s", agentName, truncate(input, 80)))
	evt.Meta.TenantID = req.TenantID
	evt.Meta.SessionID = req.SessionID
	evt.Meta.TaskID = req.TaskID
	evt.Meta.Skill = agentName
	evt.Detail = observe.HandoffDetail{Agent: agentName, Input: truncate(input, 200)}
	req.StepCallback(evt)
}

func emitHandoffDone(req PlanRequest, agentName, reply string, duration time.Duration, err error) {
	if req.StepCallback == nil {
		return
	}
	doneEvt := observe.NewEvent(req.TraceID, observe.DomainAgent, observe.EventHandoffDone,
		fmt.Sprintf("✅ [%s] 完成 (%.1fs)", agentName, duration.Seconds()))
	doneEvt.Meta.TenantID = req.TenantID
	doneEvt.Meta.SessionID = req.SessionID
	doneEvt.Meta.TaskID = req.TaskID
	doneEvt.Meta.Skill = agentName
	detail := observe.HandoffDetail{Agent: agentName, DurMs: duration.Milliseconds()}
	if err != nil {
		doneEvt.Summary = handoffFailureSummary(agentName, err)
		detail = buildHandoffFailureDetail(agentName, duration, err)
	} else {
		detail.Reply = truncate(reply, 200)
	}
	doneEvt.Detail = detail
	req.StepCallback(doneEvt)
}
