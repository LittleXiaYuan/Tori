package router

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"yunque-agent/internal/agentcore/models"
)

// Tier represents the complexity tier of a query.
type Tier int

const (
	TierFast   Tier = iota // Simple greetings, translations, factual lookups
	TierSmart              // Analysis, summarization, multi-step reasoning
	TierExpert             // Complex coding, long-form writing, research
)

func (t Tier) String() string {
	switch t {
	case TierFast:
		return "fast"
	case TierSmart:
		return "smart"
	case TierExpert:
		return "expert"
	}
	return "unknown"
}

// ModelSlot binds a tier to a model ID.
type ModelSlot struct {
	Tier    Tier   `json:"tier"`
	ModelID string `json:"model_id"`
}

// Stats tracks routing decisions for observability.
type Stats struct {
	mu       sync.Mutex
	Counts   map[string]int64         `json:"counts"`  // tier -> count
	Latency  map[string]time.Duration `json:"latency"` // model_id -> avg latency
	Fallback int64                    `json:"fallback"`
}

// Router intelligently routes queries to the best model based on complexity.
type Router struct {
	registry *models.Registry
	slots    map[Tier]string // tier -> model ID
	mu       sync.RWMutex
	stats    Stats
}

// New creates a smart router with the model registry.
func New(registry *models.Registry) *Router {
	return &Router{
		registry: registry,
		slots:    make(map[Tier]string),
		stats: Stats{
			Counts:  make(map[string]int64),
			Latency: make(map[string]time.Duration),
		},
	}
}

// SetSlot assigns a model to a complexity tier.
func (r *Router) SetSlot(tier Tier, modelID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.slots[tier] = modelID
}

// GetSlots returns the current tier-to-model mapping.
func (r *Router) GetSlots() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]string, len(r.slots))
	for t, id := range r.slots {
		out[t.String()] = id
	}
	return out
}

// Route selects the best model for a given query.
// Returns the model and the detected tier.
func (r *Router) Route(ctx context.Context, query string, hasImages bool) (*models.Model, Tier) {
	tier := r.classify(query, hasImages)

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Try the assigned model for this tier
	if modelID, ok := r.slots[tier]; ok {
		if m, ok := r.registry.Get(modelID); ok {
			r.recordRoute(tier)
			slog.Debug("router", "tier", tier.String(), "model", m.ModelID, "query_len", len(query))
			return m, tier
		}
	}

	// Fallback: try lower tiers, then primary
	for fallback := tier - 1; fallback >= TierFast; fallback-- {
		if modelID, ok := r.slots[fallback]; ok {
			if m, ok := r.registry.Get(modelID); ok {
				r.stats.mu.Lock()
				r.stats.Fallback++
				r.stats.mu.Unlock()
				r.recordRoute(fallback)
				return m, fallback
			}
		}
	}

	// Last resort: primary model
	if m, ok := r.registry.Primary(); ok {
		r.stats.mu.Lock()
		r.stats.Fallback++
		r.stats.mu.Unlock()
		return m, TierSmart
	}

	return nil, tier
}

// RecordLatency records the response latency for a model (for future adaptive routing).
func (r *Router) RecordLatency(modelID string, d time.Duration) {
	r.stats.mu.Lock()
	defer r.stats.mu.Unlock()
	existing, ok := r.stats.Latency[modelID]
	if !ok {
		r.stats.Latency[modelID] = d
	} else {
		// Exponential moving average
		r.stats.Latency[modelID] = (existing*7 + d) / 8
	}
}

// GetStats returns routing statistics.
func (r *Router) GetStats() map[string]any {
	r.stats.mu.Lock()
	defer r.stats.mu.Unlock()
	counts := make(map[string]int64, len(r.stats.Counts))
	for k, v := range r.stats.Counts {
		counts[k] = v
	}
	latency := make(map[string]string, len(r.stats.Latency))
	for k, v := range r.stats.Latency {
		latency[k] = v.Round(time.Millisecond).String()
	}
	return map[string]any{
		"counts":    counts,
		"latency":   latency,
		"fallbacks": r.stats.Fallback,
	}
}

func (r *Router) recordRoute(tier Tier) {
	r.stats.mu.Lock()
	r.stats.Counts[tier.String()]++
	r.stats.mu.Unlock()
}

// classify determines the complexity tier of a query using heuristics.
// No LLM call needed - this is instant and free.
func (r *Router) classify(query string, hasImages bool) Tier {
	// Images always need a capable model
	if hasImages {
		return TierExpert
	}

	charCount := utf8.RuneCountInString(query)
	lower := strings.ToLower(query)
	words := strings.Fields(query)
	wordCount := len(words)

	// === TierExpert signals ===
	expertSignals := 0

	// Long queries are usually complex
	if charCount > 500 {
		expertSignals += 2
	} else if charCount > 200 {
		expertSignals++
	}

	// Code-related keywords
	codeKeywords := []string{
		"代码", "code", "debug", "实现", "implement", "重构", "refactor",
		"算法", "algorithm", "架构", "architecture", "设计模式", "design pattern",
		"sql", "api", "function", "class", "struct", "interface",
		"bug", "error", "exception", "stack trace", "报错",
	}
	for _, kw := range codeKeywords {
		if strings.Contains(lower, kw) {
			expertSignals += 2
			break
		}
	}

	// Analysis/reasoning keywords
	analysisKeywords := []string{
		"分析", "analyze", "比较", "compare", "评估", "evaluate",
		"为什么", "why", "原因", "reason", "解释", "explain",
		"策略", "strategy", "方案", "plan", "优化", "optimize",
		"写一篇", "write an essay", "论文", "report", "总结", "summarize",
	}
	for _, kw := range analysisKeywords {
		if strings.Contains(lower, kw) {
			expertSignals++
			break
		}
	}

	// Multi-step indicators
	multiStepKeywords := []string{
		"步骤", "step", "首先", "first", "然后", "then",
		"第一", "第二", "第三", "1.", "2.", "3.",
		"并且", "同时", "另外", "additionally",
	}
	stepCount := 0
	for _, kw := range multiStepKeywords {
		if strings.Contains(lower, kw) {
			stepCount++
		}
	}
	if stepCount >= 2 {
		expertSignals++
	}

	if expertSignals >= 2 {
		return TierExpert
	}

	// === TierFast signals ===
	fastSignals := 0

	// Very short queries (only count as fast if truly trivial)
	if wordCount <= 2 && charCount < 10 {
		fastSignals += 2
	} else if charCount < 15 && wordCount <= 3 {
		fastSignals++
	}

	// Simple greetings
	greetings := []string{
		"你好", "hello", "hi", "hey", "嗨", "早上好", "晚上好",
		"谢谢", "thanks", "thank you", "再见", "bye", "ok", "好的",
	}
	for _, g := range greetings {
		if strings.Contains(lower, g) && charCount < 30 {
			fastSignals += 2
			break
		}
	}

	// Simple lookup patterns
	simplePatterns := []string{
		"翻译", "translate", "什么意思", "what is", "what's",
		"几点", "what time", "天气", "weather", "定义", "define",
		"多少", "how much", "how many",
	}
	for _, p := range simplePatterns {
		if strings.Contains(lower, p) && charCount < 100 {
			fastSignals++
			break
		}
	}

	if fastSignals >= 2 {
		return TierFast
	}

	// Default to smart tier
	if expertSignals >= 1 {
		return TierSmart
	}

	return TierSmart
}
