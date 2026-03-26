package planner

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"yunque-agent/internal/observe"
)

// SkillOptimizer 技能大盘聚合与自适应调参
type SkillOptimizer struct {
	mu           sync.RWMutex
	metrics      *observe.Metrics
	history      []SkillPerformance // 历史大盘持久化
	saveFile     string
	lastAnalysis time.Time
	analyzeCount int // 防抖 (5次/60s)
}

// SkillPerformance 单个技能监控指标
type SkillPerformance struct {
	Name        string  `json:"name"`
	Total       int64   `json:"total"`
	Success     int64   `json:"success"`
	Failed      int64   `json:"failed"`
	SuccessRate float64 `json:"success_rate"`
	AvgLatency  float64 `json:"avg_latency_ms"`
	LastUpdated string  `json:"last_updated"`
}

// NewSkillOptimizer 装载分析器
func NewSkillOptimizer(met *observe.Metrics, saveFile string) *SkillOptimizer {
	opt := &SkillOptimizer{
		metrics:  met,
		saveFile: saveFile,
	}
	opt.loadHistory()
	return opt
}

// Analyze 并入最新监控指标 (带限流熔断)
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
			}
		} else {
			o.history = append(o.history, SkillPerformance{
				Name:        s.Name,
				Total:       s.Total,
				Success:     s.Success,
				Failed:      s.Failed,
				SuccessRate: s.SuccessRate,
				AvgLatency:  s.Latency.Avg,
				LastUpdated: now,
			})
		}
	}

	o.persistHistory()
}

// OptimizationHints 提取经常报错/高频动作供 Prompt 避坑
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
		return // file doesn't exist yet, start fresh
	}
	var history []SkillPerformance
	if err := json.Unmarshal(data, &history); err != nil {
		slog.Warn("skill optimizer: load history failed", "err", err)
		return
	}
	o.history = history
	slog.Info("skill optimizer: loaded history", "skills", len(history))
}

func (o *SkillOptimizer) persistHistory() {
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
