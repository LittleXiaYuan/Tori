package planner

// SetFederationBridge attaches the OPP federation bridge for A2A task delegation.
func (p *Planner) SetFederationBridge(fb FederationBridge) {
	delegationRuntime := p.ensureDelegationRuntime()
	delegationRuntime.SetFederationBridge(fb)
}

// FederationBridgeRef returns the current bridge (may be nil).
func (p *Planner) FederationBridgeRef() FederationBridge {
	if p == nil {
		return nil
	}
	delegationRuntime := p.ensureDelegationRuntime()
	return delegationRuntime.FederationBridge()
}

