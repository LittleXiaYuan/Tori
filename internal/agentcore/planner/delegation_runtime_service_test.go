package planner

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"yunque-agent/internal/agentcore/subagent"
	"yunque-agent/internal/observe"
	"yunque-agent/pkg/opp"
)

func TestDelegationRuntimeServiceHandoffBoundary(t *testing.T) {
	mgr := subagent.NewManager()
	reg := subagent.NewHandoffRegistry(mgr)
	reg.SetRunFunc(func(_ context.Context, agentName, input, providerOverride string) (string, error) {
		if agentName != "research" {
			t.Fatalf("unexpected agent %q", agentName)
		}
		if input != "collect evidence" {
			t.Fatalf("unexpected input %q", input)
		}
		if providerOverride != "smart" {
			t.Fatalf("unexpected provider override %q", providerOverride)
		}
		return "handoff done", nil
	})
	if err := reg.Register(subagent.HandoffConfig{Name: "research", Description: "research agent"}); err != nil {
		t.Fatalf("register handoff: %v", err)
	}

	service := NewDelegationRuntimeService()
	service.SetHandoffRegistry(reg)

	if agent, ok := service.IsHandoffCall("transfer_to_research"); !ok || agent != "research" {
		t.Fatalf("expected research handoff call, got agent=%q ok=%v", agent, ok)
	}
	if service.HasHandoffAgents(2) {
		t.Fatal("expected one handoff agent to be below threshold 2")
	}
	if defs := service.HandoffToolDefinitions(); len(defs) != 1 {
		t.Fatalf("expected one handoff tool definition, got %d", len(defs))
	}
	got, err := service.ExecuteHandoff(context.Background(), "tenant-a", "research", "collect evidence", "smart")
	if err != nil {
		t.Fatalf("execute handoff: %v", err)
	}
	if got.Reply != "handoff done" {
		t.Fatalf("unexpected handoff reply %q", got.Reply)
	}
}

func TestDelegationRuntimeServiceExecuteHandoffForRequestEmitsEventsAndHooks(t *testing.T) {
	mgr := subagent.NewManager()
	reg := subagent.NewHandoffRegistry(mgr)
	reg.SetRunFunc(func(_ context.Context, agentName, input, providerOverride string) (string, error) {
		if agentName != "research" || input != "collect evidence" || providerOverride != "smart" {
			t.Fatalf("unexpected handoff call agent=%q input=%q provider=%q", agentName, input, providerOverride)
		}
		if StepCallbackFromCtx(context.Background()) != nil {
			t.Fatal("empty context should not contain callback")
		}
		return "handoff done", nil
	})
	if err := reg.Register(subagent.HandoffConfig{Name: "research", Description: "research agent"}); err != nil {
		t.Fatalf("register handoff: %v", err)
	}

	service := NewDelegationRuntimeService()
	service.SetHandoffRegistry(reg)
	var events []observe.AgentEvent
	var metricSkill string
	var metricErr error
	var failureRecorded []bool
	result := service.ExecuteHandoffForRequest(
		context.Background(),
		PlanRequest{
			TenantID:      "tenant-a",
			TraceID:       "trace-a",
			ModelOverride: "smart",
			StepCallback:  func(evt observe.AgentEvent) { events = append(events, evt) },
			DisableTools:  false,
			AllowedSkills: nil,
		},
		"transfer_to_research",
		map[string]any{"input": "collect evidence"},
		"unit",
		3,
		HandoffExecutionHooks{
			Metrics: func(skillName string, _ time.Duration, err error) {
				metricSkill = skillName
				metricErr = err
			},
			RecordExecutionFailure: func(failed bool) bool {
				failureRecorded = append(failureRecorded, failed)
				return false
			},
		},
	)

	if !result.Handled || result.AgentName != "research" || result.Reply != "handoff done" || result.Err != nil {
		t.Fatalf("unexpected handoff result %#v", result)
	}
	if metricSkill != "transfer_to_research" || metricErr != nil {
		t.Fatalf("unexpected metric skill=%q err=%v", metricSkill, metricErr)
	}
	if len(failureRecorded) != 1 || failureRecorded[0] {
		t.Fatalf("expected one successful failure-record hook, got %#v", failureRecorded)
	}
	if len(events) != 2 {
		t.Fatalf("expected start/done events, got %d", len(events))
	}
	if events[0].Type != observe.EventHandoffStart || events[1].Type != observe.EventHandoffDone {
		t.Fatalf("unexpected event types %q %q", events[0].Type, events[1].Type)
	}
	if events[0].Meta.TenantID != "tenant-a" || events[0].Meta.Skill != "research" {
		t.Fatalf("unexpected start metadata %#v", events[0].Meta)
	}
	if detail, ok := events[1].Detail.(observe.HandoffDetail); !ok || detail.Reply != "handoff done" || detail.Agent != "research" {
		t.Fatalf("unexpected done detail %#v", events[1].Detail)
	}
}

