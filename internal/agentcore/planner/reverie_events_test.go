package planner

import (
	"context"
	"testing"
	"time"
)

// ────────────── EventBus Tests ──────────────

func TestEventBusEmitAndReceive(t *testing.T) {
	bus := NewReverieEventBus(0) // no cooldown
	defer bus.Close()

	ev := ReverieEvent{Type: EventEmotionShift, Trigger: "happy→sad"}
	if !bus.Emit(ev) {
		t.Fatal("expected Emit to succeed")
	}

	select {
	case got := <-bus.Events():
		if got.Type != EventEmotionShift {
			t.Fatalf("expected EventEmotionShift, got %s", got.Type)
		}
		if got.Trigger != "happy→sad" {
			t.Fatalf("trigger mismatch: %s", got.Trigger)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}

func TestEventBusCooldown(t *testing.T) {
	bus := NewReverieEventBus(1 * time.Hour) // long cooldown
	defer bus.Close()

	ev := ReverieEvent{Type: EventEmotionShift, Trigger: "first"}
	if !bus.Emit(ev) {
		t.Fatal("first emit should succeed")
	}

	// Same type within cooldown → suppressed
	ev2 := ReverieEvent{Type: EventEmotionShift, Trigger: "second"}
	if bus.Emit(ev2) {
		t.Fatal("second emit should be suppressed by cooldown")
	}

	// Different type → should succeed
	ev3 := ReverieEvent{Type: EventTaskFailureSpike, Trigger: "failures"}
	if !bus.Emit(ev3) {
		t.Fatal("different event type should not be affected by cooldown")
	}

	// Drain
	<-bus.Events()
	<-bus.Events()
}

func TestEventBusClose(t *testing.T) {
	bus := NewReverieEventBus(0)
	bus.Close()

	if bus.Emit(ReverieEvent{Type: EventHighValueFact}) {
		t.Fatal("emit after close should return false")
	}
}

func TestEventBusBackpressure(t *testing.T) {
	bus := NewReverieEventBus(0)
	defer bus.Close()

	// Fill the channel (capacity is 8)
	for i := 0; i < 8; i++ {
		bus.Emit(ReverieEvent{Type: EventHighValueFact, Trigger: "fill"})
	}
	// Next one should be dropped (non-blocking)
	if bus.Emit(ReverieEvent{Type: EventHighValueFact, Trigger: "overflow"}) {
		t.Fatal("expected overflow event to be dropped")
	}
}

// ────────────── Emotion Shift Detector Tests ──────────────

func TestEmotionShiftDetectorFirstObserve(t *testing.T) {
	bus := NewReverieEventBus(0)
	defer bus.Close()
	d := NewEmotionShiftDetector(bus)

	// First observation: no previous emotion, no event
	if d.Observe("user1", "happy", 0.9) {
		t.Fatal("first observation should not trigger event")
	}
}

func TestEmotionShiftDetectorPolarityFlip(t *testing.T) {
	bus := NewReverieEventBus(0)
	defer bus.Close()
	d := NewEmotionShiftDetector(bus)

	d.Observe("user1", "happy", 0.8)

	// Polarity flip: happy → sad
	if !d.Observe("user1", "sad", 0.7) {
		t.Fatal("polarity flip should trigger event")
	}

	ev := <-bus.Events()
	if ev.Type != EventEmotionShift {
		t.Fatalf("expected EventEmotionShift, got %s", ev.Type)
	}
	if ev.Trigger != "happy→sad" {
		t.Fatalf("trigger mismatch: %s", ev.Trigger)
	}
}

func TestEmotionShiftDetectorSameEmotion(t *testing.T) {
	bus := NewReverieEventBus(0)
	defer bus.Close()
	d := NewEmotionShiftDetector(bus)

	d.Observe("user1", "happy", 0.8)

	// Same emotion: no event
	if d.Observe("user1", "happy", 0.9) {
		t.Fatal("same emotion should not trigger event")
	}
}

func TestEmotionShiftDetectorHighConfidenceNonPolarityShift(t *testing.T) {
	bus := NewReverieEventBus(0)
	defer bus.Close()
	d := NewEmotionShiftDetector(bus)

	d.Observe("user1", "happy", 0.8)

	// Same polarity (positive→positive) but high confidence shift
	if !d.Observe("user1", "surprised", 0.8) {
		t.Fatal("high confidence shift should trigger event")
	}

	ev := <-bus.Events()
	if ev.Trigger != "happy→surprised" {
		t.Fatalf("trigger mismatch: %s", ev.Trigger)
	}
}

func TestEmotionShiftDetectorNeutralToNegative(t *testing.T) {
	bus := NewReverieEventBus(0)
	defer bus.Close()
	d := NewEmotionShiftDetector(bus)

	d.Observe("user1", "neutral", 0.9)

	// neutral→sad: not a polarity flip (neutral is excluded), but if confidence is high...
	if !d.Observe("user1", "sad", 0.8) {
		t.Fatal("neutral→sad with high confidence should trigger")
	}
}

func TestEmotionShiftDetectorUserIsolation(t *testing.T) {
	bus := NewReverieEventBus(0)
	defer bus.Close()
	d := NewEmotionShiftDetector(bus)

	d.Observe("user1", "happy", 0.8)
	d.Observe("user2", "sad", 0.8)

	// user1 stays happy → no event
	if d.Observe("user1", "happy", 0.9) {
		t.Fatal("same emotion for user1 should not trigger")
	}
}

func TestEmotionShiftDetectorCleanup(t *testing.T) {
	bus := NewReverieEventBus(0)
	defer bus.Close()
	d := NewEmotionShiftDetector(bus)

	d.Observe("user1", "happy", 0.8)
	time.Sleep(2 * time.Millisecond)     // ensure record is strictly in the past
	d.CleanupStale(1 * time.Millisecond) // clean records older than 1ms

	// After cleanup, first observation again → no event (no prev)
	if d.Observe("user1", "sad", 0.9) {
		t.Fatal("after cleanup, should be treated as first observation")
	}
}

// ────────────── Task Failure Monitor Tests ──────────────

func TestTaskFailureMonitorBelowMin(t *testing.T) {
	bus := NewReverieEventBus(0)
	defer bus.Close()
	m := NewTaskFailureMonitor(bus, 0.5, 10*time.Minute, 3)

	// Only 2 calls, below minCalls=3
	m.Record(true)
	if m.Record(true) {
		t.Fatal("should not trigger with fewer than minCalls")
	}
}

func TestTaskFailureMonitorSpike(t *testing.T) {
	bus := NewReverieEventBus(0)
	defer bus.Close()
	m := NewTaskFailureMonitor(bus, 0.5, 10*time.Minute, 3)

	m.Record(true)
	m.Record(true)
	// 3rd call: 3/3 = 100% failure rate → should trigger
	if !m.Record(true) {
		t.Fatal("should trigger at 100% failure rate")
	}

	ev := <-bus.Events()
	if ev.Type != EventTaskFailureSpike {
		t.Fatalf("expected EventTaskFailureSpike, got %s", ev.Type)
	}
}

func TestTaskFailureMonitorBelowThreshold(t *testing.T) {
	bus := NewReverieEventBus(0)
	defer bus.Close()
	m := NewTaskFailureMonitor(bus, 0.5, 10*time.Minute, 3)

	m.Record(false) // success
	m.Record(true)  // fail
	// 1/3 = 33% < 50% threshold
	if m.Record(false) {
		t.Fatal("33% failure rate should not trigger (threshold=50%)")
	}
}

func TestTaskFailureMonitorWindowExpiry(t *testing.T) {
	bus := NewReverieEventBus(0)
	defer bus.Close()
	// Very short window
	m := NewTaskFailureMonitor(bus, 0.5, 1*time.Millisecond, 3)

	m.Record(true)
	m.Record(true)
	m.Record(true)
	// Drain the event from the spike
	<-bus.Events()

	// Wait for window to expire
	time.Sleep(5 * time.Millisecond)

	// New calls: only 1 success → below min
	if m.Record(false) {
		t.Fatal("old entries should have expired")
	}
}

// ────────────── Fact Event Hook Tests ──────────────

func TestFactEventHookBelowThreshold(t *testing.T) {
	bus := NewReverieEventBus(0)
	defer bus.Close()
	h := NewFactEventHook(bus, 3)

	// Only 2 facts → no event
	if h.OnExtracted([]string{"fact1", "fact2"}) {
		t.Fatal("should not trigger with fewer than threshold facts")
	}
}

func TestFactEventHookAboveThreshold(t *testing.T) {
	bus := NewReverieEventBus(0)
	defer bus.Close()
	h := NewFactEventHook(bus, 3)

	facts := []string{"用户喜欢咖啡", "用户住在上海", "用户是程序员"}
	if !h.OnExtracted(facts) {
		t.Fatal("should trigger with 3 facts (threshold=3)")
	}

	ev := <-bus.Events()
	if ev.Type != EventHighValueFact {
		t.Fatalf("expected EventHighValueFact, got %s", ev.Type)
	}
}

func TestFactEventHookEmpty(t *testing.T) {
	bus := NewReverieEventBus(0)
	defer bus.Close()
	h := NewFactEventHook(bus, 3)

	if h.OnExtracted(nil) {
		t.Fatal("nil facts should not trigger")
	}
}

// ────────────── Reverie EventBus Integration Tests ──────────────

func TestReverieThinkWithEvent(t *testing.T) {
	cfg := DefaultReverieConfig()
	cfg.Enabled = true
	cfg.SaveFile = "" // no persist
	r := NewReverie(cfg)

	called := false
	r.SetLLMCall(func(ctx context.Context, system, user string) (string, error) {
		called = true
		// Check that the prompt contains event context
		if !strContainsSubstr(user, "触发事件") {
			t.Error("event thinking prompt should contain trigger section")
		}
		return `{"content":"用户情绪变化了","category":"concern","significance":0.8,"trigger":"emotion"}`, nil
	})

	ev := ReverieEvent{
		Type:    EventEmotionShift,
		Trigger: "happy→sad",
		Data:    map[string]string{"user": "test1"},
	}
	thought, err := r.ThinkWithEvent(context.Background(), ev)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("LLM should have been called")
	}
	if thought.Category != "concern" {
		t.Fatalf("expected concern, got %s", thought.Category)
	}
	if thought.Trigger != "emotion_shift: happy→sad" {
		t.Fatalf("trigger should include event type: %s", thought.Trigger)
	}
}

func TestReverieEventBusIntegration(t *testing.T) {
	cfg := DefaultReverieConfig()
	cfg.Enabled = true
	cfg.Interval = 1 * time.Hour // long interval so periodic doesn't fire
	cfg.SaveFile = ""
	r := NewReverie(cfg)

	thinkCount := 0
	r.SetLLMCall(func(ctx context.Context, system, user string) (string, error) {
		thinkCount++
		return `{"content":"test","category":"observation","significance":0.5,"trigger":"test"}`, nil
	})

	bus := NewReverieEventBus(0)
	r.SetEventBus(bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)

	// Wait briefly for thinkLoop to start, then fire event
	time.Sleep(50 * time.Millisecond)
	bus.Emit(ReverieEvent{Type: EventHighValueFact, Trigger: "test trigger"})

	// Give a moment for the event to be processed
	time.Sleep(200 * time.Millisecond)
	r.Stop()
	bus.Close()

	// Should have at least 1 event-driven think (the periodic one hasn't fired yet)
	if thinkCount < 1 {
		t.Fatalf("expected at least 1 think call, got %d", thinkCount)
	}
}

// helpers
func strContainsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
