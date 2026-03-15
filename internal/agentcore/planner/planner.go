package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	ctxwindow "yunque-agent/internal/agentcore/context"
	"yunque-agent/internal/agentcore/emotion"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/subagent"
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
	maxSteps         int
	memory           MemorySearchFunc
	reflect          ReflectFunc
	skillMetrics     SkillMetricsFunc
	domainPrompt     string
	personaPrompt    func() string             // dynamic persona system prompt
	graphContext     func(query string) string // knowledge graph context injector
	codeContext      func(query string) string // code knowledge context injector (repo-level)
	useNativeFC      bool                      // use native LLM function calling
	windowCfg        *ctxwindow.WindowConfig   // context window trimming config
	ctxManager       *ctxwindow.Manager        // multi-stage context compression manager
	cachedSysPrompt  string                    // cached base system prompt
	sysPromptVer     int                       // incremented when skills change
	skillIndex       SkillIndexFunc            // L2 installed skill index (nil = no L2)
	handoffReg       *subagent.HandoffRegistry // handoff tool registry for subagent delegation
	skillOptimizer   *SkillOptimizer           // skill usage analytics and optimization hints
	reverie          *Reverie                  // background inner monologue system
	taskFailureMon   *TaskFailureMonitor       // event-driven trigger on skill failure spikes
	stateContext     func() string             // structured state kernel context
	strategyContext  func() string             // reflection loop strategy context
	dynContextBudget int                       // max tokens for dynamic context layer assembly (0 = unlimited)
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
//   - Stable prefix (persona+skills+domain) is a single system message → enables LLM KV-cache reuse
//   - Dynamic context (memory+graph) is a SEPARATE system message → prefix cache survives per-query changes
//   - Timestamp injected into last user message, NOT system prompt → avoids cache invalidation
//   - Goal recitation inserted before last user message in multi-turn → keeps model focused
//   - Errors preserved (append-only context) → model learns from failures
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
		lastMsg := req.Messages[len(req.Messages)-1].Content
		// Build context layers with priority-based assembly
		var layers []ctxwindow.Layer

		// P1: Task working memory (highest dynamic priority)
		if req.TaskContext != "" {
			layers = append(layers, ctxwindow.Layer{
				Name:     "task",
				Priority: ctxwindow.LayerPriorityTask,
				Content:  req.TaskContext,
			})
		}

		// P2: Memory recall
		if p.memory != nil {
			if memCtx := p.memory(ctx, req.TenantID, lastMsg); memCtx != "" {
				layers = append(layers, ctxwindow.Layer{
					Name:     "memory",
					Priority: ctxwindow.LayerPriorityMemory,
					Content:  "## 记忆上下文\n" + memCtx,
				})
			}
		}

		// P3: Retrieval — knowledge graph + code knowledge
		if p.graphContext != nil {
			if graphCtx := p.graphContext(lastMsg); graphCtx != "" {
				layers = append(layers, ctxwindow.Layer{
					Name:     "graph",
					Priority: ctxwindow.LayerPriorityRetrieval,
					Content:  "## 知识图谱\n" + graphCtx,
				})
			}
		}
		if p.codeContext != nil {
			if codeCtx := p.codeContext(lastMsg); codeCtx != "" {
				layers = append(layers, ctxwindow.Layer{
					Name:     "code",
					Priority: ctxwindow.LayerPriorityRetrieval,
					Content:  codeCtx,
				})
			}
		}

		// P4: Cognition — emotion, reverie, state, strategy
		if req.EmotionHint != nil {
			if snippet := req.EmotionHint.ContextSnippet(); snippet != "" {
				layers = append(layers, ctxwindow.Layer{
					Name:     "emotion",
					Priority: ctxwindow.LayerPriorityCognition,
					Content:  "## 情绪感知\n" + snippet,
				})
			}
		}
		if p.reverie != nil {
			if jctx := p.reverie.JournalContext(5); jctx != "" {
				layers = append(layers, ctxwindow.Layer{
					Name:     "reverie",
					Priority: ctxwindow.LayerPriorityCognition,
					Content:  "## 内心独白\n" + jctx,
				})
			}
		}
		if p.stateContext != nil {
			if sCtx := p.stateContext(); sCtx != "" {
				layers = append(layers, ctxwindow.Layer{
					Name:     "state",
					Priority: ctxwindow.LayerPriorityCognition,
					Content:  sCtx,
				})
			}
		}
		if p.strategyContext != nil {
			if strCtx := p.strategyContext(); strCtx != "" {
				layers = append(layers, ctxwindow.Layer{
					Name:     "strategy",
					Priority: ctxwindow.LayerPriorityCognition,
					Content:  "## 经验策略\n" + strCtx,
				})
			}
		}

		// P5: Hints — skill optimization
		if p.skillOptimizer != nil {
			if hints := p.skillOptimizer.OptimizationHints(); hints != "" {
				layers = append(layers, ctxwindow.Layer{
					Name:     "skill_hints",
					Priority: ctxwindow.LayerPriorityHints,
					Content:  "## 技能优化\n" + hints,
				})
			}
		}

		// Assemble layers within token budget
		assembler := ctxwindow.NewLayerAssembler(p.dynContextBudget)
		assembled, _ := assembler.Assemble(layers)
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
					llm.Message{Role: "system", Content: "[任务焦点] 用户的核心目标: " + firstGoal},
					last,
				)
			}
		}

		msgs = append(msgs, convMsgs...)
	}

	// ── 4. Context compression + window trimming ──

	// Multi-stage compression (enforce turns → LLM summary → emergency halve)
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
	return &Planner{llm: llmClient, registry: registry, maxSteps: maxSteps}
}

