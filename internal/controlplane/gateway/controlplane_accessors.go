package gateway

import (
	"context"

	"yunque-agent/internal/agentcore/approval"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/observe"
)

func (g *Gateway) ApprovalManager() *approval.Manager {
	if g == nil {
		return nil
	}
	return g.approvalMgr
}

// MetricsSnapshot exposes a user-safe copy point for the control-plane pack's
// native observability routes.
func (g *Gateway) MetricsSnapshot() observe.MetricsSnapshot {
	if g == nil || g.metrics == nil {
		return observe.MetricsSnapshot{}
	}
	return g.metrics.Snapshot()
}

func (g *Gateway) MetricsPrometheus() string {
	if g == nil || g.metrics == nil {
		return ""
	}
	return g.metrics.PrometheusFormat()
}

func (g *Gateway) ModelRuntimeHealth() planner.ModelRuntimeHealth {
	if g == nil || g.planner == nil {
		return planner.ModelRuntimeHealth{Configured: false}
	}
	return g.planner.ModelRuntimeHealth()
}

func (g *Gateway) LLMResponseCacheStats() map[string]any {
	if g == nil || g.planner == nil {
		return nil
	}
	return g.planner.LLMResponseCacheStats()
}

func (g *Gateway) SystemStats(ctx context.Context) map[string]any {
	if g == nil {
		return map[string]any{}
	}
	tid := tenantFromCtx(ctx)
	stats := map[string]any{
		"requests_total": g.reqCount.Load(),
		"tenants":        0,
		"skills":         0,
		"plugins":        0,
		"scheduler_jobs": 0,
		"conversations":  0,
		"memory":         map[string]int{},
	}
	if g.tenants != nil {
		stats["tenants"] = len(g.tenants.List())
	}
	if g.registry != nil {
		stats["skills"] = len(g.registry.All())
	}
	if g.pluginReg != nil {
		stats["plugins"] = len(g.pluginReg.AllIncludeDisabled())
	}
	if g.scheduler != nil {
		stats["scheduler_jobs"] = len(g.scheduler.List())
	}
	if g.convStore != nil {
		stats["conversations"] = g.convStore.Count()
	}
	if g.memory != nil {
		stats["memory"] = g.memory.Stats(tid)
	}
	return stats
}
