package ledger

import (
	"testing"
	"time"

	"yunque-agent/internal/cognicore/metacog"
	ldg "yunque-agent/internal/ledgercore"
	lsqlite "yunque-agent/internal/ledgercore/backend/sqlite"
)

func newMetaCogTestLedger(t *testing.T) *ldg.Ledger {
	t.Helper()
	backend, err := lsqlite.New(":memory:")
	if err != nil {
		t.Fatalf("create test backend: %v", err)
	}
	l, err := ldg.Open(backend)
	if err != nil {
		t.Fatalf("open test ledger: %v", err)
	}
	t.Cleanup(func() { l.Close() })
	return l
}

func TestMetaCogBridge_NormalFlow(t *testing.T) {
	l := newMetaCogTestLedger(t)
	b := NewMetaCogBridgeForTest(l)
	defer b.Stop()

	taskID := "test-normal-001"

	hint := b.CorrectionHint(taskID)
	if hint != "" {
		t.Errorf("expected no hint for clean task, got: %s", hint)
	}

	if b.HasAnomalies(taskID) {
		t.Error("expected no anomalies for clean task")
	}

	if b.AnomalyCount(taskID) != 0 {
		t.Errorf("expected 0 anomalies, got %d", b.AnomalyCount(taskID))
	}

	if b.ShouldEscalate(taskID) {
		t.Error("should not escalate clean task")
	}
}

func TestMetaCogBridge_LoopDetection(t *testing.T) {
	l := newMetaCogTestLedger(t)
	b := NewMetaCogBridgeForTest(l)
	defer b.Stop()

	taskID := "test-loop-001"

	// Simulate loop detection by directly calling onAlert
	b.onAlert(metacog.Alert{
		TaskID:    taskID,
		Kind:      metacog.AlertLoop,
		Severity:  metacog.SeverityWarning,
		Message:   "Loop detected: action 'search' called 3 times consecutively",
		Timestamp: time.Now(),
	})

	if !b.HasAnomalies(taskID) {
		t.Error("expected anomalies after loop alert")
	}

	if b.AnomalyCount(taskID) != 1 {
		t.Errorf("expected 1 anomaly, got %d", b.AnomalyCount(taskID))
	}

	hint := b.CorrectionHint(taskID)
	if hint == "" {
		t.Fatal("expected correction hint after loop detection")
	}
	if !contains(hint, "推理循环") {
		t.Errorf("hint should mention loop, got: %s", hint)
	}

	summary := b.FormatAnomalySummary(taskID)
	if summary == "" {
		t.Error("expected non-empty anomaly summary")
	}

	b.ClearTask(taskID)
	if b.HasAnomalies(taskID) {
		t.Error("expected no anomalies after clear")
	}
}

func TestMetaCogBridge_StallDetection(t *testing.T) {
	l := newMetaCogTestLedger(t)
	b := NewMetaCogBridgeForTest(l)
	defer b.Stop()

	taskID := "test-stall-001"

	b.onAlert(metacog.Alert{
		TaskID:    taskID,
		Kind:      metacog.AlertStall,
		Severity:  metacog.SeverityWarning,
		Message:   "No new information in 5 steps",
		Timestamp: time.Now(),
	})

	hint := b.CorrectionHint(taskID)
	if hint == "" {
		t.Fatal("expected correction hint after stall detection")
	}
	if !contains(hint, "推理停滞") {
		t.Errorf("hint should mention stall, got: %s", hint)
	}
}

func TestMetaCogBridge_Escalation(t *testing.T) {
	l := newMetaCogTestLedger(t)
	b := NewMetaCogBridgeForTest(l)
	defer b.Stop()

	taskID := "test-escalate-001"

	if b.ShouldEscalate(taskID) {
		t.Error("should not escalate with zero alerts")
	}

	// 2 critical alerts should trigger escalation
	for i := 0; i < 2; i++ {
		b.onAlert(metacog.Alert{
			TaskID:   taskID,
			Kind:     metacog.AlertExcessiveBacktrack,
			Severity: metacog.SeverityCritical,
			Message:  "Excessive backtracks",
		})
	}

	if !b.ShouldEscalate(taskID) {
		t.Error("should escalate after 2 critical alerts")
	}
}

func TestMetaCogBridge_NilSafe(t *testing.T) {
	var b *MetaCogBridge

	if b.CorrectionHint("any") != "" {
		t.Error("nil bridge should return empty hint")
	}
	if b.HasAnomalies("any") {
		t.Error("nil bridge should return false")
	}
	if b.ShouldEscalate("any") {
		t.Error("nil bridge should return false")
	}
	if b.AnomalyCount("any") != 0 {
		t.Error("nil bridge should return 0")
	}

	b.ClearTask("any")
	b.Stop()
}

func TestMetaCogBridge_MultipleAnomalyTypes(t *testing.T) {
	l := newMetaCogTestLedger(t)
	b := NewMetaCogBridgeForTest(l)
	defer b.Stop()

	taskID := "test-multi-001"

	b.onAlert(metacog.Alert{TaskID: taskID, Kind: metacog.AlertLoop, Severity: metacog.SeverityWarning})
	b.onAlert(metacog.Alert{TaskID: taskID, Kind: metacog.AlertConfidenceDrop, Severity: metacog.SeverityWarning})
	b.onAlert(metacog.Alert{TaskID: taskID, Kind: metacog.AlertStall, Severity: metacog.SeverityWarning})

	hint := b.CorrectionHint(taskID)
	if !contains(hint, "推理循环") {
		t.Error("hint should mention loop")
	}
	if !contains(hint, "置信度骤降") {
		t.Error("hint should mention confidence drop")
	}
	if !contains(hint, "推理停滞") {
		t.Error("hint should mention stall")
	}

	if b.AnomalyCount(taskID) != 3 {
		t.Errorf("expected 3 anomalies, got %d", b.AnomalyCount(taskID))
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
