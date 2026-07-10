package planner

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/subagent"
	"yunque-agent/internal/agentcore/task"
)

// newAsyncTestService builds a DelegationRuntimeService wired for the async
// path: a real JSONStore (temp dir), a handoff registry with the given
// RunFunc, and channel-based notify/broadcast spies.
func newAsyncTestService(t *testing.T, runFn subagent.RunFunc) (*DelegationRuntimeService, *task.JSONStore, chan llm.Message, chan string) {
	t.Helper()
	mgr := subagent.NewManager()
	reg := subagent.NewHandoffRegistry(mgr)
	reg.SetRunFunc(runFn)
	if err := reg.Register(subagent.HandoffConfig{Name: "research", Description: "research agent"}); err != nil {
		t.Fatalf("register handoff: %v", err)
	}

	store := task.NewJSONStore(t.TempDir())
	service := NewDelegationRuntimeService()
	service.SetHandoffRegistry(reg)
	service.SetHandoffTaskRuntime(store)

	notified := make(chan llm.Message, 4)
	broadcast := make(chan string, 4)
	service.SetHandoffAsyncNotifier(
		func(_ string, msg llm.Message) { notified <- msg },
		func(event, _, detail string) { broadcast <- event + ":" + detail },
	)
	return service, store, notified, broadcast
}

func TestExecuteHandoffForRequestAsyncReturnsImmediately(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	service, _, notified, _ := newAsyncTestService(t, func(_ context.Context, _, _, _ string) (string, string, error) {
		close(started)
		<-release // blocks until the test explicitly lets it finish
		return "done later", "", nil
	})

	result := service.ExecuteHandoffForRequest(
		context.Background(),
		PlanRequest{TenantID: "tenant-a", SessionID: "session-a"},
		"transfer_to_research",
		map[string]any{"input": "collect evidence"},
		"unit", 1, HandoffExecutionHooks{},
	)

	if !result.Handled || result.Err != nil {
		t.Fatalf("expected immediate handled result with no error, got %#v", result)
	}
	if !strings.Contains(result.Reply, "后台任务") {
		t.Fatalf("expected a backgrounded-task notice, got %q", result.Reply)
	}

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("sub-agent RunFunc never started")
	}
	close(release)

	select {
	case msg := <-notified:
		if !strings.Contains(msg.Content, "done later") {
			t.Fatalf("completion message missing reply content: %q", msg.Content)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for completion notification")
	}
}

// TestAsyncHandoffMarksTaskCompletedOnSuccess guards the pending-task leak:
// a successful async handoff must move its task record out of StatusPending to
// StatusCompleted, otherwise every successful delegation accumulates forever.
func TestAsyncHandoffMarksTaskCompletedOnSuccess(t *testing.T) {
	service, store, notified, _ := newAsyncTestService(t, func(_ context.Context, _, _, _ string) (string, string, error) {
		return "final answer", "", nil
	})

	result := service.ExecuteHandoffForRequest(
		context.Background(),
		PlanRequest{TenantID: "tenant-a", SessionID: "session-a"},
		"transfer_to_research",
		map[string]any{"input": "collect evidence"},
		"unit", 1, HandoffExecutionHooks{},
	)
	if !result.Handled {
		t.Fatalf("expected handled result, got %#v", result)
	}

	select {
	case <-notified:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for completion notification")
	}

	tasks := store.List("tenant-a", 0)
	if len(tasks) != 1 {
		t.Fatalf("expected exactly one task record, got %d", len(tasks))
	}
	if tasks[0].Status != task.StatusCompleted {
		t.Fatalf("successful handoff must mark task completed, got status %q (leak: task stuck pending)", tasks[0].Status)
	}
}

