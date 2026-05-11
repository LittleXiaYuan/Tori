package main

// schemaOverride lets us hand-write request/response schemas for a specific
// endpoint instead of leaving the generic `type: object` placeholder. Only the
// fields you set are applied — leave `RequestSchema` or `Response200Schema`
// nil to keep the auto-generated default for that side.
type schemaOverride struct {
	Path            string
	Method          string
	RequestSchema   map[string]any
	Response200     map[string]any
	Response200Desc string
}

// schemaOverrides returns precise schemas for the highest-traffic endpoints.
// Add new entries as schemas are validated against the actual handler.
func schemaOverrides() []schemaOverride {
	return []schemaOverride{
		// Planner recovery checkpoints — UI recovery flow after stream/model/tool failures.
		{
			Path: "/v1/planner/checkpoints", Method: "get",
			Response200:     plannerCheckpointListSchema(),
			Response200Desc: "Recent recoverable planner checkpoints.",
		},
		{
			Path: "/v1/planner/checkpoints/recover", Method: "post",
			RequestSchema:   plannerCheckpointActionRequestSchema(false),
			Response200:     plannerCheckpointRecoverResponseSchema(),
			Response200Desc: "Backend-owned recovery prompt and recovery plan.",
		},
		{
			Path: "/v1/planner/checkpoints/resume", Method: "post",
			RequestSchema:   plannerCheckpointActionRequestSchema(true),
			Response200:     plannerCheckpointResumeTaskResponseSchema(),
			Response200Desc: "Background recovery task created or accepted.",
		},
		{
			Path: "/v1/planner/checkpoints/resume-plan", Method: "post",
			RequestSchema:   plannerCheckpointResumePlanRequestSchema(),
			Response200:     plannerCheckpointResumePlanResponseSchema(),
			Response200Desc: "Planner DAG recovery result or async resume-plan job id.",
		},
		{
			Path: "/v1/planner/checkpoints/resume-plan/jobs", Method: "get",
			Response200:     plannerResumePlanJobResponseSchema(),
			Response200Desc: "Latest or explicit tenant-scoped resume-plan job.",
		},
		{
			Path: "/v1/planner/execution-state", Method: "get",
			Response200:     plannerExecutionStateResponseSchema(),
			Response200Desc: "Joined checkpoint, recovery plan, latest job, failure summary, and events.",
		},

		// POST /v1/chat — main synchronous chat call.
		{
			Path:   "/v1/chat",
			Method: "post",
			RequestSchema: map[string]any{
				"type":     "object",
				"required": []string{"messages"},
				"properties": map[string]any{
					"messages": map[string]any{
						"type":        "array",
						"description": "Chat message history (max 100 entries, each ≤32000 chars).",
						"minItems":    1,
						"maxItems":    100,
						"items": map[string]any{
							"type":     "object",
							"required": []string{"role", "content"},
							"properties": map[string]any{
								"role": map[string]any{
									"type":        "string",
									"description": "OpenAI-style role.",
									"enum":        []string{"system", "user", "assistant", "tool", "function"},
								},
								"content": map[string]any{
									"type":      "string",
									"maxLength": 32000,
								},
								"name":         map[string]any{"type": "string"},
								"tool_call_id": map[string]any{"type": "string"},
							},
						},
					},
					"session_id": map[string]any{"type": "string", "description": "Conversation session id (created automatically if blank)."},
					"task_id":    map[string]any{"type": "string"},
					"class_id":   map[string]any{"type": "string"},
					"teacher_id": map[string]any{"type": "string"},
					"student_id": map[string]any{"type": "string"},
					"platform":   map[string]any{"type": "string", "description": "Target platform for sticker suggestions (qq/feishu/discord/...)."},
					"thinking_level": map[string]any{
						"type":        "string",
						"description": "Override model tier for thinking budget.",
						"enum":        []string{"none", "auto", "deep"},
					},
				},
			},
			Response200: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":         map[string]any{"type": "string"},
					"reply":      map[string]any{"type": "string", "description": "Assistant reply."},
					"session_id": map[string]any{"type": "string"},
					"task_id":    map[string]any{"type": "string"},
					"emotion":    map[string]any{"type": "object", "additionalProperties": true},
					"usage": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"prompt_tokens":     map[string]any{"type": "integer"},
							"completion_tokens": map[string]any{"type": "integer"},
							"total_tokens":      map[string]any{"type": "integer"},
						},
					},
					"latency_ms": map[string]any{"type": "integer"},
					"trace_id":   map[string]any{"type": "string"},
				},
			},
			Response200Desc: "Chat completion response.",
		},

		// POST /v1/cognis/generate — natural-language to cogni.json synthesis.
		{
			Path:   "/v1/cognis/generate",
			Method: "post",
			RequestSchema: map[string]any{
				"type":     "object",
				"required": []string{"description"},
				"properties": map[string]any{
					"description": map[string]any{
						"type":        "string",
						"description": "Natural-language description of the desired Cogni (e.g. \"a code-review cogni that focuses on Go test coverage\").",
						"minLength":   1,
					},
					"auto_save": map[string]any{
						"type":        "boolean",
						"description": "If true, persist the generated declaration to the cogni directory immediately.",
						"default":     false,
					},
				},
			},
			Response200: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"declaration": map[string]any{
						"type":                 "object",
						"description":          "Generated Cogni declaration (see cogni.Declaration in pkg/cogni).",
						"additionalProperties": true,
					},
					"saved": map[string]any{"type": "boolean"},
					"path":  map[string]any{"type": "string", "description": "Filesystem path of the saved declaration (when auto_save=true)."},
				},
			},
			Response200Desc: "Generated Cogni declaration.",
		},

		// GET /healthz — public, simple health probe.
		{
			Path:   "/healthz",
			Method: "get",
			Response200: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"status":        map[string]any{"type": "string", "enum": []string{"ok", "degraded"}},
					"version":       map[string]any{"type": "string"},
					"breaker_state": map[string]any{"type": "string", "enum": []string{"closed", "open", "half-open"}},
					"uptime_sec":    map[string]any{"type": "integer"},
				},
			},
			Response200Desc: "Liveness + degradation summary.",
		},

		// POST /v1/cognis — create a Cogni declaration inline.
		// cogni.Declaration is a deeply nested config object — we expose the
		// top-level fields and leave the inner structures as
		// `additionalProperties: true` so SDK users get field names but can
		// extend without spec changes.
		{
			Path:   "/v1/cognis",
			Method: "post",
			RequestSchema: map[string]any{
				"type":        "object",
				"required":    []string{"id"},
				"description": "Cogni declaration. Same shape as a cogni.yaml file — see pkg/cogni for the full struct.",
				"properties": map[string]any{
					"id":           map[string]any{"type": "string", "description": "Unique Cogni id (also used as filename)."},
					"display_name": map[string]any{"type": "string"},
					"description":  map[string]any{"type": "string"},
					"capsule":      map[string]any{"type": "string", "description": "Capsule (persona) this Cogni binds to. Empty for free-standing routing policies."},
					"activation":   map[string]any{"type": "object", "additionalProperties": true, "description": "ActivationRules (when this Cogni engages)."},
					"surface":      map[string]any{"type": "object", "additionalProperties": true, "description": "ToolSurface (which tools/capabilities are exposed)."},
					"context":      map[string]any{"type": "object", "additionalProperties": true, "description": "ContextInjection (extra text added to the system prompt)."},
					"mcp":          map[string]any{"type": "object", "additionalProperties": true, "description": "MCPConfig (per-Cogni MCP server connections + tool filters)."},
					"workflows":    map[string]any{"type": "array", "items": map[string]any{"type": "object", "additionalProperties": true}, "description": "Multi-step workflows."},
					"experience":   map[string]any{"type": "object", "additionalProperties": true},
					"economics":    map[string]any{"type": "object", "additionalProperties": true, "description": "Per-Cogni budget / cost limits."},
					"memory":       map[string]any{"type": "object", "additionalProperties": true},
					"priority":     map[string]any{"type": "integer", "default": 100, "description": "Tie-break multiple activated Cognis (lower = higher priority)."},
					"exclusive":    map[string]any{"type": "string", "description": "If non-empty, only one Cogni with this exclusive-group may activate per turn."},
					"checks":       map[string]any{"type": "array", "items": map[string]any{"type": "object", "additionalProperties": true}, "description": "Activation self-tests."},
				},
			},
			Response200: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"status": map[string]any{"type": "string", "enum": []string{"ok"}},
					"id":     map[string]any{"type": "string"},
				},
			},
			Response200Desc: "Cogni declaration accepted.",
		},

		// POST /v1/tasks — create a new task.
		{
			Path:   "/v1/tasks",
			Method: "post",
			RequestSchema: map[string]any{
				"type":     "object",
				"required": []string{"description"},
				"properties": map[string]any{
					"title":       map[string]any{"type": "string", "description": "Optional human-readable title."},
					"description": map[string]any{"type": "string", "minLength": 1, "description": "Required goal description; the planner uses this to decompose."},
					"constraints": map[string]any{
						"type":                 "object",
						"description":          "TaskConstraints — budget, timeouts, deny-tools, etc.",
						"additionalProperties": true,
					},
				},
			},
			Response200: map[string]any{
				"type":                 "object",
				"description":          "Created Task object (full task.Task schema).",
				"additionalProperties": true,
				"properties": map[string]any{
					"id":          map[string]any{"type": "string"},
					"title":       map[string]any{"type": "string"},
					"description": map[string]any{"type": "string"},
					"status":      map[string]any{"type": "string", "enum": []string{"pending", "running", "paused", "completed", "failed", "cancelled"}},
					"created_at":  map[string]any{"type": "string", "format": "date-time"},
				},
			},
			Response200Desc: "Created task.",
		},

		// POST /v1/tasks/run — start running an existing task by id.
		{
			Path:   "/v1/tasks/run",
			Method: "post",
			RequestSchema: map[string]any{
				"type":     "object",
				"required": []string{"id"},
				"properties": map[string]any{
					"id": map[string]any{"type": "string", "description": "Task id (returned from POST /v1/tasks)."},
				},
			},
			Response200: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"status":   map[string]any{"type": "string", "enum": []string{"running", "queued"}},
					"task_id":  map[string]any{"type": "string"},
					"trace_id": map[string]any{"type": "string"},
				},
			},
			Response200Desc: "Task scheduled.",
		},

		// POST /v1/memory/search — multi-layer memory recall.
		{
			Path:   "/v1/memory/search",
			Method: "post",
			RequestSchema: map[string]any{
				"type":     "object",
				"required": []string{"query"},
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "minLength": 1},
					"limit": map[string]any{"type": "integer", "default": 10, "minimum": 1, "maximum": 100},
				},
			},
			Response200: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"results": map[string]any{
						"type":  "array",
						"items": map[string]any{"type": "object", "additionalProperties": true},
					},
					"count": map[string]any{"type": "integer"},
				},
			},
			Response200Desc: "Memory items matching the query (across short/mid/long layers).",
		},

		// POST /v1/memory/add — write a single memory entry.
		{
			Path:   "/v1/memory/add",
			Method: "post",
			RequestSchema: map[string]any{
				"type":     "object",
				"required": []string{"value"},
				"properties": map[string]any{
					"key":    map[string]any{"type": "string", "description": "Optional stable key (used for upsert)."},
					"value":  map[string]any{"type": "string", "minLength": 1},
					"layer":  map[string]any{"type": "string", "enum": []string{"short", "mid", "long"}, "description": "Memory layer; defaults to short."},
					"source": map[string]any{"type": "string", "description": "Provenance label (`user`, `system`, ...)."},
				},
			},
			Response200: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"ok": map[string]any{"type": "boolean"},
					"id": map[string]any{"type": "string"},
				},
			},
			Response200Desc: "Memory item stored.",
		},
	}
}

