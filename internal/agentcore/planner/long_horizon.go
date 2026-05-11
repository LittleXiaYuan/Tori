package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
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
		cp := buildLongHorizonCheckpoint(req, pl, "")
		p.persistLongHorizonCheckpoint(req, cp)
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
				plannerStepFailureSummary("步骤", idx, total, step.Error))
		default:
			return
		}
		evt.Meta.TenantID = req.TenantID
		evt.Meta.TaskID = req.TaskID
		evt.Detail = friendlyLongHorizonCheckpoint(cp)
		req.StepCallback(evt)
	})

	budget := plan.Budget{MaxSteps: p.maxSteps * 3, MaxRevisions: 3, MaxDuration: 5 * time.Minute}
	pl, err := mgr.CreateDAG(ctx, extractGoal(req), budget)
	if err != nil {
		slog.Warn("planner: DAG decompose failed, falling back", "err", err)
		return p.runNativeFC(ctx, req)
	}

	createdCheckpoint := buildLongHorizonCheckpoint(req, pl, "")
	p.persistLongHorizonCheckpoint(req, createdCheckpoint)
	if req.StepCallback != nil {
		evt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventPlan,
			fmt.Sprintf("📋 规划完成：%d 个步骤", len(pl.Steps)))
		evt.Meta.TenantID = req.TenantID
		evt.Meta.TaskID = req.TaskID
		evt.Detail = friendlyLongHorizonCheckpoint(createdCheckpoint)
		req.StepCallback(evt)
	}

	if err = mgr.ExecuteDAG(ctx, pl.ID); err != nil {
		p.emitLongHorizonCheckpoint(req, pl, err.Error())
		return &PlanResult{
			Reply:      longHorizonPartialReply(err.Error()),
			SkillsUsed: usedSkillsFromDAG(pl),
			Steps:      pl.Budget.StepsUsed,
			Plan:       planStepsFromDAG(pl, 2000),
		}, nil
	}

	reply := pl.Summary
	if reply == "" {
		reply = p.synthesizePlanResult(ctx, req, pl)
	}

	usedSkills := usedSkillsFromDAG(pl)
	planSteps := planStepsFromDAG(pl, 2000)
	_, ctxLayers := p.BuildMessages(ctx, req)
	return &PlanResult{Reply: reply, SkillsUsed: usedSkills, Steps: pl.Budget.StepsUsed, Plan: planSteps, ContextLayers: ctxLayers}, nil
}

