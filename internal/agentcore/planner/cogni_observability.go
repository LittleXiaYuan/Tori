package planner

import (
	"fmt"
	"strings"

	"yunque-agent/internal/observe"
)

// CogniTraceDetail is the planner-facing, UI-safe subset of pkg/cogni.Trace.
// It deliberately contains only routing/observability metadata and never the
// user's full message or injected context body.
type CogniTraceDetail struct {
	Activated         []string `json:"activated,omitempty"`
	ContextBytes      int      `json:"context_bytes"`
	TemplateFallbacks int      `json:"template_fallbacks,omitempty"`
	ToolBefore        int      `json:"tool_before,omitempty"`
	ToolAfter         int      `json:"tool_after,omitempty"`
	Removed           []string `json:"removed,omitempty"`
	FellBackToInput   bool     `json:"fell_back_to_input,omitempty"`
	MessageHash       string   `json:"message_hash,omitempty"`
	DurationMs        int64    `json:"duration_ms,omitempty"`
}

func (d CogniTraceDetail) hasVisibleEffect() bool {
	return len(d.Activated) > 0 ||
		d.ContextBytes > 0 ||
		d.ToolBefore != d.ToolAfter ||
		len(d.Removed) > 0 ||
		d.FellBackToInput ||
		d.TemplateFallbacks > 0
}

func (d CogniTraceDetail) summary() string {
	names := "未激活"
	if len(d.Activated) > 0 {
		names = strings.Join(d.Activated, "、")
	}
	if d.ToolBefore > 0 || d.ToolAfter > 0 {
		return fmt.Sprintf("Cogni 已激活：%s，工具面 %d → %d", names, d.ToolBefore, d.ToolAfter)
	}
	if d.ContextBytes > 0 {
		return fmt.Sprintf("Cogni 已激活：%s，注入上下文 %d 字节", names, d.ContextBytes)
	}
	return fmt.Sprintf("Cogni 已激活：%s", names)
}

func (p *Planner) maybeEmitCogniTrace(req PlanRequest) {
	if p == nil || p.cogniService == nil || !p.cogniService.HasTrace() || req.StepCallback == nil {
		return
	}
	detail, ok := p.cogniService.Trace(extractUserMessage(req), req.TenantID, req.ChannelType)
	if !ok || !detail.hasVisibleEffect() {
		return
	}
	evt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventPlan, detail.summary())
	evt.Meta.TenantID = req.TenantID
	evt.Meta.TaskID = req.TaskID
	evt.Detail = detail
	req.StepCallback(evt)
}
