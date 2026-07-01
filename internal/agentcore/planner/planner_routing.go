package planner

import (
	"fmt"
	"strings"

	"yunque-agent/internal/observe"
)

// ModelFallbackDetail is emitted with a fallback event so the UI can explain
// the switch without exposing low-level client errors as scary status text.
type ModelFallbackDetail struct {
	Model   string `json:"model,omitempty"`
	Attempt int    `json:"attempt"`
	Reason  string `json:"reason,omitempty"`
}

func modelFallbackSummary(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return "模型暂时没有回应，正在换一个可用模型继续。"
	}
	return fmt.Sprintf("模型暂时没有回应，正在换用 %s 继续。", model)
}

func (p *Planner) emitModelFallbackEvent(req PlanRequest, model string, attempt int, err error) {
	if req.StepCallback == nil {
		return
	}
	evt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventPlan, modelFallbackSummary(model))
	evt.Meta.TenantID = req.TenantID
	evt.Meta.SessionID = req.SessionID
	evt.Meta.TaskID = req.TaskID
	detail := ModelFallbackDetail{Model: model, Attempt: attempt}
	if err != nil {
		detail.Reason = truncate(plannerFriendlyFailureText(err.Error()), 160)
	}
	evt.Detail = detail
	req.StepCallback(evt)
}

func (p *Planner) isComplexTask(req PlanRequest) bool {
	return p.executionMode(req).Mode == PlanExecutionLongHorizon
}

func (p *Planner) modelFallbackEvents(req PlanRequest) ModelFallbackEventFunc {
	return func(model string, attempt int, err error) {
		p.emitModelFallbackEvent(req, model, attempt, err)
	}
}

func (p *Planner) modelReasoningEvents(req PlanRequest) ModelReasoningEventFunc {
	if req.StepCallback == nil {
		return nil
	}
	return func(summary string, detail map[string]any) {
		evt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventThinking, summary)
		evt.Meta.TenantID = req.TenantID
		evt.Meta.SessionID = req.SessionID
		evt.Meta.TaskID = req.TaskID
		evt.Detail = detail
		req.StepCallback(evt)
	}
}
