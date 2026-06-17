package controlplanepack

import (
	"encoding/json"
	"net/http"
	"strings"

	"yunque-agent/internal/execution/sandbox"
	"yunque-agent/internal/observe"
)

func (h *Handler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sanitizeMetricsSnapshotForUser(h.gateway.MetricsSnapshot()))
}

func (h *Handler) handleMetricsPrometheus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(h.gateway.MetricsPrometheus()))
}

func (h *Handler) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	modelHealth := h.gateway.ModelRuntimeHealth()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"system": sandbox.SystemInfo(),
		"breaker": map[string]any{
			"state":      modelHealth.BreakerState,
			"failures":   modelHealth.Failures,
			"configured": modelHealth.Configured,
		},
	})
}

func (h *Handler) handleSystemStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(h.gateway.SystemStats(r.Context()))
}

func (h *Handler) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	stats := map[string]any{}
	if cacheStats := h.gateway.LLMResponseCacheStats(); cacheStats != nil {
		stats["llm_response_cache"] = cacheStats
	}
	_ = json.NewEncoder(w).Encode(stats)
}

func sanitizeMetricsSnapshotForUser(snapshot observe.MetricsSnapshot) observe.MetricsSnapshot {
	for i := range snapshot.RecentErrors {
		snapshot.RecentErrors[i].Message = friendlyControlPlaneError(snapshot.RecentErrors[i].Message)
	}
	return snapshot
}

func friendlyControlPlaneError(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if friendly := knownFriendlyControlPlaneError(raw); friendly != "" {
		return friendly
	}
	return truncateControlPlaneString(raw, 240)
}

func knownFriendlyControlPlaneError(raw string) string {
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

func truncateControlPlaneString(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}
