package planner

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	ldg "yunque-agent/internal/ledgercore"

	ctxwindow "yunque-agent/internal/agentcore/context"
	"yunque-agent/internal/agentcore/emotion"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/localbrain"
	"yunque-agent/internal/agentcore/subagent"
	"yunque-agent/internal/experimental/recommend"
	iledger "yunque-agent/internal/ledger"
	"yunque-agent/internal/observe"
	"yunque-agent/pkg/skills"
)

type SkillMetricsFunc func(skillName string, duration time.Duration, err error)

// SkillIndexEntry is a lightweight slug+description pair for the L2 skill index.
type SkillIndexEntry struct {
	Slug        string
	Description string
}

type SkillIndexFunc func() []SkillIndexEntry

// Planner is the brain: understands intent, breaks tasks into steps, and drives skills.
type Planner struct {
	llm                    *llm.Client
	llmPool                *llm.Pool // multi-model pool (nil = single model mode)
	registry               *skills.Registry
	toolTimeout            time.Duration // per-tool execution timeout (default 60s)
	maxSteps               int
	memory                 MemorySearchFunc
	reflect                ReflectFunc
	skillMetrics           SkillMetricsFunc
	domainPrompt           string
	personaPrompt          func() string              // dynamic persona system prompt
	graphContext           func(query string) string  // knowledge graph context injector
	codeContext            func(query string) string  // code knowledge context injector (repo-level)
	useNativeFC            bool                       // use native LLM function calling
	windowCfg              *ctxwindow.WindowConfig    // context window trimming config
	ctxManager             *ctxwindow.Manager         // multi-stage context compression manager
	cachedSysPrompt        string                     // cached base system prompt
	sysPromptVer           int                        // incremented when skills change
	skillIndex             SkillIndexFunc             // L2 installed skill index (nil = no L2)
	handoffReg             *subagent.HandoffRegistry  // handoff tool registry for subagent delegation
	longHorizonCheckpoints LongHorizonCheckpointStore // recoverable DAG checkpoint persistence
	stateContext           func() string              // structured state kernel context
	strategyContext        func() string              // reflection loop strategy context
	strategyContextFor     func(query string) string  // query-aware reflection strategy context
	dynContextBudget       int                        // max tokens for dynamic context layer assembly (DynContextBudgetDefault = use 4000)
	ackEnabled             bool                       // send typing indicators / ack
	locale                 string                     // agent locale (e.g. "zh-CN")
	// browserDispatch removed — browser skills now handled via skill registry (browserskill package)
	trustGate        *SkillTrustGate            // skill execution trust gate and score feedback
	cognitiveContext CognitiveContextFunc       // CognitivePlugin dynamic context injector
	beliefContext    BeliefContextFunc          // Cognition SDK belief context injector
	ledger           *ldg.Ledger                // Ledger instance for ReAct/Reasoning/Eval
	runState         RunStateAccessor           // per-session interrupt checking (nil = no interrupt support)
	fedBridge        FederationBridge           // OPP federation bridge for A2A delegation (nil = disabled)
	cogniService     *CogniContextService       // declarative Cogni activation/context boundary
	learningSidecar  *LearningSidecar           // post-run learning and metacognition side effects
	skillRuntime     *SkillRuntimeService       // skill surface scoring/recommendation/growth boundary
	proactiveCog     *ProactiveCognitionService // proactive cognition and Reverie event boundary
	runtimeStrategy  *RuntimeStrategyService    // mode switches, local brain, and provider routing
}

// CogniContextFunc returns the assembled Cogni context block for the current
// turn — empty string when no cogni activates. Implementations are expected
// to be cheap (just rule evaluation + string concat); heavy work belongs in
// the underlying Registry or its hook.
type CogniContextFunc func(ctx context.Context, message, tenantID, channel string) string

// BeliefContextFunc returns the assembled Cognition SDK context block for the
// current turn — empty string when no pack activates.
type BeliefContextFunc func(ctx context.Context, message, tenantID, channel string) string

// CogniSkillFilterFunc narrows the candidate skill list to the union of
// every activated cogni's ToolSurface. Implementations MUST return the
// input unchanged when no cogni activates — the planner relies on this
// "no-op default" so disabling cogni keeps existing behavior intact.
type CogniSkillFilterFunc func(message, tenantID, channel string, in []skills.Skill) []skills.Skill

