package task

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/session"
	"yunque-agent/pkg/safego"
)

// kvStore abstracts Ledger KV to avoid import cycles with internal/ledger.
type kvStore interface {
	Put(ctx context.Context, key string, value any) error
	Get(ctx context.Context, key string, dest any) (bool, error)
}

// ──────────────────────────────────────────────
// Task Threads — Dedicated conversation per task
//
// Each running task gets its own conversation thread so users can
// discuss, approve, modify goals, and provide feedback around
// that specific task. Threads are backed by the same session.Store
// used for normal chat, keyed with a "task:" prefix.
//
// Enhanced capabilities:
//   - Thread state machine: open → paused → closed
//   - Channel binding: remember originating channel+target+user for push-back
//   - Structured message types: step_result, approval_request, notification
//   - Thread listing: list all active threads with filters
//   - Persistence: thread metadata saved to data/threads.json
//   - Channel push-back: notify originating channel on task events
// ──────────────────────────────────────────────

// ThreadState is the lifecycle state of a task thread.
type ThreadState string

const (
	ThreadOpen   ThreadState = "open"   // active, accepting messages
	ThreadPaused ThreadState = "paused" // temporarily suspended
	ThreadClosed ThreadState = "closed" // task done, read-only
)

// ThreadMsgType is the type of a structured thread message.
type ThreadMsgType string

const (
	MsgChat            ThreadMsgType = "chat"             // normal user/assistant message
	MsgStepResult      ThreadMsgType = "step_result"      // automatic: task step completed
	MsgStepFailed      ThreadMsgType = "step_failed"      // automatic: task step failed
	MsgApprovalRequest ThreadMsgType = "approval_request" // agent requests user approval
	MsgApprovalGrant   ThreadMsgType = "approval_grant"   // user approves
	MsgApprovalDeny    ThreadMsgType = "approval_deny"    // user denies
	MsgNotification    ThreadMsgType = "notification"     // system notification
	MsgTaskCompleted   ThreadMsgType = "task_completed"   // automatic: task done
	MsgTaskFailed      ThreadMsgType = "task_failed"      // automatic: task failed
)

// ChannelBinding records where a thread originated so updates can
// be pushed back to the same channel.
type ChannelBinding struct {
	ChannelType string `json:"channel_type"`         // "telegram", "discord", "feishu", etc.
	ChannelID   string `json:"channel_id"`           // group/chat ID on that channel
	UserID      string `json:"user_id,omitempty"`    // user who created the task
	UserName    string `json:"user_name,omitempty"`  // display name
	MessageID   string `json:"message_id,omitempty"` // original message ID (for reply threads)
}

// ThreadMessage is a single message in a task thread.
type ThreadMessage struct {
	Role      string         `json:"role"` // "user", "assistant", "system"
	Content   string         `json:"content"`
	MsgType   ThreadMsgType  `json:"msg_type,omitempty"` // structured type (default: chat)
	Channel   string         `json:"channel,omitempty"`  // originating channel type
	StepID    int            `json:"step_id,omitempty"`  // associated step (for step_result/step_failed)
	Metadata  map[string]any `json:"metadata,omitempty"` // extra data (skill name, error details, etc.)
	Timestamp time.Time      `json:"timestamp"`
}

