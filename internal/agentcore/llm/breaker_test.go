package llm

import (
	"testing"
	"time"
)

func TestCircuitBreaker_ClosedByDefault(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Second)
	if cb.State() != "closed" {
		t.Fatalf("expected closed, got %s", cb.State())
	}
	if err := cb.Allow(); err != nil {
		t.Fatalf("expected allow, got %v", err)
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Second)
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != "closed" {
		t.Fatalf("should still be closed after 2 failures")
	}
	cb.RecordFailure()
	if cb.State() != "open" {
		t.Fatalf("expected open after 3 failures, got %s", cb.State())
	}
	if err := cb.Allow(); err != ErrCircuitOpen {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreaker_SuccessResetsCount(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Second)
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess()
	if cb.Failures() != 0 {
		t.Fatalf("expected 0 failures after success, got %d", cb.Failures())
	}
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != "open" {
		t.Fatalf("expected open, got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenRecovery(t *testing.T) {
	cb := NewCircuitBreaker(2, 50*time.Millisecond)
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != "open" {
		t.Fatal("expected open")
	}

	// Wait for reset timeout
	time.Sleep(60 * time.Millisecond)

	// Should transition to half-open
	if err := cb.Allow(); err != nil {
		t.Fatalf("expected allow in half-open, got %v", err)
	}
	if cb.State() != "half-open" {
		t.Fatalf("expected half-open, got %s", cb.State())
	}

	// One success not enough (need 2)
	cb.RecordSuccess()
	if cb.State() != "half-open" {
		t.Fatalf("expected still half-open after 1 success, got %s", cb.State())
	}

	// Second success closes it
	cb.RecordSuccess()
	if cb.State() != "closed" {
		t.Fatalf("expected closed after 2 successes, got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenFailureReopens(t *testing.T) {
	cb := NewCircuitBreaker(2, 50*time.Millisecond)
	cb.RecordFailure()
	cb.RecordFailure()

	time.Sleep(60 * time.Millisecond)
	cb.Allow() // transitions to half-open

	cb.RecordFailure()
	if cb.State() != "open" {
		t.Fatalf("expected re-open after failure in half-open, got %s", cb.State())
	}
}
