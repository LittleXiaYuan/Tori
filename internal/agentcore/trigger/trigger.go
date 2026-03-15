package trigger

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// ──────────────────────────────────────────────
// Unified Trigger System — P1 Core Objects
//
// 设计目标：
// 1. 统一 4 类触发器：时间/事件/条件/认知
// 2. 统一 5 类动作：创建任务/继续任务/发送消息/调用技能/写记忆
// 3. 打通关联对象：task/thread/tenant/channel/budget
// 4. 完整执行记录：trigger_run + trigger_event
// ──────────────────────────────────────────────

// TriggerType 触发器类型
type TriggerType string

const (
	TriggerTypeTime      TriggerType = "time"      // 时间触发：每天/每周/cron
	TriggerTypeEvent     TriggerType = "event"     // 事件触发：任务失败/完成/知识库更新
	TriggerTypeCondition TriggerType = "condition" // 条件触发：状态变化/阈值超限
	TriggerTypeCognitive TriggerType = "cognitive" // 认知触发：Reverie insight/emotion shift
)

// Kind is an alias for backward compatibility
type Kind = TriggerType

const (
	KindTime      = TriggerTypeTime
	KindEvent     = TriggerTypeEvent
	KindCondition = TriggerTypeCondition
)

// ActionType 动作类型
type ActionType string

const (
	ActionCreateTask   ActionType = "create_task"   // 创建新任务
	ActionContinueTask ActionType = "continue_task" // 继续已有任务
	ActionSendMessage  ActionType = "send_message"  // 发送消息到渠道
	ActionCallSkill    ActionType = "call_skill"    // 调用技能
	ActionWriteMemory  ActionType = "write_memory"  // 写记忆/更新画像

	// Backward compatibility
	ActionAgentTurn  ActionType = "agent_turn"  // 等同于 create_task
	ActionThreadPost ActionType = "thread_post" // 等同于 send_message
	ActionWebhook    ActionType = "webhook"     // POST to URL
	ActionLog        ActionType = "log"         // log only
)

// EventName 系统事件名称
type EventName string

const (
	EventTaskCompleted     EventName = "task_completed"
	EventTaskFailed        EventName = "task_failed"
	EventTaskStatusChanged EventName = "task_status_changed"
	EventMemoryUpdated     EventName = "memory_updated"
	EventKnowledgeUpdated  EventName = "knowledge_updated"
	EventCostAlert         EventName = "cost_alert"
	EventSkillInstalled    EventName = "skill_installed"
	EventChannelMessage    EventName = "channel_message"
	EventReverieInsight    EventName = "reverie_insight"
	EventEmotionShift      EventName = "emotion_shift"
	EventCustom            EventName = "custom"
)

// TriggerStatus 触发器状态
type TriggerStatus string

const (
	TriggerStatusActive   TriggerStatus = "active"   // 激活中
	TriggerStatusPaused   TriggerStatus = "paused"   // 暂停
	TriggerStatusDisabled TriggerStatus = "disabled" // 禁用
)

// Trigger defines a reactive automation rule (legacy, use TriggerDef for new code).
type Trigger struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Kind      Kind      `json:"kind"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`

	// For KindEvent
	Event       EventName `json:"event,omitempty"`        // event to listen for
	EventFilter string    `json:"event_filter,omitempty"` // optional substring filter on event data

	// For KindCondition
	ConditionExpr string        `json:"condition_expr,omitempty"` // simple expression (e.g., "cost > 10")
	CheckInterval time.Duration `json:"check_interval,omitempty"` // how often to check

	// Action: what happens when trigger fires
	Action Action `json:"action"`

	// Runtime state
	LastFiredAt *time.Time `json:"last_fired_at,omitempty"`
	FireCount   int        `json:"fire_count"`
}

// Action defines what happens when a trigger fires (legacy).
type Action struct {
	Type    ActionType     `json:"type"`
	Message string         `json:"message,omitempty"` // prompt for agent_turn or event text
	TaskID  string         `json:"task_id,omitempty"` // target task for thread post
	Data    map[string]any `json:"data,omitempty"`
}

// ActionHandler is called when a trigger fires.
type ActionHandler func(ctx context.Context, trigger *Trigger, event *EventPayload) error

// LegacyConditionEvaluator checks if a condition expression is currently true (legacy).
type LegacyConditionEvaluator func(expr string) bool