// ThreadInfo describes a task thread's metadata.
type ThreadInfo struct {
	TaskID    string          `json:"task_id"`
	SessionID string          `json:"session_id"`
	State     ThreadState     `json:"state"`
	Binding   *ChannelBinding `json:"binding,omitempty"`
	TenantID  string          `json:"tenant_id"`
	Messages  int             `json:"messages"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// threadMeta is the in-memory metadata for a thread (persisted to disk).
type threadMeta struct {
	TaskID    string          `json:"task_id"`
	SessionID string          `json:"session_id"`
	State     ThreadState     `json:"state"`
	Binding   *ChannelBinding `json:"binding,omitempty"`
	TenantID  string          `json:"tenant_id"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// ChannelSendFunc pushes a message to a specific channel target.
// channelType is e.g. "telegram", target is the chat/group ID, content is text.
type ChannelSendFunc func(ctx context.Context, channelType, target, content string) error

// ChannelSendCardFunc pushes a structured card JSON to a specific channel target.
// cardJSON is a pre-built interactive card payload (e.g. from channel.Card.Build()).
type ChannelSendCardFunc func(ctx context.Context, channelType, target, cardJSON string) error

// ThreadManager manages task-scoped conversation threads.
type ThreadManager struct {
	mu              sync.RWMutex
	convStore       *session.Store
	threads         map[string]*threadMeta // taskID → meta
	dataFile        string  // legacy persistence path
	kvs             kvStore // Ledger KV (preferred when set)
	channelSend     ChannelSendFunc        // optional: push to channel (text)
	channelSendCard ChannelSendCardFunc    // optional: push card to channel
}

// NewThreadManager creates a thread manager backed by a session store.
// dataDir is the directory for persisting thread metadata (e.g. "data").
func NewThreadManager(convStore *session.Store, dataDir ...string) *ThreadManager {
	tm := &ThreadManager{
		convStore: convStore,
		threads:   make(map[string]*threadMeta),
	}
	if len(dataDir) > 0 && dataDir[0] != "" {
		tm.dataFile = filepath.Join(dataDir[0], "threads.json")
		tm.loadFromDisk()
	}
	return tm
}

// SetChannelSend sets the callback used to push messages back to channels.
func (tm *ThreadManager) SetChannelSend(fn ChannelSendFunc) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.channelSend = fn
}

// SetChannelSendCard sets the callback for pushing rich cards to channels.
// If set, task progress events will be sent as interactive cards instead of plain text.
func (tm *ThreadManager) SetChannelSendCard(fn ChannelSendCardFunc) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.channelSendCard = fn
}

// SetKVStore enables Ledger KV-backed persistence, replacing file I/O.
func (tm *ThreadManager) SetKVStore(kvs kvStore) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.kvs = kvs
	tm.loadFromKV()
}

// ────────── Session ID ──────────

func threadSessionID(taskID string) string {
	return fmt.Sprintf("task:%s", taskID)
}

// ────────── Core operations ──────────