// PlanRequest is the input to the planner.
type PlanRequest struct {
	Messages      []llm.Message
	ClassID       string
	TeacherID     string
	StudentID     string
	TenantID      string
	ModelOverride string          // pool key (e.g. "fast","smart","expert") to override default model
	EmotionHint   *emotion.Result // optional emotion detected from user input (STT or text analysis)
	TaskID        string          // if set, this request is part of a task thread
	TaskContext   string          // pre-rendered task working memory (injected by gateway)
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
	Reply      string     `json:"reply"`
	SkillsUsed []string   `json:"skills_used"`
	Steps      int        `json:"steps"`
	Plan       []PlanStep `json:"plan,omitempty"`
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

// truncate shortens a string to maxLen runes (not bytes), appending "..." if truncated.
// Uses rune-based counting to safely handle CJK/multi-byte characters.
func truncate(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen]) + "..."
}

// Run executes the planning loop: understand → skill calls → synthesize.
func (p *Planner) Run(ctx context.Context, req PlanRequest) (*PlanResult, error) {
	ctx, span := observe.StartSpan(ctx, "planner.Run")
	span.Attrs["tenant_id"] = req.TenantID
	span.Attrs["mode"] = "text-based"
	if p.useNativeFC {
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
	if p.useNativeFC {
		return p.runNativeFC(ctx, req)
	}
	return p.runTextBased(ctx, req)
}

// runNativeFC uses native LLM function calling (tool_calls in API response).
func (p *Planner) runNativeFC(ctx context.Context, req PlanRequest) (*PlanResult, error) {
	env := p.buildEnv(req)

	messages := p.BuildMessages(ctx, req)
	tools := p.buildFunctionDefs()

	var usedSkills []string
	var planSteps []PlanStep
	steps := 0

	for steps < p.maxSteps {
		steps++

		client := p.LLMClientFor(req.ModelOverride)
		reply, toolCalls, err := client.ChatWithTools(ctx, messages, tools, 0.7)
		if err != nil {
			return nil, fmt.Errorf("planner fc step %d: %w", steps, err)
		}

		if len(toolCalls) == 0 {
			cleaned := p.cleanReply(reply)
			if p.reflect != nil && steps < p.maxSteps {
				userIntent := ""
				if len(req.Messages) > 0 {
					userIntent = req.Messages[len(req.Messages)-1].Content
				}
				if !p.reflect(ctx, userIntent, cleaned) {
					slog.Info("planner: reflect unsatisfied, retrying", "step", steps)
					messages = append(messages,
						llm.Message{Role: "assistant", Content: reply},
						llm.Message{Role: "user", Content: "你的回答质量不够好，请重新组织更完善的回答。"},
					)
					continue
				}
			}
			return &PlanResult{Reply: cleaned, SkillsUsed: usedSkills, Steps: steps, Plan: planSteps}, nil
		}

		// Append assistant message with tool calls reference
		messages = append(messages, llm.Message{Role: "assistant", Content: reply})

		// Execute tool calls in parallel
		type tcResult struct {
			idx    int
			id     string
			name   string
			args   map[string]any
			output string
			err    error
		}
		resultsCh := make(chan tcResult, len(toolCalls))
		for i, tc := range toolCalls {
			go func(idx int, tc llm.ToolCall) {
				var args map[string]any
				json.Unmarshal([]byte(tc.Function.Arguments), &args)

				// Check for handoff (transfer_to_*) calls first
				if p.handoffReg != nil {
					if agentName, ok := p.handoffReg.IsHandoffCall(tc.Function.Name); ok {
						input, _ := args["input"].(string)
						slog.Info("planner: handoff delegation (fc)", "agent", agentName, "step", steps)
						t0 := time.Now()
						hr, err := p.handoffReg.Execute(ctx, req.TenantID, agentName, input)
						dur := time.Since(t0)
						if p.skillMetrics != nil {
							p.skillMetrics(tc.Function.Name, dur, err)
						}
						if p.taskFailureMon != nil {
							p.taskFailureMon.Record(err != nil)
						}
						if err != nil {
							resultsCh <- tcResult{idx: idx, id: tc.ID, name: tc.Function.Name, args: args, err: err}
						} else {
							resultsCh <- tcResult{idx: idx, id: tc.ID, name: tc.Function.Name, args: args, output: hr.Reply}
						}
						return
					}
				}

				skill, ok := p.registry.Get(tc.Function.Name)
				if !ok {
					resultsCh <- tcResult{idx: idx, id: tc.ID, name: tc.Function.Name, args: args, output: fmt.Sprintf("未知技能: %s", tc.Function.Name)}
					return
				}
				slog.Info("planner: executing skill (fc/parallel)", "skill", tc.Function.Name, "step", steps)
				t0 := time.Now()
				r, err := skill.Execute(ctx, args, env)
				dur := time.Since(t0)
				if p.skillMetrics != nil {
					p.skillMetrics(tc.Function.Name, dur, err)
				}
				if p.taskFailureMon != nil {
					p.taskFailureMon.Record(err != nil)
				}
				if err != nil {
					resultsCh <- tcResult{idx: idx, id: tc.ID, name: tc.Function.Name, args: args, err: err}
				} else {
					resultsCh <- tcResult{idx: idx, id: tc.ID, name: tc.Function.Name, args: args, output: r}
				}
			}(i, tc)
		}
		// Collect results in order
		tcResults := make([]tcResult, len(toolCalls))
		for range toolCalls {
			r := <-resultsCh
			tcResults[r.idx] = r
		}
		for _, r := range tcResults {
			usedSkills = append(usedSkills, r.name)
			step := PlanStep{
				ID:     len(planSteps) + 1,
				Action: fmt.Sprintf("调用 %s", r.name),
				Skill:  r.name,
				Args:   r.args,
				Status: StepDone,
				Result: r.output,
			}
			if r.err != nil {
				step.Status = StepFailed
				step.Error = r.err.Error()
				r.output = fmt.Sprintf("执行失败: %v", r.err)
			}
			planSteps = append(planSteps, step)
			messages = append(messages, llm.ToolResultMessage(r.id, r.output))
		}
	}

	client := p.LLMClientFor(req.ModelOverride)
	reply, _, err := client.ChatWithTools(ctx, messages, tools, 0.7)
	if err != nil {
		return nil, fmt.Errorf("planner fc final: %w", err)
	}
	return &PlanResult{Reply: p.cleanReply(reply), SkillsUsed: usedSkills, Steps: steps, Plan: planSteps}, nil
}

// runTextBased uses text-based skill call parsing with multi-step planning.
// Phase 1: Decompose — ask LLM to break task into steps (or handle directly for simple queries).
// Phase 2: Execute — run steps respecting dependencies, parallel when independent.
// Phase 3: Reflect — after tool results, assess if plan needs adjustment.
// Phase 4: Synthesize — produce final reply from all step results.
func (p *Planner) runTextBased(ctx context.Context, req PlanRequest) (*PlanResult, error) {
	env := p.buildEnv(req)

	messages := p.BuildMessages(ctx, req)

	var usedSkills []string
	var planSteps []PlanStep
	steps := 0

	for steps < p.maxSteps {
		steps++

		client := p.LLMClientFor(req.ModelOverride)
		reply, err := client.Chat(ctx, messages, 0.7)
		if err != nil {
			return nil, fmt.Errorf("planner step %d: %w", steps, err)
		}

		calls := p.parseSkillCalls(reply)
		if len(calls) == 0 {
			cleaned := p.cleanReply(reply)

			if p.reflect != nil && steps < p.maxSteps {
				userIntent := ""
				if len(req.Messages) > 0 {
					userIntent = req.Messages[len(req.Messages)-1].Content
				}
				if !p.reflect(ctx, userIntent, cleaned) {
					slog.Info("planner: reflect unsatisfied, retrying", "step", steps)
					messages = append(messages,
						llm.Message{Role: "assistant", Content: reply},
						llm.Message{Role: "user", Content: "你的回答质量不够好，请重新组织更完善的回答。"},
					)
					continue
				}
			}

			return &PlanResult{Reply: cleaned, SkillsUsed: usedSkills, Steps: steps, Plan: planSteps}, nil
		}

		// Execute tool calls in parallel
		type callResult struct {
			idx    int
			name   string
			output string
			err    error
		}
		ch := make(chan callResult, len(calls))
		for i, call := range calls {
			go func(idx int, c skillCall) {
				// Check for handoff (transfer_to_*) calls first
				if p.handoffReg != nil {
					if agentName, ok := p.handoffReg.IsHandoffCall(c.Name); ok {
						input, _ := c.Args["input"].(string)
						if input == "" {
							// Fallback: use first string arg as input
							for _, v := range c.Args {
								if s, ok := v.(string); ok && s != "" {
									input = s
									break
								}
							}
						}
						slog.Info("planner: handoff delegation (text)", "agent", agentName, "step", steps)
						t0 := time.Now()
						hr, err := p.handoffReg.Execute(ctx, req.TenantID, agentName, input)
						dur := time.Since(t0)
						if p.skillMetrics != nil {
							p.skillMetrics(c.Name, dur, err)
						}
						if p.taskFailureMon != nil {
							p.taskFailureMon.Record(err != nil)
						}
						if err != nil {
							ch <- callResult{idx: idx, name: c.Name, err: err}
						} else {
							ch <- callResult{idx: idx, name: c.Name, output: hr.Reply}
						}
						return
					}
				}

				skill, ok := p.registry.Get(c.Name)
				if !ok {
					ch <- callResult{idx: idx, name: c.Name, output: fmt.Sprintf("未知技能: %s", c.Name)}
					return
				}
				slog.Info("planner: executing skill", "skill", c.Name, "step", steps, "parallel", len(calls) > 1)
				t0 := time.Now()
				r, err := skill.Execute(ctx, c.Args, env)
				dur := time.Since(t0)
				if p.skillMetrics != nil {
					p.skillMetrics(c.Name, dur, err)
				}
				if p.taskFailureMon != nil {
					p.taskFailureMon.Record(err != nil)
				}
				if err != nil {
					ch <- callResult{idx: idx, name: c.Name, err: err}
				} else {
					ch <- callResult{idx: idx, name: c.Name, output: r}
				}
			}(i, call)
		}

		// Collect results preserving order
		ordered := make([]callResult, len(calls))
		for range calls {
			r := <-ch
			ordered[r.idx] = r
		}

		var results []string
		for i, r := range ordered {
			usedSkills = append(usedSkills, r.name)
			step := PlanStep{
				ID:     len(planSteps) + 1,
				Action: fmt.Sprintf("调用 %s", r.name),
				Skill:  r.name,
				Args:   calls[i].Args,
				Status: StepDone,
				Result: r.output,
			}
			if r.err != nil {
				step.Status = StepFailed
				step.Error = r.err.Error()
				results = append(results, fmt.Sprintf("[%s] 执行失败: %v", r.name, r.err))
			} else {
				results = append(results, fmt.Sprintf("[%s] %s", r.name, r.output))
			}
			planSteps = append(planSteps, step)
		}

		// Build reflection prompt: show results and ask LLM to assess + continue
		reflectPrompt := "工具调用结果:\n" + strings.Join(results, "\n\n")
		if steps < p.maxSteps-1 && len(planSteps) > 0 {
			reflectPrompt += "\n\n请评估以上结果：如果信息充足，直接给出最终回答；如果还需要更多信息，继续调用工具。"
		}

		messages = append(messages,
			llm.Message{Role: "assistant", Content: reply},
			llm.Message{Role: "user", Content: reflectPrompt},
		)
	}

	clientFinal := p.LLMClientFor(req.ModelOverride)
	reply, err := clientFinal.Chat(ctx, messages, 0.7)
	if err != nil {
		return nil, fmt.Errorf("planner final: %w", err)
	}
	return &PlanResult{Reply: p.cleanReply(reply), SkillsUsed: usedSkills, Steps: steps, Plan: planSteps}, nil
}

// buildFunctionDefs converts skill definitions to LLM FunctionDef format.
func (p *Planner) buildFunctionDefs() []llm.FunctionDef {
	allSkills := p.registry.All()
	defs := make([]llm.FunctionDef, 0, len(allSkills))
	for _, s := range allSkills {
		defs = append(defs, llm.FunctionDef{
			Name:        s.Name(),
			Description: s.Description(),
			Parameters:  s.Parameters(),
		})
	}

	// Append handoff tool definitions (transfer_to_*)
	if p.handoffReg != nil {
		for _, hd := range p.handoffReg.ToolDefinitions() {
			fn, _ := hd["function"].(map[string]any)
			if fn == nil {
				continue
			}
			name, _ := fn["name"].(string)
			desc, _ := fn["description"].(string)
			params, _ := fn["parameters"].(map[string]any)
			defs = append(defs, llm.FunctionDef{
				Name:        name,
				Description: desc,
				Parameters:  params,
			})
		}
	}

	return defs
}

type skillCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"arguments"`
}

func (p *Planner) parseSkillCalls(text string) []skillCall {
	// Look for JSON tool_calls in text
	idx := strings.Index(text, `"tool_calls"`)
	if idx < 0 {
		idx = strings.Index(text, `"skill_calls"`)
	}
	if idx < 0 {
		return nil
	}

	// Find enclosing braces
	start := strings.LastIndex(text[:idx], "{")
	if start < 0 {
		return nil
	}
	end := findClosingBrace(text, start)
	if end < 0 {
		return nil
	}

	var wrapper struct {
		ToolCalls  []skillCall `json:"tool_calls"`
		SkillCalls []skillCall `json:"skill_calls"`
	}
	if err := json.Unmarshal([]byte(text[start:end+1]), &wrapper); err != nil {
		return nil
	}
	if len(wrapper.ToolCalls) > 0 {
		return wrapper.ToolCalls
	}
	return wrapper.SkillCalls
}

func (p *Planner) cleanReply(text string) string {
	// Remove JSON skill call blocks (may appear multiple times)
	for _, marker := range []string{`"tool_calls"`, `"skill_calls"`} {
		for {
			idx := strings.Index(text, marker)
			if idx < 0 {
				break
			}
			start := strings.LastIndex(text[:idx], "{")
			if start < 0 {
				break
			}
			end := findClosingBrace(text, start)
			if end >= 0 {
				text = strings.TrimSpace(text[:start] + text[end+1:])
			} else {
				break
			}
		}
	}
	// Remove ```json blocks
	for {
		s := strings.Index(text, "```json")
		if s < 0 {
			s = strings.Index(text, "```JSON")
		}
		if s < 0 {
			break
		}
		e := strings.Index(text[s+7:], "```")
		if e < 0 {
			break
		}
		text = strings.TrimSpace(text[:s] + text[s+7+e+3:])
	}
	// Remove <think>...</think>
	for {
		s := strings.Index(text, "<think>")
		if s < 0 {
			break
		}
		e := strings.Index(text[s:], "</think>")
		if e < 0 {
			text = strings.TrimSpace(text[:s])
			break
		}
		text = strings.TrimSpace(text[:s] + text[s+e+8:])
	}
	// Remove trailing "descriptive tool call" patterns where LLM describes calling a tool
	// e.g., "让我先调用xxx来..." followed by nothing useful (already stripped JSON above)
	text = cleanTrailingCallDescription(text)
	return strings.TrimSpace(text)
}

