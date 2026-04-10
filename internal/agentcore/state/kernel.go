// Package state 提供结构化状态内核，在五层记忆之上的实时决策上下文。
// 记忆是"我记得什么"，状态是"我当前在做什么、关注什么、能做什么"。
package state

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
)

// kvStore abstracts Ledger KV to avoid import cycles with internal/ledger.
type kvStore interface {
	Put(ctx context.Context, key string, value any) error
	Get(ctx context.Context, key string, dest any) (bool, error)
}

// ---------- 数据结构 ----------

// Goal 表示一个目标（比任务 Task 更抽象）
type Goal struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Priority    int       `json:"priority"` // 1=highest
	Status      string    `json:"status"`   // active, paused, completed, abandoned
	Progress    float64   `json:"progress"` // 0.0 ~ 1.0
	ParentGoal  string    `json:"parent_goal,omitempty"`
	SubGoals    []string  `json:"sub_goals,omitempty"`
	TaskIDs     []string  `json:"task_ids,omitempty"` // 关联的 task
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Resource 表示当前正在使用/跟踪的资源
type Resource struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"` // file, url, document, service, repo
	Path      string            `json:"path"`
	Status    string            `json:"status"` // active, stale, closed
	Metadata  map[string]string `json:"metadata,omitempty"`
	TrackedAt time.Time         `json:"tracked_at"`
}

// ActionRecord 记录近期动作
type ActionRecord struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	Result    string    `json:"result,omitempty"`
	Success   bool      `json:"success"`
}

// CapSnapshot 当前能力快照
type CapSnapshot struct {
	TotalSkills    int      `json:"total_skills"`
	DynamicSkills  []string `json:"dynamic_skills,omitempty"`
	UnresolvedGaps int      `json:"unresolved_gaps"`
	RecentGaps     []string `json:"recent_gaps,omitempty"`
}

