package gateway

// registerTaskRoutes registers planner recovery, missions, state kernel,
// reflection, and document generation routes. The task surface (/v1/tasks/*)
// and project surface (/v1/projects/*) moved to the work pack
// (internal/packs/work, see HandleWorkPack) so toggling yunque.pack.work
// enables/disables them at runtime; registering them here too would panic the
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
	// templates/instantiate}) moved to the work pack bridge (HandleWorkPack).

	// Missions (/v1/missions/*) are owned by the missions pack
	// (internal/packs/missions), mounted via gw.RegisterModule.

	// State Kernel
	g.mux.HandleFunc("/v1/state", g.requireAuth(g.handleStateSnapshot))
	g.mux.HandleFunc("/v1/state/goals", g.requireAuth(g.handleStateGoals))
	g.mux.HandleFunc("/v1/state/focus", g.requireAuth(g.handleStateFocus))
	g.mux.HandleFunc("/v1/state/resources", g.requireAuth(g.handleStateResources))

	// Reflection / Experience
	g.mux.HandleFunc("/v1/reflect/experiences", g.requireAuth(g.handleExperiences))
	g.mux.HandleFunc("/v1/reflect/strategies", g.requireAuth(g.handleStrategies))

	// Document generation (/v1/documents/*) is owned by the documents pack
	// (internal/packs/documents), mounted via gw.RegisterModule.
}
