package planner

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	ctxwindow "yunque-agent/internal/agentcore/context"
	"yunque-agent/internal/agentcore/emotion"
	"yunque-agent/pkg/safego"
)

// CognitiveContextFunc collects dynamic context from all CognitivePlugins.
type CognitiveContextFunc func(ctx context.Context, userMessage string) string

// PromptBuilder assembles the 5 dynamic context layers (P1–P5)
// used by BuildMessages. Extracting this logic makes it testable
// and pluggable independently of the Planner's execution loop.
type PromptBuilder struct {
	LastIncludedLayers []string

	// Data sources (injected from Planner's callbacks)
	contextAssembly *ContextAssemblyService
	proactiveCog    *ProactiveCognitionService
	skillRuntime    *SkillRuntimeService
	dynBudget       int
	runtimeStrategy *RuntimeStrategyService // System 1 context filtering and runtime decisions
}

// NewPromptBuilder creates a PromptBuilder from Planner's callbacks.
func NewPromptBuilder(p *Planner) *PromptBuilder {
	contextAssembly := p.ensureContextAssembly()
	proactiveCognition := p.ensureProactiveCognition()
	skillRuntime := p.ensureSkillRuntime()
	runtimeStrategy := p.ensureRuntimeStrategy()

	return &PromptBuilder{
		contextAssembly: contextAssembly,
		proactiveCog:    proactiveCognition,
		skillRuntime:    skillRuntime,
		dynBudget:       p.dynamicContextBudget(),
		runtimeStrategy: runtimeStrategy,
	}
}

// DynamicContextRequest carries per-request info needed for layer assembly.
type DynamicContextRequest struct {
	LastMessage string
	TenantID    string
	Channel     string
	TaskContext string
	EmotionHint *emotion.Result
	IntentHint  string // intent from LocalBrain (code/chat/search/tool/complex); drives adaptive budget
}

// BudgetAllocation maps layer names to their token budget share (0.0-1.0).
type BudgetAllocation struct {
	Memory    float64
	Graph     float64
	Code      float64
	Cognition float64
	Hints     float64
}

// adaptiveBudgetEnabled caches the env var check at package init time.
var adaptiveBudgetEnabled = strings.EqualFold(strings.TrimSpace(os.Getenv("ADAPTIVE_BUDGET_ENABLED")), "true") ||
	os.Getenv("ADAPTIVE_BUDGET_ENABLED") == "1"

// layerInjectionEnabled reports whether a dynamic-context layer should be
// injected. Default (env unset) is true so production behavior is unchanged;
// only an explicit falsy value ("false"/"0"/"off"/"no") disables the layer.
// Intended for single-blind A/B isolation of context sources — set the flag
// and restart the process to toggle a layer.
func layerInjectionEnabled(env string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(env))) {
	case "false", "0", "off", "no":
		return false
	default:
		return true
	}
}

// Per-layer injection gates, cached at package init (require restart to change).
// All default ON; set the corresponding env to false/0/off to isolate a source.
// User-history channels (toggle these together to A/B "does it remember me"):
//   - INJECT_MEMORY:   per-user recalled memory (orchestrator.CompileContext)
//   - INJECT_STRATEGY: distilled experience/strategy
//   - INJECT_REVERIE:  idle inner-thought journal
//   - INJECT_GRAPH:    knowledge-graph entity relations
// Non-personalized controls (usually kept constant ON):
//   - INJECT_COGNI:    declarative Cogni registry context
//   - INJECT_BELIEF:   Cognition SDK pack disposition/inner-state
var (
	injectMemoryEnabled   = layerInjectionEnabled("INJECT_MEMORY")
	injectStrategyEnabled = layerInjectionEnabled("INJECT_STRATEGY")
	injectReverieEnabled  = layerInjectionEnabled("INJECT_REVERIE")
	injectGraphEnabled    = layerInjectionEnabled("INJECT_GRAPH")
	injectCogniEnabled    = layerInjectionEnabled("INJECT_COGNI")
	injectBeliefEnabled   = layerInjectionEnabled("INJECT_BELIEF")
)

// AllocateBudget returns per-layer budget proportions based on the user's
// intent classification. When intent is empty or adaptive budget is disabled,
// returns uniform allocation.
func AllocateBudget(intent string) BudgetAllocation {
	switch intent {
	case "code":
		return BudgetAllocation{Memory: 0.15, Graph: 0.05, Code: 0.60, Cognition: 0.10, Hints: 0.10}
	case "chat":
		return BudgetAllocation{Memory: 0.50, Graph: 0.10, Code: 0.00, Cognition: 0.30, Hints: 0.10}
	case "search":
		return BudgetAllocation{Memory: 0.15, Graph: 0.50, Code: 0.10, Cognition: 0.15, Hints: 0.10}
	case "tool":
		return BudgetAllocation{Memory: 0.20, Graph: 0.15, Code: 0.20, Cognition: 0.15, Hints: 0.30}
	case "complex":
		return BudgetAllocation{Memory: 0.25, Graph: 0.25, Code: 0.20, Cognition: 0.20, Hints: 0.10}
	default:
		return BudgetAllocation{Memory: 0.25, Graph: 0.20, Code: 0.20, Cognition: 0.20, Hints: 0.15}
	}
}