// Ensure creates a thread for a task if not exists.
// Returns the thread's session ID.
func (tm *ThreadManager) Ensure(taskID, tenantID string) string {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if meta, ok := tm.threads[taskID]; ok {
		return meta.SessionID
	}

	sid := threadSessionID(taskID)
	tm.convStore.GetOrCreate(sid, tenantID)
	now := time.Now()
	tm.threads[taskID] = &threadMeta{
		TaskID:    taskID,
		SessionID: sid,
		State:     ThreadOpen,
		TenantID:  tenantID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	tm.saveToDisk()
	return sid
}

// EnsureWithBinding creates a thread bound to a specific channel origin.
func (tm *ThreadManager) EnsureWithBinding(taskID, tenantID string, binding *ChannelBinding) string {
	sid := tm.Ensure(taskID, tenantID)

	tm.mu.Lock()
	defer tm.mu.Unlock()
	if meta, ok := tm.threads[taskID]; ok && meta.Binding == nil && binding != nil {
		meta.Binding = binding
		meta.UpdatedAt = time.Now()
		tm.saveToDisk()
	}
	return sid
}

// Post appends a message to a task's thread.
func (tm *ThreadManager) Post(taskID, tenantID, role, content string) {
	tm.PostTyped(taskID, tenantID, role, content, MsgChat, nil)
}

// PostTyped appends a structured message to a task's thread.
func (tm *ThreadManager) PostTyped(taskID, tenantID, role, content string, msgType ThreadMsgType, meta map[string]any) {
	sid := tm.Ensure(taskID, tenantID)
	tm.convStore.Append(sid, llm.Message{Role: role, Content: content})

	// Update timestamp
	tm.mu.Lock()
	if m, ok := tm.threads[taskID]; ok {
		m.UpdatedAt = time.Now()
	}
	tm.mu.Unlock()
}

// PostStepResult posts a step completion notification to the thread.
func (tm *ThreadManager) PostStepResult(taskID, tenantID string, stepID int, skillName, result string) {
	content := fmt.Sprintf("✅ 步骤 %d 完成", stepID)
	if skillName != "" {
		content += fmt.Sprintf(" [%s]", skillName)
	}
	if len(result) > 200 {
		content += "\n" + result[:200] + "..."
	} else if result != "" {
		content += "\n" + result
	}
	tm.PostTyped(taskID, tenantID, "system", content, MsgStepResult, map[string]any{
		"step_id":    stepID,
		"skill_name": skillName,
	})
	tm.pushToChannel(taskID, content)
}

// PostStepFailed posts a step failure notification to the thread.
func (tm *ThreadManager) PostStepFailed(taskID, tenantID string, stepID int, skillName, errMsg string) {
	content := fmt.Sprintf("❌ 步骤 %d 失败", stepID)
	if skillName != "" {
		content += fmt.Sprintf(" [%s]", skillName)
	}
	if errMsg != "" {
		content += ": " + errMsg
	}
	tm.PostTyped(taskID, tenantID, "system", content, MsgStepFailed, map[string]any{
		"step_id":    stepID,
		"skill_name": skillName,
	})
	tm.pushToChannel(taskID, content)
}

// PostTaskCompleted posts a task completion notification.
func (tm *ThreadManager) PostTaskCompleted(taskID, tenantID, summary string) {
	content := "🎉 任务完成"
	if summary != "" {
		content += "\n" + summary
	}
	tm.PostTyped(taskID, tenantID, "system", content, MsgTaskCompleted, nil)
	tm.SetState(taskID, ThreadClosed)
	tm.pushToChannel(taskID, content)
}

// PostTaskFailed posts a task failure notification.
func (tm *ThreadManager) PostTaskFailed(taskID, tenantID, errMsg string) {
	content := "💥 任务失败"
	if errMsg != "" {
		content += ": " + errMsg
	}
	tm.PostTyped(taskID, tenantID, "system", content, MsgTaskFailed, nil)
	tm.SetState(taskID, ThreadClosed)
	tm.pushToChannel(taskID, content)
}

// ────────── Rich card variants ──────────
// These methods send interactive cards when channelSendCard is configured,
// otherwise fall back to plain text via pushToChannel.

// PostStepResultRich posts a step completion with an interactive card.
func (tm *ThreadManager) PostStepResultRich(taskID, tenantID string, stepID, totalSteps int, taskTitle, action, result string) {
	content := fmt.Sprintf("✅ 步骤 %d/%d 完成 — %s", stepID, totalSteps, action)
	if len(result) > 200 {
		content += "\n" + result[:200] + "..."
	} else if result != "" {
		content += "\n" + result
	}
	tm.PostTyped(taskID, tenantID, "system", content, MsgStepResult, map[string]any{
		"step_id": stepID,
		"action":  action,
	})
	cardJSON := buildStepCard(taskTitle, stepID, totalSteps, action, "done", result)
	tm.pushCardToChannel(taskID, cardJSON, content)
}

// PostStepFailedRich posts a step failure with an interactive card.
func (tm *ThreadManager) PostStepFailedRich(taskID, tenantID string, stepID, totalSteps int, taskTitle, action, errMsg string) {
	content := fmt.Sprintf("❌ 步骤 %d/%d 失败 — %s: %s", stepID, totalSteps, action, errMsg)
	tm.PostTyped(taskID, tenantID, "system", content, MsgStepFailed, map[string]any{
		"step_id": stepID,
		"action":  action,
	})
	cardJSON := buildStepCard(taskTitle, stepID, totalSteps, action, "failed", errMsg)
	tm.pushCardToChannel(taskID, cardJSON, content)
}

// PostTaskCompletedRich posts a task completion with an interactive card.
func (tm *ThreadManager) PostTaskCompletedRich(taskID, tenantID, taskTitle, summary string) {
	content := "🎉 任务完成"
	if summary != "" {
		content += "\n" + summary
	}
	tm.PostTyped(taskID, tenantID, "system", content, MsgTaskCompleted, nil)
	tm.SetState(taskID, ThreadClosed)
	cardJSON := buildTaskCompletedCard(taskTitle, taskID, summary)
	tm.pushCardToChannel(taskID, cardJSON, content)
}

// PostTaskFailedRich posts a task failure with an interactive card.
func (tm *ThreadManager) PostTaskFailedRich(taskID, tenantID, taskTitle, errMsg string) {
	content := "💥 任务失败"
	if errMsg != "" {
		content += ": " + errMsg
	}
	tm.PostTyped(taskID, tenantID, "system", content, MsgTaskFailed, nil)
	tm.SetState(taskID, ThreadClosed)
	cardJSON := buildTaskFailedCard(taskTitle, taskID, errMsg)
	tm.pushCardToChannel(taskID, cardJSON, content)
}

// ────────── State management ──────────

// SetState transitions the thread to a new state.
func (tm *ThreadManager) SetState(taskID string, state ThreadState) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if meta, ok := tm.threads[taskID]; ok {
		meta.State = state
		meta.UpdatedAt = time.Now()
		tm.saveToDisk()
	}
}

