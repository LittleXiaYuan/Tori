package heartbeat

import (
	"context"
	"testing"
	"time"
)

func TestHeartbeatTrigger(t *testing.T) {
	called := false
	svc := New(Config{Interval: time.Hour, MaxLogs: 10}, func(ctx context.Context) (string, error) {
		called = true
		return "beat ok", nil
	})

	entry := svc.Trigger(context.Background())
	if !called {
		t.Fatal("task not called")
	}
	if entry == nil || entry.Status != "ok" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	if entry.Result != "beat ok" {
		t.Fatalf("result: got %s", entry.Result)
	}
}

func TestHeartbeatLogs(t *testing.T) {
	svc := New(Config{Interval: time.Hour, MaxLogs: 5}, func(ctx context.Context) (string, error) {
		return "ok", nil
	})

	for i := 0; i < 7; i++ {
		svc.Trigger(context.Background())
	}

	logs := svc.Logs(0) // 0 = all
	if len(logs) != 5 {
		t.Fatalf("expected 5 logs (capped), got %d", len(logs))
	}
	// Newest first
	if logs[0].ID == logs[1].ID {
		t.Fatal("logs should have unique IDs")
	}
}

func TestHeartbeatClearLogs(t *testing.T) {
	svc := New(Config{Interval: time.Hour, MaxLogs: 10}, func(ctx context.Context) (string, error) {
		return "", nil
	})
	svc.Trigger(context.Background())
	svc.ClearLogs()
	if len(svc.Logs(0)) != 0 {
		t.Fatal("logs should be empty after clear")
	}
}

func TestHeartbeatStartStop(t *testing.T) {
	svc := New(Config{Interval: 50 * time.Millisecond, Enabled: true, MaxLogs: 10}, func(ctx context.Context) (string, error) {
		return "tick", nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc.Start(ctx)
	if !svc.IsRunning() {
		t.Fatal("should be running")
	}

	time.Sleep(120 * time.Millisecond)

	svc.Stop()
	if svc.IsRunning() {
		t.Fatal("should not be running")
	}

	logs := svc.Logs(0)
	if len(logs) == 0 {
		t.Fatal("expected at least one log from auto-tick")
	}
}

func TestHeartbeatDisabledNoStart(t *testing.T) {
	svc := New(Config{Interval: time.Millisecond, Enabled: false}, func(ctx context.Context) (string, error) {
		return "", nil
	})
	svc.Start(context.Background())
	if svc.IsRunning() {
		t.Fatal("disabled heartbeat should not start")
	}
}

func TestHeartbeatErrorLogging(t *testing.T) {
	svc := New(Config{Interval: time.Hour, MaxLogs: 10}, func(ctx context.Context) (string, error) {
		return "", context.DeadlineExceeded
	})
	entry := svc.Trigger(context.Background())
	if entry.Status != "error" {
		t.Fatalf("expected error status, got %s", entry.Status)
	}
	if entry.Error == "" {
		t.Fatal("error message should not be empty")
	}
}
