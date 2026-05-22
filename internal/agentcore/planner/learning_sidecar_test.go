package planner

import (
	"context"
	"testing"
)

type fakeMetaCogSidecar struct {
	escalate bool
	hint     string
	summary  string
	cleared  string
}

func (f *fakeMetaCogSidecar) CorrectionHint(taskID string) string {
	return f.hint
}

func (f *fakeMetaCogSidecar) ShouldEscalate(taskID string) bool {
	return f.escalate
}

func (f *fakeMetaCogSidecar) FormatAnomalySummary(taskID string) string {
	return f.summary
}

func (f *fakeMetaCogSidecar) ClearTask(taskID string) {
	f.cleared = taskID
}

func TestLearningSidecarMetaCogDelegation(t *testing.T) {
	meta := &fakeMetaCogSidecar{
		escalate: true,
		hint:     "修正提示",
		summary:  "metacog[task-1]: loop×1",
	}
	sidecar := NewLearningSidecar()
	sidecar.SetMetaCogSidecar(meta)

	if !sidecar.ShouldEscalate("task-1") {
		t.Fatal("expected escalation")
	}
	if got := sidecar.CorrectionHint("task-1"); got != "修正提示" {
		t.Fatalf("hint = %q, want 修正提示", got)
	}

	sidecar.AfterRun(context.Background(), PlanRequest{TaskID: "task-1"}, &PlanResult{Reply: "ok"}, nil, nil)
	if meta.cleared != "task-1" {
		t.Fatalf("cleared task = %q, want task-1", meta.cleared)
	}
}

func TestPlannerSetLearningSidecarDependencies(t *testing.T) {
	p := &Planner{}
	meta := &fakeMetaCogSidecar{hint: "hint"}
	p.ensureLearningSidecar().SetMetaCogSidecar(meta)

	if p.learningSidecar == nil {
		t.Fatal("expected learning sidecar")
	}
	if got := p.learningSidecar.CorrectionHint("task-1"); got != "hint" {
		t.Fatalf("hint = %q, want hint", got)
	}
}
