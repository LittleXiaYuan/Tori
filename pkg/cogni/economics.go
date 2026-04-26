package cogni

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// EconomicsConfig declares per-cogni resource constraints.
type EconomicsConfig struct {
	BudgetPerRun  float64 `json:"budget_per_run,omitempty" yaml:"budget_per_run,omitempty"`   // max cost per workflow/turn
	DailyBudget   float64 `json:"daily_budget,omitempty" yaml:"daily_budget,omitempty"`       // max cost per day
	PriorityWeight float64 `json:"priority_weight,omitempty" yaml:"priority_weight,omitempty"` // weight in bidding (default 1.0)
	SharedCache   bool    `json:"shared_cache,omitempty" yaml:"shared_cache,omitempty"`       // share LLM response cache across turns
}

// CostEntry records a single cost event.
type CostEntry struct {
	CogniID   string    `json:"cogni_id"`
	Timestamp time.Time `json:"timestamp"`
	Tokens    int       `json:"tokens"`
	Cost      float64   `json:"cost"` // estimated USD
	Operation string    `json:"operation"`
}

// CostTracker tracks per-cogni resource usage and enforces budgets.
type CostTracker struct {
	mu      sync.RWMutex
	entries map[string][]CostEntry // keyed by cogni ID
	configs map[string]EconomicsConfig
}

func NewCostTracker() *CostTracker {
	return &CostTracker{
		entries: make(map[string][]CostEntry),
		configs: make(map[string]EconomicsConfig),
	}
}

// SetConfig registers budget constraints for a cogni.
func (ct *CostTracker) SetConfig(cogniID string, cfg EconomicsConfig) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.configs[cogniID] = cfg
}

// Record logs a cost event.
func (ct *CostTracker) Record(entry CostEntry) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	ct.entries[entry.CogniID] = append(ct.entries[entry.CogniID], entry)
}

// CheckBudget returns an error if the cogni would exceed its per-run or daily budget.
func (ct *CostTracker) CheckBudget(cogniID string, estimatedCost float64) error {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	cfg, ok := ct.configs[cogniID]
	if !ok {
		return nil
	}

	if cfg.BudgetPerRun > 0 && estimatedCost > cfg.BudgetPerRun {
		return fmt.Errorf("cogni %q: estimated cost $%.4f exceeds per-run budget $%.4f",
			cogniID, estimatedCost, cfg.BudgetPerRun)
	}

	if cfg.DailyBudget > 0 {
		today := dailyTotal(ct.entries[cogniID])
		if today+estimatedCost > cfg.DailyBudget {
			return fmt.Errorf("cogni %q: daily spend $%.4f + $%.4f would exceed budget $%.4f",
				cogniID, today, estimatedCost, cfg.DailyBudget)
		}
	}

	return nil
}

// DailySummary returns per-cogni cost summaries for today.
func (ct *CostTracker) DailySummary() map[string]CostSummary {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	out := make(map[string]CostSummary)
	for id, entries := range ct.entries {
		var s CostSummary
		startOfDay := time.Now().Truncate(24 * time.Hour)
		for _, e := range entries {
			if e.Timestamp.Before(startOfDay) {
				continue
			}
			s.TotalCost += e.Cost
			s.TotalTokens += e.Tokens
			s.Operations++
		}
		cfg := ct.configs[id]
		s.BudgetPerRun = cfg.BudgetPerRun
		s.DailyBudget = cfg.DailyBudget
		if cfg.DailyBudget > 0 {
			s.UtilizationPct = (s.TotalCost / cfg.DailyBudget) * 100
		}
		out[id] = s
	}
	return out
}

// CostSummary provides a snapshot of a cogni's daily resource usage.
type CostSummary struct {
	TotalCost      float64 `json:"total_cost"`
	TotalTokens    int     `json:"total_tokens"`
	Operations     int     `json:"operations"`
	BudgetPerRun   float64 `json:"budget_per_run"`
	DailyBudget    float64 `json:"daily_budget"`
	UtilizationPct float64 `json:"utilization_pct"`
}

// PruneOld removes entries older than the given duration.
func (ct *CostTracker) PruneOld(maxAge time.Duration) int {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0
	for id, entries := range ct.entries {
		var keep []CostEntry
		for _, e := range entries {
			if e.Timestamp.After(cutoff) {
				keep = append(keep, e)
			} else {
				removed++
			}
		}
		ct.entries[id] = keep
	}
	if removed > 0 {
		slog.Debug("cogni economics: pruned old entries", "removed", removed)
	}
	return removed
}

func dailyTotal(entries []CostEntry) float64 {
	startOfDay := time.Now().Truncate(24 * time.Hour)
	var total float64
	for _, e := range entries {
		if e.Timestamp.After(startOfDay) {
			total += e.Cost
		}
	}
	return total
}
