package planner

import (
	"fmt"
	"strings"
)

// PlannerFailureSummary is a compact, user-visible recovery note generated
// when the planner has enough evidence that it is repeating a failing path.
type PlannerFailureSummary struct {
	FailedCount    int      `json:"failed_count"`
	CompletedCount int      `json:"completed_count,omitempty"`
	FailedTools    []string `json:"failed_tools,omitempty"`
	Tried          []string `json:"tried,omitempty"`
	RuledOut       []string `json:"ruled_out,omitempty"`
	FailurePattern string   `json:"failure_pattern,omitempty"`
	Recommendation string   `json:"recommendation,omitempty"`
	NextStep       string   `json:"next_step"`
	Recoverable    bool     `json:"recoverable"`
}

type plannerFailureBucket string

const (
	failureBucketTimeout    plannerFailureBucket = "timeout_or_connection"
	failureBucketTool       plannerFailureBucket = "tool_unavailable"
	failureBucketTrust      plannerFailureBucket = "needs_confirmation"
	failureBucketDependency plannerFailureBucket = "dependency_blocked"
	failureBucketRuntime    plannerFailureBucket = "tool_runtime_error"
	failureBucketRepeated   plannerFailureBucket = "repeated_same_path"
)

func buildPlannerFailureSummary(steps []PlanStep) (PlannerFailureSummary, bool) {
	summary := PlannerFailureSummary{}
	seenFailed := map[string]bool{}
	buckets := map[plannerFailureBucket]int{}
	for _, step := range steps {
		label := step.Skill
		if label == "" {
			label = step.Action
		}
		if label == "" {
			label = fmt.Sprintf("step-%d", step.ID)
		}
		if step.Status == StepFailed {
			summary.FailedCount++
			if !seenFailed[label] {
				summary.FailedTools = append(summary.FailedTools, label)
				seenFailed[label] = true
			}
			errText := strings.TrimSpace(step.Error)
			buckets[classifyPlannerFailure(errText)]++
			friendlyErr := plannerFriendlyFailureText(errText)
			summary.RuledOut = append(summary.RuledOut, fmt.Sprintf("%s: %s", label, truncate(friendlyErr, 140)))
		} else if step.Status == StepDone {
			summary.CompletedCount++
			result := strings.TrimSpace(step.Result)
			if result == "" {
				result = "completed"
			}
			summary.Tried = append(summary.Tried, fmt.Sprintf("%s: %s", label, truncate(result, 120)))
		}
	}
	if summary.FailedCount < 2 {
		return summary, false
	}
	summary.FailurePattern, summary.Recommendation = summarizePlannerFailurePattern(buckets, summary.FailedTools)
	summary.NextStep = summary.Recommendation
	if summary.NextStep == "" {
		summary.NextStep = "停止重复失败路径，换一个工具、降低任务粒度，或先汇总已获得证据再继续。"
	}
	summary.Recoverable = true
	return summary, true
}

func classifyPlannerFailure(rawErr string) plannerFailureBucket {
	normalized := strings.ToLower(strings.TrimSpace(rawErr))
	switch {
	case strings.Contains(normalized, "unknown skill"), strings.Contains(normalized, "allowed tool surface"):
		return failureBucketTool
	case strings.Contains(normalized, "blocked by trust gate"), strings.Contains(normalized, "trust gate"):
		return failureBucketTrust
	case strings.Contains(normalized, "dependency"),
		strings.Contains(normalized, "depend"),
		strings.Contains(normalized, "no ready steps"):
		return failureBucketDependency
	case strings.Contains(normalized, "tool panic"), strings.Contains(normalized, "panic"):
		return failureBucketRuntime
	case strings.Contains(normalized, "context deadline exceeded"),
		strings.Contains(normalized, "deadline exceeded"),
		strings.Contains(normalized, "context canceled"),
		strings.Contains(normalized, "context cancelled"),
		strings.Contains(normalized, "timeout"),
		strings.Contains(normalized, "timed out"),
		strings.Contains(normalized, "handoff agent"),
		strings.Contains(normalized, "execution failed"),
		strings.Contains(normalized, "all fallback"),
		strings.Contains(normalized, "fallback llm"),
		strings.Contains(normalized, "eof"),
		strings.Contains(normalized, "响应超时"),
		strings.Contains(normalized, "超时"):
		return failureBucketTimeout
	default:
		return failureBucketRepeated
	}
}

func summarizePlannerFailurePattern(buckets map[plannerFailureBucket]int, failedTools []string) (string, string) {
	if len(buckets) == 0 {
		return "重复失败路径", "停止重复失败路径，换一个工具、降低任务粒度，或先汇总已获得证据再继续。"
	}
	var dominant plannerFailureBucket
	dominantCount := -1
	for bucket, count := range buckets {
		if count > dominantCount || (count == dominantCount && plannerFailureBucketPriority(bucket) < plannerFailureBucketPriority(dominant)) {
			dominant = bucket
			dominantCount = count
		}
	}
	toolHint := ""
	if len(failedTools) > 0 {
		toolHint = "，暂不重复使用 " + strings.Join(failedTools, "、")
	}
	switch dominant {
	case failureBucketTimeout:
		return "模型或子任务响应不稳定", "先返回阶段结果或切为后台任务；继续时降低任务粒度" + toolHint + "。"
	case failureBucketTool:
		return "所需工具不可用或不在当前工具范围", "改用当前可用工具，或先请求开放/替换工具后再继续。"
	case failureBucketTrust:
		return "需要用户确认或更高信任", "暂停自动推进，向用户说明需要确认的动作，确认后再继续。"
	case failureBucketDependency:
		return "规划依赖未满足", "回到最早未完成的前置步骤，先补齐依赖，再执行后续步骤。"
	case failureBucketRuntime:
		return "工具运行异常", "降低输入规模或切换等价工具；如果已有证据足够，先返回阶段结果。"
	default:
		return "重复失败路径", "停止重复失败路径，换一个工具、降低任务粒度，或先汇总已获得证据再继续。"
	}
}

func plannerFailureBucketPriority(bucket plannerFailureBucket) int {
	switch bucket {
	case failureBucketTimeout:
		return 0
	case failureBucketTool:
		return 1
	case failureBucketTrust:
		return 2
	case failureBucketDependency:
		return 3
	case failureBucketRuntime:
		return 4
	default:
		return 9
	}
}

func formatFailureRecoveryPrompt(summary PlannerFailureSummary) string {
	var b strings.Builder
	b.WriteString("【Planner 自愈提示】检测到当前任务已有多次工具/子代理失败，请不要继续重复同一路径。\n")
	if summary.FailurePattern != "" {
		b.WriteString("\n失败模式：")
		b.WriteString(summary.FailurePattern)
		b.WriteString("\n")
	}
	if summary.Recommendation != "" {
		b.WriteString("推荐策略：")
		b.WriteString(summary.Recommendation)
		b.WriteString("\n")
	}
	if len(summary.Tried) > 0 {
		b.WriteString("\n已尝试并获得结果：\n")
		for _, item := range summary.Tried {
			b.WriteString("- ")
			b.WriteString(item)
			b.WriteString("\n")
		}
	}
	if len(summary.RuledOut) > 0 {
		b.WriteString("\n已失败/暂时排除：\n")
		for _, item := range summary.RuledOut {
			b.WriteString("- ")
			b.WriteString(item)
			b.WriteString("\n")
		}
	}
	b.WriteString("\n下一步策略：")
	b.WriteString(summary.NextStep)
	b.WriteString("\n如果已有信息足够，请直接给出阶段性结论；否则选择不同工具或更小步骤继续。")
	return b.String()
}