// CogniTraceFunc returns the latest declarative Cogni decision snapshot for
// the current turn. It lets the planner surface "which Cogni activated and how
// it changed the tool surface" in the same SSE execution trace as normal
// thinking/tool events, instead of hiding it only behind admin endpoints.
type CogniTraceFunc func(message, tenantID, channel string) (CogniTraceDetail, bool)

type MemorySearchFunc func(ctx context.Context, tenantID, query string) string

type ReflectFunc func(ctx context.Context, intent, reply string) bool

func (p *Planner) SetMemory(fn MemorySearchFunc) { p.memory = fn }

func (p *Planner) SetReflect(fn ReflectFunc) { p.reflect = fn }

// SetMetaCogBridge attaches the metacognitive anomaly bridge.
// When set, the planner checks for reasoning anomalies (loops, stalls,
// drift) and injects correction hints into subsequent prompts.
func (p *Planner) SetMetaCogBridge(b *iledger.MetaCogBridge) {
	p.ensureLearningSidecar().SetMetaCogBridge(b)
}

func (p *Planner) SetPersonaPrompt(fn func() string) { p.personaPrompt = fn }

// SetGraphContext: fn(query) returns relevant entities/relations as text.
func (p *Planner) SetGraphContext(fn func(query string) string) { p.graphContext = fn }

// GraphContext returns the current graphContext callback (may be nil).
func (p *Planner) GraphContext() func(query string) string { return p.graphContext }

// SetBrowser is a no-op kept for backward compatibility — browser skills are now in the skill registry.

// SetCodeContext: fn(query) returns formatted code snippets from repo-type knowledge.
func (p *Planner) SetCodeContext(fn func(query string) string) { p.codeContext = fn }

func (p *Planner) SetStateContext(fn func() string) { p.stateContext = fn }

func (p *Planner) SetStrategyContext(fn func() string) { p.strategyContext = fn }

// SetStrategyContextFor attaches a query-aware reflection strategy provider.
// The legacy SetStrategyContext callback remains as a fallback for callers that
// cannot cheaply scope strategy context to the current user message.
func (p *Planner) SetStrategyContextFor(fn func(query string) string) { p.strategyContextFor = fn }

// DynContextBudgetDefault signals "use the built-in default" (currently 4000 tokens).
// Callers should use this constant instead of bare 0 to express intent clearly.
const DynContextBudgetDefault = 0

func (p *Planner) SetDynContextBudget(tokens int) { p.dynContextBudget = tokens }

func (p *Planner) SetDomainPrompt(prompt string) { p.domainPrompt = prompt }

func (p *Planner) SetNativeFC(enabled bool) {
	p.useNativeFC = enabled
	p.InvalidatePromptCache()
}

func (p *Planner) SetWindowConfig(cfg ctxwindow.WindowConfig) { p.windowCfg = &cfg }

func (p *Planner) SetContextManager(mgr *ctxwindow.Manager) { p.ctxManager = mgr }

func (p *Planner) SetSkillMetrics(fn SkillMetricsFunc) { p.skillMetrics = fn }

// SetSkillScorer sets the Ledger-derived skill scoring data for intent-based routing.
func (p *Planner) SetSkillScorer(scorer *skills.SkillScorer) {
	p.ensureSkillRuntime().SetScorer(scorer)
}

// SetSkillRecommendationEngine attaches the experience-distilled recommender
// used to rank the current planner skill surface. The recommender is seeded
// with the registry's current skills immediately and re-synced when the
// registry version changes.
func (p *Planner) SetSkillRecommendationEngine(engine *recommend.Engine) {
	p.ensureSkillRuntime().SetRecommendationEngine(engine)
}

// SetSkillIndex provides the L2 index: skills listed by name+description in the prompt,
// loaded on demand via use_skill(slug).
func (p *Planner) SetSkillIndex(fn SkillIndexFunc) { p.skillIndex = fn }

// SetLLMPool attaches a multi-model LLM pool for dynamic model selection.
// When set, ModelOverride in PlanRequest selects a pool client by key.
func (p *Planner) SetLLMPool(pool *llm.Pool) { p.llmPool = pool }

