package traceview

import (
	"net/http"
	"reflect"
	"strings"

	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/observe"
)

// FriendlyError maps low-level execution failures to user-facing recovery text.
func FriendlyError(raw string) string {
	message := strings.TrimSpace(raw)
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "401") || strings.Contains(lower, "unauthorized") || strings.Contains(lower, "invalid authentication") || strings.Contains(lower, "invalid_authentication") || strings.Contains(lower, "authentication fails") || strings.Contains(lower, "invalid api key") || strings.Contains(lower, "api key") && strings.Contains(lower, "invalid") || strings.Contains(lower, "token not found"):
		return "模型密钥无效或已过期，请到模型设置检查当前执行层模型。"
	case strings.Contains(message, "尚未完成") || strings.Contains(message, "依赖") || strings.Contains(lower, "dependency"):
		return "前置步骤还没有完全确认，已保留现场，可查看依赖关系后继续。"
	case strings.Contains(lower, "unknown skill") || strings.Contains(message, "未知工具") || strings.Contains(message, "未找到工具"):
		return "所需工具暂时不可用，已保留现场，可换用可用工具或调整步骤继续。"
	case strings.Contains(lower, "blocked by trust gate") || strings.Contains(message, "信任") || strings.Contains(lower, "trust gate"):
		return "这一步需要更高信任或确认，已保留现场，可确认后继续。"
	case strings.Contains(lower, "tool panic") || strings.Contains(lower, "panic"):
		return "工具运行时遇到异常，已保留现场，可重试或切换策略继续。"
	case strings.Contains(lower, "context deadline exceeded") || strings.Contains(message, "响应超时") || strings.Contains(lower, "timeout"):
		return "响应暂时超时，已保留现场，可稍后重试或先返回阶段结果。"
	case strings.Contains(message, "当前模型响应失败") || strings.Contains(message, "备用模型") || strings.Contains(message, "调用栈降级") || strings.Contains(message, "级联唤醒") || strings.Contains(message, "备用引擎"):
		return "模型暂时没有回应，已保留现场，正在换用可用模型继续。"
	case strings.Contains(lower, "execution failed") || strings.Contains(lower, "handoff agent") || strings.Contains(lower, "fallback") || strings.Contains(lower, "all fallback llm clients failed") || strings.Contains(lower, "eof"):
		return "任务暂时没有顺利完成，已保留现场，可切换策略或稍后继续。"
	default:
		return ""
	}
}

func EventsForResponse(r *http.Request, events []observe.AgentEvent) ([]observe.AgentEvent, bool) {
	if RawMode(r) {
		return events, true
	}
	out := make([]observe.AgentEvent, 0, len(events))
	for _, event := range events {
		out = append(out, FriendlyEvent(event))
	}
	return out, false
}

func RawMode(r *http.Request) bool {
	if r == nil {
		return false
	}
	raw := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("raw")))
	return raw == "1" || raw == "true" || raw == "yes"
}

// FriendlyEvent returns an event safe for user-visible progress panels and trace
// APIs. Raw audit mode should still return the original event.
func FriendlyEvent(event observe.AgentEvent) observe.AgentEvent {
	streamEvent := event
	if friendly := FriendlyError(streamEvent.Summary); friendly != "" {
		streamEvent.Summary = friendly
	}
	streamEvent.Detail = FriendlyDetail(streamEvent.Detail)
	return streamEvent
}

func FriendlyDetail(detail any) any {
	switch d := detail.(type) {
	case observe.HandoffDetail:
		if friendly := FriendlyError(d.Error); friendly != "" {
			d.Error = friendly
		}
		return d
	case *observe.HandoffDetail:
		if d == nil {
			return detail
		}
		clone := *d
		if friendly := FriendlyError(clone.Error); friendly != "" {
			clone.Error = friendly
		}
		return &clone
	case observe.ToolResultDetail:
		if friendly := FriendlyError(d.Error); friendly != "" {
			d.Error = friendly
		}
		if friendly := FriendlyError(d.Result); friendly != "" {
			d.Result = friendly
		}
		return d
	case *observe.ToolResultDetail:
		if d == nil {
			return detail
		}
		clone := *d
		if friendly := FriendlyError(clone.Error); friendly != "" {
			clone.Error = friendly
		}
		if friendly := FriendlyError(clone.Result); friendly != "" {
			clone.Result = friendly
		}
		return &clone
	case planner.ModelFallbackDetail:
		if friendly := FriendlyError(d.Reason); friendly != "" {
			d.Reason = friendly
		}
		return d
	case *planner.ModelFallbackDetail:
		if d == nil {
			return detail
		}
		clone := *d
		if friendly := FriendlyError(clone.Reason); friendly != "" {
			clone.Reason = friendly
		}
		return &clone
	case planner.PlannerFailureSummary:
		return friendlyPlannerFailureSummary(d)
	case *planner.PlannerFailureSummary:
		if d == nil {
			return detail
		}
		clone := friendlyPlannerFailureSummary(*d)
		return &clone
	default:
		return sanitizeDetailValue(detail)
	}
}

func friendlyPlannerFailureSummary(summary planner.PlannerFailureSummary) planner.PlannerFailureSummary {
	summary.Tried = friendlyStringSlice(summary.Tried)
	summary.RuledOut = friendlyStringSlice(summary.RuledOut)
	summary.FailurePattern = friendlyString(summary.FailurePattern)
	summary.Recommendation = friendlyString(summary.Recommendation)
	summary.NextStep = friendlyString(summary.NextStep)
	return summary
}

func friendlyStringSlice(items []string) []string {
	if len(items) == 0 {
		return items
	}
	out := make([]string, len(items))
	changed := false
	for i, item := range items {
		out[i] = friendlyString(item)
		if out[i] != item {
			changed = true
		}
	}
	if changed {
		return out
	}
	return items
}

func friendlyString(item string) string {
	if friendly := FriendlyError(item); friendly != "" {
		return friendly
	}
	return item
}

func sanitizeDetailValue(value any) any {
	switch v := value.(type) {
	case string:
		if friendly := FriendlyError(v); friendly != "" {
			return friendly
		}
		return value
	case map[string]any:
		clone := make(map[string]any, len(v))
		changed := false
		for key, raw := range v {
			sanitized := sanitizeDetailValue(raw)
			clone[key] = sanitized
			if !reflect.DeepEqual(sanitized, raw) {
				changed = true
			}
		}
		if changed {
			return clone
		}
		return value
	case []any:
		clone := make([]any, len(v))
		changed := false
		for i, raw := range v {
			sanitized := sanitizeDetailValue(raw)
			clone[i] = sanitized
			if !reflect.DeepEqual(sanitized, raw) {
				changed = true
			}
		}
		if changed {
			return clone
		}
		return value
	default:
		return value
	}
}
