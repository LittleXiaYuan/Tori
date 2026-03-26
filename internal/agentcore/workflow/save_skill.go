package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/pkg/skills"
)

// ──────────────────────────────────────────────
// SaveWorkflowSkill — lets the planner save its execution as a reusable workflow
//
// Usage in conversation:
//   User: "帮我查天气然后总结新闻"
//   Agent: [executes steps] → success
//   User: "保存为自动化流程，每天8点执行"
//   Agent calls: save_as_workflow(name="每日天气新闻", trigger="cron:0 8 * * *")
//
// The skill reads the most recent PlanResult from the session context,
// converts it to a Workflow Definition, and optionally binds a trigger.
// ──────────────────────────────────────────────

// SaveWorkflowSkill implements skills.Skill.
type SaveWorkflowSkill struct {
	store         Store
	triggerBinder TriggerBinder // optional: binds cron/event triggers
	lastPlanFunc  func(tenantID string) *planner.PlanResult // retrieves last plan
}

// TriggerBinder is called to bind a trigger to a workflow.
type TriggerBinder func(workflowID, triggerExpr, tenantID string) (string, error)

// NewSaveWorkflowSkill creates the save_as_workflow skill.
func NewSaveWorkflowSkill(store Store, lastPlanFunc func(string) *planner.PlanResult) *SaveWorkflowSkill {
	return &SaveWorkflowSkill{
		store:        store,
		lastPlanFunc: lastPlanFunc,
	}
}

// SetTriggerBinder sets the trigger binding function.
func (s *SaveWorkflowSkill) SetTriggerBinder(fn TriggerBinder) {
	s.triggerBinder = fn
}

func (s *SaveWorkflowSkill) Name() string { return "save_as_workflow" }

func (s *SaveWorkflowSkill) Description() string {
	return "将最近的对话执行步骤保存为可重复的自动化工作流。可以设置名称和定时触发器。" +
		"例如：save_as_workflow(name=\"每日天气\", trigger=\"cron:0 8 * * *\")"
}

func (s *SaveWorkflowSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "工作流名称，简洁描述用途",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "工作流的详细描述（可选）",
			},
			"trigger": map[string]any{
				"type":        "string",
				"description": "触发器表达式（可选）。格式：cron:<表达式> 或 event:<事件名> 或 keyword:<关键词>",
			},
		},
		"required": []string{"name"},
	}
}

func (s *SaveWorkflowSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("workflow name is required")
	}
	desc, _ := args["description"].(string)
	triggerExpr, _ := args["trigger"].(string)

	tenantID := env.TenantID

	// Get the most recent plan result
	if s.lastPlanFunc == nil {
		return "", fmt.Errorf("no plan history available")
	}
	lastPlan := s.lastPlanFunc(tenantID)
	if lastPlan == nil || len(lastPlan.Plan) == 0 {
		return "", fmt.Errorf("no recent execution found to save — please run a task first")
	}

	// Convert to workflow definition
	def := ConvertPlanToDefinition(name, desc, lastPlan.Plan, tenantID)

	// Save
	if err := s.store.SaveDefinition(def); err != nil {
		return "", fmt.Errorf("save workflow: %w", err)
	}

	slog.Info("save_as_workflow: created", "id", def.ID, "name", name, "nodes", len(def.Nodes))

	result := map[string]any{
		"workflow_id": def.ID,
		"name":        name,
		"nodes":       len(def.Nodes),
		"edges":       len(def.Edges),
		"status":      "saved",
	}

	// Bind trigger if specified
	if triggerExpr != "" && s.triggerBinder != nil {
		triggerID, err := s.triggerBinder(def.ID, triggerExpr, tenantID)
		if err != nil {
			result["trigger_error"] = err.Error()
			slog.Warn("save_as_workflow: trigger binding failed", "err", err)
		} else {
			result["trigger_id"] = triggerID
			result["trigger"] = triggerExpr
			slog.Info("save_as_workflow: trigger bound", "trigger_id", triggerID, "expr", triggerExpr)
		}
	}

	out, _ := json.Marshal(result)
	return string(out), nil
}

// ──────────────────────────────────────────────
// RunWorkflowSkill — lets the planner execute a saved workflow by name or ID
// ──────────────────────────────────────────────

// RunWorkflowSkill implements skills.Skill for executing saved workflows.
type RunWorkflowSkill struct {
	store     Store
	runFunc   func(ctx context.Context, instanceID string) error // async runner
	tenantID  string
}

// NewRunWorkflowSkill creates the run_workflow skill.
// runFunc should call engine.Run(ctx, instanceID) or similar.
func NewRunWorkflowSkill(store Store, runFunc func(ctx context.Context, instanceID string) error) *RunWorkflowSkill {
	return &RunWorkflowSkill{store: store, runFunc: runFunc}
}