// Runtime manages event and condition triggers (legacy, use Manager for new code).
type Runtime struct {
	mu         sync.RWMutex
	triggers   map[string]*Trigger
	handler    ActionHandler
	evaluator  LegacyConditionEvaluator
	stopCh     chan struct{}
	condTicker *time.Ticker
}

// NewRuntime creates a trigger runtime.
func NewRuntime(handler ActionHandler, evaluator LegacyConditionEvaluator) *Runtime {
	return &Runtime{
		triggers:  make(map[string]*Trigger),
		handler:   handler,
		evaluator: evaluator,
		stopCh:    make(chan struct{}),
	}
}

// Start begins the condition-check loop.
func (rt *Runtime) Start() {
	rt.condTicker = time.NewTicker(30 * time.Second)
	go rt.conditionLoop()
}

// Stop halts the trigger runtime.
func (rt *Runtime) Stop() {
	if rt.condTicker != nil {
		rt.condTicker.Stop()
	}
	select {
	case rt.stopCh <- struct{}{}:
	default:
	}
}

// Register adds a trigger.
func (rt *Runtime) Register(t Trigger) string {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if t.ID == "" {
		t.ID = generateID()
	}
	t.CreatedAt = time.Now()
	if !t.Enabled {
		t.Enabled = true
	}
	rt.triggers[t.ID] = &t
	return t.ID
}

// Remove deletes a trigger.
func (rt *Runtime) Remove(id string) bool {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if _, ok := rt.triggers[id]; ok {
		delete(rt.triggers, id)
		return true
	}
	return false
}

// List returns all triggers.
func (rt *Runtime) List() []Trigger {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	out := make([]Trigger, 0, len(rt.triggers))
	for _, t := range rt.triggers {
		out = append(out, *t)
	}
	return out
}

// Get returns a trigger by ID.
func (rt *Runtime) Get(id string) (*Trigger, bool) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	t, ok := rt.triggers[id]
	if !ok {
		return nil, false
	}
	cp := *t
	return &cp, true
}

// SetEnabled enables or disables a trigger.
func (rt *Runtime) SetEnabled(id string, enabled bool) bool {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	t, ok := rt.triggers[id]
	if !ok {
		return false
	}
	t.Enabled = enabled
	return true
}

// Emit dispatches a system event to all matching event triggers.
func (rt *Runtime) Emit(ctx context.Context, payload EventPayload) {
	rt.mu.RLock()
	var matched []*Trigger
	for _, t := range rt.triggers {
		if !t.Enabled || t.Kind != KindEvent {
			continue
		}
		if t.Event != payload.Event && t.Event != EventCustom {
			continue
		}
		if t.EventFilter != "" && !containsSubstring(payload.Text, t.EventFilter) {
			continue
		}
		matched = append(matched, t)
	}
	rt.mu.RUnlock()

	for _, t := range matched {
		rt.fire(ctx, t, &payload)
	}
}

// fire executes a trigger's action.
func (rt *Runtime) fire(ctx context.Context, t *Trigger, payload *EventPayload) {
	rt.mu.Lock()
	now := time.Now()
	t.LastFiredAt = &now
	t.FireCount++
	rt.mu.Unlock()

	if rt.handler != nil {
		if err := rt.handler(ctx, t, payload); err != nil {
			slog.Warn("trigger action failed", "trigger", t.ID, "name", t.Name, "err", err)
		}
	}
}

// conditionLoop periodically checks condition triggers.
func (rt *Runtime) conditionLoop() {
	for {
		select {
		case <-rt.condTicker.C:
			rt.checkConditions()
		case <-rt.stopCh:
			return
		}
	}
}

// checkConditions evaluates all condition triggers.
func (rt *Runtime) checkConditions() {
	if rt.evaluator == nil {
		return
	}

	rt.mu.RLock()
	var toCheck []*Trigger
	for _, t := range rt.triggers {
		if !t.Enabled || t.Kind != KindCondition {
			continue
		}
		if t.ConditionExpr == "" {
			continue
		}
		toCheck = append(toCheck, t)
	}
	rt.mu.RUnlock()

	ctx := context.Background()
	for _, t := range toCheck {
		if rt.evaluator(t.ConditionExpr) {
			rt.fire(ctx, t, &EventPayload{
				Event: EventCustom,
				Text:  "condition met: " + t.ConditionExpr,
			})
		}
	}
}

func containsSubstring(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && (s == sub || findSubstring(s, sub))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

var idCounter atomic.Int64

func generateID() string {
	n := idCounter.Add(1)
	return fmt.Sprintf("trg-%d-%d", time.Now().UnixNano()%1e9, n)
}