// Snapshot 完整状态快照
type Snapshot struct {
	Goals        []*Goal        `json:"goals"`
	Resources    []*Resource    `json:"resources"`
	Focus        string         `json:"focus"`
	Topics       []string       `json:"topics"`
	RecentActions []ActionRecord `json:"recent_actions"`
	Capabilities CapSnapshot    `json:"capabilities"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// ---------- State Kernel ----------

const (
	maxActions = 50
	maxTopics  = 10
	stateFile  = "state.json"
)

// Kernel 结构化状态内核
type Kernel struct {
	mu        sync.RWMutex
	goals     []*Goal
	resources map[string]*Resource
	focus     string
	topics    []string
	actions   []ActionRecord
	caps      CapSnapshot
	dataDir   string
	kvs       kvStore
	listeners []func(event string)
}

// SetKVStore enables Ledger KV-backed persistence for state kernel.
func (k *Kernel) SetKVStore(kvs kvStore) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.kvs = kvs

	hasData := len(k.goals) > 0 || len(k.resources) > 0 || k.focus != ""
	if hasData {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		data := persistData{Goals: k.goals, Resources: k.resources, Focus: k.focus, Topics: k.topics}
		if err := kvs.Put(ctx, "state", data); err != nil {
			slog.Warn("state kernel: KV migration failed", "err", err)
		} else {
			slog.Info("state kernel: migrated to KV")
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var data persistData
	found, err := kvs.Get(ctx, "state", &data)
	if err != nil {
		slog.Warn("state kernel: KV load failed", "err", err)
		return
	}
	if found {
		k.goals = data.Goals
		if data.Resources != nil {
			k.resources = data.Resources
		}
		k.focus = data.Focus
		k.topics = data.Topics
		slog.Info("state kernel: loaded from KV", "goals", len(k.goals))
	}
}

// NewKernel 创建状态内核
func NewKernel(dataDir string) *Kernel {
	k := &Kernel{
		resources: make(map[string]*Resource),
		dataDir:   dataDir,
	}
	_ = k.Load()
	return k
}

// ---------- Goal 管理 ----------

// AddGoal 添加目标，返回 ID
func (k *Kernel) AddGoal(g Goal) string {
	k.mu.Lock()
	defer k.mu.Unlock()

	if g.ID == "" {
		g.ID = fmt.Sprintf("goal-%d", time.Now().UnixMilli())
	}
	if g.Status == "" {
		g.Status = "active"
	}
	if g.Priority == 0 {
		g.Priority = 5
	}
	now := time.Now()
	g.CreatedAt = now
	g.UpdatedAt = now
	k.goals = append(k.goals, &g)
	k.notify("goal_added")
	return g.ID
}

// UpdateGoal 更新目标字段
func (k *Kernel) UpdateGoal(id string, fn func(g *Goal)) bool {
	k.mu.Lock()
	defer k.mu.Unlock()

	for _, g := range k.goals {
		if g.ID == id {
			fn(g)
			g.UpdatedAt = time.Now()
			k.notify("goal_updated")
			return true
		}
	}
	return false
}

// RemoveGoal 删除目标
func (k *Kernel) RemoveGoal(id string) bool {
	k.mu.Lock()
	defer k.mu.Unlock()

	for i, g := range k.goals {
		if g.ID == id {
			k.goals = append(k.goals[:i], k.goals[i+1:]...)
			k.notify("goal_removed")
			return true
		}
	}
	return false
}

// Goals 返回目标列表副本
func (k *Kernel) Goals() []*Goal {
	k.mu.RLock()
	defer k.mu.RUnlock()
	out := make([]*Goal, len(k.goals))
	copy(out, k.goals)
	return out
}

// ActiveGoals 返回活跃目标
func (k *Kernel) ActiveGoals() []*Goal {
	k.mu.RLock()
	defer k.mu.RUnlock()
	var out []*Goal
	for _, g := range k.goals {
		if g.Status == "active" {
			out = append(out, g)
		}
	}
	return out
}

// LinkTask 将 taskID 关联到 goal
func (k *Kernel) LinkTask(goalID, taskID string) {
	k.UpdateGoal(goalID, func(g *Goal) {
		for _, tid := range g.TaskIDs {
			if tid == taskID {
				return
			}
		}
		g.TaskIDs = append(g.TaskIDs, taskID)
	})
}

// ---------- Resource 管理 ----------

// TrackResource 跟踪资源
func (k *Kernel) TrackResource(r Resource) {
	k.mu.Lock()
	defer k.mu.Unlock()

	if r.ID == "" {
		r.ID = fmt.Sprintf("res-%d", time.Now().UnixMilli())
	}
	if r.Status == "" {
		r.Status = "active"
	}
	r.TrackedAt = time.Now()
	k.resources[r.ID] = &r
	k.notify("resource_tracked")
}

// ReleaseResource 释放资源
func (k *Kernel) ReleaseResource(id string) bool {
	k.mu.Lock()
	defer k.mu.Unlock()

	if r, ok := k.resources[id]; ok {
		r.Status = "closed"
		k.notify("resource_released")
		return true
	}
	return false
}

// Resources 返回活跃资源
func (k *Kernel) Resources() []*Resource {
	k.mu.RLock()
	defer k.mu.RUnlock()
	var out []*Resource
	for _, r := range k.resources {
		if r.Status == "active" {
			out = append(out, r)
		}
	}
	return out
}

// ---------- Working Context ----------

// SetFocus 设置当前关注焦点
func (k *Kernel) SetFocus(focus string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.focus = focus
	k.notify("focus_changed")
}

// Focus 获取当前焦点
func (k *Kernel) Focus() string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.focus
}

// AddTopic 添加活跃话题
func (k *Kernel) AddTopic(topic string) {
	k.mu.Lock()
	defer k.mu.Unlock()

	// 去重
	for _, t := range k.topics {
		if t == topic {
			return
		}
	}
	k.topics = append(k.topics, topic)
	if len(k.topics) > maxTopics {
		k.topics = k.topics[len(k.topics)-maxTopics:]
	}
}

// ClearTopics 清除话题
func (k *Kernel) ClearTopics() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.topics = nil
}

// RecordAction 记录动作
func (k *Kernel) RecordAction(a ActionRecord) {
	k.mu.Lock()
	defer k.mu.Unlock()

	if a.Timestamp.IsZero() {
		a.Timestamp = time.Now()
	}
	k.actions = append(k.actions, a)
	if len(k.actions) > maxActions {
		k.actions = k.actions[len(k.actions)-maxActions:]
	}
}

// ---------- Capability Snapshot ----------

// UpdateCapabilities 更新能力快照
func (k *Kernel) UpdateCapabilities(caps CapSnapshot) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.caps = caps
}

// ---------- 快照 & LLM 上下文 ----------

// TakeSnapshot 返回完整状态快照
func (k *Kernel) TakeSnapshot() Snapshot {
	k.mu.RLock()
	defer k.mu.RUnlock()

	goals := make([]*Goal, len(k.goals))
	copy(goals, k.goals)

	var resources []*Resource
	for _, r := range k.resources {
		resources = append(resources, r)
	}

	topics := make([]string, len(k.topics))
	copy(topics, k.topics)

	// recent 10 actions
	n := len(k.actions)
	start := 0
	if n > 10 {
		start = n - 10
	}
	recent := make([]ActionRecord, n-start)
	copy(recent, k.actions[start:])

	return Snapshot{
		Goals:         goals,
		Resources:     resources,
		Focus:         k.focus,
		Topics:        topics,
		RecentActions: recent,
		Capabilities:  k.caps,
		UpdatedAt:     time.Now(),
	}
}

// CompileForLLM 编译为 LLM 可消费的结构化上下文
func (k *Kernel) CompileForLLM() string {
	snap := k.TakeSnapshot()

	var b strings.Builder

	// Goals
	activeGoals := 0
	for _, g := range snap.Goals {
		if g.Status == "active" {
			activeGoals++
		}
	}
	if activeGoals > 0 {
		b.WriteString("## 当前目标\n")
		for _, g := range snap.Goals {
			if g.Status != "active" {
				continue
			}
			b.WriteString(fmt.Sprintf("- [P%d] %s", g.Priority, g.Title))
			if g.Progress > 0 {
				b.WriteString(fmt.Sprintf(" (%.0f%%)", g.Progress*100))
			}
			if g.Description != "" {
				b.WriteString(": " + g.Description)
			}
			b.WriteString("\n")
		}
	}

	// Focus
	if snap.Focus != "" {
		b.WriteString(fmt.Sprintf("\n## 当前焦点\n%s\n", snap.Focus))
	}

	// Topics
	if len(snap.Topics) > 0 {
		b.WriteString(fmt.Sprintf("\n## 活跃话题\n%s\n", strings.Join(snap.Topics, ", ")))
	}

	// Resources
	activeRes := 0
	for _, r := range snap.Resources {
		if r.Status == "active" {
			activeRes++
		}
	}
	if activeRes > 0 {
		b.WriteString("\n## 当前资源\n")
		for _, r := range snap.Resources {
			if r.Status != "active" {
				continue
			}
			b.WriteString(fmt.Sprintf("- [%s] %s\n", r.Type, r.Path))
		}
	}

	// Capabilities
	if snap.Capabilities.TotalSkills > 0 || snap.Capabilities.UnresolvedGaps > 0 {
		b.WriteString(fmt.Sprintf("\n## 能力状态\n- 已注册技能: %d", snap.Capabilities.TotalSkills))
		if len(snap.Capabilities.DynamicSkills) > 0 {
			b.WriteString(fmt.Sprintf(" (含动态生成: %s)", strings.Join(snap.Capabilities.DynamicSkills, ", ")))
		}
		if snap.Capabilities.UnresolvedGaps > 0 {
			b.WriteString(fmt.Sprintf("\n- 未解决缺口: %d", snap.Capabilities.UnresolvedGaps))
		}
		b.WriteString("\n")
	}

	// Recent actions (last 5 for brevity)
	n := len(snap.RecentActions)
	if n > 0 {
		b.WriteString("\n## 近期动作\n")
		start := 0
		if n > 5 {
			start = n - 5
		}
		for _, a := range snap.RecentActions[start:] {
			status := "✓"
			if !a.Success {
				status = "✗"
			}
			b.WriteString(fmt.Sprintf("- %s %s", status, a.Action))
			if a.Result != "" {
				result := a.Result
				if len(result) > 80 {
					result = result[:80] + "…"
				}
				b.WriteString(": " + result)
			}
			b.WriteString("\n")
		}
	}

	result := b.String()
	if result == "" {
		return ""
	}
	return result
}

// ---------- 持久化 ----------

type persistData struct {
	Goals     []*Goal              `json:"goals"`
	Resources map[string]*Resource `json:"resources"`
	Focus     string               `json:"focus"`
	Topics    []string             `json:"topics"`
}

// Save 持久化到 KV (优先) 或磁盘 (回退)
func (k *Kernel) Save() error {
	k.mu.RLock()
	defer k.mu.RUnlock()

	data := persistData{
		Goals:     k.goals,
		Resources: k.resources,
		Focus:     k.focus,
		Topics:    k.topics,
	}

	if k.kvs != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := k.kvs.Put(ctx, "state", data); err != nil {
			slog.Warn("state kernel: KV save failed, falling back to file", "err", err)
		} else {
			return nil
		}
	}

	if k.dataDir == "" {
		return nil
	}
	if err := os.MkdirAll(k.dataDir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(k.dataDir, stateFile), raw, 0o644)
}

// Load 从磁盘加载
func (k *Kernel) Load() error {
	if k.dataDir == "" {
		return nil
	}
	raw, err := os.ReadFile(filepath.Join(k.dataDir, stateFile))
	if err != nil {
		return nil // 文件不存在不是错误
	}
	var data persistData
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}
	k.goals = data.Goals
	if data.Resources != nil {
		k.resources = data.Resources
	}
	k.focus = data.Focus
	k.topics = data.Topics
	return nil
}

// ---------- 事件通知 ----------

// OnChange 注册状态变更回调
func (k *Kernel) OnChange(fn func(event string)) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.listeners = append(k.listeners, fn)
}

func (k *Kernel) notify(event string) {
	for _, fn := range k.listeners {
		go fn(event)
	}
}