func TestDelegationRuntimeServiceExecuteHandoffForRequestUsesFirstStringArgAndFailureDetail(t *testing.T) {
	mgr := subagent.NewManager()
	reg := subagent.NewHandoffRegistry(mgr)
	reg.SetRunFunc(func(_ context.Context, _ string, input, _ string) (string, error) {
		if input != "fallback input" {
			t.Fatalf("expected fallback input, got %q", input)
		}
		return "", context.DeadlineExceeded
	})
	if err := reg.Register(subagent.HandoffConfig{Name: "research"}); err != nil {
		t.Fatalf("register handoff: %v", err)
	}

	service := NewDelegationRuntimeService()
	service.SetHandoffRegistry(reg)
	var events []observe.AgentEvent
	var failureRecorded []bool
	result := service.ExecuteHandoffForRequest(
		context.Background(),
		PlanRequest{TraceID: "trace-b", StepCallback: func(evt observe.AgentEvent) { events = append(events, evt) }},
		"transfer_to_research",
		map[string]any{"query": "fallback input"},
		"text",
		1,
		HandoffExecutionHooks{RecordExecutionFailure: func(failed bool) bool {
			failureRecorded = append(failureRecorded, failed)
			return failed
		}},
	)

	if !result.Handled || result.Err == nil || !errors.Is(result.Err, context.DeadlineExceeded) {
		t.Fatalf("expected handled timeout error, got %#v", result)
	}
	if len(failureRecorded) != 1 || !failureRecorded[0] {
		t.Fatalf("expected failure-record hook, got %#v", failureRecorded)
	}
	if len(events) != 2 || events[1].Type != observe.EventHandoffDone {
		t.Fatalf("expected handoff done event, got %#v", events)
	}
	if !strings.Contains(events[1].Summary, "响应超时") {
		t.Fatalf("expected friendly timeout summary, got %q", events[1].Summary)
	}
	detail, ok := events[1].Detail.(observe.HandoffDetail)
	if !ok || !detail.Recoverable || detail.NextStep == "" || detail.Error == "" {
		t.Fatalf("expected recoverable failure detail, got %#v", events[1].Detail)
	}
}

func TestDelegationRuntimeServiceHandoffTimeoutAndNonHandoffRequest(t *testing.T) {
	service := NewDelegationRuntimeService()
	if got := service.HandoffTimeoutForTool("transfer_to_research", 5*time.Second); got != 5*time.Second {
		t.Fatalf("nil registry should keep fallback timeout, got %v", got)
	}
	result := service.ExecuteHandoffForRequest(context.Background(), PlanRequest{}, "regular_skill", nil, "unit", 1, HandoffExecutionHooks{})
	if result.Handled || result.ToolName != "regular_skill" {
		t.Fatalf("expected non-handoff result, got %#v", result)
	}

	reg := subagent.NewHandoffRegistry(subagent.NewManager())
	if err := reg.Register(subagent.HandoffConfig{Name: "research"}); err != nil {
		t.Fatalf("register handoff: %v", err)
	}
	service.SetHandoffRegistry(reg)
	if got := service.HandoffTimeoutForTool("transfer_to_research", 5*time.Second); got != 90*time.Second {
		t.Fatalf("expected handoff timeout, got %v", got)
	}
}

type fakeHandoffHookProvider struct {
	metrics SkillMetricsFunc
	failure func(bool) bool
}

func (p fakeHandoffHookProvider) HandoffMetricsHook() SkillMetricsFunc {
	return p.metrics
}

func (p fakeHandoffHookProvider) HandoffFailureHook() func(bool) bool {
	return p.failure
}

func TestDelegationRuntimeServiceHandoffHooks(t *testing.T) {
	service := NewDelegationRuntimeService()
	if hooks := service.HandoffHooks(nil); hooks.Metrics != nil || hooks.RecordExecutionFailure != nil {
		t.Fatalf("nil provider should produce empty hooks, got %#v", hooks)
	}

	var metricSkill string
	var failureRecorded []bool
	hooks := service.HandoffHooks(fakeHandoffHookProvider{
		metrics: func(skillName string, _ time.Duration, _ error) {
			metricSkill = skillName
		},
		failure: func(failed bool) bool {
			failureRecorded = append(failureRecorded, failed)
			return failed
		},
	})
	hooks.Metrics("transfer_to_research", time.Second, nil)
	if !hooks.RecordExecutionFailure(true) {
		t.Fatal("expected provider failure hook to be used")
	}
	if metricSkill != "transfer_to_research" || len(failureRecorded) != 1 || !failureRecorded[0] {
		t.Fatalf("unexpected hook side effects: metric=%q failures=%#v", metricSkill, failureRecorded)
	}
}

type fakeFederationBridge struct{}

func (fakeFederationBridge) Delegate(context.Context, opp.DelegatePayload, time.Duration) (*opp.DelegateResultPayload, error) {
	return nil, nil
}

func (fakeFederationBridge) LocalCaps() opp.CapabilitiesPayload {
	return opp.CapabilitiesPayload{}
}

func TestDelegationRuntimeServiceFederationBoundary(t *testing.T) {
	service := NewDelegationRuntimeService()
	bridge := fakeFederationBridge{}
	service.SetFederationBridge(bridge)
	if service.FederationBridge() == nil {
		t.Fatal("expected federation bridge to be attached")
	}
}

func TestNilDelegationRuntimeServiceIsNoop(t *testing.T) {
	var service *DelegationRuntimeService
	if agent, ok := service.IsHandoffCall("transfer_to_research"); ok || agent != "" {
		t.Fatalf("nil service should not match handoff, got agent=%q ok=%v", agent, ok)
	}
	if service.HasHandoffAgents(1) {
		t.Fatal("nil service should not report handoff agents")
	}
	if defs := service.HandoffToolDefinitions(); defs != nil {
		t.Fatalf("nil service should return no handoff definitions, got %#v", defs)
	}
	if service.FederationBridge() != nil {
		t.Fatal("nil service should have no federation bridge")
	}
	if _, err := service.ExecuteHandoff(context.Background(), "", "", "", ""); err != ErrHandoffRegistryNotConfigured {
		t.Fatalf("expected ErrHandoffRegistryNotConfigured, got %v", err)
	}
}
