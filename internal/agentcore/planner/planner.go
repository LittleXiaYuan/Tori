package planner

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	ldg "ledger"

	ctxwindow "yunque-agent/internal/agentcore/context"
	"yunque-agent/internal/agentcore/emotion"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/plan"
	"yunque-agent/internal/agentcore/subagent"
	"yunque-agent/internal/execution/browser"
	"yunque-agent/internal/observe"
	"yunque-agent/pkg/skills"
)

// SkillMetricsFunc records a skill call's duration and error.
type SkillMetricsFunc func(skillName string, duration time.Duration, err error)

// SkillIndexEntry is a lightweight descriptor for the L2 skill index (name + description only).
type SkillIndexEntry struct {
	Slug        string
	Description string
}

// SkillIndexFunc returns the current list of installed skills for the L2 index.
type SkillIndexFunc func() []SkillIndexEntry

// Planner uses LLM to understand intent, decompose tasks, and orchestrate skills.
type Planner struct {
	llm              *llm.Client
	llmPool          *llm.Pool // multi-model pool (nil = single model mode)
	registry         *skills.Registry
	toolTimeout      time.Duration // per-tool execution timeout (default 60s)
	maxSteps         int
	memory           MemorySearchFunc
	reflect          ReflectFunc
	skillMetrics     SkillMetricsFunc
	domainPrompt     string
	personaPrompt    func() string                        // dynamic persona system prompt
	graphContext     func(query string) string            // knowledge graph context injector
	codeContext      func(query string) string            // code knowledge context injector (repo-level)
	useNativeFC      bool                                 // use native LLM function calling
	windowCfg        *ctxwindow.WindowConfig              // context window trimming config
	ctxManager       *ctxwindow.Manager                   // multi-stage context compression manager
	cachedSysPrompt  string                               // cached base system prompt
	sysPromptVer     int                                  // incremented when skills change
	skillIndex       SkillIndexFunc                       // L2 installed skill index (nil = no L2)
	handoffReg       *subagent.HandoffRegistry            // handoff tool registry for subagent delegation
	skillOptimizer   *SkillOptimizer                      // skill usage analytics and optimization hints
	reverie          *Reverie                             // background inner monologue system
	taskFailureMon   *TaskFailureMonitor                  // event-driven trigger on skill failure spikes
	stateContext     func() string                        // structured state kernel context
	strategyContext  func() string                        // reflection loop strategy context
	dynContextBudget int                                  // max tokens for dynamic context layer assembly (0 = unlimited)
	ackEnabled       bool                                 // send typing indicators / ack
	locale           string                               // agent locale (e.g. "zh-CN")
	browserDispatch  *browser.Dispatcher                  // browser tool dispatcher (nil = disabled)
	trustRecord      func(skillName string, success bool) // trust score recorder (nil = disabled)
	trustCheck       func(skillName string) error         // trust gate: returns non-nil to block skill
	cognitiveContext CognitiveContextFunc                 // CognitivePlugin dynamic context injector
	ledger           *ldg.Ledger                          // Ledger instance for ReAct/Reasoning/Eval
	reactMode        bool                                 // if true, use ReAct mode instead of basic FC loop
	longHorizonMode  bool                                 // if true, use DAG planner for complex multi-step tasks
	runState         RunStateAccessor                     // per-session interrupt checking (nil = no interrupt support)
}

// MemorySearchFunc searches memory and returns context string.
type MemorySearchFunc func(ctx context.Context, tenantID, query string) string

// ReflectFunc evaluates result quality, returns true if satisfied.
type ReflectFunc func(ctx context.Context, intent, reply string) bool

// SetMemory attaches a memory search function.
func (p *Planner) SetMemory(fn MemorySearchFunc) { p.memory = fn }

// SetReflect attaches a reflection function.
func (p *Planner) SetReflect(fn ReflectFunc) { p.reflect = fn }

// SetPersonaPrompt attaches a dynamic persona prompt function (from Persona.SystemPrompt).
func (p *Planner) SetPersonaPrompt(fn func() string) { p.personaPrompt = fn }

// SetGraphContext attaches a knowledge graph context function.
// It receives the user query and returns relevant entity/relation context.
func (p *Planner) SetGraphContext(fn func(query string) string) { p.graphContext = fn }

