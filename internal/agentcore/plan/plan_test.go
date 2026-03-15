package plan

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
)

func TestCreatePlan(t *testing.T) {
	m := NewManager(
		func(ctx context.Context, task string) ([]string, error) {
			return []string{"step 1", "step 2", "step 3"}, nil
		},
		func(ctx context.Context, p *Plan, idx int) (string, []string, error) {
			return "done", nil, nil
		},
	)

	plan, err := m.Create(context.Background(), "build a website")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(plan.Steps))
	}
	if plan.Status != PlanCreated {
		t.Fatalf("expected created, got %s", plan.Status)
	}
}

func TestCreateFromSteps(t *testing.T) {
	m := NewManager(nil, func(ctx context.Context, p *Plan, idx int) (string, []string, error) {
		return "ok", nil, nil
	})
	plan := m.CreateFromSteps("task", []string{"a", "b"})
	if len(plan.Steps) != 2 {
		t.Fatal("expected 2 steps")
	}
}

func TestExecutePlan(t *testing.T) {
	var executed int32
	m := NewManager(nil, func(ctx context.Context, p *Plan, idx int) (string, []string, error) {
		atomic.AddInt32(&executed, 1)
		return fmt.Sprintf("output-%d", idx), []string{"tool-a"}, nil
	})

	plan := m.CreateFromSteps("task", []string{"s1", "s2", "s3"})
	err := m.Execute(context.Background(), plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != PlanCompleted {
		t.Fatalf("expected completed, got %s", plan.Status)
	}
	if atomic.LoadInt32(&executed) != 3 {
		t.Fatal("not all steps executed")
	}
	c, total := plan.Progress()
	if c != 3 || total != 3 {
		t.Fatal("progress wrong")
	}
}

func TestExecuteStepFail(t *testing.T) {
	m := NewManager(nil, func(ctx context.Context, p *Plan, idx int) (string, []string, error) {
		if idx == 1 {
			return "", nil, fmt.Errorf("step 1 broke")
		}
		return "ok", nil, nil
	})

	plan := m.CreateFromSteps("task", []string{"s0", "s1", "s2"})
	err := m.Execute(context.Background(), plan.ID)
	if err == nil {
		t.Fatal("expected error")
	}
	if plan.Status != PlanFailed {
		t.Fatalf("expected failed, got %s", plan.Status)
	}
	if plan.Steps[1].Status != StepFailed {
		t.Fatal("step 1 should be failed")
	}
}

func TestExecuteCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := NewManager(nil, func(ctx context.Context, p *Plan, idx int) (string, []string, error) {
		if idx == 1 {
			cancel()
		}
		return "ok", nil, nil
	})

	plan := m.CreateFromSteps("task", []string{"s0", "s1", "s2"})
	err := m.Execute(ctx, plan.ID)
	if err == nil {
		t.Fatal("expected error")
	}
	if plan.Status != PlanAborted {
		t.Fatalf("expected aborted, got %s", plan.Status)
	}
}

func TestExecuteNotFound(t *testing.T) {
	m := NewManager(nil, nil)
	err := m.Execute(context.Background(), "nope")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestOnStepUpdate(t *testing.T) {
	var updates int32
	m := NewManager(nil, func(ctx context.Context, p *Plan, idx int) (string, []string, error) {
		return "ok", nil, nil
	})
	m.SetOnStepUpdate(func(p *Plan, idx int, status StepStatus) {
		atomic.AddInt32(&updates, 1)
	})

	plan := m.CreateFromSteps("task", []string{"s0", "s1"})
	m.Execute(context.Background(), plan.ID)
	// Each step: in_progress + completed = 2 updates per step
	if atomic.LoadInt32(&updates) != 4 {
		t.Fatalf("expected 4 updates, got %d", updates)
	}
}

func TestSummarize(t *testing.T) {
	m := NewManager(nil, func(ctx context.Context, p *Plan, idx int) (string, []string, error) {
		return "ok", nil, nil
	})
	m.SetSummarize(func(ctx context.Context, p *Plan) (string, error) {
		return "all done successfully", nil
	})

	plan := m.CreateFromSteps("task", []string{"s0"})
	m.Execute(context.Background(), plan.ID)
	if plan.Summary != "all done successfully" {
		t.Fatalf("expected summary, got %q", plan.Summary)
	}
}

func TestUpdateStep(t *testing.T) {
	m := NewManager(nil, nil)
	plan := m.CreateFromSteps("task", []string{"s0", "s1"})
	err := m.UpdateStep(plan.ID, 0, StepCompleted, "manual")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Steps[0].Status != StepCompleted {
		t.Fatal("not updated")
	}
}

func TestSkipStep(t *testing.T) {
	m := NewManager(nil, nil)
	plan := m.CreateFromSteps("task", []string{"s0"})
	m.SkipStep(plan.ID, 0)
	if plan.Steps[0].Status != StepSkipped {
		t.Fatal("not skipped")
	}
}

func TestCurrentStep(t *testing.T) {
	m := NewManager(nil, nil)
	plan := m.CreateFromSteps("task", []string{"s0", "s1", "s2"})
	plan.Steps[0].Status = StepCompleted
	if plan.CurrentStep() != 1 {
		t.Fatal("expected step 1")
	}
}

func TestIsComplete(t *testing.T) {
	m := NewManager(nil, nil)
	plan := m.CreateFromSteps("task", []string{"s0", "s1"})
	if plan.IsComplete() {
		t.Fatal("should not be complete")
	}
	plan.Steps[0].Status = StepCompleted
	plan.Steps[1].Status = StepSkipped
	if !plan.IsComplete() {
		t.Fatal("should be complete")
	}
}

func TestListAndActive(t *testing.T) {
	m := NewManager(nil, nil)
	m.CreateFromSteps("a", []string{"s"})
	p2 := m.CreateFromSteps("b", []string{"s"})
	p2.Status = PlanCompleted

	if len(m.List()) != 2 {
		t.Fatal("expected 2")
	}
	active := m.ActivePlans()
	if len(active) != 1 {
		t.Fatalf("expected 1 active, got %d", len(active))
	}
}

func TestRemove(t *testing.T) {
	m := NewManager(nil, nil)
	plan := m.CreateFromSteps("task", []string{"s"})
	m.Remove(plan.ID)
	if _, ok := m.Get(plan.ID); ok {
		t.Fatal("should be removed")
	}
}

func TestNeedsPlan(t *testing.T) {
	if !NeedsPlan("first do X then Y") {
		t.Fatal("should detect plan need")
	}
	if !NeedsPlan("首先做A然后做B") {
		t.Fatal("should detect Chinese plan keywords")
	}
	if NeedsPlan("hello world") {
		t.Fatal("should not need plan")
	}
}

func TestCreateNoDecompose(t *testing.T) {
	m := NewManager(nil, nil)
	_, err := m.Create(context.Background(), "task")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateDecomposeEmpty(t *testing.T) {
	m := NewManager(
		func(ctx context.Context, task string) ([]string, error) {
			return nil, nil
		}, nil,
	)
	_, err := m.Create(context.Background(), "task")
	if err == nil {
		t.Fatal("expected error for empty steps")
	}
}