// SetHandoffRegistry attaches a handoff registry for subagent delegation.
// Handoff tools (transfer_to_*) are automatically added to LLM function definitions.
func (p *Planner) SetHandoffRegistry(hr *subagent.HandoffRegistry) { p.handoffReg = hr }

// SetSkillOptimizer attaches a skill optimizer for usage analytics-driven hints.
func (p *Planner) SetSkillOptimizer(opt *SkillOptimizer) {
	p.ensureSkillRuntime().SetOptimizer(opt)
}

// SetReverie attaches the background inner monologue system.
func (p *Planner) SetReverie(r *Reverie) {
	p.ensureProactiveCognition().SetReverie(r)
}

// SetTaskFailureMonitor attaches a monitor that emits Reverie events on skill failure spikes.
func (p *Planner) SetTaskFailureMonitor(m *TaskFailureMonitor) {
	p.ensureProactiveCognition().SetTaskFailureMonitor(m)
}

// SetTrustRecord attaches a callback called after each skill execution to update trust scores.
func (p *Planner) SetTrustRecord(fn func(skillName string, success bool)) {
	p.ensureTrustGate().SetRecord(fn)
}

// SetTrustCheck attaches a gate called before each skill execution.
func (p *Planner) SetTrustCheck(fn func(skillName string) error) {
	p.ensureTrustGate().SetCheck(fn)
}

// SetCognitiveContext attaches the CognitivePlugin dynamic context collector.
func (p *Planner) SetCognitiveContext(fn CognitiveContextFunc) { p.cognitiveContext = fn }

// SetBeliefContext attaches the Cognition SDK dynamic context collector.
func (p *Planner) SetBeliefContext(fn BeliefContextFunc) { p.beliefContext = fn }

// SetLedger attaches a Ledger instance for ReAct mode, reasoning traces, and self-evaluation.
func (p *Planner) SetLedger(l *ldg.Ledger) { p.ledger = l }

// SetReActMode enables the Ledger-powered ReAct reasoning loop.
func (p *Planner) SetReActMode(enabled bool) {
	p.ensureRuntimeStrategy().SetReActMode(enabled)
}

// SetLongHorizonMode enables the DAG-based long-horizon planner for complex tasks.
func (p *Planner) SetLongHorizonMode(enabled bool) {
	p.ensureRuntimeStrategy().SetLongHorizonMode(enabled)
}

// SetRunStateAccessor attaches the per-session interrupt checking function.
func (p *Planner) SetRunStateAccessor(fn RunStateAccessor) { p.runState = fn }

// SetLocalBrain attaches the local small model decision layer.
func (p *Planner) SetLocalBrain(lb *localbrain.LocalBrain) {
	p.ensureRuntimeStrategy().SetLocalBrain(lb)
}

// SetAgenticThinking attaches the agentic thinking engine.
func (p *Planner) SetAgenticThinking(at *localbrain.AgenticThinking) {
	p.ensureRuntimeStrategy().SetAgenticThinking(at)
}

// SetSkillGrowth attaches the autonomous skill acquisition module.
func (p *Planner) SetSkillGrowth(sg *SkillGrowth) {
	p.ensureSkillRuntime().SetGrowth(sg)
}

// SetDataCollector attaches the training data collector for LoRA pipeline.
func (p *Planner) SetDataCollector(dc *DataCollector) {
	p.ensureLearningSidecar().SetDataCollector(dc)
}

// SetProviderRegistry attaches the capability-aware provider registry for dynamic model routing.
func (p *Planner) SetProviderRegistry(reg *llm.ProviderRegistry) {
	p.ensureRuntimeStrategy().SetProviderRegistry(reg)
}

// SetCogniContext attaches a declarative Cogni context injector. The callback
// is invoked once per turn from the prompt builder; nil disables the layer.
func (p *Planner) SetCogniContext(fn CogniContextFunc) {
	p.ensureCogniService().SetContext(fn)
}

// SetCogniSkillFilter attaches a declarative Cogni surface filter. The
// callback is invoked from buildFunctionDefs to narrow the tool list to
// the union of every activated cogni's ToolSurface; nil keeps the full
// skill set.
func (p *Planner) SetCogniSkillFilter(fn CogniSkillFilterFunc) {
	p.ensureCogniService().SetSkillFilter(fn)
}

