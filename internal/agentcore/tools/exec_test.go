package tools

import (
	"context"
	"testing"
	"time"
)

func TestExecSync(t *testing.T) {
	pm := NewProcessManager()
	res, err := pm.Exec(context.Background(), ExecOptions{
		Command: "echo hello world",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.State != ProcessFinished {
		t.Fatalf("expected finished, got %s", res.State)
	}
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", res.ExitCode)
	}
}

func TestExecEmpty(t *testing.T) {
	pm := NewProcessManager()
	_, err := pm.Exec(context.Background(), ExecOptions{})
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestExecBackground(t *testing.T) {
	pm := NewProcessManager()
	res, err := pm.Exec(context.Background(), ExecOptions{
		Command:    "ping -n 100 127.0.0.1",
		Background: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.State != ProcessRunning {
		t.Fatalf("expected running, got %s", res.State)
	}
	if res.SessionID == "" {
		t.Fatal("expected session ID")
	}

	time.Sleep(500 * time.Millisecond)

	// Poll
	lines, state, err := pm.PollSession(res.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if state != ProcessRunning {
		t.Fatalf("expected running, got %s", state)
	}
	if len(lines) == 0 {
		t.Fatal("expected some output")
	}

	// Kill
	if err := pm.Kill(res.SessionID); err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)

	// Clear
	if err := pm.Clear(res.SessionID); err != nil {
		t.Fatal(err)
	}

	if len(pm.List()) != 0 {
		t.Fatal("expected empty list")
	}
}

func TestExecList(t *testing.T) {
	pm := NewProcessManager()
	res, _ := pm.Exec(context.Background(), ExecOptions{
		Command:    "ping -n 100 127.0.0.1",
		Background: true,
	})

	list := pm.List()
	if len(list) != 1 {
		t.Fatalf("expected 1, got %d", len(list))
	}

	pm.Kill(res.SessionID)
	time.Sleep(200 * time.Millisecond)
	pm.Clear(res.SessionID)
}

func TestExecLog(t *testing.T) {
	pm := NewProcessManager()
	res, _ := pm.Exec(context.Background(), ExecOptions{
		Command:    "echo line1 && echo line2",
		Background: true,
	})

	time.Sleep(500 * time.Millisecond)

	lines, err := pm.Log(res.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}

	pm.Remove(res.SessionID)
}

func TestExecRemove(t *testing.T) {
	pm := NewProcessManager()
	res, _ := pm.Exec(context.Background(), ExecOptions{
		Command:    "ping -n 100 127.0.0.1",
		Background: true,
	})

	time.Sleep(200 * time.Millisecond)

	// Remove should kill + clear
	if err := pm.Remove(res.SessionID); err != nil {
		t.Fatal(err)
	}
	if len(pm.List()) != 0 {
		t.Fatal("expected empty")
	}
}

func TestPollNotFound(t *testing.T) {
	pm := NewProcessManager()
	_, _, err := pm.PollSession("nope")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestKillNotRunning(t *testing.T) {
	pm := NewProcessManager()
	res, _ := pm.Exec(context.Background(), ExecOptions{
		Command:    "echo done",
		Background: true,
	})
	time.Sleep(500 * time.Millisecond)

	err := pm.Kill(res.SessionID)
	if err == nil {
		t.Fatal("expected error killing finished process")
	}
	pm.Clear(res.SessionID)
}

func TestClearRunning(t *testing.T) {
	pm := NewProcessManager()
	res, _ := pm.Exec(context.Background(), ExecOptions{
		Command:    "ping -n 100 127.0.0.1",
		Background: true,
	})

	err := pm.Clear(res.SessionID)
	if err == nil {
		t.Fatal("expected error clearing running process")
	}

	pm.Kill(res.SessionID)
	time.Sleep(200 * time.Millisecond)
	pm.Clear(res.SessionID)
}
