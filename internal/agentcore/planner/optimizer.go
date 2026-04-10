package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	iledger "yunque-agent/internal/ledger"
	"yunque-agent/internal/observe"
)

// SkillOptimizer analyzes skill usage metrics and generates optimization hints
// for the planner's system prompt. This enables the agent to self-optimize
// by preferring high-success-rate skills and avoiding consistently failing ones.
type SkillOptimizer struct {
	mu           sync.RWMutex
	metrics      *observe.Metrics
	history      []SkillPerformance // persisted historical performance
	saveFile     string
	kvs          *iledger.KVConfigStore
	lastAnalysis time.Time
	analyzeCount int // throttle: analyze at most every 5 calls or 60 seconds
}

// SkillPerformance tracks a skill's performance over time.
type SkillPerformance struct {
	Name         string  `json:"name"`
	Total        int64   `json:"total"`
	Success      int64   `json:"success"`
	Failed       int64   `json:"failed"`
	SuccessRate  float64 `json:"success_rate"`
	AvgLatency   float64 `json:"avg_latency_ms"`
	LastUpdated  string  `json:"last_updated"`
	LastFailedAt string  `json:"last_failed_at,omitempty"` // RFC3339 timestamp of most recent failure
}

// NewSkillOptimizer creates an optimizer linked to the metrics system.
func NewSkillOptimizer(met *observe.Metrics, saveFile string) *SkillOptimizer {
	opt := &SkillOptimizer{
		metrics:  met,
		saveFile: saveFile,
	}
	opt.loadHistory()
	return opt
}

// SetKVStore enables Ledger KV-backed persistence, replacing file I/O.
func (o *SkillOptimizer) SetKVStore(kvs *iledger.KVConfigStore) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.kvs = kvs
	o.loadHistoryFromKV()
}

