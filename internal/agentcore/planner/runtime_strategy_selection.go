package planner

import "yunque-agent/internal/agentcore/plan"

func (p *Planner) executionMode(req PlanRequest) PlanExecutionModeDecision {
	load := assessCognitiveLoad(req)
	modeReq := PlanExecutionModeRequest{
		Request:              req,
		NativeFC:             p.promptRuntime != nil && p.promptRuntime.NativeFC(),
		LedgerEnabled:        p.ledger != nil,
		ComplexTask:          load.NeedsLongHorizon() || plan.NeedsPlan(extractGoal(req)),
		CognitiveLoad:        load,
		CognitiveLoadEnabled: true,
	}
	return p.ensureRuntimeStrategy().SelectExecutionMode(modeReq)
}
