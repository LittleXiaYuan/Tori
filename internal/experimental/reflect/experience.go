package reflect

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"yunque-agent/internal/agentcore/llm"
	iledger "yunque-agent/internal/ledger"
)

// ──────────────────────────────────────────────
// Experience Store — persistent structured experience accumulation
//
// Implements the 4th core loop (Reflection Loop):
//   success/failure → extract experience → update strategy → sink
// ──────────────────────────────────────────────

// Experience is a structured lesson learned from task execution or interaction.
type Experience struct {
	ID        string    `json:"id"`
	Source    string    `json:"source"`    // "task", "interaction", "reverie"
	SourceID  string    `json:"source_id"` // task ID or session ID
	Category  string    `json:"category"`  // "skill_usage", "error_pattern", "strategy", "domain", "preference"
	Outcome   string    `json:"outcome"`   // "success", "failure", "partial"
	Lesson    string    `json:"lesson"`    // the insight extracted
	Context   string    `json:"context"`   // what was being done
	Tags      []string  `json:"tags,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ExperienceStats summarizes the experience store.
type ExperienceStats struct {
	Total      int            `json:"total"`
	BySource   map[string]int `json:"by_source"`
	ByCategory map[string]int `json:"by_category"`
	ByOutcome  map[string]int `json:"by_outcome"`
	Recent     int            `json:"recent_7d"` // last 7 days
}

// ExperienceStore provides persistent structured experience storage.
type ExperienceStore struct {
	mu   sync.RWMutex
	data []Experience
	path string
	kvs  *iledger.KVConfigStore
}

// NewExperienceStore creates a store that persists to the given file path.
func NewExperienceStore(path string) *ExperienceStore {
	s := &ExperienceStore{path: path}
	s.load()
	return s
}

// SetKVStore enables Ledger KV-backed persistence, replacing file I/O.
func (s *ExperienceStore) SetKVStore(kvs *iledger.KVConfigStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kvs = kvs
	s.loadFromKV()
}

// Add records a new experience.
func (s *ExperienceStore) Add(exp Experience) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if exp.ID == "" {
		exp.ID = fmt.Sprintf("exp-%d", time.Now().UnixMilli())
	}
	if exp.CreatedAt.IsZero() {
		exp.CreatedAt = time.Now()
	}
	s.data = append(s.data, exp)
	// Keep max 500 experiences (FIFO)
	if len(s.data) > 500 {
		s.data = s.data[len(s.data)-500:]
	}
	s.save()
}

// All returns all experiences (newest first).
func (s *ExperienceStore) All() []Experience {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Experience, len(s.data))
	for i, e := range s.data {
		out[len(s.data)-1-i] = e
	}
	return out
}

// Search returns experiences matching the query (substring match on lesson/context/tags).
func (s *ExperienceStore) Search(query string, limit int) []Experience {
	s.mu.RLock()
	defer s.mu.RUnlock()
	q := strings.ToLower(query)
	var results []Experience
	for i := len(s.data) - 1; i >= 0 && len(results) < limit; i-- {
		e := s.data[i]
		if strings.Contains(strings.ToLower(e.Lesson), q) ||
			strings.Contains(strings.ToLower(e.Context), q) ||
			containsAny(e.Tags, q) {
			results = append(results, e)
		}
	}
	return results
}

// Stats returns aggregate statistics.
func (s *ExperienceStore) Stats() ExperienceStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := ExperienceStats{
		Total:      len(s.data),
		BySource:   make(map[string]int),
		ByCategory: make(map[string]int),
		ByOutcome:  make(map[string]int),
	}
	week := time.Now().Add(-7 * 24 * time.Hour)
	for _, e := range s.data {
		st.BySource[e.Source]++
		st.ByCategory[e.Category]++
		st.ByOutcome[e.Outcome]++
		if e.CreatedAt.After(week) {
			st.Recent++
		}
	}
	return st
}

// CompileStrategies aggregates recent experiences into actionable strategy hints.
// Output is concise and directive — each line is a clear do/don't instruction.
// Only includes experiences with meaningful lessons (>20 chars), deduped by content.
func (s *ExperienceStore) CompileStrategies(limit int) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.data) == 0 {
		return ""
	}

	experiences := make([]Experience, 0, len(s.data))
	for i := len(s.data) - 1; i >= 0; i-- {
		experiences = append(experiences, s.data[i])
	}
	return CompileStrategiesFrom(experiences, limit)
}

// CompileStrategiesForQuery aggregates only experiences relevant to the query.
// Falls back to the normal recent strategy context when no query is supplied.
func (s *ExperienceStore) CompileStrategiesForQuery(query string, limit int) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return s.CompileStrategies(limit)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.data) == 0 {
		return ""
	}

	q := strings.ToLower(query)
	experiences := make([]Experience, 0, len(s.data))
	for i := len(s.data) - 1; i >= 0; i-- {
		e := s.data[i]
		if MatchesQuery(e, q) {
			experiences = append(experiences, e)
		}
	}
	return CompileStrategiesFrom(experiences, limit)
}

// CompileStrategiesFrom aggregates a newest-first experience list into actionable strategy hints.
func CompileStrategiesFrom(experiences []Experience, limit int) string {
	var avoids, improvements, uses []string
	seen := make(map[string]bool)

	for _, e := range experiences {
		if limit > 0 && (len(avoids)+len(improvements)+len(uses)) >= limit {
			break
		}
		lesson := strings.TrimSpace(e.Lesson)
		if len(lesson) < 20 {
			continue // skip trivially short lessons
		}

		// Dedup by normalized lesson content (first 60 chars)
		normKey := strings.ToLower(lesson)
		if len([]rune(normKey)) > 60 {
			normKey = string([]rune(normKey)[:60])
		}
		if seen[normKey] {
			continue
		}
		seen[normKey] = true

		switch e.Outcome {
		case "failure":
			avoids = append(avoids, fmt.Sprintf("- 避免: %s", lesson))
		case "success":
			uses = append(uses, fmt.Sprintf("- 推荐: %s", lesson))
		case "partial":
			improvements = append(improvements, fmt.Sprintf("- 改进: %s", lesson))
		}
	}

	if len(avoids) == 0 && len(improvements) == 0 && len(uses) == 0 {
		return ""
	}

	var parts []string
	// Show "do"s before "don't"s — positive guidance first
	if len(uses) > 0 {
		parts = append(parts, strings.Join(uses, "\n"))
	}
	if len(improvements) > 0 {
		parts = append(parts, strings.Join(improvements, "\n"))
	}
	if len(avoids) > 0 {
		parts = append(parts, strings.Join(avoids, "\n"))
	}
	return strings.Join(parts, "\n")
}

func (s *ExperienceStore) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var exps []Experience
	if json.Unmarshal(data, &exps) == nil {
		s.data = exps
	}
}

func (s *ExperienceStore) loadFromKV() {
	if s.kvs == nil {
		return
	}
	var exps []Experience
	found, err := s.kvs.Get(context.Background(), "data", &exps)
	if err != nil {
		slog.Warn("experience: kv load failed", "err", err)
		return
	}
	if found && len(exps) > 0 {
		s.data = exps
		slog.Info("experience: loaded from Ledger KV", "count", len(exps))
	}
}

func (s *ExperienceStore) save() {
	if s.kvs != nil {
		if err := s.kvs.Put(context.Background(), "data", s.data); err != nil {
			slog.Warn("experience: kv save failed, falling back to file", "err", err)
		} else {
			return
		}
	}
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(s.path, data, 0644)
}

func containsAny(tags []string, q string) bool {
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	return false
}

// MatchesQuery reports whether an experience is relevant to a natural-language query.
func MatchesQuery(e Experience, query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return true
	}
	haystack := strings.ToLower(e.Lesson + "\n" + e.Context + "\n" + strings.Join(e.Tags, "\n"))
	if strings.Contains(haystack, q) {
		return true
	}
	for _, token := range strings.Fields(q) {
		token = strings.Trim(token, ".,，。:：;；!?！？()（）[]【】{}<>\"'`")
		if len([]rune(token)) < 2 {
			continue
		}
		if strings.Contains(haystack, token) {
			return true
		}
	}
	return false
}