// GetState returns the current state of a thread.
func (tm *ThreadManager) GetState(taskID string) ThreadState {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if meta, ok := tm.threads[taskID]; ok {
		return meta.State
	}
	return ""
}

// ────────── Query ──────────

// Messages returns all messages in a task's thread.
func (tm *ThreadManager) Messages(taskID string) []llm.Message {
	tm.mu.RLock()
	_, ok := tm.threads[taskID]
	tm.mu.RUnlock()
	if !ok {
		return nil
	}
	return tm.convStore.Get(threadSessionID(taskID))
}

// Info returns metadata about a task's thread.
func (tm *ThreadManager) Info(taskID string) *ThreadInfo {
	tm.mu.RLock()
	meta, ok := tm.threads[taskID]
	tm.mu.RUnlock()
	if !ok {
		return nil
	}

	msgs := tm.convStore.Get(meta.SessionID)
	return &ThreadInfo{
		TaskID:    taskID,
		SessionID: meta.SessionID,
		State:     meta.State,
		Binding:   meta.Binding,
		TenantID:  meta.TenantID,
		Messages:  len(msgs),
		CreatedAt: meta.CreatedAt,
		UpdatedAt: meta.UpdatedAt,
	}
}

// HasThread checks if a task has a thread.
func (tm *ThreadManager) HasThread(taskID string) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	_, ok := tm.threads[taskID]
	return ok
}

// List returns all threads, optionally filtered by state.
func (tm *ThreadManager) List(stateFilter ThreadState) []ThreadInfo {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var out []ThreadInfo
	for _, meta := range tm.threads {
		if stateFilter != "" && meta.State != stateFilter {
			continue
		}
		msgs := tm.convStore.Get(meta.SessionID)
		out = append(out, ThreadInfo{
			TaskID:    meta.TaskID,
			SessionID: meta.SessionID,
			State:     meta.State,
			Binding:   meta.Binding,
			TenantID:  meta.TenantID,
			Messages:  len(msgs),
			CreatedAt: meta.CreatedAt,
			UpdatedAt: meta.UpdatedAt,
		})
	}
	return out
}

// Binding returns the channel binding for a thread, if set.
func (tm *ThreadManager) Binding(taskID string) *ChannelBinding {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if meta, ok := tm.threads[taskID]; ok {
		return meta.Binding
	}
	return nil
}

