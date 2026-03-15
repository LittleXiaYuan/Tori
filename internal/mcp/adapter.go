package mcp

import (
	"context"

	"yunque-agent/pkg/skills"
)

// SkillAdapter wraps an MCP tool as a yunque-agent Skill.
type SkillAdapter struct {
	tool    Tool
	gateway *Gateway
}

// NewSkillAdapter creates a skill adapter for a single MCP tool.
func NewSkillAdapter(gateway *Gateway, tool Tool) *SkillAdapter {
	return &SkillAdapter{tool: tool, gateway: gateway}
}

func (a *SkillAdapter) Name() string        { return a.tool.Name }
func (a *SkillAdapter) Description() string { return a.tool.Description }
func (a *SkillAdapter) Parameters() map[string]any {
	if a.tool.InputSchema != nil {
		return a.tool.InputSchema
	}
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

// Execute calls the MCP tool through the gateway.
func (a *SkillAdapter) Execute(ctx context.Context, args map[string]any, _ *skills.Environment) (string, error) {
	fn := a.gateway.WrapAsSkill(a.tool.Name)
	return fn(ctx, args)
}

// RegisterAll discovers all MCP tools from the gateway and registers them as skills.
func RegisterAll(gw *Gateway, registry *skills.Registry) error {
	tools, err := gw.ListTools(context.Background())
	if err != nil {
		return err
	}
	for _, tool := range tools {
		adapter := NewSkillAdapter(gw, tool)
		registry.Register(adapter)
	}
	return nil
}
