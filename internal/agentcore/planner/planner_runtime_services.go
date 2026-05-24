package planner

func (p *Planner) ensureContextAssembly() *ContextAssemblyService {
	if p.contextAssembly == nil {
		p.contextAssembly = NewContextAssemblyService()
	}
	return p.contextAssembly
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

func (p *Planner) ensureDelegationRuntime() *DelegationRuntimeService {
	if p.delegationRuntime == nil {
		p.delegationRuntime = NewDelegationRuntimeService()
	}
	return p.delegationRuntime
}

func (p *Planner) ensureRuntimeStrategy() *RuntimeStrategyService {
	if p.runtimeStrategy == nil {
		p.runtimeStrategy = NewRuntimeStrategyService()
	}
	return p.runtimeStrategy
}

func (p *Planner) ensurePromptRuntime() *PromptRuntimeService {
	if p.promptRuntime == nil {
		p.promptRuntime = NewPromptRuntimeService()
	}
	return p.promptRuntime
}

func (p *Planner) ensureExecutionRuntime() *ExecutionRuntimeService {
	if p.executionRuntime == nil {
		p.executionRuntime = NewExecutionRuntimeService(0)
	}
	return p.executionRuntime
}

func (p *Planner) ensureContextWindowRuntime() *ContextWindowRuntimeService {
	if p.contextWindowRuntime == nil {
		p.contextWindowRuntime = NewContextWindowRuntimeService()
	}
	return p.contextWindowRuntime
}

func (p *Planner) ensureModelRuntime() *ModelRuntimeService {
	if p.modelRuntime == nil {
		p.modelRuntime = NewModelRuntimeService(nil)
	}
	return p.modelRuntime
}

func (p *Planner) HandoffMetricsHook() SkillMetricsFunc {
	if p == nil {
		return nil
	}
	return p.skillMetrics
}

func (p *Planner) HandoffFailureHook() func(failed bool) bool {
	if p == nil {
		return nil
	}
	return func(failed bool) bool {
		return p.ensureProactiveCognition().RecordExecutionFailure(failed)
	}
}
