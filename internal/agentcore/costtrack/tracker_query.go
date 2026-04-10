package costtrack

import (
	"sort"
	"time"
)

// TaskCost returns aggregated cost for a specific task.
type TaskCostSummary struct {
	TaskID     string             `json:"task_id"`
	TotalCost  float64            `json:"total_cost_usd"`
	TotalIn    int64              `json:"total_tokens_in"`
	TotalOut   int64              `json:"total_tokens_out"`
	Calls      int64              `json:"calls"`
	AvgLatency int64              `json:"avg_latency_ms"`
	BySkill    map[string]float64 `json:"by_skill,omitempty"`
	ByModel    map[string]float64 `json:"by_model,omitempty"`
}

// GetTaskCost returns cost breakdown for a single task.
func (t *Tracker) GetTaskCost(taskID string) TaskCostSummary {
	t.mu.RLock()
	defer t.mu.RUnlock()

	s := TaskCostSummary{
		TaskID:  taskID,
		BySkill: make(map[string]float64),
		ByModel: make(map[string]float64),
	}
	for _, u := range t.usages {
		if u.TaskID != taskID {
			continue
		}
		s.TotalCost += u.CostUSD
		s.TotalIn += int64(u.TokensIn)
		s.TotalOut += int64(u.TokensOut)
		s.Calls++
		if s.Calls > 0 {
			s.AvgLatency = (s.AvgLatency*(s.Calls-1) + u.Latency.Milliseconds()) / s.Calls
		}
		if u.SkillName != "" {
			s.BySkill[u.SkillName] += u.CostUSD
		}
		s.ByModel[u.Model] += u.CostUSD
	}
	return s
}

// ChannelCostSummary aggregates cost by channel.
type ChannelCostSummary struct {
	Channel   string  `json:"channel"`
	CostUSD   float64 `json:"cost_usd"`
	Calls     int64   `json:"calls"`
	TokensIn  int64   `json:"tokens_in"`
	TokensOut int64   `json:"tokens_out"`
}

// GetCostByChannel returns per-channel cost breakdown.
func (t *Tracker) GetCostByChannel() []ChannelCostSummary {
	t.mu.RLock()
	defer t.mu.RUnlock()

	m := make(map[string]*ChannelCostSummary)
	for _, u := range t.usages {
		ch := u.Channel
		if ch == "" {
			ch = "unknown"
		}
		if _, ok := m[ch]; !ok {
			m[ch] = &ChannelCostSummary{Channel: ch}
		}
		s := m[ch]
		s.CostUSD += u.CostUSD
		s.Calls++
		s.TokensIn += int64(u.TokensIn)
		s.TokensOut += int64(u.TokensOut)
	}
	out := make([]ChannelCostSummary, 0, len(m))
	for _, v := range m {
		out = append(out, *v)
	}
	return out
}

// TierCostSummary aggregates cost by model tier.
type TierCostSummary struct {
	Tier      string  `json:"tier"`
	CostUSD   float64 `json:"cost_usd"`
	Calls     int64   `json:"calls"`
	TokensIn  int64   `json:"tokens_in"`
	TokensOut int64   `json:"tokens_out"`
}

// GetCostByTier returns per-tier cost breakdown.
func (t *Tracker) GetCostByTier() []TierCostSummary {
	t.mu.RLock()
	defer t.mu.RUnlock()

	m := make(map[string]*TierCostSummary)
	for _, u := range t.usages {
		tier := u.Tier
		if tier == "" {
			tier = "unknown"
		}
		if _, ok := m[tier]; !ok {
			m[tier] = &TierCostSummary{Tier: tier}
		}
		s := m[tier]
		s.CostUSD += u.CostUSD
		s.Calls++
		s.TokensIn += int64(u.TokensIn)
		s.TokensOut += int64(u.TokensOut)
	}
	out := make([]TierCostSummary, 0, len(m))
	for _, v := range m {
		out = append(out, *v)
	}
	return out
}

// RunnerTypeCost aggregates cost by runner type.
type RunnerTypeCost struct {
	RunnerType string  `json:"runner_type"`
	CostUSD    float64 `json:"cost_usd"`
	Calls      int64   `json:"calls"`
}

