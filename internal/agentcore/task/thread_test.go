package task

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"yunque-agent/internal/agentcore/session"
)

func TestThreadManagerEnsure(t *testing.T) {
	convStore := session.NewStore(50)
	defer convStore.StopGC()
	tm := NewThreadManager(convStore)

	sid := tm.Ensure("task-1", "tenant-a")
	if sid != "task:task-1" {
		t.Fatalf("expected session ID 'task:task-1', got '%s'", sid)
	}

	// Ensure is idempotent
	sid2 := tm.Ensure("task-1", "tenant-a")
	if sid2 != sid {
		t.Fatalf("expected same session ID, got '%s'", sid2)
	}
}

func TestThreadManagerPostAndMessages(t *testing.T) {
	convStore := session.NewStore(50)
	defer convStore.StopGC()
	tm := NewThreadManager(convStore)

	// No messages before ensure
	if msgs := tm.Messages("task-2"); msgs != nil {
		t.Fatal("expected nil messages for unknown task")
	}

	tm.Post("task-2", "tenant-a", "user", "请帮我分析这个数据")
	tm.Post("task-2", "tenant-a", "assistant", "好的，我来分析数据")

	msgs := tm.Messages("task-2")
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Content != "请帮我分析这个数据" {
		t.Fatalf("unexpected first message: %s", msgs[0].Content)
	}
	if msgs[1].Role != "assistant" {
		t.Fatalf("expected assistant role, got %s", msgs[1].Role)
	}
}

func TestThreadManagerInfo(t *testing.T) {
	convStore := session.NewStore(50)
	defer convStore.StopGC()
	tm := NewThreadManager(convStore)

	// No info for unknown task
	if info := tm.Info("task-3"); info != nil {
		t.Fatal("expected nil info for unknown task")
	}

	tm.Post("task-3", "tenant-a", "user", "Hello")
	tm.Post("task-3", "tenant-a", "assistant", "Hi")

	info := tm.Info("task-3")
	if info == nil {
		t.Fatal("expected non-nil info")
	}
	if info.TaskID != "task-3" {
		t.Fatalf("expected task-3, got %s", info.TaskID)
	}
	if info.Messages != 2 {
		t.Fatalf("expected 2 messages in info, got %d", info.Messages)
	}
	if info.SessionID != "task:task-3" {
		t.Fatalf("unexpected session ID: %s", info.SessionID)
	}
}

func TestThreadManagerHasThread(t *testing.T) {
	convStore := session.NewStore(50)
	defer convStore.StopGC()
	tm := NewThreadManager(convStore)

	if tm.HasThread("task-4") {
		t.Fatal("should not have thread yet")
	}

	tm.Ensure("task-4", "tenant-a")
	if !tm.HasThread("task-4") {
		t.Fatal("should have thread after ensure")
	}
}

func TestThreadManagerCleanup(t *testing.T) {
	convStore := session.NewStore(50)
	defer convStore.StopGC()
	tm := NewThreadManager(convStore)

	tm.Post("task-5", "tenant-a", "user", "test msg")
	if !tm.HasThread("task-5") {
		t.Fatal("should have thread")
	}

	tm.Cleanup("task-5")
	if tm.HasThread("task-5") {
		t.Fatal("should not have thread after cleanup")
	}
	if msgs := tm.Messages("task-5"); msgs != nil {
		t.Fatal("should have no messages after cleanup")
	}
}

func TestThreadManagerMultipleTasks(t *testing.T) {
	convStore := session.NewStore(50)
	defer convStore.StopGC()
	tm := NewThreadManager(convStore)

	tm.Post("task-a", "tenant-a", "user", "Message for A")
	tm.Post("task-b", "tenant-a", "user", "Message for B")
	tm.Post("task-a", "tenant-a", "assistant", "Reply for A")

	msgsA := tm.Messages("task-a")
	msgsB := tm.Messages("task-b")
	if len(msgsA) != 2 {
		t.Fatalf("task-a expected 2 messages, got %d", len(msgsA))
	}
	if len(msgsB) != 1 {
		t.Fatalf("task-b expected 1 message, got %d", len(msgsB))
	}
}

