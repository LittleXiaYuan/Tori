package general

import (
	"context"
	"fmt"

	"yunque-agent/internal/agentcore/workflow"
	"yunque-agent/pkg/skills"
)

type WorkflowGenSkill struct {
	store workflow.Store // shared store instance (injected from Gateway's wfStore)
}

func NewWorkflowGenSkill() *WorkflowGenSkill {
	return &WorkflowGenSkill{}
}

// SetStore wires up the shared store so generated workflows show up
// in the Gateway API immediately (same in-memory map).
func (s *WorkflowGenSkill) SetStore(st workflow.Store) {
	s.store = st
}

func (s *WorkflowGenSkill) Name() string {
	return "generate_workflow"
}

func (s *WorkflowGenSkill) Description() string {
	return "根据自然语言需求自动生成并保存有向无环图(DAG)工作流定义。当你需要帮小白用户建立业务流程、条件预警或调度任务时调用此技能。"
}

func (s *WorkflowGenSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"requirement": map[string]any{
				"type":        "string",
				"description": "用户对于工作流的详细业务需求描述，例如：'如果下雨就发短信提醒，否则就查数据库走常规审批'",
			},
		},
		"required": []string{"requirement"},
	}
}

func (s *WorkflowGenSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	req, ok := args["requirement"].(string)
	if !ok || req == "" {
		return "", fmt.Errorf("missing or invalid 'requirement'")
	}

	tenantID := "default"
	var llm workflow.LLMCallFunc
	if env != nil {
		if env.TenantID != "" {
			tenantID = env.TenantID
		}
		llm = workflow.LLMCallFunc(env.LLMCall)
	}

	result, err := workflow.GenerateDefinition(ctx, req, workflow.GeneratorOptions{
		TenantID: tenantID,
		LLMCall:  llm,
	})
	if err != nil {
		return "", err
	}

	// Use shared store if injected, otherwise fallback to local disk store.
	store := s.store
	if store == nil {
		store = workflow.NewJSONStore("data/workflows")
	}
	if err := store.SaveDefinition(result.Definition); err != nil {
		return "", fmt.Errorf("failed to save workflow definition: %w", err)
	}

	return fmt.Sprintf("工作流 '%s' (ID: %s) 已成功生成并保存。来源: %s。%s", result.Definition.Name, result.Definition.ID, result.Source, result.Message), nil
}
