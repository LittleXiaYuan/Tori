package gateway

// registerTaskRoutes keeps task-platform routes that still require direct
// gateway ownership. Task (/v1/tasks/*), project (/v1/projects/*), planner
// recovery (/v1/planner/checkpoints*), state (/v1/state*) and document surfaces
// moved to native packs, so registering them here too would panic the mux on
// duplicate patterns.
func (g *Gateway) registerTaskRoutes() {
	// Task gaps + context (/v1/tasks/gaps*, /v1/tasks/{memory,threads,templates,
	// templates/instantiate}) are owned by the work pack (internal/packs/work).

	// Planner recovery checkpoints (/v1/planner/checkpoints*) are owned by the
	// planner-recovery pack (internal/packs/plannerrecovery), mounted via
	// gw.RegisterModule.

	// Missions (/v1/missions/*) are owned by the missions pack
	// (internal/packs/missions), mounted via gw.RegisterModule.

	// State Kernel (/v1/state*) is owned by the state pack
	// (internal/packs/state), mounted via gw.RegisterModule.

	// Reflection (/v1/reflect/*) is owned by the reflection pack
	// (internal/packs/reflection), mounted via gw.RegisterModule.

	// Document generation (/v1/documents/*) is owned by the documents pack
	// (internal/packs/documents), mounted via gw.RegisterModule.
}