// SetBrowser attaches a browser tool dispatcher.
func (p *Planner) SetBrowser(d *browser.Dispatcher) { p.browserDispatch = d }

// SetCodeContext attaches a code knowledge context function.
// It searches repo-type knowledge sources and returns formatted code snippets.
func (p *Planner) SetCodeContext(fn func(query string) string) { p.codeContext = fn }

// SetStateContext attaches a state kernel context function.
func (p *Planner) SetStateContext(fn func() string) { p.stateContext = fn }

// SetStrategyContext attaches a reflection strategy context function.
func (p *Planner) SetStrategyContext(fn func() string) { p.strategyContext = fn }

// SetDynContextBudget sets the max token budget for dynamic context layers (0 = unlimited).
func (p *Planner) SetDynContextBudget(tokens int) { p.dynContextBudget = tokens }

// SetDomainPrompt sets additional domain-specific system prompt from plugins.
func (p *Planner) SetDomainPrompt(prompt string) { p.domainPrompt = prompt }

// SetNativeFC enables native LLM function calling instead of text-based parsing.
func (p *Planner) SetNativeFC(enabled bool) { p.useNativeFC = enabled }

// SetWindowConfig sets context window trimming config.
func (p *Planner) SetWindowConfig(cfg ctxwindow.WindowConfig) { p.windowCfg = &cfg }

// SetContextManager sets the multi-stage context compression manager.
func (p *Planner) SetContextManager(mgr *ctxwindow.Manager) { p.ctxManager = mgr }

// SetSkillMetrics attaches a function to record skill call metrics.
func (p *Planner) SetSkillMetrics(fn SkillMetricsFunc) { p.skillMetrics = fn }

// SetSkillIndex attaches a function that provides the L2 installed skill index.
// These skills appear as name+description in the prompt; the model uses use_skill(slug) to load full details.
func (p *Planner) SetSkillIndex(fn SkillIndexFunc) { p.skillIndex = fn }

// SetLLMPool attaches a multi-model LLM pool for dynamic model selection.
// When set, ModelOverride in PlanRequest selects a pool client by key.
func (p *Planner) SetLLMPool(pool *llm.Pool) { p.llmPool = pool }

// SetHandoffRegistry attaches a handoff registry for subagent delegation.
// Handoff tools (transfer_to_*) are automatically added to LLM function definitions.
func (p *Planner) SetHandoffRegistry(hr *subagent.HandoffRegistry) { p.handoffReg = hr }

// SetSkillOptimizer attaches a skill optimizer for usage analytics-driven hints.
func (p *Planner) SetSkillOptimizer(opt *SkillOptimizer) { p.skillOptimizer = opt }

// SetReverie attaches the background inner monologue system.
func (p *Planner) SetReverie(r *Reverie) { p.reverie = r }

// SetTaskFailureMonitor attaches a monitor that emits Reverie events on skill failure spikes.
func (p *Planner) SetTaskFailureMonitor(m *TaskFailureMonitor) { p.taskFailureMon = m }

// SetTrustRecord attaches a callback called after each skill execution to update trust scores.
func (p *Planner) SetTrustRecord(fn func(skillName string, success bool)) { p.trustRecord = fn }

// SetTrustCheck attaches a gate called before each skill execution.
func (p *Planner) SetTrustCheck(fn func(skillName string) error) { p.trustCheck = fn }

// SetCognitiveContext attaches the CognitivePlugin dynamic context collector.
func (p *Planner) SetCognitiveContext(fn CognitiveContextFunc) { p.cognitiveContext = fn }

// SetLedger attaches a Ledger instance for ReAct mode, reasoning traces, and self-evaluation.
func (p *Planner) SetLedger(l *ldg.Ledger) { p.ledger = l }

// SetReActMode enables the Ledger-powered ReAct reasoning loop.
func (p *Planner) SetReActMode(enabled bool) { p.reactMode = enabled }

// SetLongHorizonMode enables the DAG-based long-horizon planner for complex tasks.
func (p *Planner) SetLongHorizonMode(enabled bool) { p.longHorizonMode = enabled }

// SetRunStateAccessor attaches the per-session interrupt checking function.
func (p *Planner) SetRunStateAccessor(fn RunStateAccessor) { p.runState = fn }

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

