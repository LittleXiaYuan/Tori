package planner

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ────────────────────────────────────────────────────────────
// Reverie Event-Driven Triggers
//
// Instead of relying solely on a fixed-interval timer, the agent
// can now react to meaningful external signals:
//   - Emotion shift:        user mood changed dramatically
//   - Task failure spike:   recent skill calls failing too often
//   - High-value fact:      memory pipeline discovered important info
//
// Events flow through ReverieEventBus → Reverie.thinkLoop selects
// on both the timer and the event channel, triggering immediate
// reflection when something noteworthy happens.
//
// Boundary note: these events are proactive cognition triggers, not a second
// reflection owner. Post-turn learning and memory-update policy should flow
// through internal/cognikernel.ReflectiveLoop; Reverie may later emit structured
// events into that loop, but it should not reimplement the loop.
// ────────────────────────────────────────────────────────────

// ReverieEventType identifies the kind of external trigger.
type ReverieEventType string

const (
	EventEmotionShift     ReverieEventType = "emotion_shift"
	EventTaskFailureSpike ReverieEventType = "task_failure_spike"
	EventHighValueFact    ReverieEventType = "high_value_fact"
	EventMetaCogAlert     ReverieEventType = "metacog_alert"
)

// ReverieEvent carries context about what happened.
type ReverieEvent struct {
	Type    ReverieEventType
	Trigger string            // human-readable cause, e.g. "happy→sad"
	Data    map[string]string // optional extra context
}

// ────────────── EventBus ──────────────

// ReverieEventBus is a lightweight pub/sub channel with per-type cooldown.
type ReverieEventBus struct {
	mu       sync.Mutex
	ch       chan ReverieEvent
	cooldown time.Duration
	last     map[ReverieEventType]time.Time
	closed   bool
}

// NewReverieEventBus creates an event bus.
// cooldown prevents the same event type from firing more than once within the window.
func NewReverieEventBus(cooldown time.Duration) *ReverieEventBus {
	return &ReverieEventBus{
		ch:       make(chan ReverieEvent, 8),
		cooldown: cooldown,
		last:     make(map[ReverieEventType]time.Time),
	}
}

// Emit publishes an event. Returns false if suppressed by cooldown or bus is closed.
func (b *ReverieEventBus) Emit(event ReverieEvent) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return false
	}

	now := time.Now()
	if last, ok := b.last[event.Type]; ok && now.Sub(last) < b.cooldown {
		slog.Debug("reverie event suppressed by cooldown",
			"type", string(event.Type), "remaining", b.cooldown-now.Sub(last))
		return false
	}

	b.last[event.Type] = now

	// Non-blocking send — drop if channel full (backpressure)
	select {
	case b.ch <- event:
		slog.Info("reverie event emitted", "type", string(event.Type), "trigger", event.Trigger)
		return true
	default:
		slog.Warn("reverie event dropped (channel full)", "type", string(event.Type))
		return false
	}
}

// Events returns the receive-only channel to listen for events.
func (b *ReverieEventBus) Events() <-chan ReverieEvent {
	return b.ch
}

// Close shuts down the bus.
func (b *ReverieEventBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.closed {
		b.closed = true
		close(b.ch)
	}
}

// ────────────── Emotion Shift Detector ──────────────

// EmotionShiftDetector tracks per-user emotion and detects significant changes.
type EmotionShiftDetector struct {
	mu      sync.Mutex
	bus     *ReverieEventBus
	prev    map[string]emotionRecord // userID → last known emotion
	onShift func(from, to string, confidence float64)
}

type emotionRecord struct {
	emotion    string
	confidence float64
	ts         time.Time
}

// NewEmotionShiftDetector creates a detector wired to the given event bus.
func NewEmotionShiftDetector(bus *ReverieEventBus) *EmotionShiftDetector {
	return &EmotionShiftDetector{
		bus:  bus,
		prev: make(map[string]emotionRecord),
	}
}

// SetOnShift sets a callback that fires whenever a significant emotion shift is detected.
func (d *EmotionShiftDetector) SetOnShift(fn func(from, to string, confidence float64)) {
	d.onShift = fn
}

