package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/task"
	"yunque-agent/pkg/skills"
)

// OrchestrateSkill bridges the Planner to the orchestration Daemon.
// When the planner calls this skill, it creates a task and enqueues it
// for external Worker execution (Cursor, Claude Code, etc).
type OrchestrateSkill struct {
	taskStore  task.Store
	dispatcher *task.Dispatcher
	projects   *ProjectStore
}

func NewOrchestrateSkill(store task.Store, disp *task.Dispatcher, projects *ProjectStore) *OrchestrateSkill {
	return &OrchestrateSkill{
		taskStore:  store,
		dispatcher: disp,
		projects:   projects,
	}
}

func (s *OrchestrateSkill) Name() string { return "orchestrate_task" }

func (s *OrchestrateSkill) Description() string {
	return `Dispatch a coding task to an external IDE/CLI worker (Cursor, Claude Code, Windsurf, etc).
Use this when the user asks you to write code, implement features, fix bugs, or do development work on a project.
The task will be picked up by an available Worker, executed, and reviewed automatically.
Returns the created task ID and dispatch status.`
}

func (s *OrchestrateSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{
				"type":        "string",
				"description": "Short task title",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Detailed description of what the worker should do",
			},
			"project": map[string]any{
				"type":        "string",
				"description": "Project name or ID (optional, for work directory resolution)",
			},
			"success_criteria": map[string]any{
				"type":        "string",
				"description": "How to verify the task is done correctly",
			},
			"test_command": map[string]any{
				"type":        "string",
				"description": "Shell command to run after completion (exit 0 = pass)",
			},
			"risk_level": map[string]any{
				"type":        "string",
				"enum":        []string{"low", "medium", "high"},
				"description": "Risk level: low=async review, medium=blocking review, high=require human approval",
			},
			"priority": map[string]any{
				"type":        "string",
				"enum":        []string{"low", "medium", "high"},
				"description": "Task priority (default: medium)",
			},
			"required_caps": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Required worker capabilities (e.g. [\"code\", \"golang\"])",
			},
		},
		"required": []string{"title", "description"},
	}
}

func (s *OrchestrateSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	title, _ := args["title"].(string)
	desc, _ := args["description"].(string)
	if title == "" || desc == "" {
		return "", fmt.Errorf("title and description are required")
	}

	constraints := &task.TaskConstraints{
		Extra: make(map[string]any),
	}

	if sc, ok := args["success_criteria"].(string); ok && sc != "" {
		constraints.SuccessCriteria = sc
	}
	if tc, ok := args["test_command"].(string); ok && tc != "" {
		constraints.TestCommand = tc
	}
	if rl, ok := args["risk_level"].(string); ok && rl != "" {
		constraints.RiskLevel = task.RiskLevel(rl)
	}
	if pr, ok := args["priority"].(string); ok {
		constraints.Priority = pr
	}

	// Resolve project
	projectName, _ := args["project"].(string)
	if projectName != "" && s.projects != nil {
		if p, found := s.projects.FindByName(projectName); found {
			constraints.Extra["project_id"] = p.ID
			constraints.Extra["work_dir"] = p.RepoPath
		}
	}

	tenantID := ""
	if env != nil {
		tenantID = env.TenantID
	}

	req := task.CreateRequest{
		Title:       title,
		Description: desc,
		TenantID:    tenantID,
		Constraints: constraints,
	}

	t, err := s.taskStore.Create(req)
	if err != nil {
		return "", fmt.Errorf("failed to create task: %w", err)
	}

	// Enqueue for dispatch
	var requiredCaps []string
	if caps, ok := args["required_caps"].([]any); ok {
		for _, c := range caps {
			if cs, ok := c.(string); ok {
				requiredCaps = append(requiredCaps, cs)
			}
		}
	}
	if len(requiredCaps) == 0 {
		requiredCaps = []string{"code"}
	}

	priority := 5
	switch strings.ToLower(constraints.Priority) {
	case "high":
		priority = 10
	case "low":
		priority = 1
	}

	timeoutSec := 600 // 10 minutes default
	if constraints.TimeoutSec > 0 {
		timeoutSec = constraints.TimeoutSec
	}

	if s.dispatcher != nil {
		if err := s.dispatcher.Enqueue(t.ID, requiredCaps, priority, timeoutSec); err != nil {
			slog.Warn("orchestrate: enqueue failed", "task", t.ID, "err", err)
		}
	}

	slog.Info("orchestrate: task created and enqueued",
		"task_id", t.ID, "title", title, "priority", priority, "caps", requiredCaps)

	result := map[string]any{
		"task_id":    t.ID,
		"title":     title,
		"status":    "enqueued",
		"priority":  priority,
		"caps":      requiredCaps,
		"created_at": time.Now().Format(time.RFC3339),
	}
	if projectName != "" {
		result["project"] = projectName
	}

	out, _ := json.Marshal(result)
	return string(out), nil
}
