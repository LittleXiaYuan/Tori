package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/localbrain"
	"yunque-agent/internal/agentcore/plan"
	"yunque-agent/internal/observe"
)

func (p *Planner) runLongHorizon(ctx context.Context, req PlanRequest) (*PlanResult, error) {
	execFn := p.buildStepExecutor(req)
	mgr := plan.NewManager(nil, execFn)
	mgr.SetDecomposeDAG(p.buildDecomposeDAG(req))
	mgr.SetRevise(p.buildReviseFunc(req))
	mgr.SetOnStepUpdate(func(pl *plan.Plan, idx int, status plan.StepStatus) {
		if req.StepCallback == nil {
			return
		}
		step := pl.Steps[idx]
		c, total := pl.Progress()
		var evt observe.AgentEvent
		switch status {
		case plan.StepInProgress:
			evt = observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventThinking,
				fmt.Sprintf("📋 步骤 %d/%d: %s", idx+1, total, step.Description))
		case plan.StepCompleted:
			evt = observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventToolResult,
				fmt.Sprintf("步骤 %d/%d 完成 (%d/%d)", idx+1, total, c, total))
		case plan.StepFailed:
			evt = observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventToolResult,
				fmt.Sprintf("步骤 %d/%d 失败: %s", idx+1, total, step.Error))
		default:
			return
		}
		evt.Meta.TenantID = req.TenantID
		evt.Meta.TaskID = req.TaskID
		req.StepCallback(evt)
	})

	budget := plan.Budget{MaxSteps: p.maxSteps * 3, MaxRevisions: 3, MaxDuration: 5 * time.Minute}
	pl, err := mgr.CreateDAG(ctx, extractGoal(req), budget)
	if err != nil {
		slog.Warn("planner: DAG decompose failed, falling back", "err", err)
		return p.runNativeFC(ctx, req)
	}

	if req.StepCallback != nil {
		evt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventPlan,
			fmt.Sprintf("📋 规划完成：%d 个步骤", len(pl.Steps)))
		evt.Meta.TenantID = req.TenantID
		req.StepCallback(evt)
	}

	if err = mgr.ExecuteDAG(ctx, pl.ID); err != nil {
		return &PlanResult{Reply: fmt.Sprintf("任务部分完成：%v", err), Steps: pl.Budget.StepsUsed}, nil
	}

	reply := pl.Summary
	if reply == "" {
		reply = p.synthesizePlanResult(ctx, req, pl)
	}

	var usedSkills []string
	var planSteps []PlanStep
	for _, s := range pl.Steps {
		usedSkills = append(usedSkills, s.ToolsUsed...)
		planSteps = append(planSteps, PlanStep{
			ID: s.Index, Action: s.Description, Skill: s.Skill, Args: s.Args,
			DependsOn: s.DependsOn, Status: convertStatus(s.Status),
			Result: s.Output, Error: s.Error,
		})
	}
	_, ctxLayers := p.BuildMessages(ctx, req)
	return &PlanResult{Reply: reply, SkillsUsed: usedSkills, Steps: pl.Budget.StepsUsed, Plan: planSteps, ContextLayers: ctxLayers}, nil
}

func (p *Planner) buildDecomposeDAG(req PlanRequest) plan.DecomposeDAGFunc {
	return func(ctx context.Context, goal string) ([]plan.PlanStep, error) {
		skillList := p.buildSkillListForDecompose()
		prompt := fmt.Sprintf(`将目标分解为步骤。可用工具：
%s
目标：%s
返回 JSON 数组：[{"description":"","skill":"","args":{},"depends_on":[]}]
规则：独立步骤不加依赖，3-8步，只返回JSON`, skillList, goal)

		client := p.clientForRequest(req)
		reply, err := client.Chat(ctx, []llm.Message{
			{Role: "system", Content: "你是任务规划器，只输出 JSON 数组。"},
			{Role: "user", Content: prompt},
		}, 0.3)
		if err != nil {
			return nil, err
		}
		return parseDAGSteps(reply)
	}
}

func (p *Planner) buildReviseFunc(req PlanRequest) plan.ReviseFunc {
	return func(ctx context.Context, goal string, current *plan.Plan, failedStep int) ([]plan.PlanStep, error) {
		prompt := fmt.Sprintf("任务: %s\n状态:\n%s\n步骤 %d 失败，重新规划剩余部分。返回JSON数组。",
			goal, current.StepSummary(), failedStep)
		client := p.clientForRequest(req)
		reply, err := client.Chat(ctx, []llm.Message{
			{Role: "system", Content: "你是任务规划器，根据失败提出替代方案，只输出JSON数组。"},
			{Role: "user", Content: prompt},
		}, 0.4)
		if err != nil {
			return nil, err
		}
		return parseDAGSteps(reply)
	}
}

