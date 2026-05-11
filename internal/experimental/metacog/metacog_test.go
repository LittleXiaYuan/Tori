package metacog

import (
	"testing"
	"time"

	"github.com/LittleXiaYuan/ledger"
)

func newTestMonitor() *MetaCogMonitor {
	bus := ledger.NewEventBus()
	return New(bus, nil, DefaultThresholds())
}

func TestDefaultThresholds(t *testing.T) {
	th := DefaultThresholds()
	if th.MaxConsecutiveSameAction != 3 {
		t.Errorf("MaxConsecutiveSameAction = %d, want 3", th.MaxConsecutiveSameAction)
	}
	if th.StallTimeout != 2*time.Minute {
		t.Errorf("StallTimeout = %v, want 2m", th.StallTimeout)
	}
}

func TestLoopDetection(t *testing.T) {
	m := newTestMonitor()

	var alerts []Alert
	m.SetAlertFunc(func(a Alert) { alerts = append(alerts, a) })

	task := "task-1"
	// Send 3 consecutive same-action events
	for i := 0; i < 3; i++ {
		m.ProcessEvent(&ledger.Event{
			TaskID: task,
			Kind:   ledger.EventReasoningDecision,
			Payload: ledger.MakePayload(map[string]interface{}{
				"decision": "call_api",
			}),
			CreatedAt: time.Now(),
		})
	}

	found := false
	for _, a := range alerts {
		if a.Kind == AlertLoop {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected loop detection alert")
	}
}

func TestNoLoopForDifferentActions(t *testing.T) {
	m := newTestMonitor()

	var alerts []Alert
	m.SetAlertFunc(func(a Alert) { alerts = append(alerts, a) })

	actions := []string{"read_file", "write_file", "call_api"}
	for _, action := range actions {
		m.ProcessEvent(&ledger.Event{
			TaskID: "task-2",
			Kind:   ledger.EventReasoningDecision,
			Payload: ledger.MakePayload(map[string]interface{}{
				"decision": action,
			}),
			CreatedAt: time.Now(),
		})
	}

	for _, a := range alerts {
		if a.Kind == AlertLoop {
			t.Error("should not detect loop for different actions")
		}
	}
}

func TestConfidenceDrop(t *testing.T) {
	m := newTestMonitor()

	var alerts []Alert
	m.SetAlertFunc(func(a Alert) { alerts = append(alerts, a) })

	// First event: confidence 0.9
	high := 0.9
	m.ProcessEvent(&ledger.Event{
		TaskID:    "task-3",
		Kind:      ledger.EventReasoningDecision,
		Payload:   ledger.MakePayload(map[string]interface{}{"confidence": &high, "decision": "a"}),
		CreatedAt: time.Now(),
	})

	// Second event: confidence drops to 0.4 (Δ=0.5 > threshold 0.3)
	low := 0.4
	m.ProcessEvent(&ledger.Event{
		TaskID:    "task-3",
		Kind:      ledger.EventReasoningDecision,
		Payload:   ledger.MakePayload(map[string]interface{}{"confidence": &low, "decision": "b"}),
		CreatedAt: time.Now(),
	})

	found := false
	for _, a := range alerts {
		if a.Kind == AlertConfidenceDrop {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected confidence drop alert")
	}
}

func TestExcessiveBacktrack(t *testing.T) {
	m := newTestMonitor()

	var alerts []Alert
	m.SetAlertFunc(func(a Alert) { alerts = append(alerts, a) })

	for i := 0; i < 6; i++ {
		m.ProcessEvent(&ledger.Event{
			TaskID:    "task-4",
			Kind:      ledger.EventReasoningBacktrack,
			Payload:   ledger.MakePayload(map[string]interface{}{}),
			CreatedAt: time.Now(),
		})
	}

	found := false
	for _, a := range alerts {
		if a.Kind == AlertExcessiveBacktrack {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected excessive backtrack alert")
	}
}

func TestNoProgressDetection(t *testing.T) {
	m := newTestMonitor()

	var alerts []Alert
	m.SetAlertFunc(func(a Alert) { alerts = append(alerts, a) })

	// Send same observation 6 times (threshold is 5)
	for i := 0; i < 6; i++ {
		m.ProcessEvent(&ledger.Event{
			TaskID: "task-5",
			Kind:   ledger.EventReasoningObserve,
			Payload: ledger.MakePayload(map[string]interface{}{
				"observation": "same thing",
			}),
			CreatedAt: time.Now(),
		})
	}

	found := false
	for _, a := range alerts {
		if a.Kind == AlertNoProgress {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected no-progress alert")
	}
}

type fakeIsolationForest struct {
	trained bool
	score   float64
	fitN    int
}

func (f *fakeIsolationForest) IsTrained() bool { return f.trained }
func (f *fakeIsolationForest) Fit(data [][]float64) {
	f.trained = true
	f.fitN = len(data)
}
func (f *fakeIsolationForest) Score(_ []float64) float64 { return f.score }

func TestIsolationForestAnomalyAlert(t *testing.T) {
	m := newTestMonitor()
	f := &fakeIsolationForest{score: 0.82}
	m.SetIsolationForest(f, 0.65)
	if !f.trained || f.fitN == 0 {
		t.Fatal("expected MetaCog to train default Isolation Forest baseline")
	}

	var alerts []Alert
	m.SetAlertFunc(func(a Alert) { alerts = append(alerts, a) })

	conf := 0.2
	m.ProcessEvent(&ledger.Event{
		TaskID: "task-iforest",
		Kind:   ledger.EventReasoningDecision,
		Payload: ledger.MakePayload(map[string]interface{}{
			"decision":   "repeat_tool",
			"confidence": &conf,
		}),
		CreatedAt: time.Now(),
	})

	found := false
	for _, a := range alerts {
		if a.Kind == AlertStatisticalAnomaly {
			found = true
			if a.Severity != SeverityCritical {
				t.Fatalf("expected critical statistical anomaly, got %s", a.Severity)
			}
		}
	}
	if !found {
		t.Fatal("expected statistical anomaly alert")
	}
}

func TestIsolationForestAlertOnlyOnThresholdCrossing(t *testing.T) {
	m := newTestMonitor()
	f := &fakeIsolationForest{trained: true, score: 0.7}
	m.SetIsolationForest(f, 0.65)

	count := 0
	m.SetAlertFunc(func(a Alert) {
		if a.Kind == AlertStatisticalAnomaly {
			count++
		}
	})
	for i := 0; i < 3; i++ {
		m.ProcessEvent(&ledger.Event{
			TaskID:    "task-iforest-once",
			Kind:      ledger.EventReasoningBacktrack,
			Payload:   ledger.MakePayload(map[string]interface{}{}),
			CreatedAt: time.Now(),
		})
	}
	if count != 1 {
		t.Fatalf("expected one threshold-crossing alert, got %d", count)
	}
}

func TestGetState(t *testing.T) {
	m := newTestMonitor()

	// Process some backtrack events
	for i := 0; i < 3; i++ {
		m.ProcessEvent(&ledger.Event{
			TaskID:    "task-6",
			Kind:      ledger.EventReasoningBacktrack,
			Payload:   ledger.MakePayload(map[string]interface{}{}),
			CreatedAt: time.Now(),
		})
	}

	backtracks, _ := m.GetState("task-6")
	if backtracks != 3 {
		t.Errorf("backtracks = %d, want 3", backtracks)
	}

	// Non-existent task
	bt, actions := m.GetState("nonexistent")
	if bt != 0 || actions != nil {
		t.Error("expected zero state for unknown task")
	}
}
