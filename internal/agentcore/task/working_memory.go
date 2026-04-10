package task

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"yunque-agent/pkg/safego"
)

// ──────────────────────────────────────────────
// WorkingMemory — per-task short summary for context compression
//
// Each task maintains a compact working memory that tracks:
// - Goal (from task description)
// - Completed steps (condensed)
// - Blockers / failures
// - Confirmed information from user
// - Pending confirmations
// - Current artifacts
// - Suggested next action
//
// This replaces dumping the entire task history into the prompt,
// significantly reducing token consumption for multi-step tasks.
// ──────────────────────────────────────────────

// WorkingMemory holds the compact context for an active task.
type WorkingMemory struct {
	TaskID        string    `json:"task_id"`
	Goal          string    `json:"goal"`
	CompletedWork []string  `json:"completed_work,omitempty"` // condensed step results
	Blockers      []string  `json:"blockers,omitempty"`       // current blockers
	Confirmed     []string  `json:"confirmed,omitempty"`      // facts confirmed by user
	Pending       []string  `json:"pending,omitempty"`        // awaiting user confirmation
	Artifacts     []string  `json:"artifacts,omitempty"`      // current outputs
	NextAction    string    `json:"next_action,omitempty"`    // suggested next step
	UpdatedAt     time.Time `json:"updated_at"`
	TokenEstimate int       `json:"token_estimate"` // rough token count
}

// Render produces a compact prompt segment for injection into LLM context.
func (wm *WorkingMemory) Render() string {
	if wm == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## 任务工作记忆 [%s]\n", wm.TaskID))
	sb.WriteString(fmt.Sprintf("**目标**: %s\n", wm.Goal))

	if len(wm.CompletedWork) > 0 {
		sb.WriteString("**已完成**:\n")
		for _, w := range wm.CompletedWork {
			sb.WriteString(fmt.Sprintf("- %s\n", w))
		}
	}
	if len(wm.Blockers) > 0 {
		sb.WriteString("**阻塞**:\n")
		for _, b := range wm.Blockers {
			sb.WriteString(fmt.Sprintf("- ⚠ %s\n", b))
		}
	}
	if len(wm.Confirmed) > 0 {
		sb.WriteString("**已确认**:\n")
		for _, c := range wm.Confirmed {
			sb.WriteString(fmt.Sprintf("- ✓ %s\n", c))
		}
	}
	if len(wm.Pending) > 0 {
		sb.WriteString("**待确认**:\n")
		for _, p := range wm.Pending {
			sb.WriteString(fmt.Sprintf("- ? %s\n", p))
		}
	}
	if len(wm.Artifacts) > 0 {
		sb.WriteString("**产物**:\n")
		for _, a := range wm.Artifacts {
			sb.WriteString(fmt.Sprintf("- 📄 %s\n", a))
		}
	}
	if wm.NextAction != "" {
		sb.WriteString(fmt.Sprintf("**下一步**: %s\n", wm.NextAction))
	}
	return sb.String()
}

// estimateTokens returns a rough token estimate for the rendered memory.
func (wm *WorkingMemory) estimateTokens() int {
	r := wm.Render()
	// ~1 token per 1.5 Chinese chars, ~1 token per 4 English chars
	return len([]rune(r)) * 2 / 3
}

// Summarizer abstracts the text compression backend.
// Default: LLM-based. Can be replaced with a local model (e.g., llama.cpp).
type Summarizer interface {
	// Summarize compresses text, keeping key facts and structure.
	Summarize(ctx context.Context, text string) (string, error)
}

// LLMSummarizer compresses working memory using a remote LLM.
type LLMSummarizer struct {
	Call LLMFunc
}

func (s *LLMSummarizer) Summarize(ctx context.Context, text string) (string, error) {
	return s.Call(ctx,
		"你是一个任务工作记忆压缩器。将以下任务上下文压缩为更紧凑的形式，保留关键信息（目标、已完成工作摘要、阻塞点、待确认项、产物列表、下一步）。去除冗余细节。输出必须是同样的段落格式。",
		text,
	)
}

// WorkingMemoryManager manages working memory for active tasks.
type WorkingMemoryManager struct {
	mu          sync.RWMutex
	memories    map[string]*WorkingMemory // taskID → memory
	llmCall     LLMFunc                   // for compression/summarization
	summarizer  Summarizer                // pluggable summarizer (local model support)
	persistPath string  // legacy file path for persistence
	kvs         kvStore // Ledger KV (preferred when set)

	// CompressThreshold is the token estimate threshold that triggers auto-compression.
	// Default: 1000.
	CompressThreshold int
}

// NewWorkingMemoryManager creates a manager.
func NewWorkingMemoryManager(llmCall LLMFunc) *WorkingMemoryManager {
	return &WorkingMemoryManager{
		memories:          make(map[string]*WorkingMemory),
		llmCall:           llmCall,
		summarizer:        &LLMSummarizer{Call: llmCall},
		CompressThreshold: 1000,
	}
}