func (p *Planner) buildStepExecutor(req PlanRequest) plan.ExecuteStepFunc {
	env := p.buildEnv(req)
	return func(ctx context.Context, pl *plan.Plan, stepIndex int) (string, []string, error) {
		step := pl.Steps[stepIndex]
		if step.Skill == "" {
			return p.executeReasoningStep(ctx, req, pl, stepIndex)
		}
		skill, ok := p.registry.Get(step.Skill)
		if !ok {
			return "", nil, fmt.Errorf("unknown skill: %s", step.Skill)
		}
		if p.trustCheck != nil {
			if err := p.trustCheck(step.Skill); err != nil {
				return "", nil, fmt.Errorf("blocked: %w", err)
			}
		}
		args := make(map[string]any)
		for k, v := range step.Args {
			args[k] = v
		}
		t0 := time.Now()
		result, err := skill.Execute(ctx, args, env)
		dur := time.Since(t0)
		if p.skillMetrics != nil {
			p.skillMetrics(step.Skill, dur, err)
		}
		if p.trustRecord != nil {
			p.trustRecord(step.Skill, err == nil)
		}
		if err != nil {
			return "", []string{step.Skill}, err
		}
		return result, []string{step.Skill}, nil
	}
}

func (p *Planner) executeReasoningStep(ctx context.Context, req PlanRequest, pl *plan.Plan, stepIndex int) (string, []string, error) {
	step := pl.Steps[stepIndex]
	prompt := step.Description
	for _, dep := range step.DependsOn {
		if dep >= 0 && dep < len(pl.Steps) && pl.Steps[dep].Output != "" {
			out := pl.Steps[dep].Output
			if len([]rune(out)) > 500 {
				out = string([]rune(out)[:500]) + "..."
			}
			prompt += fmt.Sprintf("\n[步骤%d结果]: %s", dep, out)
		}
	}

	// AgenticThinking: 自适应选择模型层级
	selectedTier := req.ModelOverride
	if selectedTier == "" && p.agenticThinking != nil {
		thinkReq := localbrain.ThinkRequest{
			TaskID:   pl.ID,
			TenantID: req.TenantID,
			Query:    step.Description,
			StepIndex: stepIndex,
		}
		if agResult, err := p.agenticThinking.Think(ctx, thinkReq); err == nil {
			switch agResult.Level {
			case localbrain.ThinkQuick:
				selectedTier = "fast"
			case localbrain.ThinkDeep:
				selectedTier = "expert"
			default:
				selectedTier = "smart"
			}
		}
	}

	var client *llm.Client
	if req.ClientOverride != nil {
		client = req.ClientOverride
	} else {
		client = p.LLMClientFor(selectedTier)
	}
	reply, err := client.Chat(ctx, []llm.Message{
		{Role: "system", Content: "基于信息完成分析，直接给出结果。"},
		{Role: "user", Content: prompt},
	}, 0.7)
	if err != nil {
		return "", nil, err
	}
	return reply, nil, nil
}

func (p *Planner) synthesizePlanResult(ctx context.Context, req PlanRequest, pl *plan.Plan) string {
	var results string
	for _, s := range pl.Steps {
		if s.Status == plan.StepCompleted && s.Output != "" {
			out := s.Output
			if len([]rune(out)) > 300 {
				out = string([]rune(out)[:300]) + "..."
			}
			results += fmt.Sprintf("- %s: %s\n", s.Description, out)
		}
	}
	if results == "" {
		return "任务已执行完毕。"
	}
	client := p.clientForRequest(req)
	reply, err := client.Chat(ctx, []llm.Message{
		{Role: "system", Content: "根据执行结果给出完整回复。Markdown格式。"},
		{Role: "user", Content: fmt.Sprintf("目标: %s\n结果:\n%s", pl.Task, results)},
	}, 0.7)
	if err != nil {
		return "任务已完成。\n\n" + results
	}
	return reply
}

func (p *Planner) buildSkillListForDecompose() string {
	var list string
	for _, s := range p.registry.All() {
		list += fmt.Sprintf("- %s: %s\n", s.Name(), s.Description())
	}
	return list
}

func parseDAGSteps(reply string) ([]plan.PlanStep, error) {
	raw := extractJSONArray(reply)
	if raw == "" {
		return nil, fmt.Errorf("no JSON array in response")
	}
	var parsed []struct {
		Description string         `json:"description"`
		Skill       string         `json:"skill"`
		Args        map[string]any `json:"args"`
		DependsOn   []int          `json:"depends_on"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, err
	}
	steps := make([]plan.PlanStep, len(parsed))
	for i, s := range parsed {
		steps[i] = plan.PlanStep{
			Index: i, Description: s.Description, Skill: s.Skill,
			Args: s.Args, DependsOn: s.DependsOn, Status: plan.StepPending,
		}
	}
	return steps, nil
}

func extractJSONArray(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '[' {
			depth := 0
			for j := i; j < len(s); j++ {
				if s[j] == '[' {
					depth++
				} else if s[j] == ']' {
					depth--
					if depth == 0 {
						return s[i : j+1]
					}
				}
			}
		}
	}
	return ""
}

func convertStatus(s plan.StepStatus) StepStatus {
	switch s {
	case plan.StepCompleted:
		return StepDone
	case plan.StepFailed:
		return StepFailed
	case plan.StepSkipped:
		return StepSkipped
	case plan.StepInProgress:
		return StepRunning
	default:
		return StepPending
	}
}

func extractGoal(req PlanRequest) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			return req.Messages[i].Content
		}
	}
	if len(req.Messages) > 0 {
		return req.Messages[len(req.Messages)-1].Content
	}
	return ""
}
