package planner

import (
	"context"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
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

// retrievalTimeout caps how long BuildDynamicContext waits for the parallel
// retrieval layers (memory / ledger recall / code). Recall involves network
// hops (query embedding, optional cross-encoder rerank), and an unbounded
// wait lets one slow provider stall every chat turn — the reported
// "greetings feel laggy" failure mode. Late layers are dropped for this turn;
// the next turn retries. Tune via CONTEXT_RETRIEVAL_TIMEOUT_MS (0 disables).
var retrievalTimeout = func() time.Duration {
	if v := strings.TrimSpace(os.Getenv("CONTEXT_RETRIEVAL_TIMEOUT_MS")); v != "" {
		if ms, err := strconv.Atoi(v); err == nil && ms >= 0 {
			return time.Duration(ms) * time.Millisecond
		}
	}
	return 1500 * time.Millisecond
}()

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

// cognitiveLayerEnabled is the MASTER switch for the whole cognitive layer
// (memory, graph/recall, cogni, belief, strategy, reverie, emotion, state,
// cognitive plugins, skill hints, reflection/learning, dreaming, evolution).
// Default ON — when off the agent runs as a clean "planner + tools" shell.
//
// It is RUNTIME-MUTABLE (atomic) so it can be hot-toggled without a restart —
// the same hot-plug model the Pack runtime / Cogni registry already use. The
// initial value comes from COGNITIVE_LAYER_ENABLED (default on); a pack or API
// toggle can flip it live via SetCognitiveLayerEnabled.
var cognitiveLayerEnabled = func() *atomic.Bool {
	b := new(atomic.Bool)
	b.Store(layerInjectionEnabled("COGNITIVE_LAYER_ENABLED"))
	return b
}()

// CognitiveLayerEnabled reports whether the master cognitive layer is on. Boot
// wiring, gateway post-hooks, and background schedulers all gate on this so the
// whole stack flips together. Safe for concurrent reads.
func CognitiveLayerEnabled() bool { return cognitiveLayerEnabled.Load() }

// SetCognitiveLayerEnabled hot-toggles the cognitive layer at runtime (no
// restart). Intended to be driven by a pack/API toggle so the entire cognitive
// stack can be enabled/disabled live, WASM-pack style.
func SetCognitiveLayerEnabled(on bool) { cognitiveLayerEnabled.Store(on) }

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

	// Master cognitive-layer gate. When OFF, the agent is a clean planner+tools
	// shell: skip ALL cognitive context layers, keeping only the active task's
	// working memory (task execution, not the cognitive companion stack).
	if !cognitiveLayerEnabled.Load() {
		if strings.TrimSpace(req.TaskContext) != "" {
			pb.LastIncludedLayers = []string{"task"}
			return req.TaskContext
		}
		pb.LastIncludedLayers = nil
		return ""
	}

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
		elapsed  time.Duration
	}
	results := make(chan layerResult, 3)
	pending := 0
	pendingNames := make(map[string]struct{}, 3)
	msgRunes := len([]rune(req.LastMessage))
	skipRetrieval := msgRunes < 6
	// LocalBrain already classified this turn (System 1, no extra cost).
	// Casual chat turns keep the memory layer (preferences matter) but skip
	// graph/ledger recall and code retrieval: both carry network hops
	// (query embedding, rerank) with near-zero relevance for small talk.
	// Longer chat messages may reference history, so only short ones skip.
	casualChat := req.IntentHint == "chat" && msgRunes < 24

	// Bound the whole retrieval fan-out. The child context cancels in-flight
	// embedding/rerank HTTP calls once the budget is spent; the buffered
	// channel lets late goroutines finish without leaking.
	retrCtx := ctx
	var cancelRetr context.CancelFunc
	if retrievalTimeout > 0 && !skipRetrieval {
		retrCtx, cancelRetr = context.WithTimeout(ctx, retrievalTimeout)
		defer cancelRetr()
	}

	if injectMemoryEnabled && pb.contextAssembly != nil && pb.contextAssembly.memory != nil && !skipRetrieval {
		pending++
		pendingNames["memory"] = struct{}{}
		safego.Go("prompt-memory-recall", func() {
			start := time.Now()
			memCtx := pb.contextAssembly.Memory(retrCtx, req.TenantID, req.LastMessage)
			results <- layerResult{"memory", ctxwindow.LayerPriorityMemory, "## 记忆上下文\n", memCtx, time.Since(start)}
		})
	}
	if injectGraphEnabled && pb.contextAssembly != nil && pb.contextAssembly.HasGraphContext() && !skipRetrieval && !casualChat {
		pending++
		pendingNames["graph"] = struct{}{}
		safego.Go("prompt-graph-context", func() {
			start := time.Now()
			graphCtx := pb.contextAssembly.GraphContextForRequest(retrCtx, req.TenantID, req.LastMessage)
			results <- layerResult{"graph", ctxwindow.LayerPriorityRetrieval, "## 知识图谱\n", graphCtx, time.Since(start)}
		})
	}
	if pb.contextAssembly != nil && pb.contextAssembly.codeContext != nil && !skipRetrieval && !casualChat {
		pending++
		pendingNames["code"] = struct{}{}
		safego.Go("prompt-code-context", func() {
			start := time.Now()
			codeCtx := pb.contextAssembly.codeContext(req.LastMessage)
			results <- layerResult{"code", ctxwindow.LayerPriorityRetrieval, "", codeCtx, time.Since(start)}
		})
	}

	// Collect parallel results, dropping layers that miss the time budget.
	var rawRetrieval []layerResult
	var retrDeadline <-chan time.Time
	if retrievalTimeout > 0 && pending > 0 {
		timer := time.NewTimer(retrievalTimeout + 100*time.Millisecond)
		defer timer.Stop()
		retrDeadline = timer.C
	}
