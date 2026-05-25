package planner

import (
	"time"

	ldg "yunque-agent/internal/ledgercore"

	ctxwindow "yunque-agent/internal/agentcore/context"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/subagent"
	"yunque-agent/internal/cognicore/recommend"
	iledger "yunque-agent/internal/ledger"
	"yunque-agent/pkg/skills"
)

func (p *Planner) SetMemory(fn MemorySearchFunc) {
	p.ensureContextAssembly().SetMemory(fn)
}

func (p *Planner) SetReflect(fn ReflectFunc) { p.reflect = fn }

// SetMetaCogBridge attaches the metacognitive anomaly bridge.
// When set, the planner checks for reasoning anomalies (loops, stalls,
// drift) and injects correction hints into subsequent prompts.
func (p *Planner) SetMetaCogBridge(b *iledger.MetaCogBridge) {
	p.ensureLearningSidecar().SetMetaCogBridge(b)
}

func (p *Planner) SetPersonaPrompt(fn func() string) {
	promptRuntime := p.ensurePromptRuntime()
	promptRuntime.SetPersonaPrompt(fn)
}

// SetGraphContext replaces the graph context source used by dynamic context assembly.
func (p *Planner) SetGraphContext(fn func(query string) string) {
	p.ensureContextAssembly().SetGraphContext(fn)
}

// AppendGraphContext adds a graph/context source while preserving any source already wired.
func (p *Planner) AppendGraphContext(fn func(query string) string) {
	p.ensureContextAssembly().AppendGraphContext(fn)
}

// SetBrowser is a no-op kept for backward compatibility — browser skills are now in the skill registry.

// SetCodeContext: fn(query) returns formatted code snippets from repo-type knowledge.
func (p *Planner) SetCodeContext(fn func(query string) string) {
	p.ensureContextAssembly().SetCodeContext(fn)
}

func (p *Planner) SetStateContext(fn func() string) {
	p.ensureContextAssembly().SetStateContext(fn)
}

func (p *Planner) SetStrategyContext(fn func() string) {
	p.ensureContextAssembly().SetStrategyContext(fn)
}

// SetStrategyContextFor attaches a query-aware reflection strategy provider.
// The legacy SetStrategyContext callback remains as a fallback for callers that
// cannot cheaply scope strategy context to the current user message.
func (p *Planner) SetStrategyContextFor(fn func(query string) string) {
	p.ensureContextAssembly().SetStrategyContextFor(fn)
}

func (p *Planner) SetDynContextBudget(tokens int) {
	executionRuntime := p.ensureExecutionRuntime()
	executionRuntime.SetDynContextBudget(tokens)
}

func (p *Planner) SetDomainPrompt(prompt string) {
	promptRuntime := p.ensurePromptRuntime()
	promptRuntime.SetDomainPrompt(prompt)
}

func (p *Planner) SetNativeFC(enabled bool) {
	promptRuntime := p.ensurePromptRuntime()
	promptRuntime.SetNativeFC(enabled)
}

func (p *Planner) SetWindowConfig(cfg ctxwindow.WindowConfig) {
	p.ensureContextWindowRuntime().SetWindowConfig(cfg)
}

func (p *Planner) SetContextManager(mgr *ctxwindow.Manager) {
	p.ensureContextWindowRuntime().SetManager(mgr)
}

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
func (p *Planner) SetSkillIndex(fn SkillIndexFunc) {
	promptRuntime := p.ensurePromptRuntime()
	promptRuntime.SetSkillIndex(fn)
}

// SetLLMPool attaches a multi-model LLM pool for dynamic model selection.
// When set, ModelOverride in PlanRequest selects a pool client by key.
func (p *Planner) SetLLMPool(pool *llm.Pool) {
	modelRuntime := p.ensureModelRuntime()
	modelRuntime.SetPool(pool)
}

// SetHandoffRegistry attaches a handoff registry for subagent delegation.
// Handoff tools (transfer_to_*) are automatically added to LLM function definitions.
func (p *Planner) SetHandoffRegistry(hr *subagent.HandoffRegistry) {
	p.ensureDelegationRuntime().SetHandoffRegistry(hr)
}

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
func (p *Planner) SetCognitiveContext(fn CognitiveContextFunc) {
	p.ensureContextAssembly().SetCognitiveContext(fn)
}

// SetBeliefContext attaches the Cognition SDK dynamic context collector.
func (p *Planner) SetBeliefContext(fn BeliefContextFunc) {
	p.ensureContextAssembly().SetBeliefContext(fn)
}

// SetLedger attaches a Ledger instance for ReAct mode, reasoning traces, and self-evaluation.
func (p *Planner) SetLedger(l *ldg.Ledger) { p.ledger = l }

// SetReActMode enables the Ledger-powered ReAct reasoning loop.
func (p *Planner) SetReActMode(enabled bool) {
	runtimeStrategy := p.ensureRuntimeStrategy()
	runtimeStrategy.SetReActMode(enabled)
}

// SetLongHorizonMode enables the DAG-based long-horizon planner for complex tasks.
func (p *Planner) SetLongHorizonMode(enabled bool) {
	runtimeStrategy := p.ensureRuntimeStrategy()
	runtimeStrategy.SetLongHorizonMode(enabled)
}

// SetRunStateAccessor attaches the per-session interrupt checking function.
func (p *Planner) SetRunStateAccessor(fn RunStateAccessor) { p.runState = fn }

// SetLocalBrain attaches the local small model decision layer.
func (p *Planner) SetLocalBrain(lb LocalBrainRuntime) {
	runtimeStrategy := p.ensureRuntimeStrategy()
	runtimeStrategy.SetLocalBrain(lb)
}

// SetAgenticThinking attaches the agentic thinking engine.
func (p *Planner) SetAgenticThinking(at AgenticThinkerRuntime) {
	runtimeStrategy := p.ensureRuntimeStrategy()
	runtimeStrategy.SetAgenticThinking(at)
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
	runtimeStrategy := p.ensureRuntimeStrategy()
	runtimeStrategy.SetProviderRegistry(reg)
}

// SetCogniContext attaches a declarative Cogni context injector. The callback
// is invoked once per turn from the prompt builder; nil disables the layer.
func (p *Planner) SetCogniContext(fn CogniContextFunc) {
	p.ensureContextAssembly().SetCogniContext(fn)
}

// SetCogniSkillFilter attaches a declarative Cogni surface filter. The
// callback is invoked from buildFunctionDefs to narrow the tool list to
// the union of every activated cogni's ToolSurface; nil keeps the full
// skill set.
func (p *Planner) SetCogniSkillFilter(fn CogniSkillFilterFunc) {
	p.ensureContextAssembly().SetCogniSkillFilter(fn)
}

// SetCogniTrace attaches a declarative Cogni observability callback. Nil keeps
// Cogni internal-only and preserves prior behaviour.
func (p *Planner) SetCogniTrace(fn CogniTraceFunc) {
	p.ensureContextAssembly().SetCogniTrace(fn)
}

// SetToolTimeout sets the per-tool execution timeout. Default is 60s.
func (p *Planner) SetToolTimeout(d time.Duration) {
	executionRuntime := p.ensureExecutionRuntime()
	executionRuntime.SetToolTimeout(d)
}
