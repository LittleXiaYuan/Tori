package connectors

import (
	"context"
	"encoding/json"
	"fmt"

	"yunque-agent/pkg/skills"
)

// RegisterSkills creates a dynamic skill for each action of each connected connector.
// Called once at init; the skills check connection status at execution time.
func RegisterSkills(reg *skills.Registry, connReg *Registry) int {
	count := 0
	for _, def := range connReg.ListDefs() {
		for _, action := range def.Actions {
			sk := &connectorSkill{
				connReg:     connReg,
				connectorID: def.ID,
				connName:    def.Name,
				actionDef:   action,
			}
			reg.Register(sk)
			count++
		}
	}
	return count
}

type connectorSkill struct {
	connReg     *Registry
	connectorID string
	connName    string
	actionDef   ActionDef
}

func (s *connectorSkill) Name() string {
	return fmt.Sprintf("connector_%s_%s", s.connectorID, s.actionDef.ID)
}

func (s *connectorSkill) Description() string {
	return fmt.Sprintf("[%s] %s", s.connName, s.actionDef.Description)
}

func (s *connectorSkill) Parameters() map[string]any {
	props := map[string]any{}
	var required []string
	for _, p := range s.actionDef.Parameters {
		props[p.Name] = map[string]any{"type": p.Type, "description": p.Description}
		if p.Required {
			required = append(required, p.Name)
		}
	}
	schema := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func (s *connectorSkill) Execute(ctx context.Context, args map[string]any, _ *skills.Environment) (string, error) {
	inst := s.connReg.GetInstance(s.connectorID)
	if inst.Status != StatusConnected {
		return fmt.Sprintf("%s connector is not connected. Please connect it first in Settings > Connectors.", s.connName), nil
	}

	result, err := s.connReg.Execute(ctx, s.connectorID, s.actionDef.ID, args)
	if err != nil {
		return "", err
	}

	data, _ := json.Marshal(result)

	if len(data) > 100000 {
		return string(data[:100000]) + "\n... (truncated)", nil
	}
	return string(data), nil
}
