package circuit

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func okLLM(_ context.Context, _, _ string) (string, error) {
	return "ok response", nil
}

func failLLM(_ context.Context, _, _ string) (string, error) {
	return "", fmt.Errorf("API error 500")
}

func TestClosedStateNormal(t *testing.T) {
	b := New(okLLM, Config{FailureThreshold: 3, RecoveryTime: time.Second})
	resp, err := b.Call(context.Background(), "sys", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if resp != "ok response" {
		t.Fatalf("expected 'ok response', got %q", resp)
	}
	if b.State() != StateClosed {
		t.Fatalf("expected closed, got %s", b.State())
	}
}

func TestTripsAfterThreshold(t *testing.T) {
	b := New(failLLM, Config{FailureThreshold: 3, RecoveryTime: time.Second})

	for i := 0; i < 3; i++ {
		b.Call(context.Background(), "sys", "hello")
	}

	if b.State() != StateOpen {
		t.Fatalf("expected open after 3 failures, got %s", b.State())
	}
}

func TestFallbackOnFailure(t *testing.T) {
	fallbackCalled := false
	fallback := func(_ context.Context, _, _ string) (string, error) {
		fallbackCalled = true
		return "fallback response", nil
	}

	b := New(failLLM, Config{FailureThreshold: 5})
	b.AddFallback("backup", fallback)

	resp, err := b.Call(context.Background(), "sys", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if !fallbackCalled {
		t.Fatal("fallback should have been called")
	}
	if resp != "fallback response" {
		t.Fatalf("expected fallback response, got %q", resp)
	}
}

func TestCacheOnAllFail(t *testing.T) {
	// First: succeed to populate cache
	callCount := 0
	flaky := func(_ context.Context, _, _ string) (string, error) {
		callCount++
		if callCount == 1 {
			return "cached answer", nil
		}
		return "", fmt.Errorf("down")
	}

	b := New(flaky, Config{FailureThreshold: 1, RecoveryTime: 10 * time.Second})

	// First call succeeds — caches
	resp, err := b.Call(context.Background(), "sys", "question")
	if err != nil {
		t.Fatal(err)
	}
	if resp != "cached answer" {
		t.Fatalf("expected cached answer, got %q", resp)
	}

	// Second call fails — should serve cache (circuit now open after 1 failure)
	// Need to wait or manually handle state; let's trigger via half-open
	b.mu.Lock()
	b.state = StateOpen
	b.lastFailure = time.Now().Add(-20 * time.Second) // simulate past
	b.mu.Unlock()

	// Now it transitions to half-open, tries, fails, serves cache
	resp2, err := b.Call(context.Background(), "sys", "question")
	if err != nil {
		t.Fatal(err)
	}
	if len(resp2) == 0 {
		t.Fatal("expected cached response")
	}
}

func TestHalfOpenRecovery(t *testing.T) {
	callCount := 0
	recovering := func(_ context.Context, _, _ string) (string, error) {
		callCount++
		if callCount <= 3 {
			return "", fmt.Errorf("still down")
		}
		return "recovered", nil
	}

	b := New(recovering, Config{FailureThreshold: 2, RecoveryTime: 1 * time.Millisecond, HalfOpenMax: 2})

	// Trip the breaker
	b.Call(context.Background(), "sys", "q1")
	b.Call(context.Background(), "sys", "q2")
	if b.State() != StateOpen {
		t.Fatalf("expected open, got %s", b.State())
	}

	// Wait for recovery time
	time.Sleep(5 * time.Millisecond)

	// Call again — transitions to half-open, but still fails (call 3)
	b.Call(context.Background(), "sys", "q3")

	// Wait again
	time.Sleep(5 * time.Millisecond)

	// Now it succeeds (call 4+)
	resp, err := b.Call(context.Background(), "sys", "q4")
	if err != nil {
		t.Fatalf("should recover: %v", err)
	}
	if resp != "recovered" {
		t.Fatalf("expected recovered, got %q", resp)
	}
}

func TestReset(t *testing.T) {
	b := New(failLLM, Config{FailureThreshold: 1})
	b.Call(context.Background(), "sys", "q")
	if b.State() != StateOpen {
		t.Fatal("should be open")
	}

	b.Reset()
	if b.State() != StateClosed {
		t.Fatal("should be closed after reset")
	}
}

func TestStats(t *testing.T) {
	b := New(okLLM, Config{})
	b.AddFallback("fb1", okLLM)
	b.Call(context.Background(), "sys", "q")

	stats := b.Stats()
	if stats["state"] != "closed" {
		t.Fatalf("expected closed, got %v", stats["state"])
	}
	if stats["fallbacks"].(int) != 1 {
		t.Fatal("expected 1 fallback")
	}
}
