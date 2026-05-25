package planner

import (
	"fmt"
	"strings"

	"yunque-agent/internal/observe"
)

const partialPlanResultIntro = "任务已部分执行，现场已保留，可先查看阶段结果或继续恢复。"

func buildPartialResultDetail(planSteps []PlanStep, rawErr string) observe.PartialResultDetail {
	detail := observe.PartialResultDetail{
		Recoverable: true,
		NextStep:    "可以直接继续，我会基于已完成步骤往下恢复；也可以先要求整理已完成部分。",
		Reason:      plannerFriendlyFailureText(rawErr),
	}
	limit := len(planSteps)
	if limit > 8 {
		limit = 8
	}
	detail.Steps = make([]observe.PartialStepView, 0, limit)
	for i, step := range planSteps {
		if i >= limit {
			break
		}
		view := observe.PartialStepView{
			ID:     step.ID,
			Skill:  step.Skill,
			Action: step.Action,
			Status: string(step.Status),
		}
		switch step.Status {
		case StepDone:
			detail.CompletedCount++
			view.Result = truncate(sanitizePartialStepText(step.Result), 240)
		case StepFailed:
			detail.FailedCount++
			view.Error = plannerFriendlyFailureText(step.Error)
		case StepSkipped:
			detail.CompletedCount++
		}
		detail.Steps = append(detail.Steps, view)
	}
	if len(planSteps) > limit {
		detail.Steps = append(detail.Steps, observe.PartialStepView{
			ID:     len(planSteps) - limit,
			Action: fmt.Sprintf("还有 %d 个步骤已保留在执行记录中", len(planSteps)-limit),
			Status: string(StepSkipped),
		})
	}
	if detail.Reason == "" {
		detail.Reason = "现场已保留，可从恢复点继续或返回阶段结果。"
	}
	return detail
}

func buildPartialPlanReply(planSteps []PlanStep, rawErr string) string {
	var b strings.Builder
	b.WriteString(partialPlanResultIntro)

	if note := plannerFriendlyFailureText(rawErr); note != "" && note != "现场已保留，可从恢复点继续或返回阶段结果。" {
		b.WriteString("\n\n后续生成暂未完成：")
		b.WriteString(note)
	}

	if len(planSteps) == 0 {
		return b.String()
	}

	b.WriteString("\n\n阶段结果：")
	for i, step := range planSteps {
		if i >= 6 {
			b.WriteString(fmt.Sprintf("\n- 还有 %d 个步骤已保留在执行记录中。", len(planSteps)-i))
			break
		}
		label := partialStepLabel(step)
		switch step.Status {
		case StepDone:
			result := sanitizePartialStepText(step.Result)
			if result == "" {
				result = "已完成"
			}
			b.WriteString(fmt.Sprintf("\n- ✅ %s：%s", label, truncate(result, 260)))
		case StepFailed:
			b.WriteString(fmt.Sprintf("\n- ⚠️ %s：%s", label, plannerFriendlyFailureText(step.Error)))
		case StepRunning:
			b.WriteString(fmt.Sprintf("\n- ⏳ %s：仍在处理中，现场已保留。", label))
		case StepSkipped:
			b.WriteString(fmt.Sprintf("\n- ↪️ %s：已跳过。", label))
		default:
			b.WriteString(fmt.Sprintf("\n- %s：已记录。", label))
		}
	}

	b.WriteString("\n\n你可以直接继续，我会从这些阶段结果往下恢复。")
	return b.String()
}

func partialStepLabel(step PlanStep) string {
	if step.Skill != "" {
		return step.Skill
	}
	if step.Action != "" {
		return step.Action
	}
	if step.ID > 0 {
		return fmt.Sprintf("步骤 %d", step.ID)
	}
	return "当前步骤"
}

func sanitizePartialStepText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if containsPlannerRawDiagnostic(text) {
		return plannerFriendlyFailureText(text)
	}
	return text
}

func containsPlannerRawDiagnostic(text string) bool {
	lower := strings.ToLower(text)
	rawTerms := []string{
		"调用栈降级",
		"级联唤醒",
		"fallback",
		"execution failed",
		"context canceled",
		"context cancelled",
		"context deadline exceeded",
		"deadline exceeded",
		"eof",
		"handoff agent",
		"unknown skill",
		"tool panic",
		"trust gate",
	}
	for _, term := range rawTerms {
		if strings.Contains(lower, strings.ToLower(term)) {
			return true
		}
	}
	return false
}
