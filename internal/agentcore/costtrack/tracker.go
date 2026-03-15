package costtrack

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Pricing defines token cost per model (USD per 1M tokens).
type Pricing struct {
	InputPer1M  float64 `json:"input_per_1m"`
	OutputPer1M float64 `json:"output_per_1m"`
}

// Known pricing table (can be updated at runtime).
var DefaultPricing = map[string]Pricing{
	"gpt-4o-mini":       {InputPer1M: 0.15, OutputPer1M: 0.60},
	"gpt-4o":            {InputPer1M: 2.50, OutputPer1M: 10.00},
	"gpt-4.1":           {InputPer1M: 2.00, OutputPer1M: 8.00},
	"gpt-4.1-mini":      {InputPer1M: 0.40, OutputPer1M: 1.60},
	"gpt-4.1-nano":      {InputPer1M: 0.10, OutputPer1M: 0.40},
	"o1":                {InputPer1M: 15.00, OutputPer1M: 60.00},
	"o1-mini":           {InputPer1M: 1.10, OutputPer1M: 4.40},
	"o3":                {InputPer1M: 10.00, OutputPer1M: 40.00},
	"o3-mini":           {InputPer1M: 1.10, OutputPer1M: 4.40},
	"o4-mini":           {InputPer1M: 1.10, OutputPer1M: 4.40},
	"claude-sonnet":     {InputPer1M: 3.00, OutputPer1M: 15.00},
	"claude-haiku":      {InputPer1M: 0.25, OutputPer1M: 1.25},
	"claude-opus":       {InputPer1M: 15.00, OutputPer1M: 75.00},
	"deepseek-chat":     {InputPer1M: 0.27, OutputPer1M: 1.10},
	"deepseek-reasoner": {InputPer1M: 0.55, OutputPer1M: 2.19},
}

// Usage records one LLM call's token usage.
type Usage struct {
	Model      string        `json:"model"`
	TenantID   string        `json:"tenant_id"`
	UserID     string        `json:"user_id"`
	SessionID  string        `json:"session_id"`
	TaskID     string        `json:"task_id,omitempty"`     // Associated task (empty for direct chat)
	StepID     string        `json:"step_id,omitempty"`     // Step within the task
	SkillName  string        `json:"skill_name,omitempty"`  // Skill that triggered this call
	ProviderID string        `json:"provider_id,omitempty"` // Provider that served this call
	Channel    string        `json:"channel,omitempty"`     // Origin channel (telegram/feishu/web/...)
	RunnerType string        `json:"runner_type,omitempty"` // chat/task/cron/trigger
	Tier       string        `json:"tier,omitempty"`        // fast/smart/expert
	TokensIn   int           `json:"tokens_in"`
	TokensOut  int           `json:"tokens_out"`
	CostUSD    float64       `json:"cost_usd"`
	Timestamp  time.Time     `json:"timestamp"`
	Latency    time.Duration `json:"latency"`
}

// Summary aggregates cost data.
type Summary struct {
	TotalCostUSD   float64               `json:"total_cost_usd"`
	TotalTokensIn  int64                 `json:"total_tokens_in"`
	TotalTokensOut int64                 `json:"total_tokens_out"`
	TotalCalls     int64                 `json:"total_calls"`
	ByModel        map[string]*ModelCost `json:"by_model"`
	ByUser         map[string]float64    `json:"by_user"`
	ByDay          map[string]float64    `json:"by_day"`
	ByChannel      map[string]float64    `json:"by_channel,omitempty"`
	ByTier         map[string]float64    `json:"by_tier,omitempty"`
	ByRunnerType   map[string]float64    `json:"by_runner_type,omitempty"`
	ByTask         map[string]float64    `json:"by_task,omitempty"`
}

// ModelCost tracks per-model cost.
type ModelCost struct {
	Calls        int64   `json:"calls"`
	TokensIn     int64   `json:"tokens_in"`
	TokensOut    int64   `json:"tokens_out"`
	CostUSD      float64 `json:"cost_usd"`
	AvgLatencyMs int64   `json:"avg_latency_ms"`
}

// Budget defines spending limits.
type Budget struct {
	DailyLimitUSD   float64 `json:"daily_limit_usd"`
	MonthlyLimitUSD float64 `json:"monthly_limit_usd"`
	PerCallLimitUSD float64 `json:"per_call_limit_usd"`
}

// Tracker tracks token usage and costs in real-time.
type Tracker struct {
	mu                 sync.RWMutex
	pricing            map[string]Pricing
	usages             []Usage
	budget             Budget
	alerts             []Alert
	persistPath        string // path to JSONL file; empty = no persistence
	MaxInMemoryRecords int    // max records in memory; 0 = 100000
}

// Alert is triggered when spending approaches a limit.
type Alert struct {
	Type      string    `json:"type"` // "daily_limit", "monthly_limit", "per_call"
	Message   string    `json:"message"`
	CostUSD   float64   `json:"cost_usd"`
	Timestamp time.Time `json:"timestamp"`
}