// SetCogniTrace attaches a declarative Cogni observability callback. Nil keeps
// Cogni internal-only and preserves prior behaviour.
func (p *Planner) SetCogniTrace(fn CogniTraceFunc) {
	p.ensureCogniService().SetTrace(fn)
}

func (p *Planner) ensureCogniService() *CogniContextService {
	if p.cogniService == nil {
		p.cogniService = NewCogniContextService()
	}
	return p.cogniService
}

func (p *Planner) ensureLearningSidecar() *LearningSidecar {
	if p.learningSidecar == nil {
		p.learningSidecar = NewLearningSidecar()
	}
	return p.learningSidecar
}

func (p *Planner) ensureSkillRuntime() *SkillRuntimeService {
	if p.skillRuntime == nil {
		p.skillRuntime = NewSkillRuntimeService(p.registry)
	}
	return p.skillRuntime
}

func (p *Planner) ensureTrustGate() *SkillTrustGate {
	if p.trustGate == nil {
		p.trustGate = NewSkillTrustGate()
	}
	return p.trustGate
}

func (p *Planner) ensureProactiveCognition() *ProactiveCognitionService {
	if p.proactiveCog == nil {
		p.proactiveCog = NewProactiveCognitionService()
	}
	return p.proactiveCog
}

func (p *Planner) ensureRuntimeStrategy() *RuntimeStrategyService {
	if p.runtimeStrategy == nil {
		p.runtimeStrategy = NewRuntimeStrategyService()
	}
	return p.runtimeStrategy
}

// LocalBrain returns the attached local brain (may be nil).
func (p *Planner) LocalBrain() *localbrain.LocalBrain {
	if p.runtimeStrategy == nil {
		return nil
	}
	return p.runtimeStrategy.LocalBrain()
}

// LLMPool returns the attached LLM pool (may be nil).
func (p *Planner) LLMPool() *llm.Pool { return p.llmPool }

// LLMClient returns the underlying LLM client (for streaming).
func (p *Planner) LLMClient() *llm.Client { return p.llm }

// LLMClientFor returns the appropriate LLM client for a request.
// Uses ModelOverride to select from pool, falling back to the default client.
func (p *Planner) LLMClientFor(modelOverride string) *llm.Client {
	if modelOverride != "" && p.llmPool != nil {
		if c := p.llmPool.GetOrFallback(modelOverride); c != nil {
			return c
		}
	}
	return p.llm
}

// clientForRequest returns the LLM client to use for a given request.
// Priority: ClientOverride > ModelOverride (pool) > default client.
func (p *Planner) clientForRequest(req PlanRequest) *llm.Client {
	if req.ClientOverride != nil {
		return req.ClientOverride
	}
	return p.LLMClientFor(req.ModelOverride)
}

// LLMBreaker returns the circuit breaker for LLM health inspection.
func (p *Planner) LLMBreaker() *llm.CircuitBreaker { return p.llm.Breaker() }

// buildEnv constructs a skill Environment with LLM call capability.
// LLMCall honors the request's session-level provider override
// (clientForRequest) so skills invoked during planning respect the same
// provider/model the user selected for this conversation.
func (p *Planner) buildEnv(req PlanRequest) *skills.Environment {
	return &skills.Environment{
		ClassID:   req.ClassID,
		TeacherID: req.TeacherID,
		StudentID: req.StudentID,
		TenantID:  req.TenantID,
		LLMCall: func(ctx context.Context, system, user string) (string, error) {
			msgs := []llm.Message{
				{Role: "system", Content: system},
				{Role: "user", Content: user},
			}
			client := p.clientForRequest(req)
			if client == nil {
				client = p.llm
			}
			return client.Chat(ctx, msgs, 0.7)
		},
		MemorySearch: func(ctx context.Context, tenantID, query string, topK int) (string, error) {
			if p.memory != nil {
				return p.memory(ctx, tenantID, query), nil
			}
			return "", nil
		},
	}
}

