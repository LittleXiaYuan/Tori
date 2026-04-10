package task

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// executeStepWithRetry executes a step with automatic retry on failure.
func (r *Runner) executeStepWithRetry(ctx context.Context, t *Task, step *Step) (string, error) {
	var lastErr error

	for attempt := 0; attempt <= step.MaxRetries; attempt++ {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		if attempt > 0 {
			step.RetryCount = attempt
			step.Status = StepRetrying
			step.Error = fmt.Sprintf("retry %d/%d: %v", attempt, step.MaxRetries, lastErr)
			r.store.Update(t)
			slog.Info("task: retrying step", "task", t.ID, "step", step.ID, "attempt", attempt, "prev_err", lastErr)

			// Brief backoff: 1s, 2s
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}

		stepStart := time.Now()
		step.Status = StepRunning
		step.StartedAt = &stepStart
		r.store.Update(t)

		result, err := r.executeStep(ctx, t, step)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}

	return "", lastErr
}

// executeParallelStep executes a step with retries but does NOT mutate task step state.
// This is safe for concurrent goroutine execution — state writes happen on the main goroutine.
func (r *Runner) executeParallelStep(ctx context.Context, t *Task, step *Step) (string, error) {
	var lastErr error
	for attempt := 0; attempt <= step.MaxRetries; attempt++ {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if attempt > 0 {
			slog.Info("task: retrying parallel step", "task", t.ID, "step", step.ID, "attempt", attempt, "prev_err", lastErr)
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}
		result, err := r.executeStep(ctx, t, step)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}
	return "", lastErr
}

// handleStepFailure handles a failed step: gap analysis, auto-growth, and task failure marking.
// Returns nil if the step was resolved via growth loop (step.Status == StepDone).
func (r *Runner) handleStepFailure(ctx context.Context, t *Task, step *Step, err error) error {
	// Capability gap analysis
	var gapRec *GapRecord
	if r.gap != nil {
		gapRec = r.gap.Analyze(ctx, t, step)
		step.GapType = string(gapRec.GapType)
	}

	// Auto-growth: try to generate missing skill and retry once
	if gapRec != nil && gapRec.GapType == GapSkillMissing && r.generator != nil {
		slog.Info("growth: attempting auto-generation", "task", t.ID, "skill", step.SkillName)

		genSkill, genErr := r.generator.Generate(ctx, gapRec)
		if genErr == nil {
			slog.Info("growth: skill generated, retrying", "task", t.ID, "skill", genSkill.Name())

			step.Status = StepRunning
			step.Error = ""
			step.SkillName = genSkill.Name()
			r.store.Update(t)

			retryResult, retryErr := r.executeStep(ctx, t, step)
			if retryErr == nil {
				step.Status = StepDone
				step.Result = retryResult
				step.GapType = string(GapSkillMissing) + ":auto_resolved"
				stepDone := time.Now()
				step.DoneAt = &stepDone
				r.store.Update(t)

				if r.gap != nil {
					r.gap.Resolve(gapRec.ID)
				}
				return nil // resolved
			}
			slog.Warn("growth: generated skill failed", "task", t.ID, "err", retryErr)
		} else {
			slog.Warn("growth: skill generation failed", "task", t.ID, "err", genErr)
		}
	}

	stepDone := time.Now()
	step.DoneAt = &stepDone
	step.Status = StepFailed
	step.Error = err.Error()

	t.Status = StatusFailed
	t.Error = fmt.Sprintf("step %d failed after %d retries: %v", step.ID, step.RetryCount, err)
	t.FinishedAt = &stepDone
	r.store.Update(t)
	r.emit("task_failed", t.ID, t.Error)
	return err
}

