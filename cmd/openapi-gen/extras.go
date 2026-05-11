package main

// extraPaths returns endpoints that the route scanner cannot discover
// automatically. The most common case is a handler that dispatches an
// internal sub-tree based on URL prefix (e.g. handleCognis dispatches
// /v1/cognis/{id}/workflows, /experience, /evolve, ...).
//
// Each entry overrides any auto-generated entry for the same path.
//
// Keep this list aligned with the actual handler dispatch logic.
type extraEndpoint struct {
	Path        string
	Method      string
	Tag         string
	Summary     string
	OperationID string
}

func extraPaths() []extraEndpoint {
	return []extraEndpoint{
		// ── Planner recovery checkpoints ─────────────────────────
		{"/v1/planner/checkpoints", "get", "planner", "List recent tenant-scoped recoverable planner checkpoints", "list_planner_checkpoints"},
		{"/v1/planner/execution-state", "get", "planner", "Read the joined execution state for one planner checkpoint", "get_planner_execution_state"},
		{"/v1/planner/checkpoints/recover", "post", "planner", "Build a semantic recovery prompt for a planner checkpoint", "recover_planner_checkpoint"},
		{"/v1/planner/checkpoints/resume", "post", "planner", "Create a background task from a planner checkpoint recovery plan", "resume_planner_checkpoint_task"},
		{"/v1/planner/checkpoints/resume-plan", "post", "planner", "Resume a planner checkpoint through the original planner DAG runner", "resume_planner_checkpoint_plan"},
		{"/v1/planner/checkpoints/resume-plan/jobs", "get", "planner", "Read a planner resume-plan job by job id or plan id", "get_planner_resume_plan_job"},

		// ── Cognis collection ────────────────────────────────────
		{"/v1/cognis", "get", "cognis", "List every registered Cogni declaration", "list_cognis"},
		{"/v1/cognis", "post", "cognis", "Add an inline Cogni declaration (JSON body)", "create_cogni"},
		{"/v1/cognis/reload", "post", "cognis", "Re-scan the cognis directory on disk", "reload_cognis"},
		{"/v1/cognis/traces", "get", "cognis", "Recent per-turn evaluation traces (all cognis)", "list_cogni_traces"},
		{"/v1/cognis/stats", "get", "cognis", "Activation counts per cogni", "get_cogni_stats"},
		{"/v1/cognis/health", "get", "cognis", "Health metrics for every cogni recently active", "get_cogni_health_all"},
		{"/v1/cognis/alerts", "get", "cognis", "Active cogni alerts (sentinel)", "list_cogni_alerts"},
		{"/v1/cognis/alerts/scan", "post", "cognis", "Trigger sentinel scan and return current alerts", "scan_cogni_alerts"},
		{"/v1/cognis/verify", "post", "cognis", "Verify all cogni declarations (manifest signatures + checks)", "verify_cognis_all"},
		{"/v1/cognis/generate", "post", "cognis", "Self-generate a Cogni from natural language", "generate_cogni"},
		{"/v1/cognis/export", "get", "cognis", "Export selected cognis as a portable bundle", "export_cogni_bundle"},
		{"/v1/cognis/import", "post", "cognis", "Import a Cogni bundle (overwrite via ?overwrite=true)", "import_cogni_bundle"},
		{"/v1/cognis/evolution", "get", "cognis", "Global Skill Evolution experiment overview", "list_cogni_evolution_global"},
		{"/v1/cognis/federation", "get", "cognis", "Federation status (peers, exposed cognis)", "get_cogni_federation_status"},
		{"/v1/cognis/federation/peers", "get", "cognis", "List federated peers", "list_cogni_federation_peers"},
		{"/v1/cognis/federation/peers", "post", "cognis", "Add or update a federated peer", "upsert_cogni_federation_peer"},
		{"/v1/cognis/federation/discover", "post", "cognis", "Discover cognis on a remote peer", "discover_remote_cognis"},
		{"/v1/cognis/economics", "get", "cognis", "Cost summary for every cogni (per-cogni budget tracking)", "get_cogni_economics"},

		// ── Cognis by id ──────────────────────────────────────────
		{"/v1/cognis/{id}", "get", "cognis", "Fetch one Cogni declaration", "get_cogni_by_id"},
		{"/v1/cognis/{id}", "delete", "cognis", "Remove one Cogni declaration", "delete_cogni"},
		{"/v1/cognis/{id}/enable", "post", "cognis", "Enable a Cogni", "enable_cogni"},
		{"/v1/cognis/{id}/disable", "post", "cognis", "Disable a Cogni", "disable_cogni"},
		{"/v1/cognis/{id}/trace", "get", "cognis", "Traces filtered to one cogni id", "get_cogni_traces_by_id"},
		{"/v1/cognis/{id}/health", "get", "cognis", "Health rollup for one cogni", "get_cogni_health_by_id"},
		{"/v1/cognis/{id}/verify", "post", "cognis", "Verify a single cogni declaration", "verify_cogni_by_id"},
		{"/v1/cognis/{id}/workflows", "get", "cognis", "List workflows on one Cogni", "list_cogni_workflows"},
		{"/v1/cognis/{id}/workflow/{name}", "post", "cognis", "Run a named workflow on one Cogni", "run_cogni_workflow"},
		{"/v1/cognis/{id}/experience", "get", "cognis", "View experience entries for a Cogni", "get_cogni_experience"},
		{"/v1/cognis/{id}/experience/record", "post", "cognis", "Record a new experience for a Cogni", "record_cogni_experience"},
		{"/v1/cognis/{id}/evolve", "post", "cognis", "Trigger Skill Evolution for a Cogni", "evolve_cogni"},
		{"/v1/cognis/{id}/evolution", "get", "cognis", "Evolution experiment history for a Cogni", "get_cogni_evolution_by_id"},
		{"/v1/cognis/{id}/expose", "post", "cognis", "Expose a Cogni on the federation", "expose_cogni"},
		{"/v1/cognis/{id}/unexpose", "post", "cognis", "Unexpose a Cogni from the federation", "unexpose_cogni"},
	}
}
