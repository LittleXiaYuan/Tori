package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"yunque-agent/internal/agentcore/workflow"
	"yunque-agent/internal/observe"
)

func main() {
	storeDir := "data/test_workflows"
	os.RemoveAll(storeDir)
	store := workflow.NewJSONStore(storeDir)

	execSkill := func(ctx context.Context, skillName string, args map[string]any) (string, error) {
		fmt.Printf("  [Executor] ⚡ 正在调用工具: %s, 参数: %v\n", skillName, args)
		time.Sleep(200 * time.Millisecond) // 模拟网络延迟
		return "SkillResult_OK", nil
	}
	execLLM := func(ctx context.Context, system, user string) (string, error) {
		fmt.Printf("  [Executor] 🧠 大模型判断天气为...\n")
		time.Sleep(300 * time.Millisecond) // 模拟 LLM 延迟
		return "rainy", nil
	}

	eng := workflow.NewEngine(store, nil, execSkill, execLLM)
	eng.OnEvent(func(evt observe.AgentEvent) {
		if detail, ok := evt.Detail.(workflow.WorkflowEvent); ok {
			fmt.Printf(">> [%s] %s | %s\n", evt.Type, detail.NodeName, detail.Message)
		}
	})

	def := &workflow.Definition{
		Name: "分支流转测试工作流",
		Nodes: []workflow.Node{
			{ID: "n1", Name: "分析天气 (LLM)", Type: workflow.NodeLLM, Config: map[string]any{"user_prompt": "外面天气怎么样？"}},
			{ID: "n2", Name: "分支 A (带伞提醒)", Type: workflow.NodeSkill, Config: map[string]any{"skill_name": "send_msg"}},
			{ID: "n3", Name: "分支 B (涂防晒霜)", Type: workflow.NodeSkill, Config: map[string]any{"skill_name": "send_msg"}},
			{ID: "n4", Name: "最终汇总 (Join)", Type: workflow.NodeJoin},
		},
		Edges: []workflow.Edge{
			{ID: "e1", FromNode: "n1", ToNode: "n2", Condition: "rainy"},  // ✅ “雨天”匹配分支 A
			{ID: "e1_2", FromNode: "n1", ToNode: "n3", Condition: "sunny"}, // ❌ “晴天”不匹配，分支 B 会被跳过
			{ID: "e2", FromNode: "n2", ToNode: "n4"},
			{ID: "e3", FromNode: "n3", ToNode: "n4"},
		},
	}
	store.SaveDefinition(def)

	fmt.Println("======================================")
	fmt.Println("🚀 创建带有【逻辑控制分支】的工作流实例...")
	inst, err := store.CreateInstance(def.ID, "tenant1", map[string]any{"input": "test"})
	if err != nil {
		panic(err)
	}

	fmt.Println("🚀 开始运行 Workflow 引擎...")
	fmt.Println("======================================")
	
	start := time.Now()
	err = eng.Run(context.Background(), inst.ID)
	
	fmt.Println("======================================")
	if err != nil {
		fmt.Printf("❌ 运行失败: %v\n", err)
	} else {
		fmt.Printf("🎯 运行成功完成! 共耗时: %v\n", time.Since(start))
	}
}
