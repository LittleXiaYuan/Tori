package cron

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func tmpDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "tori-cron-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestAddAndList(t *testing.T) {
	m := NewManager(tmpDir(t), nil)
	id, err := m.Add("test-job", Schedule{Type: ScheduleEvery, EveryMs: 60000}, Payload{Kind: PayloadSystemEvent, Message: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}
	jobs := m.List()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Name != "test-job" {
		t.Fatalf("expected name test-job, got %s", jobs[0].Name)
	}
}

func TestGetAndRemove(t *testing.T) {
	m := NewManager(tmpDir(t), nil)
	id, _ := m.Add("j1", Schedule{Type: ScheduleEvery, EveryMs: 1000}, Payload{Kind: PayloadAgentTurn})
	j, ok := m.Get(id)
	if !ok || j.Name != "j1" {
		t.Fatal("get failed")
	}
	if err := m.Remove(id); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.Get(id); ok {
		t.Fatal("should not find removed job")
	}
}

func TestRemoveNotFound(t *testing.T) {
	m := NewManager(tmpDir(t), nil)
	if err := m.Remove("nonexistent"); err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdate(t *testing.T) {
	m := NewManager(tmpDir(t), nil)
	id, _ := m.Add("old", Schedule{Type: ScheduleEvery, EveryMs: 5000}, Payload{Kind: PayloadSystemEvent})
	newName := "new-name"
	disabled := false
	if err := m.Update(id, &newName, nil, nil, &disabled); err != nil {
		t.Fatal(err)
	}
	j, _ := m.Get(id)
	if j.Name != "new-name" {
		t.Fatalf("expected new-name, got %s", j.Name)
	}
	if j.Enabled {
		t.Fatal("should be disabled")
	}
}

func TestRunNow(t *testing.T) {
	var called int32
	handler := func(ctx context.Context, job *Job) (string, error) {
		atomic.AddInt32(&called, 1)
		return "done", nil
	}
	m := NewManager(tmpDir(t), handler)
	id, _ := m.Add("manual", Schedule{Type: ScheduleEvery, EveryMs: 999999}, Payload{Kind: PayloadAgentTurn, Message: "go"})
	rec, err := m.RunNow(id)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != RunSuccess {
		t.Fatalf("expected success, got %s", rec.Status)
	}
	if rec.Output != "done" {
		t.Fatalf("expected output 'done', got %q", rec.Output)
	}
	if atomic.LoadInt32(&called) != 1 {
		t.Fatal("handler not called")
	}
}

func TestRunNowNotFound(t *testing.T) {
	m := NewManager(tmpDir(t), nil)
	_, err := m.RunNow("nope")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPersistAndLoad(t *testing.T) {
	dir := tmpDir(t)
	m := NewManager(dir, nil)
	m.Add("persist-me", Schedule{Type: ScheduleEvery, EveryMs: 10000}, Payload{Kind: PayloadSystemEvent})
	m.Stop()

	m2 := NewManager(dir, nil)
	if err := m2.Start(); err != nil {
		t.Fatal(err)
	}
	defer m2.Stop()
	jobs := m2.List()
	if len(jobs) != 1 || jobs[0].Name != "persist-me" {
		t.Fatal("persistence failed")
	}
}

func TestScheduleAtOneShot(t *testing.T) {
	var called int32
	handler := func(ctx context.Context, job *Job) (string, error) {
		atomic.AddInt32(&called, 1)
		return "", nil
	}
	m := NewManager(tmpDir(t), handler)
	at := time.Now().Add(100 * time.Millisecond)
	m.Add("oneshot", Schedule{Type: ScheduleAt, At: &at}, Payload{Kind: PayloadSystemEvent})
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	defer m.Stop()
	time.Sleep(500 * time.Millisecond)
	if atomic.LoadInt32(&called) != 1 {
		t.Fatal("one-shot job should have fired once")
	}
}

func TestScheduleEvery(t *testing.T) {
	var called int32
	handler := func(ctx context.Context, job *Job) (string, error) {
		atomic.AddInt32(&called, 1)
		return "", nil
	}
	m := NewManager(tmpDir(t), handler)
	m.Add("recurring", Schedule{Type: ScheduleEvery, EveryMs: 100}, Payload{Kind: PayloadAgentTurn})
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(350 * time.Millisecond)
	m.Stop()
	c := atomic.LoadInt32(&called)
	if c < 2 {
		t.Fatalf("expected at least 2 calls, got %d", c)
	}
}

func TestRunHistory(t *testing.T) {
	handler := func(ctx context.Context, job *Job) (string, error) {
		return "ok", nil
	}
	dir := tmpDir(t)
	m := NewManager(dir, handler)
	id, _ := m.Add("hist", Schedule{Type: ScheduleEvery, EveryMs: 99999}, Payload{Kind: PayloadSystemEvent})
	m.RunNow(id)
	m.RunNow(id)

	recs, err := m.RunHistory(id, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 {
		t.Fatalf("expected 2 records, got %d", len(recs))
	}
}

func TestCronExprNextRun(t *testing.T) {
	// Every minute: "* * * * *"
	now := time.Date(2026, 3, 9, 10, 30, 0, 0, time.Local)
	next := nextCronTime("* * * * *", "", now)
	if next == nil {
		t.Fatal("expected next time")
	}
	if next.Minute() != 31 {
		t.Fatalf("expected minute 31, got %d", next.Minute())
	}
}

func TestCronExprStep(t *testing.T) {
	// Every 15 minutes: "*/15 * * * *"
	now := time.Date(2026, 3, 9, 10, 14, 0, 0, time.Local)
	next := nextCronTime("*/15 * * * *", "", now)
	if next == nil {
		t.Fatal("expected next time")
	}
	if next.Minute() != 15 {
		t.Fatalf("expected minute 15, got %d", next.Minute())
	}
}

func TestWithOptions(t *testing.T) {
	m := NewManager(tmpDir(t), nil)
	id, _ := m.Add("opt", Schedule{Type: ScheduleEvery, EveryMs: 1000}, Payload{Kind: PayloadAgentTurn},
		WithAgent("agent-1"),
		WithSession("sess-1"),
		WithDelivery(DeliveryWebhook),
	)
	j, _ := m.Get(id)
	if j.AgentID != "agent-1" || j.SessionTarget != "sess-1" || j.Delivery != DeliveryWebhook {
		t.Fatal("options not applied")
	}
}
