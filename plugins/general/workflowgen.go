package general

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/workflow"
	"yunque-agent/pkg/skills"
)

type WorkflowGenSkill struct {
	store workflow.Store // shared store instance (injected from Gateway's wfStore)
}

func NewWorkflowGenSkill() *WorkflowGenSkill {
	return &WorkflowGenSkill{}
}

// SetStore injects the shared workflow store so generated definitions
// are immediately visible to the Gateway API (same in-memory map).
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

const sysPrompt = `你是一个严谨且经验丰富的工作流(DAG)架构生成引擎。
你的任务是将自然语言需求转换为严格的 Workflow Definition JSON 数据结构。

支持的节点类型(type):
1. "llm" - 大模型推理 (config: {"user_prompt": "..."})
2. "skill" - 内部工具/API调用 (config: {"skill_name": "send_msg" 等等})
3. "condition" - 条件分支节点 (根据前置节点结果判断进行分支流转)
4. "parallel" - 并行执行控制 (可以多条引出)
5. "join" - 多个并行执行节点后的汇聚点等待

必须返回纯 JSON，不能有任何多余字符、注释、或 Markdown(如 ` + "`" + "`" + "`" + `json 等)包围符号。
JSON 格式示范如下:
{
  "name": "极简天气通知流程",
  "description": "天意下雨就发通知",
  "nodes": [
    {
      "id": "n1",
      "name": "开始判断(LLM)",
      "type": "llm",
      "config": {"user_prompt": "今天天气如何？"},
      "position": {"x": 250, "y": 100}
    },
    {
      "id": "n2",
      "name": "带伞提醒",
      "type": "skill",
      "config": {"skill_name": "send_msg"},
      "position": {"x": 100, "y": 300}
    }
  ],
  "edges": [
    {
      "id": "e1",
      "from_node": "n1",
      "to_node": "n2",
      "condition": "rainy" 
    }
  ]
}

深吸一口气，开始根据用户的 requirement 生成你的 JSON：`

func (s *WorkflowGenSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	req, ok := args["requirement"].(string)
	if !ok || req == "" {
		return "", fmt.Errorf("missing or invalid 'requirement'")
	}

	if env.LLMCall == nil {
		return "", fmt.Errorf("LLMCall endpoint is not available in current environment")
	}

	jsonStr, err := env.LLMCall(ctx, sysPrompt, req)
	if err != nil {
		return "", fmt.Errorf("LLM underlying call failed to generate DAG workflow: %w", err)
	}

	jsonStr = strings.TrimSpace(jsonStr)
	jsonStr = strings.TrimPrefix(jsonStr, "```json")
	jsonStr = strings.TrimPrefix(jsonStr, "```")
	jsonStr = strings.TrimSuffix(jsonStr, "```")
	jsonStr = strings.TrimSpace(jsonStr)

	var def workflow.Definition
	if err := json.Unmarshal([]byte(jsonStr), &def); err != nil {
		return "", fmt.Errorf("failed to parse LLM JSON structure: %w\nOutput was: %s", err, jsonStr)
	}

	if def.ID == "" {
		def.ID = fmt.Sprintf("wf_%d", time.Now().UnixNano())
	}
	def.Version = 1
	def.TenantID = env.TenantID
	if def.TenantID == "" {
		def.TenantID = "default"
	}

	// 赋予粗略的 y 轴坐标使得进入前端不会堆叠在一起
	for i, n := range def.Nodes {
		if n.Position.X == 0 && n.Position.Y == 0 {
			def.Nodes[i].Position.X = float64(250 + (i%2)*200)
			def.Nodes[i].Position.Y = float64(100 + i*150)
		}
	}

	// Use shared store if injected, otherwise fallback to local disk store
	store := s.store
	if store == nil {
		store = workflow.NewJSONStore("data/workflows")
	}
	if err := store.SaveDefinition(&def); err != nil {
		return "", fmt.Errorf("failed to save workflow definition: %w", err)
	}

	return fmt.Sprintf("工作流 '%s' (ID: %s) 已成功生成并保存。用户可进入工作流面板查看或编辑。", def.Name, def.ID), nil
}