collect:
	for i := 0; i < pending; i++ {
		select {
		case r := <-results:
			delete(pendingNames, r.name)
			slog.Debug("prompt_builder: retrieval layer done",
				"layer", r.name, "elapsed_ms", r.elapsed.Milliseconds(), "empty", r.content == "")
			if r.content != "" {
				rawRetrieval = append(rawRetrieval, r)
			}
		case <-retrDeadline:
			// Name the offenders so production logs point straight at the slow
			// retrieval source instead of just counting drops.
			slow := make([]string, 0, len(pendingNames))
			for name := range pendingNames {
				slow = append(slow, name)
			}
			sort.Strings(slow)
			slog.Warn("prompt_builder: retrieval budget exceeded, continuing with partial context",
				"collected", len(rawRetrieval), "dropped", strings.Join(slow, ","), "budget", retrievalTimeout)
			break collect
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

	// P3.6: Unified Cogni context — merged Declaration context (pkg/cogni) +
	// Pack perception/belief (pkg/cognisdk) in ONE layer. Step 2 of cogni
	// consolidation: the former separate P3.7 belief layer is now merged in
	// here, so the prompt has one cogni layer instead of two parallel ones.
	// scope is derived from IntentHint so the belief scope gate (#34) filters
	// scoped beliefs: emotional boundary dormant in technical turns, etc.
	if injectCogniEnabled && pb.contextAssembly != nil {
		scope := intentToScope(req.IntentHint)
		if cgCtx := pb.contextAssembly.CogniContext(ctx, req.LastMessage, req.TenantID, req.Channel, scope); cgCtx != "" {
			layers = append(layers, ctxwindow.Layer{
				Name:     "cogni",
				Priority: ctxwindow.LayerPriorityRetrieval,
				Content:  cgCtx,
			})
		}
	}

	// P3.7: belief layer REMOVED in Step 2 of cogni consolidation. The belief
	// context is now merged into P3.6's unified cogni layer via
	// CogniRuntime.BuildContext (which internally combines pkg/cogni
	// Declaration context + pkg/cognisdk Pack perception/belief). Keeping the
	// injectBeliefEnabled gate would risk double-injecting belief, so the gate
	// is now inert — the belief content flows only through P3.6. The
	// BeliefContextFunc setter is retained for backward compat but no longer
	// drives a separate prompt layer.

	// P3.8: Runtime grade — trust gate tier, available skill list, dynamic risk
	// level (#4). This is the "runtime self-awareness" layer: tells the model
	// which skills exist (no hallucination), what trust tier it's on (which ops
	// need approval), and the current risk level (calibrate caution).
	if pb.contextAssembly != nil && pb.contextAssembly.runtimeGrade != nil {
		if gradeCtx := pb.contextAssembly.runtimeGrade(req.TenantID, req.Channel); gradeCtx != "" {
			layers = append(layers, ctxwindow.Layer{
				Name:     "runtime_grade",
				Priority: ctxwindow.LayerPriorityCognition,
				Content:  gradeCtx,
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
	// Pack context — enabled capability packs (Tier 0 microkernel) that implement
	// ContextProvider inject here, so enabling a Pack actually shows up in the
	// agent's reasoning flow. Empty when no enabled pack contributes context.
	if pb.contextAssembly != nil && pb.contextAssembly.packContext != nil {
		if packCtx := pb.contextAssembly.packContext(ctx, req.TenantID, req.LastMessage); packCtx != "" {
			layers = append(layers, ctxwindow.Layer{
				Name:     "pack",
				Priority: ctxwindow.LayerPriorityCognition,
				Content:  packCtx,
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

// intentToScope maps the LocalBrain intent category to the coarse conversation
// scope used by pkg/belief's scope gate (#34). The mapping mirrors
// prompt_builder's existing intent usage (e.g. IntentHint=="chat" already flags
// casual conversation at line ~231):
//   - "chat" → "emotional"  (闲聊/情感倾诉场景，emotional boundary 该激活)
//   - "code"/"search"/"tool" → "technical" (技术任务，emotional boundary 该休眠)
//   - "complex"/空 → "" (未明确，scope gate 走 empty 分支：scoped belief 不激活，
//     只有 global belief 照常激活，保守不误触发)
func intentToScope(intentHint string) string {
	switch intentHint {
	case "chat":
		return "emotional"
	case "code", "search", "tool":
		return "technical"
	default:
		return ""
	}
}