// layerBudget computes the token budget for a specific layer given the total
// budget and the allocation proportion. Enforces a minimum of 200 tokens.
func layerBudget(total int, proportion float64) int {
	b := int(float64(total) * proportion)
	if b < 200 && proportion > 0 {
		b = 200
	}
	return b
}

// BuildDynamicContext assembles the 5-priority dynamic context layers and
// returns the assembled text ready to be injected as a system message.
// Returns "" if no layers have content.
func (pb *PromptBuilder) BuildDynamicContext(ctx context.Context, req DynamicContextRequest) string {
	t0 := time.Now()
	defer func() {
		slog.Debug("prompt_builder: dynamic context built", "elapsed_ms", time.Since(t0).Milliseconds(), "layers", len(pb.LastIncludedLayers))
	}()
	var layers []ctxwindow.Layer

	// P1: Task working memory (highest dynamic priority)
	if req.TaskContext != "" {
		layers = append(layers, ctxwindow.Layer{
			Name:     "task",
			Priority: ctxwindow.LayerPriorityTask,
			Content:  req.TaskContext,
		})
	}

	// P2+P3: Memory, graph, and code retrieval run in parallel
	// These are independent I/O calls — parallelizing saves ~200-500ms per request
	//
	// Short messages (greetings, single words) skip retrieval entirely to avoid
	// injecting low-relevance memories that pollute the context.
	type layerResult struct {
		name     string
		priority ctxwindow.LayerPriority
		prefix   string
		content  string
	}
	results := make(chan layerResult, 3)
	pending := 0
	skipRetrieval := len([]rune(req.LastMessage)) < 6

	if injectMemoryEnabled && pb.contextAssembly != nil && pb.contextAssembly.memory != nil && !skipRetrieval {
		pending++
		safego.Go("prompt-memory-recall", func() {
			if memCtx := pb.contextAssembly.Memory(ctx, req.TenantID, req.LastMessage); memCtx != "" {
				results <- layerResult{"memory", ctxwindow.LayerPriorityMemory, "## 记忆上下文\n", memCtx}
			} else {
				results <- layerResult{}
			}
		})
	}
	if injectGraphEnabled && pb.contextAssembly != nil && pb.contextAssembly.HasGraphContext() && !skipRetrieval {
		pending++
		safego.Go("prompt-graph-context", func() {
			if graphCtx := pb.contextAssembly.GraphContextForRequest(ctx, req.TenantID, req.LastMessage); graphCtx != "" {
				results <- layerResult{"graph", ctxwindow.LayerPriorityRetrieval, "## 知识图谱\n", graphCtx}
			} else {
				results <- layerResult{}
			}
		})
	}
	if pb.contextAssembly != nil && pb.contextAssembly.codeContext != nil && !skipRetrieval {
		pending++
		safego.Go("prompt-code-context", func() {
			if codeCtx := pb.contextAssembly.codeContext(req.LastMessage); codeCtx != "" {
				results <- layerResult{"code", ctxwindow.LayerPriorityRetrieval, "", codeCtx}
			} else {
				results <- layerResult{}
			}
		})
	}

	// Collect parallel results
	var rawRetrieval []layerResult
	for i := 0; i < pending; i++ {
		r := <-results
		if r.content != "" {
			rawRetrieval = append(rawRetrieval, r)
		}
	}

	// System 1 Filter: if LocalBrain is available, use it to score and filter
	// retrieval results before injecting them as context layers.
	// High-importance items bypass the filter (旁路 in the cognitive architecture).
	if pb.runtimeStrategy != nil && pb.runtimeStrategy.HasContextFilter() && len(rawRetrieval) > 0 {
		var filterItems []RuntimeContextItem
		for _, r := range rawRetrieval {
			filterItems = append(filterItems, RuntimeContextItem{
				Source:  r.name,
				Content: r.content,
			})
		}
		filtered, err := pb.runtimeStrategy.FilterContext(ctx, req.LastMessage, filterItems, 6)
		if err != nil {
			slog.Warn("prompt_builder: context filter failed, using unfiltered", "err", err)
			for _, r := range rawRetrieval {
				layers = append(layers, ctxwindow.Layer{
					Name: r.name, Priority: r.priority, Content: r.prefix + r.content,
				})
			}
		} else {
			slog.Info("prompt_builder: context filtered",
				"before", len(rawRetrieval), "after", len(filtered.Items),
				"filtered_out", filtered.Filtered, "elapsed", filtered.Elapsed)
			for _, item := range filtered.Items {
				var prefix string
				switch item.Source {
				case "memory":
					prefix = "## 记忆上下文\n" +
						"以下是与当前对话相关的记忆片段。使用规则：\n" +
						"- 如果记忆与用户问题直接相关，自然地融入回答中\n" +
						"- 如果记忆是用户偏好，默默遵循即可，不要特意提及\n" +
						"- 如果记忆与当前话题无关，完全忽略\n\n"
				case "graph":
					prefix = "## 知识图谱\n" +
						"以下是相关的实体关系。仅在有助于回答时引用。\n\n"
				default:
					prefix = ""
				}
				prio := ctxwindow.LayerPriorityRetrieval
				if item.Source == "memory" {
					prio = ctxwindow.LayerPriorityMemory
				}
				layers = append(layers, ctxwindow.Layer{
					Name:     item.Source,
					Priority: prio,
					Content:  prefix + strings.TrimSpace(item.Content),
				})
			}
		}
	} else {
		for _, r := range rawRetrieval {
			layers = append(layers, ctxwindow.Layer{
				Name: r.name, Priority: r.priority, Content: r.prefix + r.content,
			})
		}
	}

	// P3.5: CognitivePlugin dynamic context — domain-specific knowledge
	// injected by plugins that implement CognitivePlugin.DynamicContext()
	if pb.contextAssembly != nil && pb.contextAssembly.cognitiveContext != nil {
		if cogCtx := pb.contextAssembly.cognitiveContext(ctx, req.LastMessage); cogCtx != "" {
			layers = append(layers, ctxwindow.Layer{
				Name:     "cognitive_plugins",
				Priority: ctxwindow.LayerPriorityRetrieval,
				Content:  cogCtx,
			})
		}
	}

	// P3.6: Declarative Cogni context — assembled from cogni.Registry.Active()
	// declarations whose ActivationRules match the current message/tenant/channel.
	// The hook (pkg/cogni.Hook) handles evaluation, exclusivity, and rendering.
	if injectCogniEnabled && pb.contextAssembly != nil {
		if cgCtx := pb.contextAssembly.CogniContext(ctx, req.LastMessage, req.TenantID, req.Channel); cgCtx != "" {
			layers = append(layers, ctxwindow.Layer{
				Name:     "cogni",
				Priority: ctxwindow.LayerPriorityRetrieval,
				Content:  cgCtx,
			})
		}
	}

	// P3.7: Cognition SDK belief context — packed inner state and disposition.
	if injectBeliefEnabled && pb.contextAssembly != nil && pb.contextAssembly.beliefContext != nil {
		if beliefCtx := pb.contextAssembly.beliefContext(ctx, req.LastMessage, req.TenantID, req.Channel); beliefCtx != "" {
			layers = append(layers, ctxwindow.Layer{
				Name:     "belief",
				Priority: ctxwindow.LayerPriorityCognition,
				Content:  beliefCtx,
			})
		}
	}

	// P4: Cognition — only inject what's genuinely relevant to the current query.
	// Emotion and strategy have higher priority than reverie (which is speculative).
	if req.EmotionHint != nil {
		if snippet := req.EmotionHint.ContextSnippet(); snippet != "" {
			toneGuide := buildToneGuide(req.EmotionHint)
			layers = append(layers, ctxwindow.Layer{
				Name:     "emotion",
				Priority: ctxwindow.LayerPriorityCognition,
				Content:  "## 情绪感知\n" + snippet + toneGuide,
			})
		}
	}
	if injectStrategyEnabled && pb.contextAssembly != nil && (pb.contextAssembly.strategyContextFor != nil || pb.contextAssembly.strategyContext != nil) {
		strCtx := ""
		if pb.contextAssembly.strategyContextFor != nil {
			strCtx = pb.contextAssembly.strategyContextFor(req.LastMessage)
		}
		if strCtx == "" && pb.contextAssembly.strategyContext != nil {
			strCtx = pb.contextAssembly.strategyContext()
		}
		if strCtx != "" {
			layers = append(layers, ctxwindow.Layer{
				Name:     "strategy",
				Priority: ctxwindow.LayerPriorityCognition,
				Content: "## 经验与策略\n" +
					"以下是从过去对话中学到的经验。如果某条策略改善了你的回答质量，" +
					"可以简要提及「根据之前的经验」。\n\n" + strCtx,
			})
		}
	}
	if pb.contextAssembly != nil && pb.contextAssembly.stateContext != nil {
		if sCtx := pb.contextAssembly.stateContext(); sCtx != "" {
			layers = append(layers, ctxwindow.Layer{
				Name:     "state",
				Priority: ctxwindow.LayerPriorityCognition,
				Content:  sCtx,
			})
		}
	}
	// Reverie: inject only high-relevance inner thoughts with natural delivery guidance
	if injectReverieEnabled && pb.proactiveCog != nil {
		if jctx := pb.proactiveCog.JournalContext(2, req.LastMessage); jctx != "" {
			layers = append(layers, ctxwindow.Layer{
				Name:     "reverie",
				Priority: ctxwindow.LayerPriorityHints,
				Content: "## 内心洞察\n" +
					"以下是我在空闲时产生的与当前话题相关的想法。\n" +
					"如果某个洞察能直接帮助用户，可以自然分享（如「我之前想过这个问题...」），\n" +
					"否则默默参考即可。\n\n" + jctx,
			})
		}
	}

	// P5: Hints — skill optimization
	if pb.skillRuntime != nil {
		if hints := pb.skillRuntime.OptimizationHints(); hints != "" {
			layers = append(layers, ctxwindow.Layer{
				Name:     "skill_hints",
				Priority: ctxwindow.LayerPriorityHints,
				Content:  "## 技能优化\n" + hints,
			})
		}
	}

	// P4.5: Reflection visibility — appended only when the strategy layer was
	// actually included above, so we check LastIncludedLayers lazily. We avoid
	// calling strategyContext() a second time (it was already called above).
	{
		hasStrategy := false
		for _, l := range layers {
			if l.Name == "strategy" && l.Content != "" {
				hasStrategy = true
				break
			}
		}
		if hasStrategy {
			layers = append(layers, ctxwindow.Layer{
				Name:     "reflect_visibility",
				Priority: ctxwindow.LayerPriorityHints,
				Content: "## 学习标记\n" +
					"如果你在回答中运用了上面「经验与策略」中的某条经验，" +
					"可以自然地在回答末尾用一句话提及，如「我记得你之前说过...所以这次...」。" +
					"不要生硬，只在确实改善了回答时提及。",
			})
		}
	}

	// Assemble layers within token budget with anti-pollution features:
	// - Per-layer limit prevents any single source from dominating
	// - Dedup removes repeated sentences across layers
	// - Partial truncation instead of full drop preserves partial context
	budget := pb.dynBudget
	if budget <= 0 {
		budget = 4000
	}

	// Adaptive budget allocation: when enabled and intent is available,
	// assign per-layer budgets proportional to the intent type instead
	// of the fixed 33% cap.
	perLayer := budget / 3
	if perLayer < 500 {
		perLayer = 500
	}
	assembler := ctxwindow.NewLayerAssembler(budget).
		WithDedup()

	if adaptiveBudgetEnabled && req.IntentHint != "" {
		alloc := AllocateBudget(req.IntentHint)
		assembler = assembler.WithPerLayerLimits(map[string]int{
			"memory":            layerBudget(budget, alloc.Memory),
			"graph":             layerBudget(budget, alloc.Graph),
			"code":              layerBudget(budget, alloc.Code),
			"emotion":           layerBudget(budget, alloc.Cognition),
			"strategy":          layerBudget(budget, alloc.Cognition),
			"state":             layerBudget(budget, alloc.Cognition),
			"cognitive_plugins": layerBudget(budget, alloc.Cognition),
			"cogni":             layerBudget(budget, alloc.Cognition),
			"skill_hints":       layerBudget(budget, alloc.Hints),
			"reverie":           layerBudget(budget, alloc.Hints),
		})
		slog.Debug("prompt_builder: adaptive budget",
			"intent", req.IntentHint,
			"memory", layerBudget(budget, alloc.Memory),
			"code", layerBudget(budget, alloc.Code),
			"graph", layerBudget(budget, alloc.Graph),
		)
	} else {
		assembler = assembler.WithPerLayerLimit(perLayer)
	}

	assembled, included := assembler.Assemble(layers)
	pb.LastIncludedLayers = included
	return assembled
}

// buildToneGuide generates a tone/style instruction based on detected emotion.
func buildToneGuide(e *emotion.Result) string {
	if e == nil {
		return ""
	}
	switch {
	case e.Emotion == emotion.EmotionSad:
		return "\n\n【语调引导】用户当前情绪低落。请用温暖、耐心的语气回应，避免过于直接或冷冰冰的表述。"
	case e.Emotion == emotion.EmotionAngry:
		return "\n\n【语调引导】用户当前有些不满。请保持冷静、专业，先共情再给方案，避免推诿。"
	case e.Emotion == emotion.EmotionFearful:
		return "\n\n【语调引导】用户当前有些紧张。请用确定性强的语言，给出清晰的步骤，减少不确定感。"
	case e.Emotion == emotion.EmotionHappy:
		return "\n\n【语调引导】用户当前心情不错。可以稍微轻松一些回应。"
	default:
		return ""
	}
}
