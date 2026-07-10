package planner

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/subagent"
	"yunque-agent/internal/agentcore/task"
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

	// Async handoff execution (nil taskStore = feature off, sync path unchanged
	// — this is what keeps every existing test passing without modification).
	taskStore     task.Store
	notifyFn      func(sessionID string, msg llm.Message)
	broadcastFn   func(event, taskID, detail string)
	sessionModeFn func(sessionID string) string

	slotsMu sync.Mutex
	slots   map[string]chan struct{} // sessionID -> concurrency semaphore

	concurrencyXiaoyu int // max concurrent async handoffs per session in 小羽模式
	concurrencyAPI    int // max concurrent async handoffs per session in API模式
}

// defaultHandoffConcurrencyXiaoyu/API are the out-of-the-box ceilings —
// 小羽模式 stays sequential (matches today's one-thing-at-a-time feel), API模式
// allows a Codex/Claude-Code-style burst of concurrent sub-agent tasks.
const (
	defaultHandoffConcurrencyXiaoyu = 1
	defaultHandoffConcurrencyAPI    = 4
)

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
	return &DelegationRuntimeService{
		slots:             make(map[string]chan struct{}),
		concurrencyXiaoyu: defaultHandoffConcurrencyXiaoyu,
		concurrencyAPI:    defaultHandoffConcurrencyAPI,
	}
}

// SetHandoffConcurrency overrides the default per-session concurrency
// ceilings for async handoffs. Values <= 0 are ignored (keep the default).
func (s *DelegationRuntimeService) SetHandoffConcurrency(xiaoyu, api int) {
	if s == nil {
		return
	}
	if xiaoyu > 0 {
		s.concurrencyXiaoyu = xiaoyu
	}
	if api > 0 {
		s.concurrencyAPI = api
	}
}

// sessionConcurrencyLimit resolves the async-handoff concurrency ceiling for
// a session based on its 小羽/API mode.
func (s *DelegationRuntimeService) sessionConcurrencyLimit(sessionID string) int {
	mode := ""
	if s.sessionModeFn != nil {
		mode = s.sessionModeFn(sessionID)
	}
	if mode == "api" {
		return s.concurrencyAPI
	}
	return s.concurrencyXiaoyu
}

// sessionSlot returns the semaphore channel for a session, creating it with
// the session's current concurrency ceiling on first use.
func (s *DelegationRuntimeService) sessionSlot(sessionID string) chan struct{} {
	s.slotsMu.Lock()
	defer s.slotsMu.Unlock()
	if ch, ok := s.slots[sessionID]; ok {
		return ch
	}
	ch := make(chan struct{}, s.sessionConcurrencyLimit(sessionID))
	s.slots[sessionID] = ch
	return ch
}

// releaseSessionSlot returns one token to the session semaphore and evicts the
// channel from the map once it is idle (no in-flight handoffs). Without this,
// s.slots would grow one channel per distinct session for the life of the
// process — a slow but unbounded leak in a long-running local agent. Eviction
// happens under slotsMu, and sessionSlot re-creates the channel on demand, so a
// later handoff for the same session simply gets a fresh semaphore.
func (s *DelegationRuntimeService) releaseSessionSlot(sessionID string, slot chan struct{}) {
	s.slotsMu.Lock()
	defer s.slotsMu.Unlock()
	<-slot
	// Only evict the map entry if it still points at this exact channel and no
	// tokens are outstanding. Identity check guards against a racing goroutine
	// that already evicted-and-recreated the entry.
	if cur, ok := s.slots[sessionID]; ok && cur == slot && len(slot) == 0 {
		delete(s.slots, sessionID)
	}
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

// SetHandoffTaskRuntime attaches a task store so handoff delegation runs as a
// background task instead of blocking the calling chat turn. Call this from
// cmd/agent wiring; leaving it unset (nil store) keeps the synchronous path,
// which is what every existing delegation/handoff test relies on.
func (s *DelegationRuntimeService) SetHandoffTaskRuntime(store task.Store) {
	if s == nil {
		return
	}
	s.taskStore = store
}

// SetHandoffAsyncNotifier wires how the async path reports results back to
// the user: notifyFn appends a message to the conversation the delegation
// started from, broadcastFn additionally pushes a "task.<event>" SSE event
// (see broadcastTaskEvent in the gateway package) for any listener tracking
// task lifecycle directly.
func (s *DelegationRuntimeService) SetHandoffAsyncNotifier(notifyFn func(sessionID string, msg llm.Message), broadcastFn func(event, taskID, detail string)) {
	if s == nil {
		return
	}
	s.notifyFn = notifyFn
	s.broadcastFn = broadcastFn
}

// SetSessionModeResolver wires the 小羽/API session mode lookup, used to pick
// a per-session concurrency ceiling for async handoffs (小羽=1, api=higher).
// An unset resolver treats every session as 小羽 mode (ceiling 1).
func (s *DelegationRuntimeService) SetSessionModeResolver(fn func(sessionID string) string) {
	if s == nil {
		return
	}
	s.sessionModeFn = fn
}

// asyncCapable reports whether handoff delegation should run in the
// background instead of blocking. Gated purely on wiring (a task store being
// attached), not a request-level flag, so production gets the fix by default
// while every test that constructs a bare DelegationRuntimeService keeps
// exercising the original synchronous path untouched.
func (s *DelegationRuntimeService) asyncCapable() bool {
	return s != nil && s.taskStore != nil
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

	if s.asyncCapable() {
		return s.executeHandoffAsync(ctx, req, toolName, agentName, input, hooks)
	}

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
