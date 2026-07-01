package planner

import (
	"fmt"
	"net/url"
	"strings"
)

// PlannerFailureSummary is a compact, user-visible recovery note generated
// when the planner has enough evidence that it is repeating a failing path.
type PlannerFailureSummary struct {
	FailedCount    int                    `json:"failed_count"`
	CompletedCount int                    `json:"completed_count,omitempty"`
	FailedTools    []string               `json:"failed_tools,omitempty"`
	Tried          []string               `json:"tried,omitempty"`
	RuledOut       []string               `json:"ruled_out,omitempty"`
	FailurePattern string                 `json:"failure_pattern,omitempty"`
	Recommendation string                 `json:"recommendation,omitempty"`
	NextStep       string                 `json:"next_step"`
	Recoverable    bool                   `json:"recoverable"`
	PrimaryTarget  *PlannerRecoveryTarget `json:"primary_target,omitempty"`
}

type PlannerRecoveryTarget struct {
	Category string `json:"category"`
	Label    string `json:"label"`
	Href     string `json:"href,omitempty"`
	Action   string `json:"action,omitempty"`
}

type plannerFailureBucket string

const (
	failureBucketTimeout    plannerFailureBucket = "timeout_or_connection"
	failureBucketProvider   plannerFailureBucket = "provider_unavailable"
	failureBucketConnector  plannerFailureBucket = "connector_unavailable"
	failureBucketBrowser    plannerFailureBucket = "browser_unavailable"
	failureBucketSkill      plannerFailureBucket = "skill_unavailable"
	failureBucketTool       plannerFailureBucket = "tool_unavailable"
	failureBucketPath       plannerFailureBucket = "path_or_file_error"
	failureBucketTrust      plannerFailureBucket = "needs_confirmation"
	failureBucketDependency plannerFailureBucket = "dependency_blocked"
	failureBucketRuntime    plannerFailureBucket = "tool_runtime_error"
	failureBucketRepeated   plannerFailureBucket = "repeated_same_path"
)

type ToolFailureDiagnostic struct {
	ErrorCode   string `json:"error_code"`
	Cause       string `json:"cause"`
	Recoverable bool   `json:"recoverable"`
	NextStep    string `json:"next_step"`
}

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
	dominant := dominantPlannerFailureBucket(buckets)
	summary.FailurePattern, summary.Recommendation = summarizePlannerFailurePattern(dominant, summary.FailedTools)
	summary.PrimaryTarget = plannerRecoveryTargetForFailureBucket(dominant, steps)
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
	case strings.Contains(normalized, "access denied"),
		strings.Contains(normalized, "not under an allowed"),
		strings.Contains(normalized, "not under allowed"),
		strings.Contains(normalized, "permission denied"):
		return failureBucketTool
	case strings.Contains(normalized, "unknown tool"),
		strings.Contains(normalized, "allowed tool surface"),
		strings.Contains(normalized, "unknown skill") && strings.Contains(normalized, "_tool"):
		return failureBucketTool
	case strings.Contains(normalized, "unknown skill"),
		strings.Contains(normalized, "missing skill"),
		strings.Contains(normalized, "skill not found"),
		strings.Contains(normalized, "skill growth disabled"),
		strings.Contains(normalized, "no skill providers configured"):
		return failureBucketSkill
	case plannerFailureMentionsBrowserRecovery(normalized):
		return failureBucketBrowser
	case plannerFailureMentionsConnectorRecovery(normalized):
		return failureBucketConnector
	case plannerFailureMentionsProviderRecovery(normalized):
		return failureBucketProvider
	case strings.Contains(normalized, "no such file"),
		strings.Contains(normalized, "file not found"),
		strings.Contains(normalized, "cannot find"),
		strings.Contains(normalized, "is a directory"),
		strings.Contains(normalized, "not a directory"),
		strings.Contains(normalized, "invalid path"),
		strings.Contains(normalized, "cannot read"):
		return failureBucketPath
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

func plannerFailureMentionsBrowserRecovery(normalized string) bool {
	return strings.Contains(normalized, "browser extension") ||
		strings.Contains(normalized, "browser pairing") ||
		strings.Contains(normalized, "extension pairing") ||
		strings.Contains(normalized, "browser not paired") ||
		strings.Contains(normalized, "browser pack") ||
		strings.Contains(normalized, "chrome extension") ||
		strings.Contains(normalized, "浏览器扩展") ||
		strings.Contains(normalized, "浏览器配对")
}

func plannerFailureMentionsConnectorRecovery(normalized string) bool {
	if strings.Contains(normalized, "allowlist") || strings.Contains(normalized, "allow-list") {
		return true
	}
	if !strings.Contains(normalized, "connector") && plannerFailureConnectorID(normalized) == "" {
		return false
	}
	return plannerFailureMentionsConnectorFailureWord(normalized)
}