// ResumeLongHorizonCheckpoint rebuilds a DAG plan from a persisted checkpoint
// and executes only the safe remaining scope. Completed/skipped checkpoint
// steps keep their output and are never re-run; selected pending/failed steps
// are reset to pending and executed under the normal DAG dependency gate.
func (p *Planner) ResumeLongHorizonCheckpoint(ctx context.Context, req PlanRequest, cp LongHorizonCheckpoint, action string) (*PlanResult, error) {
	action = normalizeCheckpointResumeAction(action)
	if action == "partial" {
		return partialCheckpointResult(cp), nil
	}
	steps, err := rebuildCheckpointDAGSteps(cp, action)
	if err != nil {
		return nil, err
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("planner: checkpoint has no resumable steps")
	}
	req = requestWithCheckpointGoal(req, cp)

	execFn := p.buildStepExecutor(req)
	mgr := plan.NewManager(nil, execFn)
	mgr.SetDecomposeDAG(func(context.Context, string) ([]plan.PlanStep, error) {
		return cloneDAGSteps(steps), nil
	})
	mgr.SetRevise(p.buildReviseFunc(req))
	mgr.SetOnStepUpdate(func(pl *plan.Plan, idx int, status plan.StepStatus) {
		currentCheckpoint := resumedLongHorizonCheckpoint(req, cp, pl, "")
		p.persistLongHorizonCheckpoint(req, currentCheckpoint)
		if req.StepCallback == nil {
			return
		}
		step := pl.Steps[idx]
		c, total := pl.Progress()
		var evt observe.AgentEvent
		switch status {
		case plan.StepInProgress:
			evt = observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventThinking,
				fmt.Sprintf("📋 恢复步骤 %d/%d: %s", idx+1, total, step.Description))
		case plan.StepCompleted:
			evt = observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventToolResult,
				fmt.Sprintf("恢复步骤 %d/%d 完成 (%d/%d)", idx+1, total, c, total))
		case plan.StepFailed:
			evt = observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventToolResult,
				plannerStepFailureSummary("恢复步骤", idx, total, step.Error))
		default:
			return
		}
		evt.Meta.TenantID = req.TenantID
		evt.Meta.TaskID = req.TaskID
		evt.Detail = friendlyLongHorizonCheckpoint(currentCheckpoint)
		req.StepCallback(evt)
	})

	maxSteps := p.maxSteps
	if maxSteps <= 0 {
		maxSteps = 15
	}
	budget := plan.Budget{
		MaxSteps:      cp.StepsUsed + maxSteps*3,
		MaxRevisions:  3,
		MaxDuration:   5 * time.Minute,
		StepsUsed:     cp.StepsUsed,
		RevisionsUsed: cp.Revisions,
	}
	pl, err := mgr.CreateDAG(ctx, checkpointResumeGoal(req, cp), budget)
	if err != nil {
		return nil, err
	}
	pl.Steps = cloneDAGSteps(steps)
	pl.Revisions = cp.Revisions

	createdCheckpoint := resumedLongHorizonCheckpoint(req, cp, pl, "")
	p.persistLongHorizonCheckpoint(req, createdCheckpoint)
	if req.StepCallback != nil {
		evt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventPlan,
			fmt.Sprintf("📋 已从恢复点继续：%d 个步骤", len(pl.Steps)))
		evt.Meta.TenantID = req.TenantID
		evt.Meta.TaskID = req.TaskID
		evt.Detail = friendlyLongHorizonCheckpoint(createdCheckpoint)
		req.StepCallback(evt)
	}

	if err = mgr.ExecuteDAG(ctx, pl.ID); err != nil {
		p.emitResumedLongHorizonCheckpoint(req, cp, pl, err.Error())
		return &PlanResult{
			Reply:      longHorizonPartialReply(err.Error()),
			SkillsUsed: usedSkillsFromDAG(pl),
			Steps:      pl.Budget.StepsUsed,
			Plan:       planStepsFromDAG(pl, 2000),
		}, nil
	}
	reply := pl.Summary
	if reply == "" {
		reply = p.synthesizePlanResult(ctx, req, pl)
	}
	p.persistLongHorizonCheckpoint(req, resumedLongHorizonCheckpoint(req, cp, pl, ""))
	_, ctxLayers := p.BuildMessages(ctx, req)
	return &PlanResult{Reply: reply, SkillsUsed: usedSkillsFromDAG(pl), Steps: pl.Budget.StepsUsed, Plan: planStepsFromDAG(pl, 2000), ContextLayers: ctxLayers}, nil
}

func resumedLongHorizonCheckpoint(req PlanRequest, source LongHorizonCheckpoint, pl *plan.Plan, errText string) LongHorizonCheckpoint {
	cp := buildLongHorizonCheckpoint(req, pl, errText)
	if strings.TrimSpace(source.PlanID) != "" {
		cp.PlanID = source.PlanID
	}
	if strings.TrimSpace(source.TaskID) != "" {
		cp.TaskID = source.TaskID
	}
	if strings.TrimSpace(source.Goal) != "" {
		cp.Goal = source.Goal
	}
	return cp
}

func (p *Planner) emitResumedLongHorizonCheckpoint(req PlanRequest, source LongHorizonCheckpoint, pl *plan.Plan, errText string) {
	if pl == nil {
		return
	}
	cp := resumedLongHorizonCheckpoint(req, source, pl, errText)
	p.persistLongHorizonCheckpoint(req, cp)
	if req.StepCallback == nil {
		return
	}
	summary := fmt.Sprintf("长程规划进度：%d/%d", cp.Completed, cp.Total)
	if errText != "" {
		summary = "长程规划已保存失败现场，可继续恢复"
	}
	evt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventPlan, summary)
	evt.Meta.TenantID = req.TenantID
	evt.Meta.TaskID = req.TaskID
	evt.Detail = friendlyLongHorizonCheckpoint(cp)
	req.StepCallback(evt)
}