// plan uses LLM to generate execution steps from the task description.
func (r *Runner) plan(ctx context.Context, t *Task) ([]Step, error) {
	// Build available skills list
	var skillDescriptions []string
	for _, sk := range r.registry.All() {
		skillDescriptions = append(skillDescriptions, fmt.Sprintf("- %s: %s", sk.Name(), sk.Description()))
	}

	systemPrompt := `你是一个任务规划器。用户给出任务描述，你需要将它分解为可执行的步骤。

可用技能：
` + strings.Join(skillDescriptions, "\n") + `

规则：
1. 每个步骤要明确、可执行
2. 如果需要调用技能，填写 skill_name 和 args
3. 如果是纯文字/分析步骤（不需要调用技能），skill_name 留空
4. 按实际需要拆分步骤，不要遗漏关键环节；通常一个完整开发任务在10~20步之间
5. 按执行顺序排列
6. 后续步骤可以依赖前面步骤的结果（会自动传递）
7. 如果多个步骤互不依赖，设置相同的 group 编号使它们并行执行（默认0=顺序）
8. 最后一步建议是验证/测试步骤，确保产出物符合预期

返回JSON数组，格式：
[{"action":"步骤描述","skill_name":"技能名或空","args":{"key":"value"},"group":0}]
仅返回JSON数组。`

	userPrompt := fmt.Sprintf("任务：%s\n\n详细描述：%s", t.Title, t.Description)

	// Inject constraints into planning prompt
	if t.Constraints != nil {
		if t.Constraints.SuccessCriteria != "" {
			userPrompt += "\n\n验收标准：" + t.Constraints.SuccessCriteria
		}
		if t.Constraints.TestCommand != "" {
			userPrompt += "\n\n验证命令（最后一步必须执行此命令确认结果）：" + t.Constraints.TestCommand
		}
		if t.Constraints.MaxSteps > 0 {
			userPrompt += fmt.Sprintf("\n\n注意：步骤数不得超过 %d 步。", t.Constraints.MaxSteps)
		}
	}

	resp, err := r.llmCall(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("LLM planning: %w", err)
	}

	// Parse steps from LLM response
	resp = strings.TrimSpace(resp)
	resp = trimCodeFences(resp)

	var rawSteps []struct {
		Action    string         `json:"action"`
		SkillName string         `json:"skill_name"`
		Args      map[string]any `json:"args"`
		Group     int            `json:"group"`
	}
	if err := json.Unmarshal([]byte(resp), &rawSteps); err != nil {
		// Fallback: single step with the whole description
		return []Step{{
			ID:         1,
			Action:     t.Description,
			Status:     StepPending,
			MaxRetries: DefaultMaxRetries,
		}}, nil
	}

	steps := make([]Step, len(rawSteps))
	for i, raw := range rawSteps {
		steps[i] = Step{
			ID:         i + 1,
			Action:     raw.Action,
			SkillName:  raw.SkillName,
			Args:       raw.Args,
			Status:     StepPending,
			MaxRetries: DefaultMaxRetries,
			Group:      raw.Group,
		}
	}

	// Enforce max_steps constraint (default 20)
	maxAllowed := 20
	if t.Constraints != nil && t.Constraints.MaxSteps > 0 {
		maxAllowed = t.Constraints.MaxSteps
	}
	if len(steps) > maxAllowed {
		slog.Warn("task: plan exceeded max_steps, truncating", "task", t.ID, "planned", len(steps), "max", maxAllowed)
		steps = steps[:maxAllowed]
	}

	return steps, nil
}

// executeStep runs a single step, using a skill or LLM.
// Step chaining: step.Input contains the previous step's result for context.
func (r *Runner) executeStep(ctx context.Context, t *Task, step *Step) (string, error) {
	// Inject cost context so wrapped llmCall can attribute cost to this task+step
	ctx = WithTaskCost(ctx, &TaskCostContext{
		TaskID:    t.ID,
		StepID:    fmt.Sprintf("step-%d", step.ID),
		SkillName: step.SkillName,
	})

	// Tag cost with task_id and skill_name for telemetry (legacy callback)
	if r.CostTag != nil {
		r.CostTag(t.ID, step.SkillName)
	}

	if step.SkillName == "" {
		// LLM-only step: ask LLM to perform the action
		userMsg := fmt.Sprintf("任务：%s\n当前步骤：%s", t.Description, step.Action)
		if step.Input != "" {
			userMsg += fmt.Sprintf("\n\n上一步的结果（可作为参考）：\n%s", truncate(step.Input, 2000))
		}
		return r.llmCall(ctx,
			"你正在执行一个任务的步骤。请根据步骤描述完成工作，返回结果。如果提供了上一步的结果，可以基于它继续工作。",
			userMsg,
		)
	}

	// Skill-based step
	sk, ok := r.registry.Get(step.SkillName)
	if !ok {
		return "", fmt.Errorf("skill %q not found", step.SkillName)
	}

	args := step.Args
	if args == nil {
		args = make(map[string]any)
	}

	// Inject chained input if the skill args don't already have relevant content
	if step.Input != "" {
		if _, exists := args["_prev_result"]; !exists {
			args["_prev_result"] = truncate(step.Input, 2000)
		}
	}

	result, err := sk.Execute(ctx, args, r.env)
	if err != nil {
		return "", fmt.Errorf("skill %s: %w", step.SkillName, err)
	}
	return result, nil
}

// groupSteps groups step indices by their Group field for parallel execution.
// Steps with Group=0 (default) each form their own group (sequential).
// Steps sharing the same non-zero Group value run concurrently.
func groupSteps(steps []Step) [][]int {
	var groups [][]int
	i := 0
	for i < len(steps) {
		if steps[i].Group == 0 {
			groups = append(groups, []int{i})
			i++
		} else {
			grp := []int{i}
			gID := steps[i].Group
			j := i + 1
			for j < len(steps) && steps[j].Group == gID {
				grp = append(grp, j)
				j++
			}
			groups = append(groups, grp)
			i = j
		}
	}
	return groups
}

// trimCodeFences strips markdown code fences from LLM output.
func trimCodeFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
	}
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// truncate limits a string to maxLen runes, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen]) + "..."
}
