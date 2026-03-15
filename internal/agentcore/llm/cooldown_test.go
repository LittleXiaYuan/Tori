package llm

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestEndpointCooldown(t *testing.T) {
	ep := &Endpoint{ID: "ep1", Enabled: true}
	if ep.IsCoolingDown() {
		t.Fatal("should not be cooling down initially")
	}
	cd := ep.RecordFailure()
	if cd <= 0 {
		t.Fatal("expected positive cooldown")
	}
	if !ep.IsCoolingDown() {
		t.Fatal("should be cooling down after failure")
	}
}

func TestEndpointRecordSuccess(t *testing.T) {
	ep := &Endpoint{ID: "ep1", Enabled: true}
	ep.RecordSuccess(100 * time.Millisecond)
	stats := ep.Stats()
	if stats.SuccessCount != 1 {
		t.Fatal("expected 1 success")
	}
	if stats.AvgLatency != 100*time.Millisecond {
		t.Fatal("wrong avg latency")
	}
}

func TestEndpointFailCountResetOnSuccess(t *testing.T) {
	ep := &Endpoint{ID: "ep1", Enabled: true}
	ep.RecordFailure()
	ep.RecordSuccess(10 * time.Millisecond)
	stats := ep.Stats()
	if stats.FailCount != 0 {
		t.Fatal("fail count should reset on success")
	}
}

func TestEndpointHasCapability(t *testing.T) {
	ep := &Endpoint{Capabilities: []Capability{CapChat, CapTools}}
	if !ep.HasCapability(CapChat) {
		t.Fatal("should have chat")
	}
	if ep.HasCapability(CapVision) {
		t.Fatal("should not have vision")
	}
}

func TestRouterSelect(t *testing.T) {
	r := NewEndpointRouter()
	r.AddEndpoint(&Endpoint{ID: "a", Priority: 2, Enabled: true, Capabilities: []Capability{CapChat}})
	r.AddEndpoint(&Endpoint{ID: "b", Priority: 1, Enabled: true, Capabilities: []Capability{CapChat}})

	ep, err := r.Select(CapChat)
	if err != nil {
		t.Fatal(err)
	}
	if ep.ID != "b" {
		t.Fatalf("expected b (lower priority), got %s", ep.ID)
	}
}

func TestRouterSelectSkipsCooling(t *testing.T) {
	r := NewEndpointRouter()
	ep1 := &Endpoint{ID: "a", Priority: 1, Enabled: true, Capabilities: []Capability{CapChat}}
	ep2 := &Endpoint{ID: "b", Priority: 2, Enabled: true, Capabilities: []Capability{CapChat}}
	r.AddEndpoint(ep1)
	r.AddEndpoint(ep2)

	ep1.RecordFailure() // put ep1 in cooldown

	ep, err := r.Select(CapChat)
	if err != nil {
		t.Fatal(err)
	}
	if ep.ID != "b" {
		t.Fatal("should skip cooling endpoint")
	}
}

func TestRouterSelectCapabilityFilter(t *testing.T) {
	r := NewEndpointRouter()
	r.AddEndpoint(&Endpoint{ID: "a", Priority: 1, Enabled: true, Capabilities: []Capability{CapChat}})
	r.AddEndpoint(&Endpoint{ID: "b", Priority: 2, Enabled: true, Capabilities: []Capability{CapChat, CapVision}})

	ep, err := r.Select(CapVision)
	if err != nil {
		t.Fatal(err)
	}
	if ep.ID != "b" {
		t.Fatal("only b has vision")
	}
}

