package planner

import (
	"context"
	"errors"
	"strings"
	"time"

	"yunque-agent/internal/observe"
)

const handoffRecoveryNextStep = "已保留失败原因；主规划器会缩小任务粒度、改用直接工具或返回已完成部分。"

func isHandoffTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "context deadline") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "timed out") ||
		strings.Contains(msg, "响应超时") ||
		strings.Contains(msg, "超时")
}

func handoffFailureSummary(agentName string, err error) string {
	if isHandoffTimeout(err) {
		return "子代理 [" + agentName + "] 响应超时，正在切换策略或返回已完成部分。"
	}
	return "子代理 [" + agentName + "] 暂时没有完成，已保留失败原因并切换策略。"
}

func buildHandoffFailureDetail(agentName string, dur time.Duration, err error) observe.HandoffDetail {
	detail := observe.HandoffDetail{
		Agent:       agentName,
		DurMs:       dur.Milliseconds(),
		Recoverable: true,
		NextStep:    handoffRecoveryNextStep,
	}
	if err != nil {
		detail.Error = plannerFriendlyFailureText(err.Error())
	}
	return detail
}
