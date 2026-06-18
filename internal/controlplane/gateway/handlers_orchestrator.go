package gateway

func (g *Gateway) registerOrchestratorRoutes() {
	// IDE/work orchestration routes (/v1/orchestrator/*) are owned by the
	// orchestrator pack (internal/packs/orchestrator), mounted via gw.RegisterModule.
}
