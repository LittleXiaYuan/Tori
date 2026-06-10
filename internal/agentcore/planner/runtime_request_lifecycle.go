package planner

import (
	"context"
	"time"

	"yunque-agent/internal/observe"
)

// Run executes the planning loop: understand → skill calls → synthesize.
func (p *Planner) Run(ctx context.Context, req PlanRequest) (*PlanResult, error) {
	ctx, span := observe.StartSpan(ctx, "planner.Run")
	span.Attrs["tenant_id"] = req.TenantID
	span.Attrs["mode"] = string(p.executionMode(req).Mode)
	start := time.Now()
	result, err := p.runInner(ctx, req)
	observe.EndSpan(span, err)

	learningSidecar := p.ensureLearningSidecar()
	learningSidecar.AfterRun(ctx, req, result, err, p.reflect, time.Since(start))

	return result, err
}