func (p *Planner) buildDecomposeDAG(req PlanRequest) plan.DecomposeDAGFunc {
	return func(ctx context.Context, goal string) ([]plan.PlanStep, error) {
		skillList := p.buildSkillListForDecomposeWithAllow(allowedSkillSet(req.AllowedSkills))
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
		steps, err := parseDAGSteps(reply)
		if err != nil {
			return nil, err
		}
		return ensureInitialDAGMinimumSteps(steps), nil
	}
}

func (p *Planner) buildReviseFunc(req PlanRequest) plan.ReviseFunc {
	return func(ctx context.Context, goal string, current *plan.Plan, failedStep int) ([]plan.PlanStep, error) {
		status := friendlyPlanStepSummary(current)
		prompt := fmt.Sprintf("任务: %s\n状态:\n%s\n步骤 %d 失败，重新规划剩余部分。返回JSON数组。",
			goal, status, failedStep)
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

func friendlyPlanStepSummary(pl *plan.Plan) string {
	if pl == nil {
		return ""
	}
	var b strings.Builder
	for _, s := range pl.Steps {
		line := fmt.Sprintf("[%d] %s — %s: %s", s.Index, s.Status, s.Description, s.Skill)
		if s.Output != "" {
			out := plannerFriendlyOutputForModel(s.Output)
			if len([]rune(out)) > 100 {
				out = string([]rune(out)[:100]) + "..."
			}
			line += " → " + out
		}
		if s.Error != "" {
			line += " ✗ " + plannerFriendlyFailureText(s.Error)
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func (p *Planner) buildStepExecutor(req PlanRequest) plan.ExecuteStepFunc {
	env := p.buildEnv(req)
	allowed := allowedSkillSet(req.AllowedSkills)
	return func(ctx context.Context, pl *plan.Plan, stepIndex int) (string, []string, error) {
		step := pl.Steps[stepIndex]
		if step.Skill == "" {
			return p.executeReasoningStep(ctx, req, pl, stepIndex)
		}
		if len(allowed) > 0 && !allowed[step.Skill] {
			return "", []string{step.Skill}, fmt.Errorf("skill %q is not in the allowed tool surface", step.Skill)
		}
		args := make(map[string]any)
		for k, v := range step.Args {
			args[k] = v
		}
		if evidence := completedDependencyEvidence(pl, step); evidence != "" {
			if _, exists := args["dependency_results"]; !exists {
				args["dependency_results"] = evidence
			}
		}
		exec := p.executeSkill(ctx, step.Skill, args, env)
		if exec.Err != nil {
			return "", []string{exec.SkillName}, exec.Err
		}
		return exec.Output, []string{exec.SkillName}, nil
	}
}

func completedDependencyEvidence(pl *plan.Plan, step plan.PlanStep) string {
	if pl == nil || len(step.DependsOn) == 0 {
		return ""
	}
	var b strings.Builder
	for _, dep := range step.DependsOn {
		if dep < 0 || dep >= len(pl.Steps) {
			continue
		}
		depStep := pl.Steps[dep]
		if depStep.Status != plan.StepCompleted && depStep.Status != plan.StepSkipped {
			continue
		}
		out := plannerFriendlyOutputForModel(depStep.Output)
		if out == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		if len([]rune(out)) > 1200 {
			out = string([]rune(out)[:1200]) + "..."
		}
		label := strings.TrimSpace(depStep.Description)
		if label == "" {
			label = fmt.Sprintf("步骤%d", dep)
		}
		b.WriteString(fmt.Sprintf("[步骤%d %s]: %s", dep, label, out))
	}
	return b.String()
}

func (p *Planner) executeReasoningStep(ctx context.Context, req PlanRequest, pl *plan.Plan, stepIndex int) (string, []string, error) {
	step := pl.Steps[stepIndex]
	prompt := step.Description
	for _, dep := range step.DependsOn {
		if dep >= 0 && dep < len(pl.Steps) && pl.Steps[dep].Output != "" {
			out := plannerFriendlyOutputForModel(pl.Steps[dep].Output)
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
			TaskID:    pl.ID,
			TenantID:  req.TenantID,
			Query:     step.Description,
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
			out := plannerFriendlyOutputForModel(s.Output)
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
	if client == nil {
		return "任务已完成。\n\n" + results
	}
	reply, err := client.Chat(ctx, []llm.Message{
		{Role: "system", Content: "根据执行结果给出完整回复。Markdown格式。"},
		{Role: "user", Content: fmt.Sprintf("目标: %s\n结果:\n%s", pl.Task, results)},
	}, 0.7)
	if err != nil {
		return "任务已完成。\n\n" + results
	}
	return reply
}

func plannerFriendlyOutputForModel(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}
	if !plannerOutputLooksDiagnostic(output) {
		return output
	}
	friendly := plannerFriendlyFailureText(output)
	if friendly == "" {
		return "现场已保留，可继续恢复。"
	}
	return friendly
}

func plannerOutputLooksDiagnostic(output string) bool {
	normalized := strings.ToLower(output)
	return strings.Contains(normalized, "handoff agent") ||
		strings.Contains(normalized, "execution failed") ||
		strings.Contains(normalized, "context deadline exceeded") ||
		strings.Contains(normalized, "context canceled") ||
		strings.Contains(normalized, "context cancelled") ||
		strings.Contains(normalized, "fallback") ||
		strings.Contains(normalized, "eof") ||
		strings.Contains(normalized, "tool panic")
}

func (p *Planner) buildSkillListForDecompose() string {
	return p.buildSkillListForDecomposeWithAllow(nil)
}

func (p *Planner) buildSkillListForDecomposeWithAllow(allowed map[string]bool) string {
	var list string
	for _, s := range p.registry.All() {
		if len(allowed) > 0 && !allowed[s.Name()] {
			continue
		}
		list += fmt.Sprintf("- %s: %s\n", s.Name(), s.Description())
	}
	return list
}

func allowedSkillSet(names []string) map[string]bool {
	if len(names) == 0 {
		return nil
	}
	set := make(map[string]bool, len(names))
	for _, name := range names {
		if name = strings.TrimSpace(name); name != "" {
			set[name] = true
		}
	}
	return set
}

func plannerStepFailureSummary(label string, idx, total int, rawErr string) string {
	return fmt.Sprintf("%s %d/%d 暂停：%s", label, idx+1, total, plannerFriendlyFailureText(rawErr))
}

func longHorizonPartialReply(rawErr string) string {
	return "任务已部分执行，" + plannerFriendlyFailureText(rawErr)
}

func plannerFriendlyFailureText(rawErr string) string {
	normalized := strings.ToLower(strings.TrimSpace(rawErr))
	switch {
	case normalized == "":
		return "现场已保留，可从恢复点继续或返回阶段结果。"
	case strings.Contains(normalized, "unknown skill"):
		return "所需工具暂时不可用，现场已保留，可换用可用工具或调整步骤继续。"
	case strings.Contains(normalized, "blocked by trust gate") || strings.Contains(normalized, "trust gate"):
		return "这一步需要更高信任或确认，现场已保留，可确认后继续。"
	case strings.Contains(normalized, "tool panic") || strings.Contains(normalized, "panic"):
		return "工具运行时遇到异常，现场已保留，可重试或切换策略继续。"
	case strings.Contains(normalized, "context canceled"),
		strings.Contains(normalized, "context cancelled"):
		return "连接暂时中断，现场已保留，可稍后继续或先查看阶段结果。"
	case strings.Contains(normalized, "context deadline exceeded"),
		strings.Contains(normalized, "deadline exceeded"),
		strings.Contains(normalized, "timeout"),
		strings.Contains(normalized, "timed out"),
		strings.Contains(normalized, "响应超时"),
		strings.Contains(normalized, "超时"):
		return "等待时间过长，现场已保留，可稍后重试或改为后台任务。"
	case strings.Contains(normalized, "handoff agent"),
		strings.Contains(normalized, "execution failed"),
		strings.Contains(normalized, "all fallback"),
		strings.Contains(normalized, "fallback llm"),
		strings.Contains(normalized, "eof"):
		return "当前模型连接不稳定，现场已保留，可切换为后台任务或稍后重试。"
	case strings.Contains(normalized, "dependency"),
		strings.Contains(normalized, "depend"),
		strings.Contains(normalized, "no ready steps"):
		return "前置步骤还未满足，现场已保留，请先检查依赖关系。"
	case strings.Contains(normalized, "allowed tool surface"):
		return "所需工具暂未开放，现场已保留，可调整工具范围后继续。"
	default:
		return "现场已保留，可从恢复点继续或返回阶段结果。"
	}
}

func parseDAGSteps(reply string) ([]plan.PlanStep, error) {
	candidates := extractJSONArrays(reply)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no JSON array in response")
	}
	var parsed []dagStepDraft
	var lastErr error
	for _, raw := range candidates {
		parsed = nil
		var err error
		parsed, err = parseDAGStepDrafts(raw)
		if err != nil {
			lastErr = err
			continue
		}
		if len(parsed) == 0 {
			lastErr = fmt.Errorf("empty plan steps")
			continue
		}
		if !hasPlanLikeStep(parsed) {
			lastErr = fmt.Errorf("json array does not contain plan steps")
			continue
		}
		break
	}
	if len(parsed) == 0 {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, fmt.Errorf("no valid plan steps in response")
	}
	parsed = capDAGStepDrafts(parsed, maxLongHorizonDAGSteps)
	steps := make([]plan.PlanStep, len(parsed))
	for i, s := range parsed {
		steps[i] = plan.PlanStep{
			Index: i, Description: s.Description, Skill: s.Skill,
			Args: s.Args, DependsOn: normalizeDAGDepends(i, len(parsed), s.DependsOn), Status: plan.StepPending,
		}
	}
	return steps, nil
}

type dagStepDraft struct {
	Description string
	Skill       string
	Args        map[string]any
	DependsOn   []int
}

const (
	minLongHorizonDAGSteps = 3
	maxLongHorizonDAGSteps = 8
)

func capDAGStepDrafts(steps []dagStepDraft, max int) []dagStepDraft {
	if max <= 0 || len(steps) <= max {
		return steps
	}
	return steps[:max]
}

func ensureInitialDAGMinimumSteps(steps []plan.PlanStep) []plan.PlanStep {
	for len(steps) > 0 && len(steps) < minLongHorizonDAGSteps {
		deps := make([]int, 0, len(steps))
		for i := range steps {
			deps = append(deps, i)
		}
		steps = append(steps, plan.PlanStep{
			Index:       len(steps),
			Description: "汇总已完成步骤，确认结果并给出下一步最小动作",
			DependsOn:   deps,
			Status:      plan.StepPending,
		})
	}
	return steps
}

func parseDAGStepDrafts(raw string) ([]dagStepDraft, error) {
	var items []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, err
	}
	steps := make([]dagStepDraft, len(items))
	for i, item := range items {
		steps[i] = normalizeDAGStepDraft(item)
	}
	return steps, nil
}

func normalizeDAGStepDraft(item map[string]json.RawMessage) dagStepDraft {
	argsRaw := firstRaw(item["args"], item["arguments"], item["params"], item["input"])
	if fnRaw := firstRaw(item["function"], item["tool_call"], item["function_call"]); len(fnRaw) > 0 {
		if call, ok := parseRawSkillCall(fnRaw); ok {
			if len(argsRaw) == 0 && len(call.Args) > 0 {
				if rawArgs, err := json.Marshal(call.Args); err == nil {
					argsRaw = rawArgs
				}
			}
			if !hasAnyRaw(item, "tool", "tool_name", "skill", "skill_name") {
				if rawName, err := json.Marshal(call.Name); err == nil {
					item["tool"] = rawName
				}
			}
		}
		var fn map[string]json.RawMessage
		if json.Unmarshal(fnRaw, &fn) == nil {
			if len(argsRaw) == 0 {
				argsRaw = firstRaw(fn["args"], fn["arguments"], fn["params"], fn["input"])
			}
			if !hasAnyRaw(item, "tool", "tool_name", "skill", "skill_name") {
				item["tool"] = firstRaw(fn["name"], fn["tool"], fn["tool_name"], fn["skill"], fn["skill_name"])
			}
		}
	}
	return dagStepDraft{
		Description: firstJSONString(item, "description", "task", "step", "action", "name"),
		Skill:       firstJSONString(item, "skill", "skill_name", "tool", "tool_name"),
		Args:        parseSkillCallArgs(argsRaw),
		DependsOn:   parseDAGDepends(firstRaw(item["depends_on"], item["depends"], item["dependencies"], item["dependsOn"])),
	}
}

func hasAnyRaw(item map[string]json.RawMessage, keys ...string) bool {
	for _, key := range keys {
		if len(item[key]) > 0 && string(item[key]) != "null" {
			return true
		}
	}
	return false
}

func firstJSONString(item map[string]json.RawMessage, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(rawJSONString(item[key])); value != "" {
			return value
		}
	}
	return ""
}

func parseDAGDepends(raw json.RawMessage) []int {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var deps []int
	if err := json.Unmarshal(raw, &deps); err == nil {
		return deps
	}
	var rawList []json.RawMessage
	if err := json.Unmarshal(raw, &rawList); err == nil {
		for _, item := range rawList {
			deps = append(deps, parseDAGDepends(item)...)
		}
		return deps
	}
	if dep, ok := parseDAGDependsScalar(raw); ok {
		return []int{dep}
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err == nil {
		for _, key := range []string{
			"depends_on", "depends", "dependencies", "dependsOn",
			"steps", "step", "ids", "id", "indexes", "index",
			"previous", "prerequisites", "upstream",
		} {
			if value := obj[key]; len(value) > 0 && string(value) != "null" {
				deps = append(deps, parseDAGDepends(value)...)
			}
		}
		return deps
	}
	return nil
}

func parseDAGDependsScalar(raw json.RawMessage) (int, bool) {
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, true
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil && f == float64(int(f)) {
		return int(f), true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, false
	}
	s = normalizeDAGDependString(s)
	if s == "" {
		return 0, false
	}
	var parsed int
	if _, err := fmt.Sscanf(s, "%d", &parsed); err == nil {
		return parsed, true
	}
	return 0, false
}

func normalizeDAGDependString(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	replacers := []struct {
		old string
		new string
	}{
		{"第", ""},
		{"步骤", ""},
		{"step", ""},
		{"#", ""},
		{"：", " "},
		{":", " "},
		{"，", " "},
		{",", " "},
		{"号", " "},
		{"步", " "},
		{"前置", " "},
		{"依赖", " "},
	}
	for _, r := range replacers {
		s = strings.ReplaceAll(s, r.old, r.new)
	}
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func normalizeDAGDepends(stepIndex, totalSteps int, deps []int) []int {
	if len(deps) == 0 {
		return nil
	}
	out := make([]int, 0, len(deps))
	seen := make(map[int]bool, len(deps))
	for _, dep := range deps {
		dep = normalizeDAGDependIndex(stepIndex, totalSteps, dep)
		if dep < 0 || dep >= stepIndex || seen[dep] {
			continue
		}
		seen[dep] = true
		out = append(out, dep)
	}
	return out
}

func normalizeDAGDependIndex(stepIndex, totalSteps, dep int) int {
	if dep >= 0 && dep < stepIndex {
		return dep
	}
	// Some models number DAG dependencies as 1-based step IDs.  Convert only
	// when doing so points to a previous step; this preserves existing 0-based
	// behavior and still rejects self/future dependencies.
	if dep >= 1 && dep <= totalSteps {
		candidate := dep - 1
		if candidate >= 0 && candidate < stepIndex {
			return candidate
		}
	}
	return dep
}

func hasPlanLikeStep(steps []dagStepDraft) bool {
	for _, step := range steps {
		if strings.TrimSpace(step.Description) != "" || strings.TrimSpace(step.Skill) != "" {
			return true
		}
	}
	return false
}

func extractJSONArray(s string) string {
	candidates := extractJSONArrays(s)
	if len(candidates) == 0 {
		return ""
	}
	return candidates[0]
}

func extractJSONArrays(s string) []string {
	var out []string
	for i := 0; i < len(s); i++ {
		if s[i] == '[' {
			depth := 0
			inString := false
			escaped := false
			for j := i; j < len(s); j++ {
				ch := s[j]
				if inString {
					if escaped {
						escaped = false
						continue
					}
					if ch == '\\' {
						escaped = true
						continue
					}
					if ch == '"' {
						inString = false
					}
					continue
				}
				if ch == '"' {
					inString = true
					continue
				}
				if ch == '[' {
					depth++
				} else if ch == ']' {
					depth--
					if depth == 0 {
						candidate := s[i : j+1]
						if json.Valid([]byte(candidate)) {
							out = append(out, candidate)
						}
						break
					}
				}
			}
		}
	}
	return out
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