// ────────── New tests for enhanced Thread features ──────────

func TestThreadState(t *testing.T) {
	convStore := session.NewStore(50)
	defer convStore.StopGC()
	tm := NewThreadManager(convStore)

	tm.Ensure("task-state", "tenant-a")

	// Default state is open
	state := tm.GetState("task-state")
	if state != ThreadOpen {
		t.Fatalf("expected open, got %s", state)
	}

	// Transition to paused
	tm.SetState("task-state", ThreadPaused)
	if s := tm.GetState("task-state"); s != ThreadPaused {
		t.Fatalf("expected paused, got %s", s)
	}

	// Transition to closed
	tm.SetState("task-state", ThreadClosed)
	if s := tm.GetState("task-state"); s != ThreadClosed {
		t.Fatalf("expected closed, got %s", s)
	}

	// Unknown task returns empty
	if s := tm.GetState("no-such-task"); s != "" {
		t.Fatalf("expected empty, got %s", s)
	}
}

func TestThreadChannelBinding(t *testing.T) {
	convStore := session.NewStore(50)
	defer convStore.StopGC()
	tm := NewThreadManager(convStore)

	binding := &ChannelBinding{
		ChannelType: "telegram",
		ChannelID:   "chat-123",
		UserID:      "user-42",
		UserName:    "Alice",
	}

	sid := tm.EnsureWithBinding("task-bind", "tenant-a", binding)
	if sid != "task:task-bind" {
		t.Fatalf("unexpected sid: %s", sid)
	}

	// Verify binding
	b := tm.Binding("task-bind")
	if b == nil {
		t.Fatal("expected binding")
	}
	if b.ChannelType != "telegram" {
		t.Fatalf("expected telegram, got %s", b.ChannelType)
	}
	if b.ChannelID != "chat-123" {
		t.Fatalf("expected chat-123, got %s", b.ChannelID)
	}
	if b.UserID != "user-42" {
		t.Fatalf("expected user-42, got %s", b.UserID)
	}

	// Binding should appear in Info
	info := tm.Info("task-bind")
	if info == nil || info.Binding == nil {
		t.Fatal("expected info with binding")
	}
	if info.Binding.ChannelType != "telegram" {
		t.Fatalf("expected telegram in info, got %s", info.Binding.ChannelType)
	}

	// Binding is not overwritten on second Ensure
	tm.EnsureWithBinding("task-bind", "tenant-a", &ChannelBinding{
		ChannelType: "discord",
		ChannelID:   "guild-999",
	})
	if b2 := tm.Binding("task-bind"); b2.ChannelType != "telegram" {
		t.Fatal("binding should not be overwritten")
	}

	// Unknown task has no binding
	if b3 := tm.Binding("no-such"); b3 != nil {
		t.Fatal("expected nil binding for unknown task")
	}
}

func TestThreadList(t *testing.T) {
	convStore := session.NewStore(50)
	defer convStore.StopGC()
	tm := NewThreadManager(convStore)

	tm.Post("t1", "a", "user", "msg1")
	tm.Post("t2", "a", "user", "msg2")
	tm.Post("t3", "a", "user", "msg3")
	tm.SetState("t2", ThreadClosed)

	// All threads
	all := tm.List("")
	if len(all) != 3 {
		t.Fatalf("expected 3 threads, got %d", len(all))
	}

	// Only open
	open := tm.List(ThreadOpen)
	if len(open) != 2 {
		t.Fatalf("expected 2 open threads, got %d", len(open))
	}

	// Only closed
	closed := tm.List(ThreadClosed)
	if len(closed) != 1 {
		t.Fatalf("expected 1 closed thread, got %d", len(closed))
	}
	if closed[0].TaskID != "t2" {
		t.Fatalf("expected t2, got %s", closed[0].TaskID)
	}
}

func TestThreadPostStepResult(t *testing.T) {
	convStore := session.NewStore(50)
	defer convStore.StopGC()
	tm := NewThreadManager(convStore)

	tm.PostStepResult("task-sr", "tenant-a", 1, "web_search", "found 5 results")
	msgs := tm.Messages("task-sr")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Fatalf("expected system role, got %s", msgs[0].Role)
	}
}

