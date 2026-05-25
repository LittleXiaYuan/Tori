package planner

import (
	"context"
	"log/slog"
)

func (p *Planner) runInner(ctx context.Context, req PlanRequest) (*PlanResult, error) {
	if req.ModelOverride != "" {
		slog.Debug("planner: model override", "override", req.ModelOverride)
	}

	if req.DisableTools {
		slog.Info("planner: chat-only mode, skipping all tools")
		return p.runToolFreeChat(ctx, req, "planner chat-only", 0)
	}

	// LocalBrain 预分类：用本地小模型决定路由（省 API token）
	classified := p.applyRuntimeClassification(ctx, req)
	req = classified.Request

	// Fast-path: LocalBrain determined no tools needed → pure chat, skip all tool-enabled engines.
	if classified.ToolFree {
		slog.Info("planner: NeedTools=false, using tool-free chat path")
		return p.runToolFreeChat(ctx, req, "planner tool-free chat", 1)
	}

	return p.dispatchExecutionMode(ctx, req)
}