// cleanTrailingCallDescription removes trailing sentences where the LLM describes
// calling a tool but the actual JSON was already stripped, leaving orphaned text like
// "让我先调用use_skill来加载Chirp的详细说明：" at the end.
func cleanTrailingCallDescription(text string) string {
	// Patterns that indicate "I'm going to call a tool" — orphaned at end of text
	suffixes := []string{
		"让我", "我来调用", "我先调用", "让我先", "让我尝试", "我将调用", "我会调用",
		"Let me call", "I'll invoke", "Let me try",
	}
	trimmed := strings.TrimSpace(text)
	lines := strings.Split(trimmed, "\n")
	if len(lines) == 0 {
		return text
	}
	lastLine := strings.TrimSpace(lines[len(lines)-1])
	// Only strip if the last line looks like an orphaned tool call description
	for _, suffix := range suffixes {
		if strings.Contains(lastLine, suffix) && (strings.HasSuffix(lastLine, "：") || strings.HasSuffix(lastLine, ":")) {
			return strings.TrimSpace(strings.Join(lines[:len(lines)-1], "\n"))
		}
	}
	return text
}

// InvalidatePromptCache forces rebuild of the cached system prompt on next call.
func (p *Planner) InvalidatePromptCache() { p.sysPromptVer++ }

