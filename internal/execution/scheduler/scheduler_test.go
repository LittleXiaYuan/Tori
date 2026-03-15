package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestAddAndList(t *testing.T) {
	s := New(func(ctx context.Context, job Job) {})
	s.Add(Job{ID: "j1", Name: "test1", Interval: time.Hour})
	s.Add(Job{ID: "j2", Name: "test2", Interval: time.Minute})
	jobs := s.List()
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
}

func TestRemove(t *testing.T) {
	s := New(func(ctx context.Context, job Job) {})
	s.Add(Job{ID: "j1", Name: "test1", Interval: time.Hour})
	s.Remove("j1")
	if len(s.List()) != 0 {
		t.Fatal("expected 0 jobs after remove")
	}
}

func TestSetEnabled(t *testing.T) {
	s := New(func(ctx context.Context, job Job) {})
	s.Add(Job{ID: "j1", Name: "test1", Interval: time.Hour})
	s.SetEnabled("j1", false)
	jobs := s.List()
	if jobs[0].Enabled {
		t.Fatal("expected job to be disabled")
	}
}

func TestTickTriggersJob(t *testing.T) {
	var called atomic.Int32
	s := New(func(ctx context.Context, job Job) {
		called.Add(1)
	})
	s.Add(Job{ID: "j1", Name: "fast", Interval: 1 * time.Millisecond})

	// Wait for NextRun to be in the past
	time.Sleep(5 * time.Millisecond)
	s.tick(context.Background(), time.Now())

	// Give goroutine time to execute
	time.Sleep(50 * time.Millisecond)
	if called.Load() != 1 {
		t.Fatalf("expected handler called once, got %d", called.Load())
	}
}

func TestDisabledJobNotTriggered(t *testing.T) {
	var called atomic.Int32
	s := New(func(ctx context.Context, job Job) {
		called.Add(1)
	})
	s.Add(Job{ID: "j1", Name: "disabled", Interval: 1 * time.Millisecond})
	s.SetEnabled("j1", false)

	time.Sleep(5 * time.Millisecond)
	s.tick(context.Background(), time.Now())
	time.Sleep(50 * time.Millisecond)

	if called.Load() != 0 {
		t.Fatal("disabled job should not trigger")
	}
}
