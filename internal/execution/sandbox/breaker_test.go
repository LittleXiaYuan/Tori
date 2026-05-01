package sandbox

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestCircuitRunner_NormalOperation(t *testing.T) {
	inner := &succeedingRunner{typ: "cloud", stdout: "ok"}
	cr := NewCircuitRunner(inner, DefaultCircuitConfig())

	result, err := cr.Run(context.Background(), RunRequest{Command: "echo test"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout != "ok" {
		t.Fatalf("expected ok, got %s", result.Stdout)
	}
}

func TestCircuitRunner_TripsAfterThreshold(t *testing.T) {
	inner := &failingRunner{failErr: fmt.Errorf("timeout"), typ: "cloud"}
	cfg := CircuitConfig{FailureThreshold: 2, ProbeInterval: 100 * time.Millisecond, HalfOpenMax: 1}
	cr := NewCircuitRunner(inner, cfg)

	for i := 0; i < 2; i++ {
		cr.Run(context.Background(), RunRequest{Command: "test"})
	}

	_, err := cr.Run(context.Background(), RunRequest{Command: "test"})
	if err != ErrSandboxCircuitOpen {
		t.Fatalf("expected ErrSandboxCircuitOpen, got %v", err)
	}
}

func TestCircuitRunner_RecoverAfterProbe(t *testing.T) {
	calls := 0
	toggle := &toggleRunner{failUntil: 2, calls: &calls}
	cfg := CircuitConfig{FailureThreshold: 2, ProbeInterval: 50 * time.Millisecond, HalfOpenMax: 1}
	cr := NewCircuitRunner(toggle, cfg)

	cr.Run(context.Background(), RunRequest{Command: "1"})
	cr.Run(context.Background(), RunRequest{Command: "2"})

	_, err := cr.Run(context.Background(), RunRequest{Command: "blocked"})
	if err != ErrSandboxCircuitOpen {
		t.Fatal("should be open")
	}

	time.Sleep(60 * time.Millisecond)

	result, err := cr.Run(context.Background(), RunRequest{Command: "probe"})
	if err != nil {
		t.Fatalf("probe should succeed: %v", err)
	}
	if result.Stdout != "recovered" {
		t.Fatalf("expected recovered, got %s", result.Stdout)
	}

	stats := cr.BreakerStats()
	if stats["state"] != "closed" {
		t.Fatalf("expected closed after recovery, got %s", stats["state"])
	}
}

func TestCircuitRunner_Stats(t *testing.T) {
	inner := &succeedingRunner{typ: "test", stdout: "ok"}
	cr := NewCircuitRunner(inner, DefaultCircuitConfig())
	stats := cr.BreakerStats()
	if stats["state"] != "closed" {
		t.Fatalf("expected closed, got %v", stats["state"])
	}
}

type toggleRunner struct {
	failUntil int
	calls     *int
}

func (r *toggleRunner) Run(_ context.Context, _ RunRequest) (*RunResult, error) {
	*r.calls++
	if *r.calls <= r.failUntil {
		return nil, fmt.Errorf("fail #%d", *r.calls)
	}
	return &RunResult{Stdout: "recovered", ExitCode: 0}, nil
}
func (r *toggleRunner) Type() string  { return "toggle" }
func (r *toggleRunner) Close() error  { return nil }
