package workflow

import (
	"fmt"
	"time"

	"yunque-agent/internal/agentcore/planner"
)

// ──────────────────────────────────────────────
// Converter — PlanStep[] → Workflow Definition
//
// Converts a Planner's execution trace ([]PlanStep) into a reusable
// Workflow Definition (DAG). Each skill call becomes a Skill node,
// LLM reasoning becomes an LLM node, and DependsOn creates edges.
//
// Usage:
//   result := planner.Run(ctx, req)
//   def := workflow.ConvertPlanToDefinition("my flow", result.Plan, tenantID)
//   store.SaveDefinition(def)
// ──────────────────────────────────────────────

// ConvertPlanToDefinition converts planner execution steps into a workflow DAG.
func ConvertPlanToDefinition(name, description string, steps []planner.PlanStep, tenantID string) *Definition {
	def := &Definition{
		Name:        name,
		Description: description,
		Version:     1,
		TenantID:    tenantID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if len(steps) == 0 {
		return def
	}

	// Map step ID → node ID
	nodeIDs := make(map[int]string, len(steps))
	for _, s := range steps {
		nodeIDs[s.ID] = fmt.Sprintf("node_%d", s.ID)
	}

	// Convert each PlanStep to a Node
	yOffset := 0.0
	for _, step := range steps {
		nodeID := nodeIDs[step.ID]

		node := Node{
			ID:       nodeID,
			Name:     stepName(step),
			Type:     stepNodeType(step),
			Config:   stepConfig(step),
			Position: Position{X: 200, Y: yOffset},
		}
		def.Nodes = append(def.Nodes, node)
		yOffset += 120

		// Create edges from DependsOn
		for _, depID := range step.DependsOn {
			fromNode, ok := nodeIDs[depID]
			if !ok {
				continue
			}
			def.Edges = append(def.Edges, Edge{
				ID:       fmt.Sprintf("edge_%s_%s", fromNode, nodeID),
				FromNode: fromNode,
				ToNode:   nodeID,
			})
		}
	}

	// If no explicit dependencies, create a linear chain
	if len(def.Edges) == 0 && len(def.Nodes) > 1 {
		for i := 0; i < len(def.Nodes)-1; i++ {
			def.Edges = append(def.Edges, Edge{
				ID:       fmt.Sprintf("edge_linear_%d", i),
				FromNode: def.Nodes[i].ID,
				ToNode:   def.Nodes[i+1].ID,
			})
		}
	}

	return def
}

// stepName generates a human-readable node name from a PlanStep.
func stepName(step planner.PlanStep) string {
	if step.Skill != "" {
		return step.Skill
	}
	if step.Action != "" {
		action := step.Action
		if len(action) > 40 {
			action = action[:40] + "..."
		}
		return action
	}
	return fmt.Sprintf("Step %d", step.ID)
}

// stepNodeType maps a PlanStep to a workflow NodeType.
func stepNodeType(step planner.PlanStep) NodeType {
	if step.Skill != "" {
		return NodeSkill
	}
	return NodeLLM
}

// stepConfig builds the node config map from a PlanStep.
func stepConfig(step planner.PlanStep) map[string]any {
	cfg := make(map[string]any)

	if step.Skill != "" {
		cfg["skill_name"] = step.Skill
		if len(step.Args) > 0 {
			cfg["args_template"] = step.Args
		}
	} else {
		// LLM node: use the action description as the prompt template
		if step.Action != "" {
			cfg["system_prompt"] = "你是一个智能助手。"
			cfg["user_prompt"] = step.Action
		}
	}

	return cfg
}