// NewWorkingMemoryManagerWithPersistence creates a manager that persists to disk.
func NewWorkingMemoryManagerWithPersistence(llmCall LLMFunc, dataDir string) *WorkingMemoryManager {
	m := NewWorkingMemoryManager(llmCall)
	m.persistPath = filepath.Join(dataDir, "working_memory.json")
	m.loadFromDisk()
	return m
}

// SetSummarizer replaces the default LLM summarizer with a custom one.
// Use this to plug in a local model (e.g., llama.cpp, Ollama).
func (m *WorkingMemoryManager) SetSummarizer(s Summarizer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.summarizer = s
}

// SetKVStore enables Ledger KV-backed persistence, replacing file I/O.
func (m *WorkingMemoryManager) SetKVStore(kvs kvStore) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.kvs = kvs
	m.loadFromKV()
}

// Init initializes working memory for a task.
func (m *WorkingMemoryManager) Init(t *Task) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.memories[t.ID] = &WorkingMemory{
		TaskID:    t.ID,
		Goal:      t.Description,
		UpdatedAt: time.Now(),
	}
	m.persist()
}

// Get returns the working memory for a task.
func (m *WorkingMemoryManager) Get(taskID string) *WorkingMemory {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.memories[taskID]
}

// UpdateAfterStep updates working memory after a step completes.
func (m *WorkingMemoryManager) UpdateAfterStep(t *Task, step *Step) {
	m.mu.Lock()
	defer m.mu.Unlock()

	wm, ok := m.memories[t.ID]
	if !ok {
		wm = &WorkingMemory{TaskID: t.ID, Goal: t.Description}
		m.memories[t.ID] = wm
	}

	if step.Status == StepDone {
		// Condense step result into a short summary line
		summary := condenseFact(step.Action, step.Result)
		wm.CompletedWork = append(wm.CompletedWork, summary)

		// Remove from blockers if it was there
		wm.Blockers = removeMatching(wm.Blockers, step.Action)
	} else if step.Status == StepFailed {
		blocker := fmt.Sprintf("步骤[%s]失败: %s", step.Action, step.Error)
		wm.Blockers = append(wm.Blockers, blocker)
	}

	// Track artifacts
	for _, art := range t.Artifacts {
		artDesc := fmt.Sprintf("%s (%s)", art.Name, art.Path)
		if !contains(wm.Artifacts, artDesc) {
			wm.Artifacts = append(wm.Artifacts, artDesc)
		}
	}

	// Determine next action
	next := t.CurrentStep()
	if next != nil {
		wm.NextAction = next.Action
	} else if t.Status == StatusCompleted {
		wm.NextAction = "任务已完成"
	}

	wm.UpdatedAt = time.Now()
	wm.TokenEstimate = wm.estimateTokens()

	// Auto-compress if token estimate exceeds threshold
	threshold := m.CompressThreshold
	if threshold <= 0 {
		threshold = 1000
	}
	if wm.TokenEstimate > threshold && m.summarizer != nil {
		safego.Go("wm-auto-compress-"+t.ID, func() {
			if err := m.Compress(context.Background(), t.ID); err != nil {
				slog.Warn("working memory auto-compress failed", "task", t.ID, "err", err)
			}
		})
	}

	m.persist()
}

// AddConfirmed records a user-confirmed fact.
func (m *WorkingMemoryManager) AddConfirmed(taskID, fact string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	wm := m.memories[taskID]
	if wm == nil {
		return
	}
	wm.Confirmed = append(wm.Confirmed, fact)
	wm.Pending = removeMatching(wm.Pending, fact)
	wm.UpdatedAt = time.Now()
	m.persist()
}

// AddPending records something that needs user confirmation.
func (m *WorkingMemoryManager) AddPending(taskID, question string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	wm := m.memories[taskID]
	if wm == nil {
		return
	}
	wm.Pending = append(wm.Pending, question)
	wm.UpdatedAt = time.Now()
	m.persist()
}

// Compress uses the summarizer to compress working memory if it grows too large.
// Threshold: >CompressThreshold estimated tokens triggers compression.
func (m *WorkingMemoryManager) Compress(ctx context.Context, taskID string) error {
	m.mu.RLock()
	wm := m.memories[taskID]
	m.mu.RUnlock()

	threshold := m.CompressThreshold
	if threshold <= 0 {
		threshold = 1000
	}
	if wm == nil || wm.estimateTokens() < threshold {
		return nil
	}
	if m.summarizer == nil {
		return nil
	}

	rendered := wm.Render()
	compressed, err := m.summarizer.Summarize(ctx, rendered)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	wm = m.memories[taskID]
	if wm == nil {
		return nil
	}

	// Parse compressed result back into working memory
	// For simplicity, replace CompletedWork with a single compressed entry
	wm.CompletedWork = []string{compressed}
	wm.TokenEstimate = wm.estimateTokens()
	wm.UpdatedAt = time.Now()
	m.persist()
	return nil
}