// New creates a cost tracker.
func New() *Tracker {
	pricing := make(map[string]Pricing, len(DefaultPricing))
	for k, v := range DefaultPricing {
		pricing[k] = v
	}
	return &Tracker{
		pricing: pricing,
	}
}

// NewWithPersistence creates a cost tracker that persists usage to a JSONL file.
// Existing records are loaded on creation.
func NewWithPersistence(dataDir string) *Tracker {
	t := New()
	t.persistPath = filepath.Join(dataDir, "cost_telemetry.jsonl")
	t.loadFromDisk()
	return t
}

// SetBudget configures spending limits.
func (t *Tracker) SetBudget(b Budget) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.budget = b
}

// SetPricing updates pricing for a model.
func (t *Tracker) SetPricing(model string, p Pricing) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pricing[model] = p
}

// Record logs a new LLM usage event and returns the computed cost.
func (t *Tracker) Record(model, tenantID, userID, sessionID string, tokensIn, tokensOut int, latency time.Duration) (float64, *Alert) {
	return t.RecordExt(RecordOpts{
		Model:     model,
		TenantID:  tenantID,
		UserID:    userID,
		SessionID: sessionID,
		TokensIn:  tokensIn,
		TokensOut: tokensOut,
		Latency:   latency,
	})
}

// RecordOpts provides all fields for an enriched usage record.
type RecordOpts struct {
	Model      string
	TenantID   string
	UserID     string
	SessionID  string
	TaskID     string
	StepID     string
	SkillName  string
	ProviderID string
	Channel    string
	RunnerType string // "chat", "task", "cron", "trigger"
	Tier       string // "fast", "smart", "expert"
	TokensIn   int
	TokensOut  int
	Latency    time.Duration
}

// RecordExt logs a usage event with extended telemetry fields.
func (t *Tracker) RecordExt(opts RecordOpts) (float64, *Alert) {
	t.mu.Lock()
	defer t.mu.Unlock()

	cost := t.computeCost(opts.Model, opts.TokensIn, opts.TokensOut)

	u := Usage{
		Model:      opts.Model,
		TenantID:   opts.TenantID,
		UserID:     opts.UserID,
		SessionID:  opts.SessionID,
		TaskID:     opts.TaskID,
		StepID:     opts.StepID,
		SkillName:  opts.SkillName,
		ProviderID: opts.ProviderID,
		Channel:    opts.Channel,
		RunnerType: opts.RunnerType,
		Tier:       opts.Tier,
		TokensIn:   opts.TokensIn,
		TokensOut:  opts.TokensOut,
		CostUSD:    cost,
		Timestamp:  time.Now(),
		Latency:    opts.Latency,
	}
	t.usages = append(t.usages, u)
	t.evictOldRecords()
	t.appendToDisk(u)

	alert := t.checkBudget(u)
	if alert != nil {
		t.alerts = append(t.alerts, *alert)
	}

	return cost, alert
}

// CheckBudget returns true if the next call with estimated cost would exceed budget.
func (t *Tracker) WouldExceedBudget(model string, estimatedTokensIn, estimatedTokensOut int) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.budget.DailyLimitUSD <= 0 {
		return false
	}

	estimatedCost := t.computeCost(model, estimatedTokensIn, estimatedTokensOut)
	todayCost := t.dailyCostLocked(time.Now())

	return (todayCost + estimatedCost) > t.budget.DailyLimitUSD
}

// GetSummary returns aggregated cost data.
func (t *Tracker) GetSummary() Summary {
	t.mu.RLock()
	defer t.mu.RUnlock()

	s := Summary{
		ByModel:      make(map[string]*ModelCost),
		ByUser:       make(map[string]float64),
		ByDay:        make(map[string]float64),
		ByChannel:    make(map[string]float64),
		ByTier:       make(map[string]float64),
		ByRunnerType: make(map[string]float64),
		ByTask:       make(map[string]float64),
	}

	for _, u := range t.usages {
		s.TotalCostUSD += u.CostUSD
		s.TotalTokensIn += int64(u.TokensIn)
		s.TotalTokensOut += int64(u.TokensOut)
		s.TotalCalls++

		// By model
		mc, ok := s.ByModel[u.Model]
		if !ok {
			mc = &ModelCost{}
			s.ByModel[u.Model] = mc
		}
		mc.Calls++
		mc.TokensIn += int64(u.TokensIn)
		mc.TokensOut += int64(u.TokensOut)
		mc.CostUSD += u.CostUSD
		mc.AvgLatencyMs = (mc.AvgLatencyMs*(mc.Calls-1) + u.Latency.Milliseconds()) / mc.Calls

		// By user
		if u.UserID != "" {
			s.ByUser[u.UserID] += u.CostUSD
		}

		// By day
		day := u.Timestamp.Format("2006-01-02")
		s.ByDay[day] += u.CostUSD

		// By channel
		if u.Channel != "" {
			s.ByChannel[u.Channel] += u.CostUSD
		}

		// By tier
		if u.Tier != "" {
			s.ByTier[u.Tier] += u.CostUSD
		}

		// By runner type
		if u.RunnerType != "" {
			s.ByRunnerType[u.RunnerType] += u.CostUSD
		}

		// By task
		if u.TaskID != "" {
			s.ByTask[u.TaskID] += u.CostUSD
		}
	}

	return s
}