// BuildMessages constructs the full message list using Manus-style context engineering.
//
// Layout: [stable_prefix] [dynamic_context?] [history...] [goal_recitation?] [last_user_msg+timestamp]
//
// Key principles:
//   - Stable prefix (persona+skills+domain) is a single system message �?enables LLM KV-cache reuse
//   - Dynamic context (memory+graph) is a SEPARATE system message �?prefix cache survives per-query changes
//   - Timestamp injected into last user message, NOT system prompt �?avoids cache invalidation
//   - Goal recitation inserted before last user message in multi-turn �?keeps model focused
//   - Errors preserved (append-only context) �?model learns from failures
func (p *Planner) BuildMessages(ctx context.Context, req PlanRequest) ([]llm.Message, []string) {
	// ── 1. Stable prefix: persona (highest priority) + base + domain ──
	var stablePrefix string

	if p.personaPrompt != nil {
		if pp := p.personaPrompt(); pp != "" {
			stablePrefix = pp + "\n\n"
		}
	}

	if req.DisableDelegation {
		stablePrefix += p.buildSubagentSystemPrompt()
	} else {
		stablePrefix += p.buildSystemPrompt()
	}
	if p.domainPrompt != "" {
		stablePrefix += "\n\n" + p.domainPrompt
	}
	if req.GroupSystemPrompt != "" {
		stablePrefix += "\n\n" + req.GroupSystemPrompt
	}
	msgs := []llm.Message{{Role: "system", Content: stablePrefix}}

	// ── 2. Dynamic context: memory + graph (per-query, separate message to preserve prefix cache) ──
	var includedLayers []string
	if len(req.Messages) > 0 {
		pb := NewPromptBuilder(p)
		assembled := pb.BuildDynamicContext(ctx, DynamicContextRequest{
			LastMessage: req.Messages[len(req.Messages)-1].Content,
			TenantID:    req.TenantID,
			Channel:     req.ChannelType,
			TaskContext: req.TaskContext,
			EmotionHint: req.EmotionHint,
		})
		includedLayers = pb.LastIncludedLayers
		if assembled != "" {
			msgs = append(msgs, llm.Message{
				Role:    "system",
				Content: "[动态上下文]\n" + assembled,
			})
		}
	}

	// ── 3. Conversation messages: timestamp + goal recitation ──
	if len(req.Messages) > 0 {
		convMsgs := make([]llm.Message, len(req.Messages))
		copy(convMsgs, req.Messages)

		// Inject timestamp into the last user message (avoids system prompt cache invalidation)
		for i := len(convMsgs) - 1; i >= 0; i-- {
			if convMsgs[i].Role == "user" {
				ts := fmt.Sprintf("[时间: %s]\n", time.Now().Format("2006-01-02 15:04"))
				if len(convMsgs[i].ContentParts) > 0 {
					// Multimodal message: prepend timestamp to first text part, preserve all parts
					updated := convMsgs[i]
					parts := make([]llm.ContentPart, len(updated.ContentParts))
					copy(parts, updated.ContentParts)
					prefixed := false
					for j := range parts {
						if parts[j].Type == "text" && !prefixed {
							parts[j].Text = ts + parts[j].Text
							prefixed = true
						}
					}
					if !prefixed {
						parts = append([]llm.ContentPart{{Type: "text", Text: ts}}, parts...)
					}
					updated.ContentParts = parts
					updated.Content = ts + updated.Content
					convMsgs[i] = updated
				} else {
					convMsgs[i] = llm.Message{
						Role:    "user",
						Content: ts + convMsgs[i].Content,
					}
				}
				break
			}
		}

		// Goal recitation: in multi-turn (>2 messages), remind model of initial user goal
		if len(convMsgs) > 2 {
			var firstGoal string
			for _, m := range convMsgs {
				if m.Role == "user" {
					firstGoal = m.Content
					break
				}
			}
			if firstGoal != "" {
				goalRunes := []rune(firstGoal)
				if len(goalRunes) > 100 {
					firstGoal = string(goalRunes[:100]) + "..."
				}
				// Insert goal recitation before the last message
				last := convMsgs[len(convMsgs)-1]
				convMsgs = append(convMsgs[:len(convMsgs)-1],
					llm.Message{Role: "system", Content: "[任务焦点] 用户的核心目�? " + firstGoal},
					last,
				)
			}
		}

		msgs = append(msgs, convMsgs...)
	}

	// ── 4. Pre-compress tool results ──
	for i := range msgs {
		if msgs[i].Role == "tool" && len(msgs[i].Content) > 6000 {
			msgs[i].Content = ctxwindow.PruneToolOutput(msgs[i].Content, 6000)
		}
	}

	// ── 5. Context compression + window trimming ──

	// Multi-stage compression (enforce turns �?LLM summary �?emergency halve)
	if p.ctxManager != nil {
		winMsgs := make([]ctxwindow.Message, len(msgs))
		for i, m := range msgs {
			winMsgs[i] = ctxwindow.Message{Role: m.Role, Content: m.Content}
		}
		compressed, err := p.ctxManager.Process(ctx, winMsgs)
		if err != nil {
			slog.Warn("context compression failed, falling back to window trim", "err", err)
		} else if len(compressed) < len(msgs) {
			slog.Info("context compressed", "before", len(msgs), "after", len(compressed))
			trimmed := make([]llm.Message, len(compressed))
			for i, m := range compressed {
				trimmed[i] = llm.Message{Role: m.Role, Content: m.Content}
			}
			msgs = trimmed
		}
	}

	// Window trimming (hard token/message cap)
	// Use model's actual context window if available, otherwise fall back to default config
	windowCfg := p.windowCfg
	if client := p.clientForRequest(req); client != nil {
		modelTokens := client.ContextWindowTokens()
		if windowCfg == nil || modelTokens != windowCfg.MaxTokens {
			cfg := ctxwindow.ConfigForWindow(modelTokens / 1024)
			windowCfg = &cfg
		}
	}
	if windowCfg != nil {
		winMsgs := make([]ctxwindow.Message, len(msgs))
		for i, m := range msgs {
			winMsgs[i] = ctxwindow.Message{Role: m.Role, Content: m.Content}
		}
		result := ctxwindow.TrimToFit(winMsgs, *windowCfg)
		if result.DroppedCount > 0 {
			slog.Info("context window trimmed", "dropped", result.DroppedCount, "remaining", len(result.Messages), "model_window_k", windowCfg.MaxTokens/1024)
			trimmed := make([]llm.Message, len(result.Messages))
			for i, m := range result.Messages {
				trimmed[i] = llm.Message{Role: m.Role, Content: m.Content}
			}
			return trimmed, includedLayers
		}
	}
	return msgs, includedLayers
}

