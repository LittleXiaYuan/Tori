package gateway

// ide_accessors.go — narrow host accessors for the IDE pack (internal/packs/ide),
// which owns /v1/ide/* natively. The pack reaches the planner only through
// ReviewPlan, so it never imports the planner package directly.

import (
	"context"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/planner"
)

// ReviewPlan runs the LLM review pipeline for the IDE pack and returns the raw
// reply. May fail if the planner is not configured.
func (g *Gateway) ReviewPlan(ctx context.Context, tenantID, prompt string) (string, error) {
	res, err := g.planner.Run(ctx, planner.PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: prompt}},
		TenantID: tenantID,
	})
	if err != nil {
		return "", err
	}
	return res.Reply, nil
}

// SkillCount returns the number of registered skills (for the IDE status view).
func (g *Gateway) SkillCount() int {
	if g.registry == nil {
		return 0
	}
	return len(g.registry.All())
}

// Uptime returns how long the gateway has been running (for the IDE status view).
func (g *Gateway) Uptime() time.Duration { return time.Since(g.startTime) }
