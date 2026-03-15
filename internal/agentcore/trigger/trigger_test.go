package trigger

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestRuntimeRegisterAndList(t *testing.T) {
	rt := NewRuntime(nil, nil)
	id := rt.Register(Trigger{
		Name:  "test trigger",
		Kind:  KindEvent,
		Event: EventTaskCompleted,
		Action: Action{
			Type:    ActionLog,
			Message: "task done",
		},
	})
	if id == "" {
		t.Fatal("expected non-empty ID")
	}

	triggers := rt.List()
	if len(triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(triggers))
	}
	if triggers[0].Name != "test trigger" {
		t.Fatalf("unexpected name: %s", triggers[0].Name)
	}
}

func TestRuntimeGet(t *testing.T) {
	rt := NewRuntime(nil, nil)
	id := rt.Register(Trigger{Name: "get test", Kind: KindEvent, Event: EventCostAlert})

	got, ok := rt.Get(id)
	if !ok {
		t.Fatal("expected to find trigger")
	}
	if got.Name != "get test" {
		t.Fatal("unexpected name")
	}

	_, ok = rt.Get("nonexistent")
	if ok {
		t.Fatal("should not find nonexistent trigger")
	}
}

func TestRuntimeRemove(t *testing.T) {
	rt := NewRuntime(nil, nil)
	id := rt.Register(Trigger{Name: "to remove", Kind: KindEvent})

	if !rt.Remove(id) {
		t.Fatal("expected successful removal")
	}
	if rt.Remove(id) {
		t.Fatal("should not remove twice")
	}
	if len(rt.List()) != 0 {
		t.Fatal("expected empty list")
	}
}

func TestRuntimeSetEnabled(t *testing.T) {
	rt := NewRuntime(nil, nil)
	id := rt.Register(Trigger{Name: "toggle", Kind: KindEvent})

	rt.SetEnabled(id, false)
	got, _ := rt.Get(id)
	if got.Enabled {
		t.Fatal("expected disabled")
	}

	rt.SetEnabled(id, true)
	got, _ = rt.Get(id)
	if !got.Enabled {
		t.Fatal("expected enabled")
	}
}

func TestRuntimeEmitEvent(t *testing.T) {
	var mu sync.Mutex
	var fired []string

	rt := NewRuntime(func(ctx context.Context, trig *Trigger, event *EventPayload) error {
		mu.Lock()
		fired = append(fired, trig.Name)
		mu.Unlock()
		return nil
	}, nil)

	rt.Register(Trigger{
		Name:   "on-complete",
		Kind:   KindEvent,
		Event:  EventTaskCompleted,
		Action: Action{Type: ActionLog},
	})
	rt.Register(Trigger{
		Name:   "on-fail",
		Kind:   KindEvent,
		Event:  EventTaskFailed,
		Action: Action{Type: ActionLog},
	})

	// Emit task_completed → only "on-complete" should fire
	rt.Emit(context.Background(), EventPayload{
		Event: EventTaskCompleted,
		Text:  "task abc done",
	})

	mu.Lock()
	if len(fired) != 1 || fired[0] != "on-complete" {
		t.Fatalf("expected only 'on-complete' fired, got %v", fired)
	}
	mu.Unlock()
}

func TestRuntimeEmitWithFilter(t *testing.T) {
	var fired int

	rt := NewRuntime(func(ctx context.Context, trig *Trigger, event *EventPayload) error {
		fired++
		return nil
	}, nil)

	rt.Register(Trigger{
		Name:        "filtered",
		Kind:        KindEvent,
		Event:       EventTaskCompleted,
		EventFilter: "important",
		Action:      Action{Type: ActionLog},
	})

	// Emit without matching filter
	rt.Emit(context.Background(), EventPayload{
		Event: EventTaskCompleted,
		Text:  "some regular task",
	})
	if fired != 0 {
		t.Fatalf("expected 0 fires, got %d", fired)
	}

	// Emit with matching filter
	rt.Emit(context.Background(), EventPayload{
		Event: EventTaskCompleted,
		Text:  "this is important!",
	})
	if fired != 1 {
		t.Fatalf("expected 1 fire, got %d", fired)
	}
}

func TestRuntimeDisabledTrigger(t *testing.T) {
	var fired int

	rt := NewRuntime(func(ctx context.Context, trig *Trigger, event *EventPayload) error {
		fired++
		return nil
	}, nil)

	id := rt.Register(Trigger{
		Name:   "disabled",
		Kind:   KindEvent,
		Event:  EventTaskCompleted,
		Action: Action{Type: ActionLog},
	})
	rt.SetEnabled(id, false)

	rt.Emit(context.Background(), EventPayload{
		Event: EventTaskCompleted,
		Text:  "test",
	})
	if fired != 0 {
		t.Fatal("disabled trigger should not fire")
	}
}

func TestRuntimeFireCount(t *testing.T) {
	rt := NewRuntime(func(ctx context.Context, trig *Trigger, event *EventPayload) error {
		return nil
	}, nil)

	id := rt.Register(Trigger{
		Name:   "counter",
		Kind:   KindEvent,
		Event:  EventTaskCompleted,
		Action: Action{Type: ActionLog},
	})

	for i := 0; i < 3; i++ {
		rt.Emit(context.Background(), EventPayload{Event: EventTaskCompleted})
	}

	got, _ := rt.Get(id)
	if got.FireCount != 3 {
		t.Fatalf("expected 3 fires, got %d", got.FireCount)
	}
	if got.LastFiredAt == nil {
		t.Fatal("expected LastFiredAt to be set")
	}
}

func TestRuntimeConditionCheck(t *testing.T) {
	var fired int

	rt := NewRuntime(func(ctx context.Context, trig *Trigger, event *EventPayload) error {
		fired++
		return nil
	}, func(expr string) bool {
		return expr == "cost > 10"
	})

	rt.Register(Trigger{
		Name:          "cost check",
		Kind:          KindCondition,
		ConditionExpr: "cost > 10",
		Action:        Action{Type: ActionLog},
	})
	rt.Register(Trigger{
		Name:          "never true",
		Kind:          KindCondition,
		ConditionExpr: "always_false",
		Action:        Action{Type: ActionLog},
	})

	// Manually call checkConditions
	rt.checkConditions()

	if fired != 1 {
		t.Fatalf("expected 1 condition fire, got %d", fired)
	}
}

func TestContainsSubstring(t *testing.T) {
	if !containsSubstring("hello world", "world") {
		t.Fatal("should find 'world'")
	}
	if containsSubstring("hello", "world") {
		t.Fatal("should not find 'world' in 'hello'")
	}
	if containsSubstring("hi", "") {
		t.Fatal("empty substring should return false")
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	time.Sleep(time.Millisecond)
	id2 := generateID()
	if id1 == id2 {
		t.Fatal("expected unique IDs")
	}
	if len(id1) < 5 {
		t.Fatal("ID too short")
	}
}
