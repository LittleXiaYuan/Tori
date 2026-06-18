package gateway

// registerTaskRoutes registers planner recovery routes. The task
// (/v1/tasks/*), project (/v1/projects/*), state (/v1/state*) and document
// surfaces moved to native packs, so registering them here too would panic the
// mux on duplicate patterns.
func (g *Gateway) registerTaskRoutes() {
	// Planner recovery checkpoints
	g.mux.HandleFunc("/v1/planner/checkpoints", g.requireAuth(g.handlePlannerCheckpoints))
	g.mux.HandleFunc("/v1/planner/execution-state", g.requireAuth(g.handlePlannerExecutionState))
	g.mux.HandleFunc("/v1/planner/checkpoints/recover", g.requireAuth(g.handlePlannerCheckpointRecover))
	g.mux.HandleFunc("/v1/planner/checkpoints/resume", g.requireAuth(g.handlePlannerCheckpointResumeTask))
	g.mux.HandleFunc("/v1/planner/checkpoints/resume-plan", g.requireAuth(g.handlePlannerCheckpointResumePlan))
	g.mux.HandleFunc("/v1/planner/checkpoints/resume-plan/jobs", g.requireAuth(g.handlePlannerCheckpointResumePlanJob))

	// Task gaps + context (/v1/tasks/gaps*, /v1/tasks/{memory,threads,templates,
	// templates/instantiate}) are owned by the work pack (internal/packs/work).

	// Missions (/v1/missions/*) are owned by the missions pack
	// (internal/packs/missions), mounted via gw.RegisterModule.

	// State Kernel (/v1/state*) is owned by the state pack
	// (internal/packs/state), mounted via gw.RegisterModule.

	// Reflection (/v1/reflect/*) is owned by the reflection pack
	// (internal/packs/reflection), mounted via gw.RegisterModule.

	// Document generation (/v1/documents/*) is owned by the documents pack
	// (internal/packs/documents), mounted via gw.RegisterModule.
}
