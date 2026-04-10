package federation

import (
	"context"
	"time"

	"yunque-agent/pkg/opp"
)

// PlannerAdapter wraps OPPBridge to satisfy planner.FederationBridge interface.
// This avoids the planner package importing federation directly.
type PlannerAdapter struct {
	bridge    *OPPBridge
	transport *Transport
}

// NewPlannerAdapter creates an adapter that implements planner.FederationBridge.
func NewPlannerAdapter(bridge *OPPBridge, transport *Transport) *PlannerAdapter {
	return &PlannerAdapter{bridge: bridge, transport: transport}
}

// Delegate sends a task to the best-matching remote agent.
func (a *PlannerAdapter) Delegate(ctx context.Context, dp opp.DelegatePayload, timeout time.Duration) (*opp.DelegateResultPayload, error) {
	return a.bridge.Delegate(ctx, a.transport, dp, timeout)
}

// LocalCaps returns the local agent's capabilities.
func (a *PlannerAdapter) LocalCaps() opp.CapabilitiesPayload {
	return a.bridge.LocalCaps()
}