// Analyze collects current metrics snapshot and merges with historical data.
// Throttled: runs at most every 60 seconds or every 5 calls, whichever comes first.
func (o *SkillOptimizer) Analyze() {
	if o.metrics == nil {
		return
	}

	o.mu.Lock()
	o.analyzeCount++
	if o.analyzeCount < 5 && time.Since(o.lastAnalysis) < 60*time.Second {
		o.mu.Unlock()
		return
	}
	o.analyzeCount = 0
	o.lastAnalysis = time.Now()
	o.mu.Unlock()

	snap := o.metrics.Snapshot()
	if len(snap.Skills) == 0 {
		return
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	// Merge current snapshot into historical data
	existing := make(map[string]*SkillPerformance)
	for i := range o.history {
		existing[o.history[i].Name] = &o.history[i]
	}

	now := time.Now().Format(time.RFC3339)
	for _, s := range snap.Skills {
		if p, ok := existing[s.Name]; ok {
			// Merge: take max of current vs. historical
			if s.Total > p.Total {
				p.Total = s.Total
				p.Success = s.Success
				p.Failed = s.Failed
				p.SuccessRate = s.SuccessRate
				p.AvgLatency = s.Latency.Avg
				p.LastUpdated = now
				if s.Failed > 0 {
					p.LastFailedAt = now
				}
			}
		} else {
			entry := SkillPerformance{
				Name:        s.Name,
				Total:       s.Total,
				Success:     s.Success,
				Failed:      s.Failed,
				SuccessRate: s.SuccessRate,
				AvgLatency:  s.Latency.Avg,
				LastUpdated: now,
			}
			if s.Failed > 0 {
				entry.LastFailedAt = now
			}
			o.history = append(o.history, entry)
		}
	}

	o.persistHistory()
}

// OptimizationHints generates a system prompt snippet with skill performance insights.
// Returns empty string if no significant insights exist.
func (o *SkillOptimizer) OptimizationHints() string {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if len(o.history) == 0 {
		return ""
	}

	// Categorize skills by performance
	var highPerf, lowPerf, frequent []SkillPerformance
	for _, p := range o.history {
		if p.Total < 3 {
			continue // not enough data
		}
		if p.SuccessRate >= 0.9 {
			highPerf = append(highPerf, p)
		}
		if p.SuccessRate < 0.5 && p.Failed > 2 {
			lowPerf = append(lowPerf, p)
		}
		if p.Total >= 10 {
			frequent = append(frequent, p)
		}
	}

	if len(highPerf) == 0 && len(lowPerf) == 0 && len(frequent) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("[技能使用洞察]\n")

	if len(frequent) > 0 {
		sort.Slice(frequent, func(i, j int) bool { return frequent[i].Total > frequent[j].Total })
		b.WriteString("常用技能: ")
		for i, p := range frequent {
			if i >= 5 {
				break
			}
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(fmt.Sprintf("%s(%d次,%.0f%%成功)", p.Name, p.Total, p.SuccessRate*100))
		}
		b.WriteString("\n")
	}

	if len(lowPerf) > 0 {
		b.WriteString("⚠ 低成功率技能(考虑替代方案): ")
		for i, p := range lowPerf {
			if i >= 3 {
				break
			}
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(fmt.Sprintf("%s(%.0f%%成功,失败%d次)", p.Name, p.SuccessRate*100, p.Failed))
		}
		b.WriteString("\n")
	}

	// Show suppressed skills
	suppressed := o.suppressedLocked()
	if len(suppressed) > 0 {
		b.WriteString("🚫 已暂停(成功率过低): ")
		for i, name := range suppressed {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(name)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// PerformanceReport returns a structured summary for API/debugging.
func (o *SkillOptimizer) PerformanceReport() []SkillPerformance {
	o.mu.RLock()
	defer o.mu.RUnlock()
	out := make([]SkillPerformance, len(o.history))
	copy(out, o.history)
	sort.Slice(out, func(i, j int) bool { return out[i].Total > out[j].Total })
	return out
}

func (o *SkillOptimizer) loadHistory() {
	if o.saveFile == "" {
		return
	}
	data, err := os.ReadFile(o.saveFile)
	if err != nil {
		return
	}
	var history []SkillPerformance
	if err := json.Unmarshal(data, &history); err != nil {
		slog.Warn("skill optimizer: load history failed", "err", err)
		return
	}
	o.history = history
	slog.Info("skill optimizer: loaded history", "skills", len(history))
}

func (o *SkillOptimizer) loadHistoryFromKV() {
	if o.kvs == nil {
		return
	}
	var history []SkillPerformance
	found, err := o.kvs.Get(context.Background(), "history", &history)
	if err != nil {
		slog.Warn("skill optimizer: kv load failed", "err", err)
		return
	}
	if found && len(history) > 0 {
		o.history = history
		slog.Info("skill optimizer: loaded from Ledger KV", "skills", len(history))
	}
}

func (o *SkillOptimizer) persistHistory() {
	if o.kvs != nil {
		if err := o.kvs.Put(context.Background(), "history", o.history); err != nil {
			slog.Warn("skill optimizer: kv persist failed, falling back to file", "err", err)
		} else {
			return
		}
	}
	if o.saveFile == "" {
		return
	}
	data, err := json.MarshalIndent(o.history, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(o.saveFile, data, 0644); err != nil {
		slog.Warn("skill optimizer: persist failed", "err", err)
	}
}

// ShouldSuppress returns true if a skill should be hidden from the LLM tool list.
// Criteria: success rate < 30% AND failed > 5 AND last failure within 24 hours.
func (o *SkillOptimizer) ShouldSuppress(skillName string) bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	for _, p := range o.history {
		if p.Name == skillName {
			return o.isSkillSuppressed(p)
		}
	}
	return false
}

// SuppressedSkills returns the list of skill names currently suppressed.
func (o *SkillOptimizer) SuppressedSkills() []string {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.suppressedLocked()
}

// suppressedLocked returns suppressed skill names. Caller must hold at least RLock.
func (o *SkillOptimizer) suppressedLocked() []string {
	var result []string
	for _, p := range o.history {
		if o.isSkillSuppressed(p) {
			result = append(result, p.Name)
		}
	}
	return result
}

// isSkillSuppressed checks suppression criteria for a single skill entry.
func (o *SkillOptimizer) isSkillSuppressed(p SkillPerformance) bool {
	if p.Total < 6 || p.SuccessRate >= 0.3 || p.Failed <= 5 {
		return false
	}
	if p.LastFailedAt == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, p.LastFailedAt)
	if err != nil {
		return false
	}
	return time.Since(t) < 24*time.Hour
}