// NewPlanner creates a planner with the given LLM client and skill registry.
func NewPlanner(llmClient *llm.Client, registry *skills.Registry, maxSteps int) *Planner {
	if maxSteps <= 0 {
		maxSteps = 15
	}
	return &Planner{llm: llmClient, registry: registry, maxSteps: maxSteps, toolTimeout: 60 * time.Second}
}

// SetToolTimeout sets the per-tool execution timeout. Default is 60s.
func (p *Planner) SetToolTimeout(d time.Duration) { p.toolTimeout = d }

// safeToolGo runs fn in a goroutine with panic recovery and a timeout derived from ctx.
// If fn panics, it sends an error result on resultsCh. If the context deadline is exceeded,
// the tool's context is cancelled (the tool must respect ctx.Done()).
func safeToolGo(ctx context.Context, timeout time.Duration, fn func(ctx context.Context)) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("planner: tool goroutine panic", "panic", r)
			}
		}()
		if timeout <= 0 {
			fn(ctx)
			return
		}
		toolCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		fn(toolCtx)
	}()
}

// PlanRequest is the input to the planner.
type PlanRequest struct {
	Messages          []llm.Message
	ClassID           string
	TeacherID         string
	StudentID         string
	TenantID          string
	ModelOverride     string          // pool key (e.g. "fast","smart","expert") to override default model
	EmotionHint       *emotion.Result // optional emotion detected from user input (STT or text analysis)
	TaskID            string          // if set, this request is part of a task thread
	TaskContext       string          // pre-rendered task working memory (injected by gateway)
	IsGroup           bool            // true if this request comes from a group chat
	GroupSystemPrompt string          // extra system prompt for group context
	ChannelType       string          // source channel type (e.g. "telegram", "feishu")
	ChatType          string          // chat type ("group", "private", etc.)
	InboxContext      string          // buffered group inbox messages for context
	StepCallback      StepCallback    // optional: called for each intermediate step (thinking, tool call, etc.)
	TraceID           string          // trace context ID for unified event protocol
	ThinkingEnabled   *bool           // nil = model default; true/false = explicit override
	DisableDelegation bool            // when true, buildFunctionDefs exposes direct skills instead of handoff tools
	DisableTools      bool            // when true, skip all tools — pure chat mode
	ClientOverride    *llm.Client     // if set, bypass pool and use this client directly (session-level provider override)
	AllowedSkills     []string        // if non-empty, buildFunctionDefs restricts to exactly these skill names (user-picked tool whitelist)
}