// Cleanup removes working memory for completed tasks.
func (m *WorkingMemoryManager) Cleanup(taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.memories, taskID)
	m.persist()
}

// RenderForTask returns the prompt segment for a task, or empty string if no memory.
func (m *WorkingMemoryManager) RenderForTask(taskID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	wm := m.memories[taskID]
	if wm == nil {
		return ""
	}
	return wm.Render()
}

// ── Helpers ──

// condenseFact gives a condensed one-liner from step action + result.
func condenseFact(action, result string) string {
	r := []rune(result)
	if len(r) > 80 {
		result = string(r[:80]) + "..."
	}
	if result == "" {
		return action + " → 完成"
	}
	return fmt.Sprintf("%s → %s", action, result)
}

func removeMatching(slice []string, substr string) []string {
	var out []string
	for _, s := range slice {
		if !strings.Contains(s, substr) {
			out = append(out, s)
		}
	}
	return out
}

func contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// ── Thread confirmation extraction ──

// ExtractConfirmFromThread extracts user confirmations from a thread message.
// Detects patterns like "确认", "同意", "OK", "行", "好的" and records as confirmed fact.
// Returns true if a confirmation was detected and recorded.
func (m *WorkingMemoryManager) ExtractConfirmFromThread(taskID, userMessage string) bool {
	m.mu.RLock()
	wm := m.memories[taskID]
	m.mu.RUnlock()
	if wm == nil || len(wm.Pending) == 0 {
		return false
	}

	msg := strings.TrimSpace(userMessage)
	lower := strings.ToLower(msg)

	// Detection: affirmative patterns
	confirmPatterns := []string{"确认", "同意", "好的", "可以", "没问题", "行", "ok", "yes", "确定", "approve", "通过"}
	isConfirm := false
	for _, p := range confirmPatterns {
		if strings.Contains(lower, p) {
			isConfirm = true
			break
		}
	}
	if !isConfirm {
		return false
	}

	// Move first pending item to confirmed
	m.mu.Lock()
	defer m.mu.Unlock()
	wm = m.memories[taskID]
	if wm == nil || len(wm.Pending) == 0 {
		return false
	}
	confirmed := wm.Pending[0]
	wm.Pending = wm.Pending[1:]
	wm.Confirmed = append(wm.Confirmed, confirmed)
	wm.UpdatedAt = time.Now()
	m.persist()
	return true
}

// GetAll returns all active working memories (for API/debug).
func (m *WorkingMemoryManager) GetAll() map[string]*WorkingMemory {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]*WorkingMemory, len(m.memories))
	for k, v := range m.memories {
		out[k] = v
	}
	return out
}

// ── Persistence ──

func (m *WorkingMemoryManager) persist() {
	if m.kvs != nil {
		snapshot := make(map[string]*WorkingMemory, len(m.memories))
		for k, v := range m.memories {
			snapshot[k] = v
		}
		if err := m.kvs.Put(context.Background(), "data", snapshot); err != nil {
			slog.Warn("working memory: kv save failed, falling back to file", "err", err)
		} else {
			return
		}
	}
	if m.persistPath == "" {
		return
	}
	snapshot := make(map[string]*WorkingMemory, len(m.memories))
	for k, v := range m.memories {
		snapshot[k] = v
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		slog.Warn("working memory persist failed", "err", err)
		return
	}
	if err := os.WriteFile(m.persistPath, data, 0644); err != nil {
		slog.Warn("working memory persist failed", "err", err)
	}
}

func (m *WorkingMemoryManager) loadFromKV() {
	if m.kvs == nil {
		return
	}
	var loaded map[string]*WorkingMemory
	found, err := m.kvs.Get(context.Background(), "data", &loaded)
	if err != nil {
		slog.Warn("working memory: kv load failed", "err", err)
		return
	}
	if found && len(loaded) > 0 {
		m.memories = loaded
		slog.Info("working memory: loaded from Ledger KV", "tasks", len(loaded))
	}
}

func (m *WorkingMemoryManager) loadFromDisk() {
	if m.persistPath == "" {
		return
	}
	data, err := os.ReadFile(m.persistPath)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("working memory load failed", "err", err)
		}
		return
	}
	var loaded map[string]*WorkingMemory
	if err := json.Unmarshal(data, &loaded); err != nil {
		slog.Warn("working memory parse failed", "err", err)
		return
	}
	m.memories = loaded
	slog.Info("working memory loaded", "tasks", len(loaded))
}