// ────────── Cleanup ──────────

// Cleanup removes a task's thread data.
func (tm *ThreadManager) Cleanup(taskID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if meta, ok := tm.threads[taskID]; ok {
		tm.convStore.Delete(meta.SessionID)
		delete(tm.threads, taskID)
		tm.saveToDisk()
	}
}

// ────────── Channel push-back ──────────

// pushToChannel sends a notification to the thread's bound channel, if any.
func (tm *ThreadManager) pushToChannel(taskID, content string) {
	tm.mu.RLock()
	meta, ok := tm.threads[taskID]
	send := tm.channelSend
	tm.mu.RUnlock()

	if !ok || send == nil || meta.Binding == nil {
		return
	}

	b := meta.Binding
	if b.ChannelType == "" || b.ChannelID == "" {
		return
	}

	safego.Go("thread-push-"+taskID, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := send(ctx, b.ChannelType, b.ChannelID, content); err != nil {
			slog.Warn("thread push to channel failed",
				"task_id", taskID,
				"channel", b.ChannelType,
				"target", b.ChannelID,
				"error", err)
		}
	})
}

// pushCardToChannel sends a rich card to the thread's bound channel.
// Falls back to pushToChannel(text) if card sender is not configured.
func (tm *ThreadManager) pushCardToChannel(taskID, cardJSON, fallbackText string) {
	tm.mu.RLock()
	meta, ok := tm.threads[taskID]
	sendCard := tm.channelSendCard
	sendText := tm.channelSend
	tm.mu.RUnlock()

	if !ok || meta.Binding == nil {
		return
	}

	b := meta.Binding
	if b.ChannelType == "" || b.ChannelID == "" {
		return
	}

	safego.Go("thread-push-card-"+taskID, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Try card first, fall back to plain text
		if sendCard != nil {
			if err := sendCard(ctx, b.ChannelType, b.ChannelID, cardJSON); err != nil {
				slog.Warn("thread push card failed, falling back to text",
					"task_id", taskID, "error", err)
			} else {
				return
			}
		}

		if sendText != nil {
			if err := sendText(ctx, b.ChannelType, b.ChannelID, fallbackText); err != nil {
				slog.Warn("thread push to channel failed",
					"task_id", taskID, "error", err)
			}
		}
	})
}

// ────────── Persistence ──────────

func (tm *ThreadManager) saveToDisk() {
	if tm.kvs != nil {
		if err := tm.kvs.Put(context.Background(), "threads", tm.threads); err != nil {
			slog.Warn("thread: kv save failed, falling back to file", "err", err)
		} else {
			return
		}
	}
	if tm.dataFile == "" {
		return
	}
	data, err := json.MarshalIndent(tm.threads, "", "  ")
	if err != nil {
		slog.Warn("thread save failed", "error", err)
		return
	}
	dir := filepath.Dir(tm.dataFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(tm.dataFile, data, 0o644)
}

func (tm *ThreadManager) loadFromKV() {
	if tm.kvs == nil {
		return
	}
	var loaded map[string]*threadMeta
	found, err := tm.kvs.Get(context.Background(), "threads", &loaded)
	if err != nil {
		slog.Warn("thread: kv load failed", "err", err)
		return
	}
	if found && len(loaded) > 0 {
		tm.threads = loaded
		slog.Info("thread: loaded from Ledger KV", "count", len(loaded))
	}
}

func (tm *ThreadManager) loadFromDisk() {
	if tm.dataFile == "" {
		return
	}
	data, err := os.ReadFile(tm.dataFile)
	if err != nil {
		return
	}
	var loaded map[string]*threadMeta
	if err := json.Unmarshal(data, &loaded); err != nil {
		slog.Warn("thread load failed", "error", err)
		return
	}
	for k, v := range loaded {
		tm.threads[k] = v
	}
	slog.Info("loaded thread metadata", "count", len(loaded))
}
