package planner

import (
	"context"
	"log/slog"
)

type AppliedRuntimeClassification struct {
	Request  PlanRequest
	ToolFree bool
}

func (p *Planner) applyRuntimeClassification(ctx context.Context, req PlanRequest) AppliedRuntimeClassification {
	applied := AppliedRuntimeClassification{Request: req}
	runtimeStrategy := p.ensureRuntimeStrategy()
	if runtimeStrategy == nil {
		return applied
	}
	classified, err := runtimeStrategy.ClassifyRequest(ctx, req, extractGoal(req))
	if err != nil || classified == nil || classified.Decision == nil {
		return applied
	}
	slog.Info(
		"planner: localbrain decision",
		"handler", classified.LogHandler,
		"intent", classified.LogIntent,
		"need_tools", classified.LogNeedTools,
		"reason", classified.LogReason,
	)
	applied.Request = classified.Request
	applied.ToolFree = classified.ToolFree
	if p.ledger != nil {
		tracer := p.ledger.Reasoning(applied.Request.TaskID, "localbrain")
		tracer.Decide(ctx, classified.TraceHandler, classified.TraceReason, classified.TraceScore, classified.TraceMeta)
	}
	return applied
}
