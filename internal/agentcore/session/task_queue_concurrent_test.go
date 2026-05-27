package session

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestTaskQueue_ConcurrentExecution verifies that tasks can run concurrently.
func TestTaskQueue_ConcurrentExecution(t *testing.T) {
	var (
		runningCount int32
		maxRunning   int32
		mu           sync.Mutex
		startTimes   = make(map[string]time.Time)
	)

	handler := func(ctx context.Context, entry *TaskEntry) (string, error) {
		// Track when this task started
		mu.Lock()
		startTimes[entry.ID] = time.Now()
		mu.Unlock()

		// Increment running count
		current := atomic.AddInt32(&runningCount, 1)

		// Track max concurrent
		for {
			max := atomic.LoadInt32(&maxRunning)
			if current <= max || atomic.CompareAndSwapInt32(&maxRunning, max, current) {
				break
			}
		}

		// Simulate work
		time.Sleep(100 * time.Millisecond)

		// Decrement running count
		atomic.AddInt32(&runningCount, -1)
		return "done", nil
	}

	t.Run("serial execution (maxConcurrent=1)", func(t *testing.T) {
		atomic.StoreInt32(&runningCount, 0)
		atomic.StoreInt32(&maxRunning, 0)
		mu.Lock()
		startTimes = make(map[string]time.Time)
		mu.Unlock()

		q := NewTaskQueue("test-session", handler, 10)
		q.SetMaxConcurrent(1) // serial

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go q.Start(ctx)

		// Enqueue 3 tasks
		for i := 0; i < 3; i++ {
			err := q.Enqueue(&TaskEntry{
				ID:       string(rune('A' + i)),
				Prompt:   "test",
				Parallel: true, // marked parallel but should still be serial
			})
			if err != nil {
				t.Fatal(err)
			}
		}

		// Wait for completion
		time.Sleep(500 * time.Millisecond)

		max := atomic.LoadInt32(&maxRunning)
		if max != 1 {
			t.Errorf("expected max 1 concurrent task, got %d", max)
		}
	})

	t.Run("concurrent execution (maxConcurrent=3)", func(t *testing.T) {
		atomic.StoreInt32(&runningCount, 0)
		atomic.StoreInt32(&maxRunning, 0)
		mu.Lock()
		startTimes = make(map[string]time.Time)
		mu.Unlock()

		q := NewTaskQueue("test-session", handler, 10)
		q.SetMaxConcurrent(3) // concurrent

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go q.Start(ctx)

		// Enqueue 5 tasks
		for i := 0; i < 5; i++ {
			err := q.Enqueue(&TaskEntry{
				ID:       string(rune('A' + i)),
				Prompt:   "test",
				Parallel: true,
			})
			if err != nil {
				t.Fatal(err)
			}
		}

		// Wait for completion
		time.Sleep(500 * time.Millisecond)

		max := atomic.LoadInt32(&maxRunning)
		if max < 2 {
			t.Errorf("expected at least 2 concurrent tasks, got %d", max)
		}
		if max > 3 {
			t.Errorf("expected max 3 concurrent tasks, got %d", max)
		}

		// Verify tasks started concurrently
		mu.Lock()
		defer mu.Unlock()
		if len(startTimes) < 5 {
			t.Errorf("expected 5 tasks to start, got %d", len(startTimes))
		}

		// Check that at least 2 tasks started within 50ms of each other
		var times []time.Time
		for _, ts := range startTimes {
			times = append(times, ts)
		}
		if len(times) >= 2 {
			diff := times[1].Sub(times[0])
			if diff > 50*time.Millisecond {
				t.Logf("warning: tasks may not have started concurrently (diff=%v)", diff)
			}
		}
	})

	t.Run("non-parallel task blocks others", func(t *testing.T) {
		atomic.StoreInt32(&runningCount, 0)
		atomic.StoreInt32(&maxRunning, 0)

		q := NewTaskQueue("test-session", handler, 10)
		q.SetMaxConcurrent(3)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go q.Start(ctx)

		// Enqueue a non-parallel task first
		err := q.Enqueue(&TaskEntry{
			ID:       "blocking",
			Prompt:   "test",
			Parallel: false, // blocks others
		})
		if err != nil {
			t.Fatal(err)
		}

		// Enqueue parallel tasks
		for i := 0; i < 3; i++ {
			err := q.Enqueue(&TaskEntry{
				ID:       string(rune('A' + i)),
				Prompt:   "test",
				Parallel: true,
			})
			if err != nil {
				t.Fatal(err)
			}
		}

		// Wait a bit
		time.Sleep(150 * time.Millisecond)

		// Should only have 1 running (the blocking task)
		running := q.Running()
		if running > 1 {
			t.Errorf("non-parallel task should block others, got %d running", running)
		}

		// Wait for completion
		time.Sleep(500 * time.Millisecond)
	})
}

// TestTaskQueue_ConcurrencyMethods verifies the concurrency tracking methods.
func TestTaskQueue_ConcurrencyMethods(t *testing.T) {
	handler := func(ctx context.Context, entry *TaskEntry) (string, error) {
		time.Sleep(50 * time.Millisecond)
		return "done", nil
	}

	q := NewTaskQueue("test-session", handler, 10)
	q.SetMaxConcurrent(3)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go q.Start(ctx)

	// Initially no tasks
	running := q.Running()
	if running != 0 {
		t.Errorf("expected 0 running tasks, got %d", running)
	}

	current, max := q.Concurrency()
	if current != 0 || max != 3 {
		t.Errorf("expected (0, 3), got (%d, %d)", current, max)
	}

	// Enqueue tasks
	for i := 0; i < 5; i++ {
		err := q.Enqueue(&TaskEntry{
			ID:       string(rune('A' + i)),
			Prompt:   "test",
			Parallel: true,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Wait a bit for tasks to start
	time.Sleep(20 * time.Millisecond)

	running = q.Running()
	if running < 1 || running > 3 {
		t.Errorf("expected 1-3 running tasks, got %d", running)
	}

	pending := q.Pending()
	if pending < 2 {
		t.Errorf("expected at least 2 pending tasks, got %d", pending)
	}

	// Wait for completion
	time.Sleep(200 * time.Millisecond)

	running = q.Running()
	if running != 0 {
		t.Errorf("expected 0 running tasks after completion, got %d", running)
	}
}

// TestQueueManager_Concurrency verifies QueueManager concurrency configuration.
func TestQueueManager_Concurrency(t *testing.T) {
	handler := func(ctx context.Context, entry *TaskEntry) (string, error) {
		return "done", nil
	}

	qm := NewQueueManager(handler, 10)

	// Default should be 1 (serial)
	q1 := qm.GetOrCreate("session1")
	_, max := q1.Concurrency()
	if max != 1 {
		t.Errorf("expected default concurrency 1, got %d", max)
	}

	// Set default concurrency
	qm.SetDefaultConcurrency(3)

	// New queues should use the new default
	q2 := qm.GetOrCreate("session2")
	_, max = q2.Concurrency()
	if max != 3 {
		t.Errorf("expected default concurrency 3, got %d", max)
	}

	// Set session-specific concurrency
	qm.SetSessionConcurrency("session2", 5)
	_, max = q2.Concurrency()
	if max != 5 {
		t.Errorf("expected session concurrency 5, got %d", max)
	}
}