// TodayCost returns today's total spending.
func (t *Tracker) TodayCost() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.dailyCostLocked(time.Now())
}

// GetAlerts returns recent alerts.
func (t *Tracker) GetAlerts() []Alert {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]Alert, len(t.alerts))
	copy(out, t.alerts)
	return out
}

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

func (t *Tracker) computeCost(model string, tokensIn, tokensOut int) float64 {
	p, ok := t.pricing[model]
	if !ok {
		// Try prefix match (e.g., "gpt-4o-2024-08-06" matches "gpt-4o")
		for name, price := range t.pricing {
			if len(model) >= len(name) && model[:len(name)] == name {
				p = price
				ok = true
				break
			}
		}
	}
	if !ok {
		// Unknown model: estimate conservatively
		p = Pricing{InputPer1M: 1.0, OutputPer1M: 3.0}
	}
	return (float64(tokensIn)*p.InputPer1M + float64(tokensOut)*p.OutputPer1M) / 1_000_000
}

func (t *Tracker) dailyCostLocked(now time.Time) float64 {
	today := now.Format("2006-01-02")
	var cost float64
	for _, u := range t.usages {
		if u.Timestamp.Format("2006-01-02") == today {
			cost += u.CostUSD
		}
	}
	return cost
}

func (t *Tracker) checkBudget(u Usage) *Alert {
	// Per-call check
	if t.budget.PerCallLimitUSD > 0 && u.CostUSD > t.budget.PerCallLimitUSD {
		return &Alert{
			Type:      "per_call",
			Message:   fmt.Sprintf("Single call cost $%.4f exceeds limit $%.4f", u.CostUSD, t.budget.PerCallLimitUSD),
			CostUSD:   u.CostUSD,
			Timestamp: u.Timestamp,
		}
	}

	// Daily check
	if t.budget.DailyLimitUSD > 0 {
		daily := t.dailyCostLocked(u.Timestamp)
		if daily > t.budget.DailyLimitUSD*0.8 {
			return &Alert{
				Type:      "daily_limit",
				Message:   fmt.Sprintf("Daily spending $%.4f approaching limit $%.4f (%.0f%%)", daily, t.budget.DailyLimitUSD, daily/t.budget.DailyLimitUSD*100),
				CostUSD:   daily,
				Timestamp: u.Timestamp,
			}
		}
	}

	return nil
}

// ── New query methods for Task Cost Telemetry ──

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

// ProviderCostSummary aggregates cost by provider.
type ProviderCostSummary struct {
	ProviderID string  `json:"provider_id"`
	CostUSD    float64 `json:"cost_usd"`
	Calls      int64   `json:"calls"`
	TokensIn   int64   `json:"tokens_in"`
	TokensOut  int64   `json:"tokens_out"`
}

// ── Persistence (append-only JSONL) ──

func (t *Tracker) appendToDisk(u Usage) {
	if t.persistPath == "" {
		return
	}
	f, err := os.OpenFile(t.persistPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		slog.Warn("cost telemetry: append failed", "err", err)
		return
	}
	defer f.Close()
	_ = json.NewEncoder(f).Encode(u)
}

func (t *Tracker) loadFromDisk() {
	if t.persistPath == "" {
		return
	}
	f, err := os.Open(t.persistPath)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("cost telemetry: load failed", "err", err)
		}
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
	var count int
	for scanner.Scan() {
		var u Usage
		if err := json.Unmarshal(scanner.Bytes(), &u); err != nil {
			continue
		}
		t.usages = append(t.usages, u)
		count++
	}
	if count > 0 {
		slog.Info("cost telemetry: loaded history", "records", count)
	}
	t.evictOldRecords()
}

// evictOldRecords drops the oldest in-memory records when limit is exceeded.
// Note: disk records are NOT truncated (append-only JSONL). Only memory is managed.
func (t *Tracker) evictOldRecords() {
	max := t.MaxInMemoryRecords
	if max <= 0 {
		max = 100000 // default: retain up to 100k records in memory
	}
	if len(t.usages) > max {
		drop := len(t.usages) - max
		// Release references for GC
		for i := 0; i < drop; i++ {
			t.usages[i] = Usage{}
		}
		t.usages = t.usages[drop:]
	}
}