func plannerRecoveryActionSchema() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "Recovery action selected by the user or UI.",
		"enum":        []string{"continue", "retry_failed", "partial"},
	}
}

func plannerCheckpointActionRequestSchema(includeRun bool) map[string]any {
	props := map[string]any{
		"plan_id": map[string]any{"type": "string", "minLength": 1},
		"action":  plannerRecoveryActionSchema(),
	}
	if includeRun {
		props["run"] = map[string]any{"type": "boolean", "description": "Whether to start the created background recovery task immediately.", "default": true}
	}
	return map[string]any{
		"type":       "object",
		"required":   []string{"plan_id", "action"},
		"properties": props,
	}
}

func plannerCheckpointResumePlanRequestSchema() map[string]any {
	s := plannerCheckpointActionRequestSchema(false)
	s["properties"].(map[string]any)["async"] = map[string]any{
		"type":        "boolean",
		"description": "If true, return a resume-plan job id immediately instead of holding the request open.",
		"default":     false,
	}
	return s
}

func plannerCheckpointListSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"checkpoints": map[string]any{"type": "array", "items": plannerCheckpointSummarySchema()},
			"limit":       map[string]any{"type": "integer"},
			"count":       map[string]any{"type": "integer"},
		},
	}
}

func plannerCheckpointSummarySchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"plan_id":       map[string]any{"type": "string"},
			"task_id":       map[string]any{"type": "string"},
			"goal":          map[string]any{"type": "string"},
			"status":        map[string]any{"type": "string", "enum": []string{"pending", "running", "completed", "failed", "cancelled", "interrupted"}},
			"current_step":  map[string]any{"type": "integer"},
			"completed":     map[string]any{"type": "integer"},
			"total":         map[string]any{"type": "integer"},
			"steps_used":    map[string]any{"type": "integer"},
			"revisions":     map[string]any{"type": "integer"},
			"error":         map[string]any{"type": "string", "description": "UI-safe friendly error."},
			"recoverable":   map[string]any{"type": "boolean"},
			"resume_hint":   map[string]any{"type": "string"},
			"updated_at":    map[string]any{"type": "string", "format": "date-time"},
			"plan_snapshot": map[string]any{"type": "array", "items": plannerCheckpointStepSchema()},
		},
	}
}

func plannerCheckpointStepSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id":         map[string]any{"type": "integer"},
			"action":     map[string]any{"type": "string"},
			"skill":      map[string]any{"type": "string"},
			"status":     map[string]any{"type": "string", "enum": []string{"pending", "running", "done", "completed", "failed", "skipped"}},
			"depends_on": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
			"result":     map[string]any{"type": "string", "description": "Sanitized completed-step evidence."},
			"error":      map[string]any{"type": "string", "description": "UI-safe friendly step error."},
		},
	}
}

func plannerCheckpointPlanSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"mode":       plannerRecoveryActionSchema(),
			"executable": map[string]any{"type": "boolean"},
			"reason":     map[string]any{"type": "string"},
			"task_id":    map[string]any{"type": "string"},
			"prompt":     map[string]any{"type": "string"},
			"steps": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":         map[string]any{"type": "integer"},
						"action":     map[string]any{"type": "string"},
						"skill":      map[string]any{"type": "string"},
						"status":     map[string]any{"type": "string"},
						"depends_on": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
						"selected":   map[string]any{"type": "boolean"},
						"reason":     map[string]any{"type": "string"},
					},
				},
			},
		},
	}
}

func plannerCheckpointRecoverResponseSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action":        plannerRecoveryActionSchema(),
			"plan_id":       map[string]any{"type": "string"},
			"task_id":       map[string]any{"type": "string"},
			"prompt":        map[string]any{"type": "string"},
			"recovery_plan": plannerCheckpointPlanSchema(),
			"checkpoint":    plannerCheckpointSummarySchema(),
		},
	}
}

func plannerCheckpointResumeTaskResponseSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"status":        map[string]any{"type": "string", "enum": []string{"created", "accepted"}},
			"task_id":       map[string]any{"type": "string"},
			"run":           map[string]any{"type": "boolean"},
			"recovery_plan": plannerCheckpointPlanSchema(),
			"checkpoint":    plannerCheckpointSummarySchema(),
		},
	}
}

func plannerCheckpointResumePlanResponseSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"status":         map[string]any{"type": "string", "enum": []string{"accepted", "completed", "failed"}},
			"action":         plannerRecoveryActionSchema(),
			"plan_id":        map[string]any{"type": "string"},
			"job_id":         map[string]any{"type": "string"},
			"friendly_error": map[string]any{"type": "string"},
			"recoverable":    map[string]any{"type": "boolean"},
			"next_action":    map[string]any{"type": "string"},
			"result":         plannerPlanResultSchema(),
			"recovery_plan":  plannerCheckpointPlanSchema(),
			"checkpoint":     plannerCheckpointSummarySchema(),
		},
	}
}

func plannerPlanResultSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"reply":       map[string]any{"type": "string"},
			"skills_used": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"steps":       map[string]any{"type": "integer"},
			"plan":        map[string]any{"type": "array", "items": plannerCheckpointStepSchema()},
		},
	}
}

func plannerResumePlanJobSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id":             map[string]any{"type": "string"},
			"status":         map[string]any{"type": "string", "enum": []string{"running", "completed", "failed", "cancelled"}},
			"action":         plannerRecoveryActionSchema(),
			"plan_id":        map[string]any{"type": "string"},
			"task_id":        map[string]any{"type": "string"},
			"error":          map[string]any{"type": "string", "description": "UI-safe friendly error."},
			"friendly_error": map[string]any{"type": "string"},
			"recoverable":    map[string]any{"type": "boolean"},
			"next_action":    map[string]any{"type": "string"},
			"result":         plannerPlanResultSchema(),
			"events":         map[string]any{"type": "array", "items": plannerResumePlanJobEventSchema()},
			"started_at":     map[string]any{"type": "string", "format": "date-time"},
			"finished_at":    map[string]any{"type": "string", "format": "date-time"},
		},
	}
}

func plannerResumePlanJobEventSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id":        map[string]any{"type": "string"},
			"type":      map[string]any{"type": "string"},
			"summary":   map[string]any{"type": "string", "description": "UI-safe event summary."},
			"skill":     map[string]any{"type": "string"},
			"timestamp": map[string]any{"type": "string", "format": "date-time"},
		},
	}
}

func plannerResumePlanJobResponseSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"job": plannerResumePlanJobSchema()},
	}
}

func plannerExecutionStateResponseSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"plan_id":         map[string]any{"type": "string"},
			"status":          map[string]any{"type": "string"},
			"action":          plannerRecoveryActionSchema(),
			"next_action":     map[string]any{"type": "string"},
			"updated_at":      map[string]any{"type": "string", "format": "date-time"},
			"checkpoint":      plannerCheckpointSummarySchema(),
			"latest_job":      plannerResumePlanJobSchema(),
			"recovery_plan":   plannerCheckpointPlanSchema(),
			"failure_summary": plannerExecutionFailureSummarySchema(),
			"cogni":           map[string]any{"type": "object", "additionalProperties": true},
			"events":          map[string]any{"type": "array", "items": plannerResumePlanJobEventSchema()},
		},
	}
}

func plannerExecutionFailureSummarySchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"failed_count":    map[string]any{"type": "integer"},
			"completed_count": map[string]any{"type": "integer"},
			"failed_tools":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"last_summary":    map[string]any{"type": "string"},
			"event_count":     map[string]any{"type": "integer"},
			"events":          map[string]any{"type": "array", "items": plannerResumePlanJobEventSchema()},
		},
	}
}
