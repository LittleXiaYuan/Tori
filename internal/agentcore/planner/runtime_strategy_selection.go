package planner

import "yunque-agent/internal/agentcore/plan"

func (p *Planner) executionMode(req PlanRequest) PlanExecutionModeDecision {
	promptRuntime := p.ensurePromptRuntime()
	runtimeStrategy := p.ensureRuntimeStrategy()
	load := assessCognitiveLoad(req)
	modeReq := PlanExecutionModeRequest{
		Request:              req,
		NativeFC:             promptRuntime != nil && promptRuntime.NativeFC(),
		LedgerEnabled:        p.ledger != nil,
		ComplexTask:          load.NeedsLongHorizon() || plan.NeedsPlan(extractGoal(req)),
		CognitiveLoad:        load,
		CognitiveLoadEnabled: true,
	}
	return runtimeStrategy.SelectExecutionMode(modeReq)
}