func (p *Planner) buildSystemPrompt() string {
	currentVer := p.registry.Version()
	if p.cachedSysPrompt != "" && p.sysPromptVer == currentVer {
		return p.cachedSysPrompt
	}

	// ── L1: Core tools (full definitions, always in prompt) ──
	defs := p.registry.Definitions()
	defsJSON, _ := json.MarshalIndent(defs, "", "  ")

	var b strings.Builder
	b.WriteString(`你是Yunque Agent智能体引擎，负责理解用户意图、分解任务、调度技能并综合结果。

## 核心技能 (L1 — 可直接调用)
`)
	b.Write(defsJSON)

	// ── L2: Installed skill index (name + description only, loaded via use_skill) ──
	if p.skillIndex != nil {
		index := p.skillIndex()
		if len(index) > 0 {
			b.WriteString("\n\n## 已安装扩展技能 (L2 — 需通过 use_skill 加载)\n")
			b.WriteString("以下技能已安装但未完整加载。需要使用时，先调用 `use_skill` 传入 slug 加载完整指令。\n")
			for _, s := range index {
				b.WriteString(fmt.Sprintf("- [skill:%s] %s\n", s.Slug, s.Description))
			}
		}
	}

	// ── L3: Remote market hint ──
	b.WriteString(`
## 远程技能市场 (L3 — 按需搜索与安装)
如果核心技能和已安装技能都无法满足用户需求：
1. 先调用 search_skills(query) 搜索远程市场
2. 找到合适技能后，调用 install_skill(slug) 安装（含安全审计，安装后自动加载完整指令）
3. 安装成功后即可按指令使用新技能

## 调用格式
当你需要使用技能时，输出如下JSON（支持同时调用多个独立技能）：
{"tool_calls": [{"name": "技能名", "arguments": {参数}}]}

## 工作流程
1. **分析意图** — 理解用户真正需要什么
2. **规划步骤** — 复杂任务先分解为多个步骤，简单问题直接回答
3. **调用技能** — 独立的技能可以同时调用，有依赖关系的按顺序调用
4. **评估结果** — 收到技能结果后，评估是否足够回答用户问题
5. **继续或总结** — 信息不足则继续调用技能，充足则综合分析给出回答
6. **自主扩展** — 如果现有技能不够用，搜索市场 → 安装 → 使用，一步到位

## 规则
- 不要编造数据，所有数据必须来自技能调用结果
- 收到工具结果后，先评估质量和相关性，再决定下一步
- 如果某个工具调用失败，尝试替代方案而非直接报错
- 回复使用Markdown格式，温暖专业
- 不要在回复中描述你"将要"调用工具，直接调用即可

## 能力边界（硬约束）
- 你只能使用上面列出的「核心技能」和「已安装扩展技能」，严禁声称拥有未注册的技能或功能
- 如果用户请求超出你当前技能范围，先搜索远程市场看是否有可用技能，确实没有再诚实告知用户
- 不要声称自己可以访问互联网、发送邮件、操作文件系统等，除非在可用技能中明确列出
- 不要编造不存在的工具名称或参数
- 如果你不确定某件事，请回答"我不确定"而非胡编答案`)

	p.cachedSysPrompt = b.String()
	p.sysPromptVer = currentVer
	return p.cachedSysPrompt
}

func findClosingBrace(s string, start int) int {
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