func TestRouterSelectNone(t *testing.T) {
	r := NewEndpointRouter()
	r.AddEndpoint(&Endpoint{ID: "a", Priority: 1, Enabled: true, Capabilities: []Capability{CapChat}})

	_, err := r.Select(CapEmbedding)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRouterSelectDisabled(t *testing.T) {
	r := NewEndpointRouter()
	r.AddEndpoint(&Endpoint{ID: "a", Enabled: false, Capabilities: []Capability{CapChat}})

	_, err := r.Select(CapChat)
	if err == nil {
		t.Fatal("disabled should not be selected")
	}
}

func TestRouterSelectWithFallback(t *testing.T) {
	r := NewEndpointRouter()
	ep := &Endpoint{ID: "a", Priority: 1, Enabled: true, Capabilities: []Capability{CapChat}}
	r.AddEndpoint(ep)
	ep.RecordFailure()

	result, err := r.SelectWithFallback(CapChat)
	if err != nil {
		t.Fatal(err)
	}
	if result.ID != "a" {
		t.Fatal("fallback should return cooling endpoint")
	}
}

func TestRouterRemoveEndpoint(t *testing.T) {
	r := NewEndpointRouter()
	r.AddEndpoint(&Endpoint{ID: "a", Enabled: true, Capabilities: []Capability{CapChat}})
	r.RemoveEndpoint("a")

	_, err := r.Select(CapChat)
	if err == nil {
		t.Fatal("should be empty")
	}
}

func TestRouterModelSwitch(t *testing.T) {
	r := NewEndpointRouter()
	r.AddEndpoint(&Endpoint{ID: "a", Model: "gpt-4", Enabled: true})

	err := r.ModelSwitch("a", "gpt-3.5")
	if err != nil {
		t.Fatal(err)
	}
	eps := r.Endpoints()
	if eps[0].Model != "gpt-3.5" {
		t.Fatal("model not switched")
	}
}

func TestRouterModelSwitchNotFound(t *testing.T) {
	r := NewEndpointRouter()
	err := r.ModelSwitch("nonexistent", "x")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRouterAllStats(t *testing.T) {
	r := NewEndpointRouter()
	r.AddEndpoint(&Endpoint{ID: "a", Enabled: true})
	r.AddEndpoint(&Endpoint{ID: "b", Enabled: true})

	stats := r.AllStats()
	if len(stats) != 2 {
		t.Fatal("expected 2 stats")
	}
}

func TestExecuteWithRetrySuccess(t *testing.T) {
	r := NewEndpointRouter()
	r.AddEndpoint(&Endpoint{ID: "a", Priority: 1, Enabled: true, Capabilities: []Capability{CapChat}})

	err := r.ExecuteWithRetry(context.Background(), 3, func(ctx context.Context, ep *Endpoint) error {
		return nil
	}, CapChat)
	if err != nil {
		t.Fatal(err)
	}
}

func TestExecuteWithRetryFailover(t *testing.T) {
	r := NewEndpointRouter()
	r.AddEndpoint(&Endpoint{ID: "a", Priority: 1, Enabled: true, Capabilities: []Capability{CapChat}})
	r.AddEndpoint(&Endpoint{ID: "b", Priority: 2, Enabled: true, Capabilities: []Capability{CapChat}})

	var attempts int
	err := r.ExecuteWithRetry(context.Background(), 3, func(ctx context.Context, ep *Endpoint) error {
		attempts++
		if attempts == 1 {
			return fmt.Errorf("first fail")
		}
		return nil
	}, CapChat)
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestExecuteWithRetryExhausted(t *testing.T) {
	r := NewEndpointRouter()
	r.AddEndpoint(&Endpoint{ID: "a", Priority: 1, Enabled: true, Capabilities: []Capability{CapChat}})

	err := r.ExecuteWithRetry(context.Background(), 2, func(ctx context.Context, ep *Endpoint) error {
		return fmt.Errorf("always fail")
	}, CapChat)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCooldownDuration(t *testing.T) {
	if cooldownDuration(1) != 30*time.Second {
		t.Fatal("wrong for 1")
	}
	if cooldownDuration(3) != 2*time.Minute {
		t.Fatal("wrong for 3")
	}
	if cooldownDuration(5) != 5*time.Minute {
		t.Fatal("wrong for 5")
	}
	if cooldownDuration(10) != 15*time.Minute {
		t.Fatal("wrong for 10")
	}
}
