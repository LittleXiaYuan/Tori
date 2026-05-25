package planner

import (
	"context"

	"yunque-agent/internal/observe"
)

// Run executes the planning loop: understand → skill calls → synthesize.
func (p *Planner) Run(ctx context.Context, req PlanRequest) (*PlanResult, error) {
	ctx, span := observe.StartSpan(ctx, "planner.Run")
	span.Attrs["tenant_id"] = req.TenantID
	span.Attrs["mode"] = string(p.executionMode(req).Mode)
	result, err := p.runInner(ctx, req)
	observe.EndSpan(span, err)

	if p.learningSidecar != nil {
		p.learningSidecar.AfterRun(ctx, req, result, err, p.reflect)
	}

	return result, err
}
