package metacog

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/LittleXiaYuan/ledger"
)

// MetaCogMonitor monitors the agent's reasoning quality in real-time.
// It detects anomalous patterns: loops, goal drift, confidence drops, stalls.
type MetaCogMonitor struct {
	bus        *ledger.EventBus
	events     *ledger.EventStore
	thresholds Thresholds
	sub        *ledger.Subscription
	alertFn    AlertFunc
	state      map[string]*taskMonitorState
}

// Thresholds configures when to fire alerts.
type Thresholds struct {
	MaxConsecutiveSameAction int
	ConfidenceDropThreshold  float64
	MaxBacktracksPerTask     int
	StallTimeout             time.Duration
	MaxStepsWithoutProgress  int
}

// DefaultThresholds returns sensible defaults.
func DefaultThresholds() Thresholds {
	return Thresholds{
		MaxConsecutiveSameAction: 3,
		ConfidenceDropThreshold:  0.3,
		MaxBacktracksPerTask:     5,
		StallTimeout:             2 * time.Minute,
		MaxStepsWithoutProgress:  5,
	}
}

// Alert represents a detected anomaly.
type Alert struct {
	TaskID    string        `json:"task_id"`
	Kind      AlertKind     `json:"kind"`
	Severity  Severity      `json:"severity"`
	Message   string        `json:"message"`
	Details   ledger.JSON   `json:"details"`
	Timestamp time.Time     `json:"timestamp"`
}

// AlertKind classifies the type of metacognitive alert.
type AlertKind string

const (
	AlertLoop               AlertKind = "loop_detected"
	AlertConfidenceDrop     AlertKind = "confidence_drop"
	AlertExcessiveBacktrack AlertKind = "excessive_backtrack"
	AlertStall              AlertKind = "stall"
	AlertNoProgress         AlertKind = "no_progress"
	AlertGoalDrift          AlertKind = "goal_drift"
)

// Severity levels for alerts.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// AlertFunc is called when an anomaly is detected.
type AlertFunc func(alert Alert)

type taskMonitorState struct {
	taskID         string
	lastActions    []string
	lastConfidence float64
	backtracks     int
	stepsSinceNew  int
	lastEventAt    time.Time
	observations   map[string]bool
}

const EventMetaCogAlert ledger.EventKind = "metacog.alert"

// New creates a metacognitive monitor.
func New(bus *ledger.EventBus, events *ledger.EventStore, thresholds Thresholds) *MetaCogMonitor {
	return &MetaCogMonitor{
		bus:        bus,
		events:     events,
		thresholds: thresholds,
		state:      make(map[string]*taskMonitorState),
	}
}

// NewFromLedger creates a monitor from a Ledger instance.
func NewFromLedger(ldg *ledger.Ledger, thresholds Thresholds) *MetaCogMonitor {
	return New(ldg.Bus, ldg.Events, thresholds)
}

// SetAlertFunc sets the callback for anomaly alerts.
func (m *MetaCogMonitor) SetAlertFunc(fn AlertFunc) { m.alertFn = fn }

// Start begins monitoring by subscribing to reasoning events.
func (m *MetaCogMonitor) Start() {
	m.sub = m.bus.Subscribe(ledger.EventFilter{Reasoning: true}, 256)
	go m.monitorLoop()
}

// Stop ends monitoring.
func (m *MetaCogMonitor) Stop() {
	if m.sub != nil {
		m.bus.Unsubscribe(m.sub)
		m.sub = nil
	}
}

func (m *MetaCogMonitor) monitorLoop() {
	for event := range m.sub.C {
		m.processEvent(event)
	}
}

// ProcessEvent analyzes a single event (can be called directly for testing).
func (m *MetaCogMonitor) ProcessEvent(e *ledger.Event) {
	m.processEvent(e)
}

func (m *MetaCogMonitor) processEvent(e *ledger.Event) {
	state := m.getOrCreateState(e.TaskID)
	state.lastEventAt = e.CreatedAt

	var p struct {
		Decision   string   `json:"decision,omitempty"`
		Confidence *float64 `json:"confidence,omitempty"`
		Thought    string   `json:"thought,omitempty"`
		Observation string  `json:"observation,omitempty"`
	}
	json.Unmarshal(e.Payload, &p)

	switch e.Kind {
	case ledger.EventReasoningDecision:
		m.checkLoop(state, p.Decision)
		if p.Confidence != nil {
			m.checkConfidenceDrop(state, *p.Confidence)
			state.lastConfidence = *p.Confidence
		}
	case ledger.EventReasoningBacktrack:
		state.backtracks++
		m.checkExcessiveBacktrack(state)
	case ledger.EventReasoningObserve:
		obs := p.Observation
		if obs == "" {
			obs = p.Thought
		}
		if obs != "" {
			if state.observations[obs] {
				state.stepsSinceNew++
				m.checkNoProgress(state)
			} else {
				state.observations[obs] = true
				state.stepsSinceNew = 0
			}
		}
	case ledger.EventReasoningThought:
		if p.Confidence != nil {
			m.checkConfidenceDrop(state, *p.Confidence)
			state.lastConfidence = *p.Confidence
		}
	}
}