// GetCostByRunnerType returns per-runner-type cost breakdown.
func (t *Tracker) GetCostByRunnerType() []RunnerTypeCost {
	t.mu.RLock()
	defer t.mu.RUnlock()

	m := make(map[string]*RunnerTypeCost)
	for _, u := range t.usages {
		rt := u.RunnerType
		if rt == "" {
			rt = "chat"
		}
		if _, ok := m[rt]; !ok {
			m[rt] = &RunnerTypeCost{RunnerType: rt}
		}
		s := m[rt]
		s.CostUSD += u.CostUSD
		s.Calls++
	}
	out := make([]RunnerTypeCost, 0, len(m))
	for _, v := range m {
		out = append(out, *v)
	}
	return out
}

// GetTaskTimeline returns ordered usage events for a specific task.
func (t *Tracker) GetTaskTimeline(taskID string) []Usage {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var out []Usage
	for _, u := range t.usages {
		if u.TaskID == taskID {
			out = append(out, u)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	return out
}

// UsageFilter defines criteria for querying usage history.
type UsageFilter struct {
	TaskID     string
	Model      string
	Channel    string
	RunnerType string
	ProviderID string
	Since      time.Time // zero = no lower bound
	Until      time.Time // zero = no upper bound
	Page       int       // 1-based; 0 defaults to 1
	Limit      int       // 0 defaults to 50
}

// UsagePage is a paginated result of usage records.
type UsagePage struct {
	Items []Usage `json:"items"`
	Total int     `json:"total"`
	Page  int     `json:"page"`
	Limit int     `json:"limit"`
}

// GetUsageHistory returns filtered, paginated usage records (newest first).
func (t *Tracker) GetUsageHistory(f UsageFilter) UsagePage {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if f.Page <= 0 {
		f.Page = 1
	}
	if f.Limit <= 0 {
		f.Limit = 50
	}

	var filtered []Usage
	for _, u := range t.usages {
		if f.TaskID != "" && u.TaskID != f.TaskID {
			continue
		}
		if f.Model != "" && u.Model != f.Model {
			continue
		}
		if f.Channel != "" && u.Channel != f.Channel {
			continue
		}
		if f.RunnerType != "" && u.RunnerType != f.RunnerType {
			continue
		}
		if f.ProviderID != "" && u.ProviderID != f.ProviderID {
			continue
		}
		if !f.Since.IsZero() && u.Timestamp.Before(f.Since) {
			continue
		}
		if !f.Until.IsZero() && u.Timestamp.After(f.Until) {
			continue
		}
		filtered = append(filtered, u)
	}

	// Sort newest first
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.After(filtered[j].Timestamp)
	})

	total := len(filtered)
	start := (f.Page - 1) * f.Limit
	if start >= total {
		return UsagePage{Items: []Usage{}, Total: total, Page: f.Page, Limit: f.Limit}
	}
	end := start + f.Limit
	if end > total {
		end = total
	}
	return UsagePage{Items: filtered[start:end], Total: total, Page: f.Page, Limit: f.Limit}
}

// MonthCost returns the current month's total spending.
func (t *Tracker) MonthCost() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	now := time.Now()
	month := now.Format("2006-01")
	var cost float64
	for _, u := range t.usages {
		if u.Timestamp.Format("2006-01") == month {
			cost += u.CostUSD
		}
	}
	return cost
}

// ProviderCostSummary aggregates cost by provider.
type ProviderCostSummary struct {
	ProviderID string  `json:"provider_id"`
	CostUSD    float64 `json:"cost_usd"`
	Calls      int64   `json:"calls"`
	TokensIn   int64   `json:"tokens_in"`
	TokensOut  int64   `json:"tokens_out"`
}

// GetCostByProvider returns cost breakdown by provider ID.
func (t *Tracker) GetCostByProvider() []ProviderCostSummary {
	t.mu.RLock()
	defer t.mu.RUnlock()

	m := make(map[string]*ProviderCostSummary)
	for _, u := range t.usages {
		pid := u.ProviderID
		if pid == "" {
			pid = "default"
		}
		if _, ok := m[pid]; !ok {
			m[pid] = &ProviderCostSummary{ProviderID: pid}
		}
		s := m[pid]
		s.CostUSD += u.CostUSD
		s.Calls++
		s.TokensIn += int64(u.TokensIn)
		s.TokensOut += int64(u.TokensOut)
	}
	out := make([]ProviderCostSummary, 0, len(m))
	for _, v := range m {
		out = append(out, *v)
	}
	return out
}