// TestAsyncHandoffSurfacesPartialResultOnTimeout guards #33 on the async path.
// The registry only populates HandoffResult.PartialResult on the error/timeout
// branch (a nominal success carries everything in Reply), so the realistic
// "recovered but did not finish" case is an error WITH partial work. That
// partial must reach the user instead of being silently dropped — the exact
// regression the async path introduced versus the synchronous one.
func TestAsyncHandoffSurfacesPartialResultOnTimeout(t *testing.T) {
	service, _, notified, _ := newAsyncTestService(t, func(_ context.Context, _, _, _ string) (string, string, error) {
		return "", "salvaged findings so far", context.DeadlineExceeded // timed out mid-flight, partial recovered
	})

	service.ExecuteHandoffForRequest(
		context.Background(),
		PlanRequest{TenantID: "tenant-a", SessionID: "session-a"},
		"transfer_to_research",
		map[string]any{"input": "collect evidence"},
		"unit", 1, HandoffExecutionHooks{},
	)

	select {
	case msg := <-notified:
		if !strings.Contains(msg.Content, "salvaged findings so far") {
			t.Fatalf("expected recovered partial result surfaced in completion message, got %q", msg.Content)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for completion notification")
	}
}

func TestExecuteHandoffForRequestAsyncFailureUsesRecoveryHintNotRawError(t *testing.T) {
	rawErr := `chat API status 401: {"error":{"message":"Authentication Fails, Your api key: ****_KEY is invalid","type":"authentication_error"}}`
	service, _, notified, broadcast := newAsyncTestService(t, func(_ context.Context, _, _, _ string) (string, string, error) {
		return "", "", errors.New(rawErr)
	})

	result := service.ExecuteHandoffForRequest(
		context.Background(),
		PlanRequest{TenantID: "tenant-a", SessionID: "session-a"},
		"transfer_to_research",
		map[string]any{"input": "collect evidence"},
		"unit", 1, HandoffExecutionHooks{},
	)
	if !result.Handled || result.Err != nil {
		t.Fatalf("expected immediate handled result with no error even though the sub-agent will fail, got %#v", result)
	}

	select {
	case msg := <-notified:
		if strings.Contains(msg.Content, "****_KEY") || strings.Contains(msg.Content, "authentication_error") {
			t.Fatalf("completion message leaked the raw provider error instead of a recovery hint: %q", msg.Content)
		}
		if !strings.Contains(msg.Content, "供应商") && !strings.Contains(msg.Content, "API Key") {
			t.Fatalf("expected an honest provider/API-key hint in the failure message, got %q", msg.Content)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for failure notification")
	}

	select {
	case evt := <-broadcast:
		if !strings.HasPrefix(evt, "failed:") {
			t.Fatalf("expected a failed broadcast event, got %q", evt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for broadcast event")
	}
}

func TestSessionConcurrencyLimitByMode(t *testing.T) {
	service := NewDelegationRuntimeService()
	service.SetSessionModeResolver(func(sessionID string) string {
		if sessionID == "api-session" {
			return "api"
		}
		return "xiaoyu"
	})
	service.SetHandoffConcurrency(2, 5)

	if got := service.sessionConcurrencyLimit("xiaoyu-session"); got != 2 {
		t.Fatalf("expected 小羽模式 ceiling 2, got %d", got)
	}
	if got := service.sessionConcurrencyLimit("api-session"); got != 5 {
		t.Fatalf("expected API模式 ceiling 5, got %d", got)
	}
}

func TestAsyncHandoffRespectsConcurrencyCeiling(t *testing.T) {
	var running int32
	var mu sync.Mutex
	maxObserved := 0
	release := make(chan struct{})
	var wg sync.WaitGroup

	service, _, notified, _ := newAsyncTestService(t, func(_ context.Context, _, _, _ string) (string, string, error) {
		mu.Lock()
		running++
		if int(running) > maxObserved {
			maxObserved = int(running)
		}
		mu.Unlock()
		<-release
		mu.Lock()
		running--
		mu.Unlock()
		return "ok", "", nil
	})
	service.SetHandoffConcurrency(1, 1) // ceiling 1: second call must queue behind the first

	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			service.ExecuteHandoffForRequest(
				context.Background(),
				PlanRequest{TenantID: "tenant-a", SessionID: "same-session"},
				"transfer_to_research",
				map[string]any{"input": "collect evidence"},
				"unit", 1, HandoffExecutionHooks{},
			)
		}()
	}

	time.Sleep(200 * time.Millisecond) // let both calls reach the semaphore
	close(release)
	wg.Wait()

	for i := 0; i < 2; i++ {
		select {
		case <-notified:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for both completions")
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if maxObserved > 1 {
		t.Fatalf("expected concurrency ceiling of 1 to serialize execution, observed %d concurrent runs", maxObserved)
	}
}