// Observe records a new emotion result and fires an event on significant shift.
// Returns true if an event was emitted.
func (d *EmotionShiftDetector) Observe(userID, emotion string, confidence float64) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	prev, hasPrev := d.prev[userID]
	d.prev[userID] = emotionRecord{emotion: emotion, confidence: confidence, ts: time.Now()}

	if !hasPrev || prev.emotion == emotion {
		return false
	}

	// Detect significant shift: polarity reversal or any shift with high confidence.
	prevPositive := isPositiveEmotion(prev.emotion)
	nowPositive := isPositiveEmotion(emotion)
	polarityFlip := prevPositive != nowPositive && prev.emotion != "neutral" && emotion != "neutral"

	if polarityFlip || confidence >= 0.7 {
		if d.onShift != nil {
			d.onShift(prev.emotion, emotion, confidence)
		}
		return d.bus.Emit(ReverieEvent{
			Type:    EventEmotionShift,
			Trigger: prev.emotion + "→" + emotion,
			Data:    map[string]string{"user": userID, "confidence": formatFloat(confidence)},
		})
	}
	return false
}

// CleanupStale removes records older than maxAge.
func (d *EmotionShiftDetector) CleanupStale(maxAge time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()
	cutoff := time.Now().Add(-maxAge)
	for k, v := range d.prev {
		if v.ts.Before(cutoff) {
			delete(d.prev, k)
		}
	}
}

func isPositiveEmotion(e string) bool {
	return e == "happy" || e == "surprised"
}

func formatFloat(f float64) string {
	return fmt.Sprintf("%.2f", f)
}

// ────────────── Task Failure Monitor ──────────────

// TaskFailureMonitor watches recent skill failures via a sliding window.
type TaskFailureMonitor struct {
	mu        sync.Mutex
	bus       *ReverieEventBus
	window    []taskOutcome
	windowDur time.Duration
	threshold float64 // failure rate to trigger (e.g. 0.5 = 50%)
	minCalls  int     // minimum calls in window before evaluation
}

type taskOutcome struct {
	ts     time.Time
	failed bool
}

// NewTaskFailureMonitor creates a monitor.
// threshold: failure rate (0-1) that triggers an event.
// windowDur: sliding window duration.
// minCalls: minimum number of calls in window to evaluate.
func NewTaskFailureMonitor(bus *ReverieEventBus, threshold float64, windowDur time.Duration, minCalls int) *TaskFailureMonitor {
	if minCalls < 1 {
		minCalls = 3
	}
	return &TaskFailureMonitor{
		bus:       bus,
		window:    make([]taskOutcome, 0, 64),
		windowDur: windowDur,
		threshold: threshold,
		minCalls:  minCalls,
	}
}

// Record adds a skill call outcome and checks the failure rate.
// Returns true if a spike event was emitted.
func (m *TaskFailureMonitor) Record(failed bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.window = append(m.window, taskOutcome{ts: now, failed: failed})

	// Trim expired entries
	cutoff := now.Add(-m.windowDur)
	start := 0
	for start < len(m.window) && m.window[start].ts.Before(cutoff) {
		start++
	}
	m.window = m.window[start:]

	if len(m.window) < m.minCalls {
		return false
	}

	failures := 0
	for _, o := range m.window {
		if o.failed {
			failures++
		}
	}

	rate := float64(failures) / float64(len(m.window))
	if rate >= m.threshold {
		return m.bus.Emit(ReverieEvent{
			Type:    EventTaskFailureSpike,
			Trigger: fmt.Sprintf("failure_rate=%.0f%% (%d/%d in %s)", rate*100, failures, len(m.window), m.windowDur),
		})
	}
	return false
}

// ────────────── High-Value Fact Hook ──────────────

// FactEventHook emits an event when the memory pipeline extracts notable facts.
type FactEventHook struct {
	bus       *ReverieEventBus
	threshold int // minimum facts in a single extraction to trigger
}

// NewFactEventHook creates a hook that fires when ≥ threshold facts are extracted at once.
func NewFactEventHook(bus *ReverieEventBus, threshold int) *FactEventHook {
	if threshold < 1 {
		threshold = 3
	}
	return &FactEventHook{bus: bus, threshold: threshold}
}

// OnExtracted should be called after memory pipeline extracts facts.
// Returns true if an event was emitted.
func (h *FactEventHook) OnExtracted(facts []string) bool {
	if len(facts) < h.threshold {
		return false
	}
	preview := facts[0]
	if len(preview) > 60 {
		preview = preview[:60] + "..."
	}
	return h.bus.Emit(ReverieEvent{
		Type:    EventHighValueFact,
		Trigger: fmt.Sprintf("%d facts extracted (e.g. %s)", len(facts), preview),
	})
}