// ──────────────────────────────────────────────
// TaskReflector — post-task reflection
// ──────────────────────────────────────────────

// TaskTrace is the summary of a completed task for reflection.
type TaskTrace struct {
	TaskID      string
	Title       string
	Description string
	Outcome     string // "completed", "failed", "cancelled"
	Steps       []StepTrace
	Duration    time.Duration
}

// StepTrace is the summary of a single step.
type StepTrace struct {
	Action    string
	SkillName string
	Status    string // "done", "failed", "skipped"
	Error     string
	Retries   int
	GapType   string
}

// TaskReflector performs post-task reflection, extracting experiences from task execution.
type TaskReflector struct {
	llm   *llm.Client
	store *ExperienceStore
}

// NewTaskReflector creates a reflector that stores experiences.
func NewTaskReflector(llmClient *llm.Client, store *ExperienceStore) *TaskReflector {
	return &TaskReflector{llm: llmClient, store: store}
}

// AfterTask reflects on a completed/failed task and extracts experiences.
func (tr *TaskReflector) AfterTask(ctx context.Context, trace TaskTrace) {
	// 1. Rule-based quick experiences (no LLM needed)
	tr.extractRuleBasedExperiences(trace)

	// 2. LLM-powered deep reflection (async)
	if tr.llm != nil {
		go tr.extractDeepExperiences(ctx, trace)
	}
}