func (m *MetaCogMonitor) checkLoop(state *taskMonitorState, action string) {
	if action == "" {
		return
	}
	state.lastActions = append(state.lastActions, action)
	maxHistory := m.thresholds.MaxConsecutiveSameAction + 1
	if len(state.lastActions) > maxHistory {
		state.lastActions = state.lastActions[len(state.lastActions)-maxHistory:]
	}

	if len(state.lastActions) >= m.thresholds.MaxConsecutiveSameAction {
		allSame := true
		for i := len(state.lastActions) - m.thresholds.MaxConsecutiveSameAction; i < len(state.lastActions); i++ {
			if state.lastActions[i] != action {
				allSame = false
				break
			}
		}
		if allSame {
			m.fireAlert(Alert{
				TaskID:   state.taskID,
				Kind:     AlertLoop,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("Loop detected: action '%s' called %d times consecutively", action, m.thresholds.MaxConsecutiveSameAction),
				Details:  ledger.MakePayload(map[string]interface{}{"action": action, "count": m.thresholds.MaxConsecutiveSameAction}),
			})
		}
	}
}

func (m *MetaCogMonitor) checkConfidenceDrop(state *taskMonitorState, newConf float64) {
	if state.lastConfidence > 0 {
		drop := state.lastConfidence - newConf
		if drop >= m.thresholds.ConfidenceDropThreshold {
			m.fireAlert(Alert{
				TaskID:   state.taskID,
				Kind:     AlertConfidenceDrop,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("Confidence dropped from %.2f to %.2f (Δ=%.2f)", state.lastConfidence, newConf, drop),
				Details:  ledger.MakePayload(map[string]interface{}{"from": state.lastConfidence, "to": newConf, "drop": drop}),
			})
		}
	}
}

func (m *MetaCogMonitor) checkExcessiveBacktrack(state *taskMonitorState) {
	if state.backtracks >= m.thresholds.MaxBacktracksPerTask {
		severity := SeverityWarning
		if state.backtracks >= m.thresholds.MaxBacktracksPerTask*2 {
			severity = SeverityCritical
		}
		m.fireAlert(Alert{
			TaskID:   state.taskID,
			Kind:     AlertExcessiveBacktrack,
			Severity: severity,
			Message:  fmt.Sprintf("Excessive backtracks: %d (threshold: %d)", state.backtracks, m.thresholds.MaxBacktracksPerTask),
			Details:  ledger.MakePayload(map[string]interface{}{"count": state.backtracks}),
		})
	}
}

func (m *MetaCogMonitor) checkNoProgress(state *taskMonitorState) {
	if state.stepsSinceNew >= m.thresholds.MaxStepsWithoutProgress {
		m.fireAlert(Alert{
			TaskID:   state.taskID,
			Kind:     AlertNoProgress,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("No new information in %d steps", state.stepsSinceNew),
			Details:  ledger.MakePayload(map[string]interface{}{"steps_without_progress": state.stepsSinceNew}),
		})
	}
}

func (m *MetaCogMonitor) fireAlert(alert Alert) {
	alert.Timestamp = time.Now()
	if m.alertFn != nil {
		m.alertFn(alert)
	}

	if m.events != nil && alert.TaskID != "" {
		payload, _ := json.Marshal(alert)
		m.events.Append(context.Background(), &ledger.Event{
			TaskID:    alert.TaskID,
			Kind:      EventMetaCogAlert,
			Actor:     "metacog",
			Payload:   payload,
			CreatedAt: alert.Timestamp,
		})
	}
}

func (m *MetaCogMonitor) getOrCreateState(taskID string) *taskMonitorState {
	if s, ok := m.state[taskID]; ok {
		return s
	}
	s := &taskMonitorState{
		taskID:       taskID,
		observations: make(map[string]bool),
	}
	m.state[taskID] = s
	return s
}

// GetState returns the monitoring state for a task (for testing/inspection).
func (m *MetaCogMonitor) GetState(taskID string) (backtracks int, lastActions []string) {
	if s, ok := m.state[taskID]; ok {
		return s.backtracks, s.lastActions
	}
	return 0, nil
}
