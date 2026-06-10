package planner

import (
	"context"
	"errors"
	"testing"
	"time"

	"yunque-agent/internal/agentcore/llm"
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

	sidecar.AfterRun(context.Background(), PlanRequest{TaskID: "task-1"}, &PlanResult{Reply: "ok"}, nil, nil, 0)
	if meta.cleared != "task-1" {
		t.Fatalf("cleared task = %q, want task-1", meta.cleared)
	}
}

func TestLearningSidecarTaskOutcomeSink(t *testing.T) {
	sidecar := NewLearningSidecar()

	type sinkCall struct {
		req     PlanRequest
		result  *PlanResult
		runErr  error
		score   float64
		elapsed time.Duration
	}
	got := make(chan sinkCall, 1)
	sidecar.SetTaskOutcomeSink(func(_ context.Context, req PlanRequest, result *PlanResult, runErr error, score float64, elapsed time.Duration) {
		got <- sinkCall{req: req, result: result, runErr: runErr, score: score, elapsed: elapsed}
	})

	reflect := func(_ context.Context, _, _ string) bool { return true }
	req := PlanRequest{
		TaskID:   "task-sink",
		TenantID: "tenant-sink",
		Messages: []llm.Message{{Role: "user", Content: "帮我跑一个足够长的任务请求"}},
	}
	sidecar.AfterRun(context.Background(), req, &PlanResult{Reply: "完成", Steps: 2}, nil, reflect, 1500*time.Millisecond)

	select {
	case call := <-got:
		if call.req.TaskID != "task-sink" || call.req.TenantID != "tenant-sink" {
			t.Fatalf("unexpected req: %#v", call.req)
		}
		if call.result == nil || call.result.Reply != "完成" {
			t.Fatalf("unexpected result: %#v", call.result)
		}
		if call.runErr != nil {
			t.Fatalf("unexpected err: %v", call.runErr)
		}
		if call.score != 0.8 {
			t.Fatalf("score = %v, want 0.8 (reflect passed)", call.score)
		}
		if call.elapsed != 1500*time.Millisecond {
			t.Fatalf("elapsed = %v", call.elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("outcome sink was not invoked")
	}
}

func TestLearningSidecarTaskOutcomeSinkReceivesFailures(t *testing.T) {
	sidecar := NewLearningSidecar()
	got := make(chan error, 1)
	sidecar.SetTaskOutcomeSink(func(_ context.Context, _ PlanRequest, _ *PlanResult, runErr error, _ float64, _ time.Duration) {
		got <- runErr
	})

	wantErr := errors.New("boom")
	sidecar.AfterRun(context.Background(), PlanRequest{TaskID: "task-fail"}, nil, wantErr, nil, 0)

	select {
	case runErr := <-got:
		if runErr == nil || runErr.Error() != "boom" {
			t.Fatalf("runErr = %v, want boom", runErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("outcome sink was not invoked for failed run")
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
