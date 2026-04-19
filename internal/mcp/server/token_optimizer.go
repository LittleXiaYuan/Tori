package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// TokenOptimizer implements strategies to reduce MCP token consumption:
//   - Tool schema caching: only send full schemas once per session
//   - Context summarization: trim task descriptions to essential info
//   - Local-first routing: flag tasks that can run locally without MCP
//   - Batch coalescing: merge small steps into single calls
type TokenOptimizer struct {
	mu           sync.RWMutex
	schemaCache  map[string]*cachedSchema // session_id -> schema hash
	summaryCache map[string]string        // task_id -> summarized description
}

type cachedSchema struct {
	hash      string
	expiresAt time.Time
}

// NewTokenOptimizer creates a new optimizer.
func NewTokenOptimizer() *TokenOptimizer {
	return &TokenOptimizer{
		schemaCache:  make(map[string]*cachedSchema),
		summaryCache: make(map[string]string),
	}
}

// ShouldSendFullSchema checks if the tool schema has changed since
// the last time this session received it. Returns true on first call
// or when the schema has changed.
func (o *TokenOptimizer) ShouldSendFullSchema(sessionID string, tools []ToolDef) bool {
	hash := hashTools(tools)

	o.mu.Lock()
	defer o.mu.Unlock()

	cached, ok := o.schemaCache[sessionID]
	if !ok || time.Now().After(cached.expiresAt) || cached.hash != hash {
		o.schemaCache[sessionID] = &cachedSchema{
			hash:      hash,
			expiresAt: time.Now().Add(10 * time.Minute),
		}
		return true
	}
	return false
}

// CompactToolList returns a minimal version of the tool list (names + short
// descriptions only) for sessions that already have the full schema cached.
func CompactToolList(tools []ToolDef) []map[string]any {
	compact := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		compact = append(compact, map[string]any{
			"name":        t.Name,
			"description": truncateDesc(t.Description, 80),
		})
	}
	return compact
}

// SummarizeContext produces a condensed version of a task description,
// keeping only the first N characters plus key structural elements.
func (o *TokenOptimizer) SummarizeContext(taskID, description string, maxChars int) string {
	if maxChars <= 0 {
		maxChars = 500
	}

	o.mu.RLock()
	if cached, ok := o.summaryCache[taskID]; ok {
		o.mu.RUnlock()
		return cached
	}
	o.mu.RUnlock()

	summary := summarize(description, maxChars)

	o.mu.Lock()
	o.summaryCache[taskID] = summary
	o.mu.Unlock()

	return summary
}

// InvalidateSummary removes a cached summary when the task is updated.
func (o *TokenOptimizer) InvalidateSummary(taskID string) {
	o.mu.Lock()
	delete(o.summaryCache, taskID)
	o.mu.Unlock()
}

// IsLocalTask determines if a task can be handled locally without
// dispatching to an external worker (simple read/search/file ops).
func IsLocalTask(skillName string, tags []string) bool {
	localSkills := map[string]bool{
		"file_read":    true,
		"file_write":   true,
		"web_fetch":    true,
		"web_search":   true,
		"shell_exec":   true,
		"list_files":   true,
		"search_files": true,
	}
	if localSkills[skillName] {
		return true
	}
	for _, tag := range tags {
		if tag == "local" || tag == "simple" {
			return true
		}
	}
	return false
}

// CoalesceSteps merges adjacent small steps with the same skill into
// a single batch description to reduce round-trips.
type StepInfo struct {
	Action    string
	SkillName string
}

func CoalesceSteps(steps []StepInfo) []StepInfo {
	if len(steps) <= 1 {
		return steps
	}

	var coalesced []StepInfo
	i := 0
	for i < len(steps) {
		current := steps[i]
		// Look ahead for same-skill adjacent steps
		j := i + 1
		for j < len(steps) && steps[j].SkillName == current.SkillName && steps[j].SkillName != "" {
			j++
		}
		if j > i+1 {
			// Merge steps i..j-1
			var actions []string
			for k := i; k < j; k++ {
				actions = append(actions, steps[k].Action)
			}
			coalesced = append(coalesced, StepInfo{
				Action:    strings.Join(actions, "; "),
				SkillName: current.SkillName,
			})
			slog.Debug("token optimizer: coalesced steps", "skill", current.SkillName, "count", j-i)
		} else {
			coalesced = append(coalesced, current)
		}
		i = j
	}
	return coalesced
}

// CleanExpiredSessions removes stale schema cache entries.
func (o *TokenOptimizer) CleanExpiredSessions() {
	o.mu.Lock()
	defer o.mu.Unlock()

	now := time.Now()
	for id, cached := range o.schemaCache {
		if now.After(cached.expiresAt) {
			delete(o.schemaCache, id)
		}
	}
}

func hashTools(tools []ToolDef) string {
	h := sha256.New()
	for _, t := range tools {
		b, _ := json.Marshal(map[string]any{
			"name":   t.Name,
			"schema": t.InputSchema,
		})
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func truncateDesc(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen]) + "..."
}

func summarize(desc string, maxChars int) string {
	desc = strings.TrimSpace(desc)
	r := []rune(desc)
	if len(r) <= maxChars {
		return desc
	}

	// Try to break at sentence boundary
	candidate := string(r[:maxChars])
	if idx := strings.LastIndexAny(candidate, ".。!！\n"); idx > maxChars/2 {
		return candidate[:idx+1]
	}
	return candidate + "..."
}