func plannerFailureMentionsConnectorFailureWord(normalized string) bool {
	return strings.Contains(normalized, "oauth") ||
		strings.Contains(normalized, "credential") ||
		strings.Contains(normalized, "unauthorized") ||
		strings.Contains(normalized, "forbidden") ||
		strings.Contains(normalized, "authentication") ||
		strings.Contains(normalized, "authorization") ||
		strings.Contains(normalized, "token expired") ||
		strings.Contains(normalized, "invalid token") ||
		strings.Contains(normalized, "rate limit") ||
		strings.Contains(normalized, "429") ||
		strings.Contains(normalized, "upstream") ||
		strings.Contains(normalized, "expired") ||
		strings.Contains(normalized, "denied") ||
		strings.Contains(normalized, "failed") ||
		strings.Contains(normalized, "unavailable")
}

func plannerFailureConnectorID(normalized string) string {
	for _, id := range []string{"github", "gmail", "google_calendar", "calendar", "slack", "notion", "linear", "jira"} {
		if strings.Contains(normalized, id) {
			if id == "calendar" {
				return "google_calendar"
			}
			return id
		}
	}
	return ""
}

func plannerFailureMentionsProviderRecovery(normalized string) bool {
	providerTerm := strings.Contains(normalized, "provider") ||
		strings.Contains(normalized, "model") ||
		strings.Contains(normalized, "llm") ||
		strings.Contains(normalized, "openai") ||
		strings.Contains(normalized, "qwen") ||
		strings.Contains(normalized, "moonshot") ||
		strings.Contains(normalized, "api key") ||
		strings.Contains(normalized, "apikey") ||
		strings.Contains(normalized, "模型") ||
		strings.Contains(normalized, "供应商") ||
		strings.Contains(normalized, "密钥")
	if !providerTerm {
		return false
	}
	return strings.Contains(normalized, "401") ||
		strings.Contains(normalized, "402") ||
		strings.Contains(normalized, "403") ||
		strings.Contains(normalized, "429") ||
		strings.Contains(normalized, "unauthorized") ||
		strings.Contains(normalized, "forbidden") ||
		strings.Contains(normalized, "invalid authentication") ||
		strings.Contains(normalized, "authentication fails") ||
		strings.Contains(normalized, "invalid api key") ||
		strings.Contains(normalized, "quota") ||
		strings.Contains(normalized, "rate limit") ||
		strings.Contains(normalized, "too many requests") ||
		strings.Contains(normalized, "balance") ||
		strings.Contains(normalized, "billing") ||
		strings.Contains(normalized, "payment") ||
		strings.Contains(normalized, "认证") ||
		strings.Contains(normalized, "鉴权") ||
		strings.Contains(normalized, "余额") ||
		strings.Contains(normalized, "额度") ||
		strings.Contains(normalized, "限流") ||
		strings.Contains(normalized, "欠费")
}

func plannerFailureConnectorIDFromSteps(steps []PlanStep) string {
	for _, step := range steps {
		text := strings.ToLower(strings.Join([]string{step.Error, step.Action, step.Skill}, " "))
		if id := plannerFailureConnectorID(text); id != "" {
			return id
		}
	}
	return ""
}

func plannerConnectorRecoveryHref(connectorID string) string {
	connectorID = strings.TrimSpace(connectorID)
	if connectorID == "" {
		return "/settings/connectors"
	}
	return "/settings/connectors?focus=" + url.QueryEscape(connectorID)
}

func dominantPlannerFailureBucket(buckets map[plannerFailureBucket]int) plannerFailureBucket {
	if len(buckets) == 0 {
		return failureBucketRepeated
	}
	var dominant plannerFailureBucket
	dominantCount := -1
	for bucket, count := range buckets {
		if count > dominantCount || (count == dominantCount && plannerFailureBucketPriority(bucket) < plannerFailureBucketPriority(dominant)) {
			dominant = bucket
			dominantCount = count
		}
	}
	return dominant
}