func (tr *TaskReflector) extractRuleBasedExperiences(trace TaskTrace) {
	// Gap patterns
	for _, s := range trace.Steps {
		if s.GapType != "" && !strings.Contains(s.GapType, "auto_resolved") {
			tr.store.Add(Experience{
				Source:   "task",
				SourceID: trace.TaskID,
				Category: "error_pattern",
				Outcome:  "failure",
				Lesson:   fmt.Sprintf("步骤 '%s' 遇到能力缺口: %s", s.Action, s.GapType),
				Context:  trace.Title,
				Tags:     []string{"gap", s.GapType, s.SkillName},
			})
		}
		if s.GapType != "" && strings.Contains(s.GapType, "auto_resolved") {
			tr.store.Add(Experience{
				Source:   "task",
				SourceID: trace.TaskID,
				Category: "strategy",
				Outcome:  "success",
				Lesson:   fmt.Sprintf("自动生成技能解决了 '%s' 的能力缺口 (%s)", s.Action, s.GapType),
				Context:  trace.Title,
				Tags:     []string{"growth", "auto_generated", s.SkillName},
			})
		}
		if s.Retries > 0 && s.Status == "done" {
			tr.store.Add(Experience{
				Source:   "task",
				SourceID: trace.TaskID,
				Category: "strategy",
				Outcome:  "success",
				Lesson:   fmt.Sprintf("步骤 '%s' 在重试 %d 次后成功", s.Action, s.Retries),
				Context:  trace.Title,
				Tags:     []string{"retry", s.SkillName},
			})
		}
	}

	// Task-level outcome
	if trace.Outcome == "completed" {
		skills := collectSkills(trace.Steps)
		if len(skills) > 0 {
			tr.store.Add(Experience{
				Source:   "task",
				SourceID: trace.TaskID,
				Category: "skill_usage",
				Outcome:  "success",
				Lesson:   fmt.Sprintf("任务 '%s' 成功，使用技能: %s", trace.Title, strings.Join(skills, ", ")),
				Context:  trace.Description,
				Tags:     skills,
			})
		}
	}
}

func (tr *TaskReflector) extractDeepExperiences(ctx context.Context, trace TaskTrace) {
	// Build trace summary for LLM
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("任务: %s\n描述: %s\n结果: %s\n耗时: %v\n\n步骤:\n",
		trace.Title, trace.Description, trace.Outcome, trace.Duration.Round(time.Second)))

	for _, s := range trace.Steps {
		sb.WriteString(fmt.Sprintf("- %s [%s] %s", s.Action, s.Status, s.SkillName))
		if s.Error != "" {
			sb.WriteString(fmt.Sprintf(" 错误: %s", s.Error))
		}
		if s.Retries > 0 {
			sb.WriteString(fmt.Sprintf(" (重试%d次)", s.Retries))
		}
		if s.GapType != "" {
			sb.WriteString(fmt.Sprintf(" [gap:%s]", s.GapType))
		}
		sb.WriteString("\n")
	}

	prompt := sb.String() + `
请从这次任务执行中提取可学习的经验教训。输出JSON数组：
[{"category":"...", "outcome":"success/failure", "lesson":"...", "tags":["..."]}]

category取值: skill_usage, error_pattern, strategy, domain
只输出JSON数组。`

	reply, err := tr.llm.Chat(ctx, []llm.Message{
		{Role: "system", Content: "你是任务反思引擎。从任务执行痕迹中提取结构化经验。只输出JSON数组。"},
		{Role: "user", Content: prompt},
	}, 0.1)
	if err != nil {
		slog.Warn("task reflector: LLM failed", "err", err)
		return
	}

	var items []struct {
		Category string   `json:"category"`
		Outcome  string   `json:"outcome"`
		Lesson   string   `json:"lesson"`
		Tags     []string `json:"tags"`
	}
	raw := extractJSONArray(reply)
	if json.Unmarshal([]byte(raw), &items) != nil {
		return
	}

	for _, item := range items {
		tr.store.Add(Experience{
			Source:   "task",
			SourceID: trace.TaskID,
			Category: item.Category,
			Outcome:  item.Outcome,
			Lesson:   item.Lesson,
			Context:  trace.Title,
			Tags:     item.Tags,
		})
	}
}

func collectSkills(steps []StepTrace) []string {
	seen := make(map[string]bool)
	var skills []string
	for _, s := range steps {
		if s.SkillName != "" && !seen[s.SkillName] {
			seen[s.SkillName] = true
			skills = append(skills, s.SkillName)
		}
	}
	return skills
}

func extractJSONArray(s string) string {
	start := -1
	for i, c := range s {
		if c == '[' {
			start = i
			break
		}
	}
	if start < 0 {
		return "[]"
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return "[]"
}
