package gateway

import (
	"yunque-agent/internal/orchestrator"
)

func (g *Gateway) registerProjectRoutes() {
	if g.projectStore == nil {
		g.projectStore = orchestrator.NewProjectStore("data/projects")
	}
	// /v1/projects/* are owned by the work pack (internal/packs/work), mounted
	// via gw.RegisterModule in cmd/agent/init_task_engine.go. The project store
	// init stays here so the pack's native handlers have their backing store.
}

// ProjectStore exposes the project store to the work pack (internal/packs/work),
// which owns the de-shelled /v1/projects/* surface natively. May be nil.
func (g *Gateway) ProjectStore() *orchestrator.ProjectStore { return g.projectStore }
