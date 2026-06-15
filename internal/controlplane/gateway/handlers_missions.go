package gateway

import (
	"context"

	"yunque-agent/internal/agentcore/planner"
)

// MissionParseResult is the structured intent returned from NL mission parsing.
type MissionParseResult = planner.MissionParseResult

// ParseMissionIntent exposes NL mission parsing to the missions pack
// (internal/packs/missions), which owns /v1/missions/* natively. Returns any so
// the pack stays decoupled from the planner package.
func (g *Gateway) ParseMissionIntent(ctx context.Context, description string) (any, error) {
	res, err := g.planner.ParseMissionIntent(ctx, description)
	return res, err
}