func TestThreadPostStepFailed(t *testing.T) {
	convStore := session.NewStore(50)
	defer convStore.StopGC()
	tm := NewThreadManager(convStore)

	tm.PostStepFailed("task-sf", "tenant-a", 2, "email_send", "SMTP timeout")
	msgs := tm.Messages("task-sf")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Fatalf("expected system role, got %s", msgs[0].Role)
	}
}

func TestThreadTaskCompletedClosesThread(t *testing.T) {
	convStore := session.NewStore(50)
	defer convStore.StopGC()
	tm := NewThreadManager(convStore)

	tm.Ensure("task-done", "tenant-a")
	tm.PostTaskCompleted("task-done", "tenant-a", "all steps passed")

	if s := tm.GetState("task-done"); s != ThreadClosed {
		t.Fatalf("expected closed after completion, got %s", s)
	}

	msgs := tm.Messages("task-done")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
}

func TestThreadTaskFailedClosesThread(t *testing.T) {
	convStore := session.NewStore(50)
	defer convStore.StopGC()
	tm := NewThreadManager(convStore)

	tm.Ensure("task-fail", "tenant-a")
	tm.PostTaskFailed("task-fail", "tenant-a", "step 3 crashed")

	if s := tm.GetState("task-fail"); s != ThreadClosed {
		t.Fatalf("expected closed after failure, got %s", s)
	}
}

func TestThreadChannelPushBack(t *testing.T) {
	convStore := session.NewStore(50)
	defer convStore.StopGC()
	tm := NewThreadManager(convStore)

	var pushCount int64
	tm.SetChannelSend(func(ctx context.Context, channelType, target, content string) error {
		atomic.AddInt64(&pushCount, 1)
		if channelType != "telegram" {
			t.Errorf("expected telegram, got %s", channelType)
		}
		if target != "chat-999" {
			t.Errorf("expected chat-999, got %s", target)
		}
		return nil
	})

	tm.EnsureWithBinding("task-push", "tenant-a", &ChannelBinding{
		ChannelType: "telegram",
		ChannelID:   "chat-999",
	})

	tm.PostStepResult("task-push", "tenant-a", 1, "web_search", "ok")

	// The push is async in a goroutine, wait briefly
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(&pushCount) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if atomic.LoadInt64(&pushCount) != 1 {
		t.Fatalf("expected 1 push, got %d", atomic.LoadInt64(&pushCount))
	}
}

func TestThreadPersistence(t *testing.T) {
	dir := t.TempDir()
	convStore := session.NewStore(50)
	defer convStore.StopGC()

	// Create threads and persist
	tm1 := NewThreadManager(convStore, dir)
	tm1.EnsureWithBinding("pt-1", "tenant-a", &ChannelBinding{
		ChannelType: "discord",
		ChannelID:   "guild-1",
	})
	tm1.Post("pt-1", "tenant-a", "user", "hello")
	tm1.SetState("pt-1", ThreadPaused)

	// Load into new manager — thread meta should survive
	tm2 := NewThreadManager(convStore, dir)
	if !tm2.HasThread("pt-1") {
		t.Fatal("thread should persist across loads")
	}
	if s := tm2.GetState("pt-1"); s != ThreadPaused {
		t.Fatalf("expected paused after reload, got %s", s)
	}
	if b := tm2.Binding("pt-1"); b == nil || b.ChannelType != "discord" {
		t.Fatal("binding should persist across loads")
	}
	info := tm2.Info("pt-1")
	if info == nil {
		t.Fatal("expected info after reload")
	}
	if info.TenantID != "tenant-a" {
		t.Fatalf("expected tenant-a, got %s", info.TenantID)
	}
}

func TestThreadInfoIncludesState(t *testing.T) {
	convStore := session.NewStore(50)
	defer convStore.StopGC()
	tm := NewThreadManager(convStore)

	tm.Ensure("task-info-state", "tenant-a")
	info := tm.Info("task-info-state")
	if info == nil {
		t.Fatal("expected info")
	}
	if info.State != ThreadOpen {
		t.Fatalf("expected open state in info, got %s", info.State)
	}
}