func summarizePlannerFailurePattern(dominant plannerFailureBucket, failedTools []string) (string, string) {
	toolHint := ""
	if len(failedTools) > 0 {
		toolHint = "，暂不重复使用 " + strings.Join(failedTools, "、")
	}
	switch dominant {
	case failureBucketTimeout:
		return "模型或子任务响应不稳定", "先返回阶段结果或切为后台任务；继续时降低任务粒度" + toolHint + "。"
	case failureBucketProvider:
		return "模型供应商不可用", "先检查模型密钥、额度、限流或供应商配置，再继续执行失败步骤" + toolHint + "。"
	case failureBucketConnector:
		return "连接器不可用", "先修复连接器授权、allowlist 或限流，再继续执行失败步骤" + toolHint + "。"
	case failureBucketBrowser:
		return "浏览器连接不可用", "先恢复浏览器扩展配对或打开浏览器包，再继续执行失败步骤" + toolHint + "。"
	case failureBucketSkill:
		return "所需技能不可用", "先安装、启用或替换技能，再继续执行失败步骤" + toolHint + "。"
	case failureBucketTool:
		return "所需工具不可用或不在当前工具范围", "改用当前可用工具，或先请求开放/替换工具后再继续。"
	case failureBucketPath:
		return "目标路径、文件类型或读取范围不匹配", "先确认路径存在且在允许范围内；如果要跨目录搜索，请传目录路径并缩小搜索范围。"
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

func plannerRecoveryTargetForFailureBucket(bucket plannerFailureBucket, steps []PlanStep) *PlannerRecoveryTarget {
	switch bucket {
	case failureBucketTimeout, failureBucketProvider:
		return &PlannerRecoveryTarget{
			Category: "provider",
			Label:    "检查模型供应商",
			Href:     "/settings/providers?tab=providers",
			Action:   "open_provider_settings",
		}
	case failureBucketConnector:
		return &PlannerRecoveryTarget{
			Category: "connector",
			Label:    "修复连接器",
			Href:     plannerConnectorRecoveryHref(plannerFailureConnectorIDFromSteps(steps)),
			Action:   "repair_connector",
		}
	case failureBucketBrowser:
		return &PlannerRecoveryTarget{
			Category: "browser",
			Label:    "打开浏览器包",
			Href:     "/packs/browser",
			Action:   "repair_browser",
		}
	case failureBucketSkill:
		return &PlannerRecoveryTarget{
			Category: "skill",
			Label:    "检查技能",
			Href:     "/skills",
			Action:   "repair_skill",
		}
	case failureBucketTool, failureBucketRuntime:
		return &PlannerRecoveryTarget{
			Category: "tool",
			Label:    "检查工具",
			Href:     "/tools",
			Action:   "repair_tool",
		}
	case failureBucketTrust:
		return &PlannerRecoveryTarget{
			Category: "approval",
			Label:    "处理审批",
			Href:     "/approvals",
			Action:   "open_approvals",
		}
	case failureBucketDependency:
		return &PlannerRecoveryTarget{
			Category: "dependency",
			Label:    "查看依赖关系",
			Action:   "inspect_dependencies",
		}
	default:
		return nil
	}
}

func plannerFailureBucketPriority(bucket plannerFailureBucket) int {
	switch bucket {
	case failureBucketTimeout:
		return 0
	case failureBucketProvider:
		return 1
	case failureBucketConnector:
		return 2
	case failureBucketBrowser:
		return 3
	case failureBucketSkill:
		return 4
	case failureBucketTool:
		return 5
	case failureBucketPath:
		return 6
	case failureBucketTrust:
		return 7
	case failureBucketDependency:
		return 8
	case failureBucketRuntime:
		return 9
	default:
		return 10
	}
}

func buildToolFailureDiagnostic(rawErr string) ToolFailureDiagnostic {
	bucket := classifyPlannerFailure(rawErr)
	d := ToolFailureDiagnostic{
		ErrorCode:   string(bucket),
		Recoverable: true,
	}
	switch bucket {
	case failureBucketTimeout:
		d.Cause = "等待时间过长或连接中断"
		d.NextStep = "稍后重试、缩小输入范围，或改为后台任务。"
	case failureBucketProvider:
		d.Cause = "模型供应商认证、额度、限流或配置不可用"
		d.NextStep = "先检查模型密钥、额度、限流或供应商配置，再继续失败步骤。"
	case failureBucketConnector:
		d.Cause = "连接器授权、allowlist、限流或上游服务不可用"
		d.NextStep = "先修复连接器，再重试失败步骤。"
	case failureBucketBrowser:
		d.Cause = "浏览器扩展、配对或远控入口不可用"
		d.NextStep = "先打开浏览器包并恢复配对，再继续任务。"
	case failureBucketSkill:
		d.Cause = "技能不可用或尚未安装"
		d.NextStep = "先安装、启用或替换技能后继续。"
	case failureBucketTool:
		d.Cause = "工具不可用或访问范围不足"
		d.NextStep = "改用当前可用工具，或先开放/替换工具后继续。"
	case failureBucketPath:
		d.Cause = "目标路径、文件类型或读取范围不匹配"
		d.NextStep = "确认路径存在且在允许读取范围内；跨目录搜索时传目录路径并缩小范围。"
	case failureBucketTrust:
		d.Cause = "需要用户确认或更高信任"
		d.NextStep = "暂停自动推进，说明需要确认的动作，确认后继续。"
	case failureBucketDependency:
		d.Cause = "前置依赖尚未满足"
		d.NextStep = "回到最早未完成的前置步骤，先补齐依赖。"
	case failureBucketRuntime:
		d.Cause = "工具运行时异常"
		d.NextStep = "降低输入规模或切换等价工具；已有证据足够时先返回阶段结果。"
	default:
		d.Cause = "执行路径暂时不可完成"
		d.NextStep = "停止重复同一路径，换一个工具、降低任务粒度，或先汇总已获得证据。"
	}
	return d
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
