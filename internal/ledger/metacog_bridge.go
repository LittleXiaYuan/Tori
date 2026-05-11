package ledger

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	ldg "github.com/LittleXiaYuan/ledger"
	"yunque-agent/internal/experimental/metacog"
)

// MetaCogBridge connects the Ledger MetaCogMonitor to the Planner's
// reasoning loop. It collects real-time anomaly alerts and translates
// them into correction hints that the Planner injects into subsequent
// system prompts.
//
// Enabled via METACOG_ENABLED=true (default: false for gradual rollout).
type MetaCogBridge struct {
	mu      sync.RWMutex
	monitor *metacog.MetaCogMonitor
	mcog    *ldg.MetaCognition
	alerts  map[string][]metacog.Alert // taskID → recent alerts
	enabled bool
}

// NewMetaCogBridge creates a bridge from Ledger's metacognition subsystem
// to the Planner. Returns nil if METACOG_ENABLED is not set to "true".
func NewMetaCogBridge(l *ldg.Ledger) *MetaCogBridge {
	envVal := strings.ToLower(strings.TrimSpace(os.Getenv("METACOG_ENABLED")))
	if envVal != "true" && envVal != "1" {
		return nil
	}

	mon := metacog.NewFromLedger(l, metacog.DefaultThresholds())
	mcog := ldg.NewMetaCognition(nil, l.Events)

	b := &MetaCogBridge{
		monitor: mon,
		mcog:    mcog,
		alerts:  make(map[string][]metacog.Alert),
		enabled: true,
	}

	mon.SetAlertFunc(b.onAlert)
	mon.Start()

	slog.Info("metacog_bridge: started (METACOG_ENABLED=true)")
	return b
}

// NewMetaCogBridgeForTest creates a bridge for testing without env check.
// Unlike the production constructor, it does NOT start the monitor goroutine
// to avoid EventBus subscription requirements in unit tests. Call onAlert
// directly to simulate anomaly detection.
func NewMetaCogBridgeForTest(l *ldg.Ledger) *MetaCogBridge {
	b := &MetaCogBridge{
		alerts:  make(map[string][]metacog.Alert),
		enabled: true,
	}

	if l != nil && l.Bus != nil && l.Events != nil {
		mon := metacog.NewFromLedger(l, metacog.DefaultThresholds())
		mon.SetAlertFunc(b.onAlert)
		b.monitor = mon
		b.mcog = ldg.NewMetaCognition(nil, l.Events)
	}

	return b
}

func (b *MetaCogBridge) onAlert(alert metacog.Alert) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.alerts[alert.TaskID] = append(b.alerts[alert.TaskID], alert)
	const maxAlerts = 20
	if len(b.alerts[alert.TaskID]) > maxAlerts {
		b.alerts[alert.TaskID] = b.alerts[alert.TaskID][len(b.alerts[alert.TaskID])-maxAlerts:]
	}

	slog.Warn("metacog_bridge: anomaly detected",
		"task", alert.TaskID,
		"kind", string(alert.Kind),
		"severity", string(alert.Severity),
		"message", alert.Message,
	)
}

// CorrectionHint returns a system prompt snippet with anomaly-based
// correction guidance for the given task. Empty string if no anomalies.
func (b *MetaCogBridge) CorrectionHint(taskID string) string {
	if b == nil || !b.enabled {
		return ""
	}

	b.mu.RLock()
	alerts := b.alerts[taskID]
	b.mu.RUnlock()

	if len(alerts) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("[元认知修正建议]\n")

	hasLoop := false
	hasStall := false
	hasBacktrack := false
	hasConfDrop := false

	for _, a := range alerts {
		switch a.Kind {
		case metacog.AlertLoop:
			hasLoop = true
		case metacog.AlertStall, metacog.AlertNoProgress:
			hasStall = true
		case metacog.AlertExcessiveBacktrack:
			hasBacktrack = true
		case metacog.AlertConfidenceDrop:
			hasConfDrop = true
		}
	}

	if hasLoop {
		sb.WriteString("⚠ 检测到推理循环：你在重复相同的操作。请换一种完全不同的方法解决问题。\n")
	}
	if hasStall {
		sb.WriteString("⚠ 检测到推理停滞：最近几步没有产生新信息。请重新审视目标，尝试分解为更小的子任务。\n")
	}
	if hasBacktrack {
		sb.WriteString("⚠ 回溯过多：当前方法效率低下。建议切换到分步规划模式，先制定整体方案再执行。\n")
	}
	if hasConfDrop {
		sb.WriteString("⚠ 置信度骤降：你可能偏离了任务目标。请回顾原始需求，确认当前方向正确。\n")
	}

	return sb.String()
}

// HasAnomalies returns whether any anomalies have been detected for the task.
func (b *MetaCogBridge) HasAnomalies(taskID string) bool {
	if b == nil || !b.enabled {
		return false
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.alerts[taskID]) > 0
}

// AnomalyCount returns the number of anomalies detected for a task.
func (b *MetaCogBridge) AnomalyCount(taskID string) int {
	if b == nil || !b.enabled {
		return 0
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.alerts[taskID])
}

// ShouldEscalate returns true if the anomaly pattern suggests the
// planner should switch to a more powerful model or decompose the task.
func (b *MetaCogBridge) ShouldEscalate(taskID string) bool {
	if b == nil || !b.enabled {
		return false
	}
	b.mu.RLock()
	alerts := b.alerts[taskID]
	b.mu.RUnlock()

	criticalCount := 0
	for _, a := range alerts {
		if a.Severity == metacog.SeverityCritical {
			criticalCount++
		}
	}
	return criticalCount >= 2 || len(alerts) >= 5
}

// ClearTask removes all recorded anomalies for a completed task.
func (b *MetaCogBridge) ClearTask(taskID string) {
	if b == nil {
		return
	}
	b.mu.Lock()
	delete(b.alerts, taskID)
	b.mu.Unlock()
}

// Stop shuts down the monitor goroutine.
func (b *MetaCogBridge) Stop() {
	if b == nil || b.monitor == nil {
		return
	}
	b.monitor.Stop()
}

// FormatAnomalySummary returns a human-readable summary for logging.
func (b *MetaCogBridge) FormatAnomalySummary(taskID string) string {
	if b == nil || !b.enabled {
		return ""
	}
	b.mu.RLock()
	alerts := b.alerts[taskID]
	b.mu.RUnlock()

	if len(alerts) == 0 {
		return ""
	}

	kinds := make(map[metacog.AlertKind]int)
	for _, a := range alerts {
		kinds[a.Kind]++
	}

	var parts []string
	for k, v := range kinds {
		parts = append(parts, fmt.Sprintf("%s×%d", k, v))
	}
	return fmt.Sprintf("metacog[%s]: %s", taskID, strings.Join(parts, ", "))
}
