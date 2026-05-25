package planner

import (
	ldg "yunque-agent/internal/ledgercore"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/pkg/skills"
)

// Planner is the brain: understands intent, breaks tasks into steps, and drives skills.
type Planner struct {
	registry     *skills.Registry
	reflect      ReflectFunc
	skillMetrics SkillMetricsFunc
	// browserDispatch removed — browser skills now handled via skill registry (browserskill package)
	trustGate            *SkillTrustGate              // skill execution trust gate and score feedback
	ledger               *ldg.Ledger                  // Ledger instance for ReAct/Reasoning/Eval
	runState             RunStateAccessor             // per-session interrupt checking (nil = no interrupt support)
	contextAssembly      *ContextAssemblyService      // dynamic context and declarative Cogni boundary
	learningSidecar      *LearningSidecar             // post-run learning and metacognition side effects
	skillRuntime         *SkillRuntimeService         // skill surface scoring/recommendation/growth boundary
	proactiveCog         *ProactiveCognitionService   // proactive cognition and Reverie event boundary
	delegationRuntime    *DelegationRuntimeService    // handoff and federation delegation boundary
	runtimeStrategy      *RuntimeStrategyService      // mode switches, local brain, and provider routing
	promptRuntime        *PromptRuntimeService        // system prompt, locale, native-FC and skill-index runtime
	executionRuntime     *ExecutionRuntimeService     // step/time budgets, context budget and ack behavior
	contextWindowRuntime *ContextWindowRuntimeService // compression and context-window trimming boundary
	modelRuntime         *ModelRuntimeService         // default LLM, model pool and fallback lookup boundary
}

// NewPlanner creates a planner with the given LLM client and skill registry.
func NewPlanner(llmClient *llm.Client, registry *skills.Registry, maxSteps int) *Planner {
	return &Planner{modelRuntime: NewModelRuntimeService(llmClient), registry: registry, executionRuntime: NewExecutionRuntimeService(maxSteps)}
}
