package gateway

// registerTriggerRoutes registers trigger, cron, scheduler, and tools routes.
func (g *Gateway) registerTriggerRoutes() {
	// Triggers (legacy)
	g.mux.HandleFunc("/v1/triggers", g.requireAuth(g.handleTriggers))
	g.mux.HandleFunc("/v1/triggers/emit", g.requireAuth(g.handleTriggerEmit))

	// Triggers v2 (unified)
	g.mux.HandleFunc("/v1/triggers/v2", g.requireAuth(g.handleTriggersV2))
	g.mux.HandleFunc("/v1/triggers/v2/emit", g.requireAuth(g.handleTriggersV2Emit))
	g.mux.HandleFunc("/v1/triggers/v2/runs", g.requireAuth(g.handleTriggersV2Runs))
	g.mux.HandleFunc("/v1/triggers/v2/events", g.requireAuth(g.handleTriggersV2Events))

	// Cron
	g.mux.HandleFunc("/v1/cron/list", g.requireAuth(g.handleCronList))
	g.mux.HandleFunc("/v1/cron/add", g.requireAuth(g.handleCronAdd))
	g.mux.HandleFunc("/v1/cron/remove", g.requireAuth(g.handleCronRemove))
	g.mux.HandleFunc("/v1/cron/run", g.requireAuth(g.handleCronRun))

	// Scheduler
	g.mux.HandleFunc("/v1/scheduler/jobs", g.requireAuth(g.handleSchedulerJobs))
	g.mux.HandleFunc("/v1/scheduler/add", g.requireAuth(g.handleSchedulerAdd))
	g.mux.HandleFunc("/v1/scheduler/remove", g.requireAuth(g.handleSchedulerRemove))

	// Tools (process execution)
	g.mux.HandleFunc("/v1/tools/exec", g.requireAuth(g.handleToolExec))
	g.mux.HandleFunc("/v1/tools/list", g.requireAuth(g.handleToolList))
	g.mux.HandleFunc("/v1/tools/poll", g.requireAuth(g.handleToolPoll))
	g.mux.HandleFunc("/v1/tools/kill", g.requireAuth(g.handleToolKill))

	// Sandbox
	g.mux.HandleFunc("/v1/sandbox/exec", g.requireAuth(g.handleSandboxExec))
}
