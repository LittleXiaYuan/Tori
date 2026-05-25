package planner

import "context"

func (p *Planner) dispatchExecutionMode(ctx context.Context, req PlanRequest) (*PlanResult, error) {
	mode := p.executionMode(req)
	switch mode.Mode {
	case PlanExecutionLongHorizon:
		p.emitCognitiveLoadEvent(req, mode.CognitiveLoad)
		return p.runLongHorizon(ctx, req)
	case PlanExecutionReAct:
		return p.runReAct(ctx, req)
	case PlanExecutionNativeFC:
		return p.runNativeFC(ctx, req)
	default:
		return p.runTextBased(ctx, req)
	}
}
