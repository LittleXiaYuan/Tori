package planner

import (
	"errors"
	"testing"
	"time"
)

func TestPlannerRuntimeServicesEnsureLazyInitialization(t *testing.T) {
	t.Parallel()

	p := &Planner{}

	if p.ensureContextAssembly() == nil || p.contextAssembly == nil {
		t.Fatal("expected context assembly service to initialize")
	}
	if p.ensureLearningSidecar() == nil || p.learningSidecar == nil {
		t.Fatal("expected learning sidecar to initialize")
	}
	if p.ensureSkillRuntime() == nil || p.skillRuntime == nil {
		t.Fatal("expected skill runtime service to initialize")
	}
	if p.ensureTrustGate() == nil || p.trustGate == nil {
		t.Fatal("expected trust gate to initialize")
	}
	if p.ensureProactiveCognition() == nil || p.proactiveCog == nil {
		t.Fatal("expected proactive cognition service to initialize")
	}
	if p.ensureDelegationRuntime() == nil || p.delegationRuntime == nil {
		t.Fatal("expected delegation runtime service to initialize")
	}
	if p.ensureRuntimeStrategy() == nil || p.runtimeStrategy == nil {
		t.Fatal("expected runtime strategy service to initialize")
	}
	if p.ensurePromptRuntime() == nil || p.promptRuntime == nil {
		t.Fatal("expected prompt runtime service to initialize")
	}
	if p.ensureExecutionRuntime() == nil || p.executionRuntime == nil {
		t.Fatal("expected execution runtime service to initialize")
	}
	if p.ensureContextWindowRuntime() == nil || p.contextWindowRuntime == nil {
		t.Fatal("expected context window runtime service to initialize")
	}
	if p.ensureModelRuntime() == nil || p.modelRuntime == nil {
		t.Fatal("expected model runtime service to initialize")
	}
}

func TestPlannerRuntimeServicesHandoffHooksProvider(t *testing.T) {
	t.Parallel()

	p := &Planner{}
	var metricSkill string
	p.SetSkillMetrics(func(skillName string, _ time.Duration, _ error) {
		metricSkill = skillName
	})
	if p.HandoffMetricsHook() == nil {
		t.Fatal("expected handoff metrics hook")
	}
	p.HandoffMetricsHook()("transfer_to_research", time.Second, errors.New("boom"))
	if metricSkill != "transfer_to_research" {
		t.Fatalf("unexpected metric skill %q", metricSkill)
	}

	bus := NewReverieEventBus(0)
	monitor := NewTaskFailureMonitor(bus, 0.5, time.Minute, 2)
	p.SetTaskFailureMonitor(monitor)
	failureHook := p.HandoffFailureHook()
	if failureHook == nil {
		t.Fatal("expected handoff failure hook")
	}
	if failureHook(true) {
		t.Fatal("first failure should stay below minCalls")
	}
	if !failureHook(true) {
		t.Fatal("second failure should emit proactive event")
	}
}
