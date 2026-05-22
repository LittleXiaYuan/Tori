package planner

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"yunque-agent/pkg/skills"
)

type skillExecutionResult struct {
	RequestedName string
	SkillName     string
	Args          map[string]any
	Output        string
	Duration      time.Duration
	Err           error
}

func (p *Planner) resolveExecutableSkill(name string, args map[string]any) (skills.Skill, map[string]any, string, bool) {
	name = strings.TrimSpace(name)
	if p.registry == nil {
		return nil, args, name, false
	}
	if skill, ok := p.registry.Get(name); ok {
		return skill, args, name, true
	}
	if !strings.HasPrefix(name, "use_") {
		return nil, args, name, false
	}
	actionName, _ := args["action"].(string)
	actionName = strings.TrimSpace(actionName)
	if actionName == "" {
		return nil, args, name, false
	}
	skill, ok := p.registry.Get(actionName)
	if !ok {
		return nil, args, actionName, false
	}
	if innerArgs, ok := args["args"].(map[string]any); ok && innerArgs != nil {
		args = innerArgs
	}
	return skill, args, actionName, true
}

func (p *Planner) executeSkill(ctx context.Context, name string, args map[string]any, env *skills.Environment) (res skillExecutionResult) {
	if args == nil {
		args = map[string]any{}
	}
	name = strings.TrimSpace(name)
	res = skillExecutionResult{RequestedName: name, SkillName: name, Args: args}
	skill, execArgs, execName, ok := p.resolveExecutableSkill(name, args)
	res.SkillName = execName
	res.Args = execArgs
	if !ok {
		res.Err = fmt.Errorf("unknown skill: %s", name)
		return res
	}
	if err := p.trustGate.Check(execName); err != nil {
		res.Err = fmt.Errorf("blocked by trust gate: %w", err)
		return res
	}
	t0 := time.Now()
	defer func() {
		if r := recover(); r != nil {
			res.Err = fmt.Errorf("tool panic: %v", r)
			slog.Error("planner: skill execution panic", "panic", r, "skill", execName)
		}
		res.Duration = time.Since(t0)
		if p.skillMetrics != nil {
			p.skillMetrics(execName, res.Duration, res.Err)
		}
		p.proactiveCog.RecordExecutionFailure(res.Err != nil)
		p.trustGate.Record(execName, res.Err == nil)
	}()
	res.Output, res.Err = skill.Execute(ctx, execArgs, env)
	return res
}
