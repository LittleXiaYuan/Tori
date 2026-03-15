package plugin

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

func TestHookManager_RegisterAndEmit(t *testing.T) {
	hm := NewHookManager()
	var called int32

	hm.Register(HookChatBefore, "test-plugin", func(ctx context.Context, p HookPayload) error {
		atomic.AddInt32(&called, 1)
		if p.Event != HookChatBefore {
			t.Errorf("expected event %q, got %q", HookChatBefore, p.Event)
		}
		if p.Data["user"] != "alice" {
			t.Errorf("expected user alice, got %v", p.Data["user"])
		}
		return nil
	})

	err := hm.Emit(context.Background(), HookChatBefore, map[string]any{"user": "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&called) != 1 {
		t.Fatalf("expected 1 call, got %d", called)
	}
}

func TestHookManager_EmitNoHandlers(t *testing.T) {
	hm := NewHookManager()
	err := hm.Emit(context.Background(), "nonexistent", nil)
	if err != nil {
		t.Fatal("should not error with no handlers")
	}
}

func TestHookManager_EmitFailFast(t *testing.T) {
	hm := NewHookManager()
	var order []string

	hm.Register(HookChatAfter, "p1", func(ctx context.Context, p HookPayload) error {
		order = append(order, "p1")
		return errors.New("p1 failed")
	})
	hm.Register(HookChatAfter, "p2", func(ctx context.Context, p HookPayload) error {
		order = append(order, "p2")
		return nil
	})

	err := hm.Emit(context.Background(), HookChatAfter, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if len(order) != 1 {
		t.Fatalf("fail-fast should stop after first error, got %v", order)
	}
}

func TestHookManager_EmitAll(t *testing.T) {
	hm := NewHookManager()

	hm.Register(HookAgentStart, "p1", func(ctx context.Context, p HookPayload) error {
		return errors.New("p1 err")
	})
	hm.Register(HookAgentStart, "p2", func(ctx context.Context, p HookPayload) error {
		return nil
	})
	hm.Register(HookAgentStart, "p3", func(ctx context.Context, p HookPayload) error {
		return errors.New("p3 err")
	})

	errs := hm.EmitAll(context.Background(), HookAgentStart, nil)
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(errs))
	}
}

func TestHookManager_UnregisterPlugin(t *testing.T) {
	hm := NewHookManager()
	var called int32

	hm.Register(HookChatBefore, "keep", func(ctx context.Context, p HookPayload) error {
		atomic.AddInt32(&called, 1)
		return nil
	})
	hm.Register(HookChatBefore, "remove", func(ctx context.Context, p HookPayload) error {
		atomic.AddInt32(&called, 1)
		return nil
	})

	hm.UnregisterPlugin("remove")
	hm.Emit(context.Background(), HookChatBefore, nil)

	if atomic.LoadInt32(&called) != 1 {
		t.Fatalf("expected 1 call after unregister, got %d", called)
	}
}

func TestHookManager_Events(t *testing.T) {
	hm := NewHookManager()
	hm.Register(HookAgentStart, "p1", func(ctx context.Context, p HookPayload) error { return nil })
	hm.Register(HookAgentStop, "p1", func(ctx context.Context, p HookPayload) error { return nil })

	events := hm.Events()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestHookManager_HandlersFor(t *testing.T) {
	hm := NewHookManager()
	hm.Register(HookChatBefore, "p1", func(ctx context.Context, p HookPayload) error { return nil })
	hm.Register(HookChatBefore, "p2", func(ctx context.Context, p HookPayload) error { return nil })

	handlers := hm.HandlersFor(HookChatBefore)
	if len(handlers) != 2 {
		t.Fatalf("expected 2 handlers, got %d", len(handlers))
	}
}
