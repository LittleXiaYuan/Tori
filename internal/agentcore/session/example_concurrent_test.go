package session_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"yunque-agent/internal/agentcore/session"
)

// Example_basicConcurrency demonstrates basic concurrent task execution.
func Example_basicConcurrency() {
	// Create a simple task handler
	handler := func(ctx context.Context, entry *session.TaskEntry) (string, error) {
		// Simulate work
		time.Sleep(100 * time.Millisecond)
		return fmt.Sprintf("processed: %s", entry.Prompt), nil
	}

	// Create queue manager with default settings (serial)
	qm := session.NewQueueManager(handler, 100)

	// Enable concurrency (3 tasks at a time)
	qm.SetDefaultConcurrency(3)

	// Create a session and enqueue tasks
	sessionID := "example-session"
	for i := 0; i < 5; i++ {
		err := qm.Enqueue(&session.TaskEntry{
			ID:        fmt.Sprintf("task-%d", i),
			SessionID: sessionID,
			Prompt:    fmt.Sprintf("Process item %d", i),
			Parallel:  true, // Allow concurrent execution
		})
		if err != nil {
			fmt.Printf("Enqueue error: %v\n", err)
		}
	}

	// Wait for tasks to complete
	time.Sleep(500 * time.Millisecond)

	// Check queue status
	queue := qm.GetOrCreate(sessionID)
	running := queue.Running()
	pending := queue.Pending()
	current, max := queue.Concurrency()

	fmt.Printf("Running: %d, Pending: %d, Concurrency: %d/%d\n", running, pending, current, max)
	// Output: Running: 0, Pending: 0, Concurrency: 0/3
}

// Example_environmentConfig demonstrates reading concurrency from environment.
func Example_environmentConfig() {
	handler := func(ctx context.Context, entry *session.TaskEntry) (string, error) {
		return "done", nil
	}

	qm := session.NewQueueManager(handler, 100)

	// Read from environment variable
	if concStr := os.Getenv("TASK_QUEUE_CONCURRENCY"); concStr != "" {
		if conc, err := strconv.Atoi(concStr); err == nil && conc > 0 {
			qm.SetDefaultConcurrency(conc)
			slog.Info("task queue concurrency configured", "max_concurrent", conc)
		}
	}

	// Use the queue manager...
	fmt.Println("Queue manager initialized")
	// Output: Queue manager initialized
}

// Example_perSessionConcurrency demonstrates different concurrency per session.
func Example_perSessionConcurrency() {
	handler := func(ctx context.Context, entry *session.TaskEntry) (string, error) {
		return "done", nil
	}

	qm := session.NewQueueManager(handler, 100)

	// Default: serial execution
	qm.SetDefaultConcurrency(1)

	// Set session-specific concurrency BEFORE creating queues
	// Training session: high concurrency for data collection
	trainQueue := qm.GetOrCreate("training-session")
	trainQueue.SetMaxConcurrent(5)

	// Debug session: keep serial for easier debugging
	debugQueue := qm.GetOrCreate("debug-session")
	debugQueue.SetMaxConcurrent(1)

	// Production session: moderate concurrency
	prodQueue := qm.GetOrCreate("prod-session")
	prodQueue.SetMaxConcurrent(3)

	// Check configurations
	_, trainMax := trainQueue.Concurrency()
	_, debugMax := debugQueue.Concurrency()
	_, prodMax := prodQueue.Concurrency()

	fmt.Printf("Training: %d, Debug: %d, Production: %d\n", trainMax, debugMax, prodMax)
	// Output: Training: 5, Debug: 1, Production: 3
}

// Example_monitoring demonstrates how to monitor queue status.
func Example_monitoring() {
	handler := func(ctx context.Context, entry *session.TaskEntry) (string, error) {
		time.Sleep(50 * time.Millisecond)
		return "done", nil
	}

	qm := session.NewQueueManager(handler, 100)
	qm.SetDefaultConcurrency(3)

	sessionID := "monitor-session"

	// Enqueue some tasks
	for i := 0; i < 10; i++ {
		qm.Enqueue(&session.TaskEntry{
			ID:        fmt.Sprintf("task-%d", i),
			SessionID: sessionID,
			Prompt:    fmt.Sprintf("Task %d", i),
			Parallel:  true,
		})
	}

	// Monitor queue status
	queue := qm.GetOrCreate(sessionID)

	// Check immediately after enqueuing
	time.Sleep(10 * time.Millisecond)
	running := queue.Running()
	pending := queue.Pending()
	current, max := queue.Concurrency()

	fmt.Printf("Status: running=%d pending=%d concurrency=%d/%d\n",
		running, pending, current, max)

	// Wait for completion
	time.Sleep(300 * time.Millisecond)

	running = queue.Running()
	pending = queue.Pending()
	fmt.Printf("After completion: running=%d pending=%d\n", running, pending)
	// Output: Status: running=3 pending=7 concurrency=3/3
	// After completion: running=0 pending=0
}

// Example_mixedParallelism demonstrates mixing parallel and non-parallel tasks.
func Example_mixedParallelism() {
	handler := func(ctx context.Context, entry *session.TaskEntry) (string, error) {
		time.Sleep(50 * time.Millisecond)
		return "done", nil
	}

	qm := session.NewQueueManager(handler, 100)
	qm.SetDefaultConcurrency(3)

	sessionID := "mixed-session"

	// Enqueue parallel tasks
	for i := 0; i < 3; i++ {
		qm.Enqueue(&session.TaskEntry{
			ID:        fmt.Sprintf("read-%d", i),
			SessionID: sessionID,
			Prompt:    fmt.Sprintf("Read file %d", i),
			Parallel:  true, // Can run concurrently
		})
	}

	// Enqueue a non-parallel task (will block others)
	qm.Enqueue(&session.TaskEntry{
		ID:        "write-config",
		SessionID: sessionID,
		Prompt:    "Write config file",
		Parallel:  false, // Must run alone
	})

	// Enqueue more parallel tasks
	for i := 0; i < 3; i++ {
		qm.Enqueue(&session.TaskEntry{
			ID:        fmt.Sprintf("process-%d", i),
			SessionID: sessionID,
			Prompt:    fmt.Sprintf("Process data %d", i),
			Parallel:  true,
		})
	}

	fmt.Println("Mixed parallel and non-parallel tasks enqueued")
	// Output: Mixed parallel and non-parallel tasks enqueued
}