// StepEventType classifies the kind of intermediate step event.
type StepEventType string

const (
	StepEventThinking   StepEventType = "thinking"    // agent is reasoning
	StepEventToolStart  StepEventType = "tool_start"  // about to call a skill
	StepEventToolResult StepEventType = "tool_result" // skill returned
	StepEventReflect    StepEventType = "reflect"     // self-reflection
	StepEventPlan       StepEventType = "plan"        // decomposed plan
)

// StepEvent is an intermediate step notification during planning.
type StepEvent struct {
	Type      StepEventType  `json:"type"`
	Step      int            `json:"step"`
	Message   string         `json:"message"` // human-readable description
	SkillName string         `json:"skill_name,omitempty"`
	Args      map[string]any `json:"args,omitempty"`
	Result    string         `json:"result,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// StepCallback is called for each intermediate step during planning.
// Uses the unified AgentEvent protocol from observe package.
// If nil, no intermediate notifications are sent.
type StepCallback func(event observe.AgentEvent)

type ctxKeyStepCB struct{}

// WithStepCallback attaches a StepCallback to the context so exec agents can emit SSE events.
func WithStepCallback(ctx context.Context, cb StepCallback) context.Context {
	return context.WithValue(ctx, ctxKeyStepCB{}, cb)
}

// StepCallbackFromCtx retrieves the StepCallback attached to the context.
func StepCallbackFromCtx(ctx context.Context) StepCallback {
	cb, _ := ctx.Value(ctxKeyStepCB{}).(StepCallback)
	return cb
}

// StepStatus tracks the state of a plan step.
type StepStatus string

const (
	StepPending StepStatus = "pending"
	StepRunning StepStatus = "running"
	StepDone    StepStatus = "done"
	StepFailed  StepStatus = "failed"
	StepSkipped StepStatus = "skipped"
)

// PlanStep represents one step in a multi-step plan.
type PlanStep struct {
	ID        int            `json:"id"`
	Action    string         `json:"action"` // what to do
	Skill     string         `json:"skill"`  // skill to call (empty = LLM reasoning)
	Args      map[string]any `json:"args,omitempty"`
	DependsOn []int          `json:"depends_on"` // IDs of steps this depends on
	Status    StepStatus     `json:"status"`
	Result    string         `json:"result,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// PlanResult is the output of the planner.
type PlanResult struct {
	Reply            string        `json:"reply"`
	ReasoningContent string        `json:"reasoning_content,omitempty"`
	Actions          []AgentAction `json:"actions,omitempty"`
	SkillsUsed       []string      `json:"skills_used"`
	Steps            int           `json:"steps"`
	Plan             []PlanStep    `json:"plan,omitempty"`
	ContextLayers    []string      `json:"context_layers,omitempty"`
	Suggestions      []string      `json:"suggestions,omitempty"`
}

// ExecutionSummary builds a concise summary of skill executions for session persistence.
// This allows the next conversation turn to see what tools were called and their results,
// enabling multi-turn task continuity.
// Returns empty string if no skills were used.
func (r *PlanResult) ExecutionSummary() string {
	if r == nil || len(r.Plan) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("[执行记录] ")
	for i, step := range r.Plan {
		if i > 0 {
			b.WriteString(" → ")
		}
		if step.Status == StepFailed {
			b.WriteString(fmt.Sprintf("%s(失败: %s)", step.Skill, truncate(step.Error, 80)))
		} else {
			b.WriteString(fmt.Sprintf("%s(✓ %s)", step.Skill, truncate(step.Result, 120)))
		}
	}
	return b.String()
}

// Run executes the planning loop: understand → skill calls → synthesize.
func (p *Planner) Run(ctx context.Context, req PlanRequest) (*PlanResult, error) {
	ctx, span := observe.StartSpan(ctx, "planner.Run")
	span.Attrs["tenant_id"] = req.TenantID
	span.Attrs["mode"] = "text-based"
	if p.shouldUseLongHorizon(req) {
		span.Attrs["mode"] = "long-horizon"
	} else if p.runtimeStrategy != nil && p.runtimeStrategy.ReActMode() && p.ledger != nil {
		span.Attrs["mode"] = "react"
	} else if p.useNativeFC {
		span.Attrs["mode"] = "native-fc"
	}
	result, err := p.runInner(ctx, req)
	observe.EndSpan(span, err)

	if p.learningSidecar != nil {
		p.learningSidecar.AfterRun(ctx, req, result, err, p.reflect)
	}

	return result, err
}

func (p *Planner) runInner(ctx context.Context, req PlanRequest) (*PlanResult, error) {
	if req.ModelOverride != "" {
		slog.Debug("planner: model override", "override", req.ModelOverride)
	}

	if req.DisableTools {
		slog.Info("planner: chat-only mode, skipping all tools")
		messages, layers := p.BuildMessages(ctx, req)
		p.maybeEmitCogniTrace(req)
		reply, err := p.chatFallback(ctx, req, messages)
		if err != nil {
			return nil, fmt.Errorf("planner chat-only: %w", err)
		}
		cleaned := p.cleanReply(reply)
		cleaned, nextMoves := extractNextMoves(cleaned)
		return &PlanResult{Reply: cleaned, Steps: 0, ContextLayers: layers, Suggestions: nextMoves}, nil
	}

	// LocalBrain 预分类：用本地小模型决定路由（省 API token）
	var lbNoTools bool
	if p.runtimeStrategy != nil && p.runtimeStrategy.LocalBrain() != nil && req.ModelOverride == "" && !req.DisableDelegation {
		query := extractGoal(req)
		if decision, err := p.runtimeStrategy.Classify(ctx, query, req.TenantID); err == nil {
			slog.Info("planner: localbrain decision", "handler", decision.Handler, "intent", decision.Intent.Category, "need_tools", decision.Intent.NeedTools, "reason", decision.Reason)
			if decision.Handler != "local" {
				req.ModelOverride = decision.Handler
			}
			if !decision.Intent.NeedTools {
				lbNoTools = true
			}
			if p.ledger != nil {
				tracer := p.ledger.Reasoning(req.TaskID, "localbrain")
				tracer.Decide(ctx, decision.Handler, decision.Reason, decision.Intent.Confidence, map[string]interface{}{
					"category":   decision.Intent.Category,
					"complexity": decision.Intent.Complexity,
					"need_tools": decision.Intent.NeedTools,
				})
			}
		}
	}

	// Fast-path: LocalBrain determined no tools needed → pure chat, skip all tool-enabled engines.
	if lbNoTools {
		slog.Info("planner: NeedTools=false, using tool-free chat path")
		messages, layers := p.BuildMessages(ctx, req)
		p.maybeEmitCogniTrace(req)
		reply, err := p.chatFallback(ctx, req, messages)
		if err != nil {
			return nil, fmt.Errorf("planner tool-free chat: %w", err)
		}
		cleaned := p.cleanReply(reply)
		cleaned, nextMoves := extractNextMoves(cleaned)
		return &PlanResult{Reply: cleaned, Steps: 1, ContextLayers: layers, Suggestions: nextMoves}, nil
	}

	if p.shouldUseLongHorizon(req) {
		p.emitCognitiveLoadEvent(req, assessCognitiveLoad(req))
		return p.runLongHorizon(ctx, req)
	}
	if p.runtimeStrategy != nil && p.runtimeStrategy.ReActMode() && p.ledger != nil {
		return p.runReAct(ctx, req)
	}
	if p.useNativeFC {
		return p.runNativeFC(ctx, req)
	}
	return p.runTextBased(ctx, req)
}

// isComplexTask heuristically determines if a request needs DAG planning.
// Execution engines are split into separate files:
//   - executor_fc.go:     runNativeFC(), buildFunctionDefs()
//   - executor_text.go:   runTextBased(), parseSkillCalls()
//   - prompt.go:          buildSystemPrompt(), cleanReply(), truncate(), findClosingBrace()
//   - planner_routing.go: routing, fallback chains, auto-thinking