// SetTenantID sets the default tenant ID for workflow operations.
func (s *RunWorkflowSkill) SetTenantID(id string) { s.tenantID = id }

func (s *RunWorkflowSkill) Name() string { return "run_workflow" }

func (s *RunWorkflowSkill) Description() string {
	return "按名称或ID执行已保存的工作流。支持传入变量覆盖。" +
		"先调用 list_workflows 查看可用工作流，再用本技能执行。"
}

func (s *RunWorkflowSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name_or_id": map[string]any{
				"type":        "string",
				"description": "工作流名称（模糊匹配）或工作流ID",
			},
			"variables": map[string]any{
				"type":        "object",
				"description": "可选：覆盖工作流变量，如 {\"date\": \"2024-01-01\"}",
			},
		},
		"required": []string{"name_or_id"},
	}
}

func (s *RunWorkflowSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	nameOrID, _ := args["name_or_id"].(string)
	if nameOrID == "" {
		return "", fmt.Errorf("name_or_id is required")
	}

	tenantID := s.tenantID
	if env != nil && env.TenantID != "" {
		tenantID = env.TenantID
	}

	// Find the workflow definition
	defs, err := s.store.ListDefinitions(tenantID)
	if err != nil {
		return "", fmt.Errorf("list workflows: %w", err)
	}

	var def *Definition
	// Exact ID match first
	for _, d := range defs {
		if d.ID == nameOrID {
			def = d
			break
		}
	}
	// Fuzzy name match
	if def == nil {
		query := strings.ToLower(nameOrID)
		for _, d := range defs {
			if strings.Contains(strings.ToLower(d.Name), query) {
				def = d
				break
			}
		}
	}
	if def == nil {
		// List available workflows in the error
		names := make([]string, 0, len(defs))
		for _, d := range defs {
			names = append(names, fmt.Sprintf("%s (%s)", d.Name, d.ID[:8]))
		}
		return "", fmt.Errorf("workflow %q not found; available: %s", nameOrID, strings.Join(names, ", "))
	}

	// Build variables
	vars := make(map[string]any)
	if v, ok := args["variables"].(map[string]any); ok {
		for k, val := range v {
			vars[k] = val
		}
	}

	// Create instance
	inst, err := s.store.CreateInstance(def.ID, tenantID, vars)
	if err != nil {
		return "", fmt.Errorf("create workflow instance: %w", err)
	}

	slog.Info("run_workflow: starting", "workflow_id", def.ID, "name", def.Name, "instance", inst.ID)

	// Run (may be async)
	if s.runFunc != nil {
		if err := s.runFunc(ctx, inst.ID); err != nil {
			return "", fmt.Errorf("run workflow %q: %w", def.Name, err)
		}
	}

	result := map[string]any{
		"instance_id":   inst.ID,
		"workflow_id":   def.ID,
		"workflow_name": def.Name,
		"status":        "started",
		"variables":     vars,
	}
	out, _ := json.Marshal(result)
	return fmt.Sprintf("工作流 %q 已启动。实例ID: %s\n详情: %s", def.Name, inst.ID[:8], string(out)), nil
}

// ──────────────────────────────────────────────
// ListWorkflowsSkill — lets the planner list saved workflows
// ──────────────────────────────────────────────

// ListWorkflowsSkill implements skills.Skill for listing saved workflows.
type ListWorkflowsSkill struct {
	store    Store
	tenantID string
}

// NewListWorkflowsSkill creates the list_workflows skill.
func NewListWorkflowsSkill(store Store) *ListWorkflowsSkill {
	return &ListWorkflowsSkill{store: store}
}

func (s *ListWorkflowsSkill) SetTenantID(id string) { s.tenantID = id }
func (s *ListWorkflowsSkill) Name() string        { return "list_workflows" }
func (s *ListWorkflowsSkill) Description() string {
	return "列出所有已保存的工作流。返回名称、ID和描述，供 run_workflow 使用。"
}
func (s *ListWorkflowsSkill) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (s *ListWorkflowsSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	tenantID := s.tenantID
	if env != nil && env.TenantID != "" {
		tenantID = env.TenantID
	}

	defs, err := s.store.ListDefinitions(tenantID)
	if err != nil {
		return "", fmt.Errorf("list workflows: %w", err)
	}
	if len(defs) == 0 {
		return "当前没有已保存的工作流。使用 save_as_workflow 保存执行步骤为工作流。", nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("已保存工作流 (%d 个):\n\n", len(defs)))
	for i, d := range defs {
		b.WriteString(fmt.Sprintf("%d. **%s** (ID: %s)\n", i+1, d.Name, d.ID[:8]))
		if d.Description != "" {
			b.WriteString(fmt.Sprintf("   描述: %s\n", d.Description))
		}
		b.WriteString(fmt.Sprintf("   节点数: %d\n", len(d.Nodes)))
	}
	return b.String(), nil
}
