package planner

import (
	"context"
	"time"

	"yunque-agent/internal/observe"
)

// Run executes the planning loop: understand → skill calls → synthesize.
func (p *Planner) Run(ctx context.Context, req PlanRequest) (*PlanResult, error) {
	// Thread the user's forced-cogni pick down to the declarative Cogni runtime
	// (Decide/BuildContext/Tools read it from ctx) so a `/智能体` chat turn
	// force-activates that Cogni regardless of keyword score.
	ctx = WithForcedCognis(ctx, req.ForceCogniIDs)
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