// LLMBreaker returns the circuit breaker for LLM health inspection.
func (p *Planner) LLMBreaker() *llm.CircuitBreaker { return p.llm.Breaker() }

// buildEnv constructs a skill Environment with LLM call capability.
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
			return p.llm.Chat(ctx, msgs, 0.7)
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
func (p *Planner) BuildMessages(ctx context.Context, req PlanRequest) []llm.Message {
	// ── 1. Stable prefix: base + persona + domain (rarely changes, KV-cache friendly) ──
	stablePrefix := p.buildSystemPrompt()
	if p.personaPrompt != nil {
		if pp := p.personaPrompt(); pp != "" {
			stablePrefix += "\n\n" + pp
		}
	}
	if p.domainPrompt != "" {
		stablePrefix += "\n\n" + p.domainPrompt
	}
	msgs := []llm.Message{{Role: "system", Content: stablePrefix}}

	// ── 2. Dynamic context: memory + graph (per-query, separate message to preserve prefix cache) ──
	if len(req.Messages) > 0 {
		pb := NewPromptBuilder(p)
		assembled := pb.BuildDynamicContext(ctx, DynamicContextRequest{
			LastMessage: req.Messages[len(req.Messages)-1].Content,
			TenantID:    req.TenantID,
			TaskContext: req.TaskContext,
			EmotionHint: req.EmotionHint,
		})
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
				convMsgs[i] = llm.Message{
					Role:    "user",
					Content: fmt.Sprintf("[时间: %s]\n%s", time.Now().Format("2006-01-02 15:04"), convMsgs[i].Content),
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

	// ── 4. Context compression + window trimming ──

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
	if p.windowCfg != nil {
		winMsgs := make([]ctxwindow.Message, len(msgs))
		for i, m := range msgs {
			winMsgs[i] = ctxwindow.Message{Role: m.Role, Content: m.Content}
		}
		result := ctxwindow.TrimToFit(winMsgs, *p.windowCfg)
		if result.DroppedCount > 0 {
			slog.Info("context window trimmed", "dropped", result.DroppedCount, "remaining", len(result.Messages))
			trimmed := make([]llm.Message, len(result.Messages))
			for i, m := range result.Messages {
				trimmed[i] = llm.Message{Role: m.Role, Content: m.Content}
			}
			return trimmed
		}
	}
	return msgs
}

// NewPlanner creates a planner with the given LLM client and skill registry.
func NewPlanner(llmClient *llm.Client, registry *skills.Registry, maxSteps int) *Planner {
	if maxSteps <= 0 {
		maxSteps = 8
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
	Reply      string        `json:"reply"`
	Actions    []AgentAction `json:"actions,omitempty"`
	SkillsUsed []string      `json:"skills_used"`
	Steps      int           `json:"steps"`
	Plan       []PlanStep    `json:"plan,omitempty"`
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
	if p.longHorizonMode && p.isComplexTask(req) {
		span.Attrs["mode"] = "long-horizon"
	} else if p.reactMode && p.ledger != nil {
		span.Attrs["mode"] = "react"
	} else if p.useNativeFC {
		span.Attrs["mode"] = "native-fc"
	}
	result, err := p.runInner(ctx, req)
	observe.EndSpan(span, err)
	return result, err
}

func (p *Planner) runInner(ctx context.Context, req PlanRequest) (*PlanResult, error) {
	if req.ModelOverride != "" {
		slog.Debug("planner: model override", "override", req.ModelOverride)
	}

	if p.longHorizonMode && p.isComplexTask(req) {
		return p.runLongHorizon(ctx, req)
	}
	if p.reactMode && p.ledger != nil {
		return p.runReAct(ctx, req)
	}
	if p.useNativeFC {
		return p.runNativeFC(ctx, req)
	}
	return p.runTextBased(ctx, req)
}

// isComplexTask heuristically determines if a request needs DAG planning.
func (p *Planner) isComplexTask(req PlanRequest) bool {
	goal := extractGoal(req)
	if len([]rune(goal)) > 200 {
		return true
	}
	return plan.NeedsPlan(goal)
}

// Execution engines are split into separate files:
//   - executor_fc.go:   runNativeFC(), buildFunctionDefs()
//   - executor_text.go: runTextBased(), parseSkillCalls()
//   - prompt.go:        buildSystemPrompt(), cleanReply(), truncate(), findClosingBrace()
