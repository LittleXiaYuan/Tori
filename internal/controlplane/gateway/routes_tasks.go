package gateway

import (
	"net/http"

	"yunque-agent/internal/apperror"
)

// registerTaskRoutes registers task runtime, state kernel, reflection, and document generation routes.
func (g *Gateway) registerTaskRoutes() {
	// Task CRUD
	g.mux.HandleFunc("/v1/tasks", g.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			g.handleTaskList(w, r)
		case http.MethodPost:
			g.handleTaskCreate(w, r)
		case http.MethodDelete:
			g.handleTaskDelete(w, r)
		default:
			apperror.WriteCode(w, apperror.CodeBadRequest, "method not allowed")
		}
	}))

	// Task operations
	g.mux.HandleFunc("/v1/tasks/run", g.requireAuth(g.handleTaskRun))
	g.mux.HandleFunc("/v1/tasks/cancel", g.requireAuth(g.handleTaskCancel))
	g.mux.HandleFunc("/v1/tasks/pause", g.requireAuth(g.handleTaskPause))
	g.mux.HandleFunc("/v1/tasks/resume", g.requireAuth(g.handleTaskResume))
	g.mux.HandleFunc("/v1/tasks/restart", g.requireAuth(g.handleTaskRestart))

	// Planner recovery checkpoints
	g.mux.HandleFunc("/v1/planner/checkpoints", g.requireAuth(g.handlePlannerCheckpoints))
	g.mux.HandleFunc("/v1/planner/execution-state", g.requireAuth(g.handlePlannerExecutionState))
	g.mux.HandleFunc("/v1/planner/checkpoints/recover", g.requireAuth(g.handlePlannerCheckpointRecover))
	g.mux.HandleFunc("/v1/planner/checkpoints/resume", g.requireAuth(g.handlePlannerCheckpointResumeTask))
	g.mux.HandleFunc("/v1/planner/checkpoints/resume-plan", g.requireAuth(g.handlePlannerCheckpointResumePlan))
	g.mux.HandleFunc("/v1/planner/checkpoints/resume-plan/jobs", g.requireAuth(g.handlePlannerCheckpointResumePlanJob))

	// Gaps
	g.mux.HandleFunc("/v1/tasks/gaps", g.requireAuth(g.handleGaps))
	g.mux.HandleFunc("/v1/tasks/gaps/resolve", g.requireAuth(g.handleGapResolve))

	// Task context
	g.mux.HandleFunc("/v1/tasks/memory", g.requireAuth(g.handleTaskWorkingMemory))
	g.mux.HandleFunc("/v1/tasks/threads", g.requireAuth(g.handleTaskThread))
	g.mux.HandleFunc("/v1/tasks/templates", g.requireAuth(g.handleTemplates))
	g.mux.HandleFunc("/v1/tasks/templates/instantiate", g.requireAuth(g.handleTemplateInstantiate))

	// Missions
	g.mux.HandleFunc("/v1/missions/parse", g.requireAuth(g.handleMissionParse))

	// State Kernel
	g.mux.HandleFunc("/v1/state", g.requireAuth(g.handleStateSnapshot))
	g.mux.HandleFunc("/v1/state/goals", g.requireAuth(g.handleStateGoals))
	g.mux.HandleFunc("/v1/state/focus", g.requireAuth(g.handleStateFocus))
	g.mux.HandleFunc("/v1/state/resources", g.requireAuth(g.handleStateResources))

	// Reflection / Experience
	g.mux.HandleFunc("/v1/reflect/experiences", g.requireAuth(g.handleExperiences))
	g.mux.HandleFunc("/v1/reflect/strategies", g.requireAuth(g.handleStrategies))

	// Document Generation
	g.mux.HandleFunc("/v1/documents/generate", g.requireAuth(g.handleDocGenerate))
	g.mux.HandleFunc("/v1/documents/templates", g.requireAuth(g.handleDocTemplates))
}
