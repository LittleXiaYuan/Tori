package task

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// GapAnalyzer — analyzes task step failures to identify capability gaps
//
// When a step fails, the analyzer classifies the failure into:
//   - SkillMissing: the required skill doesn't exist in the registry
//   - ParamError:   the skill exists but received wrong parameters
//   - EnvError:     external dependency (API, service) is unavailable
//   - Unknown:      unclassifiable failure
//
// Gap records are accumulated and can be queried via API to drive
// the Capability Growth Loop.
// ──────────────────────────────────────────────

// GapType classifies why a step failed.
type GapType string

const (
	GapSkillMissing GapType = "skill_missing" // skill doesn't exist
	GapParamError   GapType = "param_error"   // skill exists, wrong args
	GapEnvError     GapType = "env_error"     // external dependency failure
	GapUnknown      GapType = "unknown"       // unclassifiable
)

// GapRecord is a single capability gap observation.
type GapRecord struct {
	ID          string    `json:"id"`
	TaskID      string    `json:"task_id"`
	StepID      int       `json:"step_id"`
	StepAction  string    `json:"step_action"`
	SkillName   string    `json:"skill_name,omitempty"`
	ErrorMsg    string    `json:"error_msg"`
	GapType     GapType   `json:"gap_type"`
	Suggestion  string    `json:"suggestion,omitempty"` // LLM-generated fix suggestion
	Resolved    bool      `json:"resolved"`
	OccurredAt  time.Time `json:"occurred_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}

// GapAnalyzer detects and records capability gaps from task failures.
type GapAnalyzer struct {
	mu      sync.RWMutex
	records []GapRecord
	llmCall LLMFunc // optional: LLM for deeper analysis
	counter int
}

// NewGapAnalyzer creates a gap analyzer.
func NewGapAnalyzer(llmCall LLMFunc) *GapAnalyzer {
	return &GapAnalyzer{
		llmCall: llmCall,
	}
}

// Analyze examines a failed step and produces a GapRecord.
// This is the core classification logic — runs synchronously.
func (g *GapAnalyzer) Analyze(ctx context.Context, t *Task, step *Step) *GapRecord {
	g.mu.Lock()
	g.counter++
	id := fmt.Sprintf("gap-%d", g.counter)
	g.mu.Unlock()

	rec := &GapRecord{
		ID:         id,
		TaskID:     t.ID,
		StepID:     step.ID,
		StepAction: step.Action,
		SkillName:  step.SkillName,
		ErrorMsg:   step.Error,
		OccurredAt: time.Now(),
	}

	// Rule-based classification
	rec.GapType = g.classify(step)

	// Generate suggestion with LLM if available
	if g.llmCall != nil {
		suggestion := g.suggest(ctx, rec)
		if suggestion != "" {
			rec.Suggestion = suggestion
		}
	}

	// Store record
	g.mu.Lock()
	g.records = append(g.records, *rec)
	g.mu.Unlock()

	slog.Info("gap: detected", "id", id, "type", rec.GapType, "skill", rec.SkillName, "error", rec.ErrorMsg)
	return rec
}

// classify uses rule-based heuristics to determine the gap type.
func (g *GapAnalyzer) classify(step *Step) GapType {
	errLower := strings.ToLower(step.Error)

	// 1. Skill not found → missing
	if strings.Contains(errLower, "not found") && strings.Contains(errLower, "skill") {
		return GapSkillMissing
	}
	if strings.Contains(errLower, "not registered") {
		return GapSkillMissing
	}
	if strings.Contains(errLower, "not installed") {
		return GapSkillMissing
	}

	// 2. Parameter/argument errors
	paramKeywords := []string{
		"required", "invalid", "missing", "parameter",
		"argument", "type mismatch", "is required",
		"bad request", "validation",
	}
	for _, kw := range paramKeywords {
		if strings.Contains(errLower, kw) {
			return GapParamError
		}
	}

	// 3. Environment/external errors
	envKeywords := []string{
		"connection", "timeout", "dns", "network",
		"503", "502", "500", "rate limit", "429",
		"api key", "unauthorized", "forbidden",
		"certificate", "tls", "ssl",
	}
	for _, kw := range envKeywords {
		if strings.Contains(errLower, kw) {
			return GapEnvError
		}
	}

	// 4. If skill_name is set but not found in error, and error mentions something else
	if step.SkillName != "" && strings.Contains(errLower, "no such") {
		return GapSkillMissing
	}

	return GapUnknown
}

// suggest uses LLM to generate a fix suggestion for the gap.
func (g *GapAnalyzer) suggest(ctx context.Context, rec *GapRecord) string {
	prompt := fmt.Sprintf(`任务步骤失败，请分析原因并给出简短修复建议（1-2句话）。

步骤描述：%s
技能名称：%s
错误信息：%s
缺口类型：%s

要求：
- 如果是技能缺失，说明需要什么样的技能
- 如果是参数错误，说明正确的参数格式
- 如果是环境问题，说明需要什么配置
- 简明扼要，不超过100字`, rec.StepAction, rec.SkillName, rec.ErrorMsg, rec.GapType)

	resp, err := g.llmCall(ctx, "你是一个错误分析助手，只返回简短的修复建议。", prompt)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(resp)
}

// Records returns all gap records, optionally filtered.
func (g *GapAnalyzer) Records(gapType GapType, unresolved bool) []GapRecord {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var out []GapRecord
	for _, r := range g.records {
		if gapType != "" && r.GapType != gapType {
			continue
		}
		if unresolved && r.Resolved {
			continue
		}
		out = append(out, r)
	}
	return out
}

// Stats returns aggregated gap statistics.
func (g *GapAnalyzer) Stats() map[string]int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	stats := map[string]int{
		"total":         len(g.records),
		"unresolved":    0,
		"skill_missing": 0,
		"param_error":   0,
		"env_error":     0,
		"unknown":       0,
	}
	for _, r := range g.records {
		if !r.Resolved {
			stats["unresolved"]++
		}
		stats[string(r.GapType)]++
	}
	return stats
}

// Resolve marks a gap as resolved.
func (g *GapAnalyzer) Resolve(gapID string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	for i := range g.records {
		if g.records[i].ID == gapID {
			g.records[i].Resolved = true
			now := time.Now()
			g.records[i].ResolvedAt = &now
			return true
		}
	}
	return false
}

// TopGaps returns the most frequent gap types as a JSON summary.
func (g *GapAnalyzer) TopGaps(limit int) string {
	recs := g.Records("", true) // unresolved only

	// Count by skill name
	skillCount := make(map[string]int)
	for _, r := range recs {
		key := r.SkillName
		if key == "" {
			key = "(llm-only)"
		}
		skillCount[key]++
	}

	type entry struct {
		Skill string `json:"skill"`
		Count int    `json:"count"`
		Type  string `json:"type"`
	}
	var entries []entry
	for _, r := range recs {
		key := r.SkillName
		if key == "" {
			key = "(llm-only)"
		}
		if cnt, ok := skillCount[key]; ok {
			entries = append(entries, entry{Skill: key, Count: cnt, Type: string(r.GapType)})
			delete(skillCount, key) // deduplicate
		}
	}

	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}

	b, _ := json.Marshal(entries)
	return string(b)
}
