package trigger

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"yunque-agent/internal/agentcore/cron"
)

// ──────────────────────────────────────────────
// Manager — 统一触发器管理器
//
// 整合 4 类触发器：
// 1. 时间触发 → 委托给 cron.Manager
// 2. 事件触发 → 通过 Emit() 分发
// 3. 条件触发 → 定期检查
// 4. 认知触发 → 接收 Reverie/Emotion 事件
// ──────────────────────────────────────────────

// Manager 统一触发器管理器
type Manager struct {
	store    *Store
	executor *Executor
	cronMgr  *cron.Manager

	// 条件检查
	conditionEvaluator ConditionEvaluator
	conditionTicker    *time.Ticker
	stopCh             chan struct{}
}

// ConditionEvaluator 条件评估器
type ConditionEvaluator func(ctx context.Context, config *ConditionConfig) (bool, error)

// NewManager 创建触发器管理器
func NewManager(store *Store, executor *Executor, cronMgr *cron.Manager) *Manager {
	return &Manager{
		store:    store,
		executor: executor,
		cronMgr:  cronMgr,
		stopCh:   make(chan struct{}),
	}
}

// SetConditionEvaluator 设置条件评估器
func (m *Manager) SetConditionEvaluator(fn ConditionEvaluator) {
	m.conditionEvaluator = fn
}

// Start 启动管理器
func (m *Manager) Start() {
	// 启动条件检查循环
	m.conditionTicker = time.NewTicker(30 * time.Second)
	go m.conditionLoop()

	// 注册所有时间触发器到 cron.Manager
	m.registerTimeTriggers()

	slog.Info("trigger manager started")
}

// Stop 停止管理器
func (m *Manager) Stop() {
	if m.conditionTicker != nil {
		m.conditionTicker.Stop()
	}
	select {
	case m.stopCh <- struct{}{}:
	default:
	}
	slog.Info("trigger manager stopped")
}

// ──────────────────────────────────────────────
// 触发器 CRUD（委托给 Store）
// ──────────────────────────────────────────────

func (m *Manager) Create(t *TriggerDef) error {
	if err := m.store.Create(t); err != nil {
		return err
	}

	// 如果是时间触发器，注册到 cron
	if t.Type == TriggerTypeTime && t.Status == TriggerStatusActive {
		m.registerTimeTrigger(t)
	}

	return nil
}

func (m *Manager) Get(id string) (*TriggerDef, bool) {
	return m.store.Get(id)
}

func (m *Manager) Update(t *TriggerDef) error {
	old, ok := m.store.Get(t.ID)
	if !ok {
		return fmt.Errorf("trigger not found: %s", t.ID)
	}

	if err := m.store.Update(t); err != nil {
		return err
	}

	// 如果是时间触发器，更新 cron 注册
	if t.Type == TriggerTypeTime {
		if old.Status == TriggerStatusActive && t.Status != TriggerStatusActive {
			// 禁用：从 cron 移除
			m.cronMgr.Remove(t.ID)
		} else if old.Status != TriggerStatusActive && t.Status == TriggerStatusActive {
			// 启用：注册到 cron
			m.registerTimeTrigger(t)
		}
	}

	return nil
}

func (m *Manager) Delete(id string) error {
	t, ok := m.store.Get(id)
	if ok && t.Type == TriggerTypeTime {
		m.cronMgr.Remove(id)
	}
	return m.store.Delete(id)
}

func (m *Manager) List(tenantID string, filter func(*TriggerDef) bool) []*TriggerDef {
	return m.store.List(tenantID, filter)
}

// ──────────────────────────────────────────────
// 事件触发（核心方法）
// ──────────────────────────────────────────────

// Emit 发送系统事件，触发匹配的触发器
func (m *Manager) Emit(ctx context.Context, payload EventPayload) {
	// 查找匹配的事件触发器
	triggers := m.store.List(payload.TenantID, func(t *TriggerDef) bool {
		if t.Status != TriggerStatusActive {
			return false
		}
		if t.Type != TriggerTypeEvent {
			return false
		}
		if t.EventConfig == nil {
			return false
		}
		// 匹配事件类型
		if t.EventConfig.EventType != string(payload.Event) {
			return false
		}
		// 匹配源 ID（如果指定）
		if t.EventConfig.SourceID != "" {
			if payload.TaskID != t.EventConfig.SourceID {
				return false
			}
		}
		return true
	})

	// 执行匹配的触发器
	for _, t := range triggers {
		go func(trigger *TriggerDef) {
			_, err := m.executor.Execute(ctx, trigger, &payload)
			if err != nil {
				slog.Warn("trigger execution failed", "trigger", trigger.ID, "event", payload.Event, "err", err)
			}
		}(t)
	}

	if len(triggers) > 0 {
		slog.Info("event triggered", "event", payload.Event, "matched_triggers", len(triggers))
	}
}

// ──────────────────────────────────────────────
// 时间触发器注册
// ──────────────────────────────────────────────

func (m *Manager) registerTimeTriggers() {
	triggers := m.store.List("", func(t *TriggerDef) bool {
		return t.Type == TriggerTypeTime && t.Status == TriggerStatusActive
	})

	for _, t := range triggers {
		m.registerTimeTrigger(t)
	}

	slog.Info("registered time triggers", "count", len(triggers))
}

func (m *Manager) registerTimeTrigger(t *TriggerDef) {
	if t.TimeConfig == nil {
		return
	}

	// 构造 cron Schedule
	var schedule cron.Schedule
	if t.TimeConfig.CronExpr != "" {
		schedule = cron.Schedule{
			Type:     cron.ScheduleCron,
			CronExpr: t.TimeConfig.CronExpr,
			Timezone: t.TimeConfig.Timezone,
		}
	} else if t.TimeConfig.Interval != "" {
		// 解析间隔字符串（如 "1h", "30m"）
		duration, err := time.ParseDuration(t.TimeConfig.Interval)
		if err != nil {
			slog.Warn("invalid interval", "trigger", t.ID, "interval", t.TimeConfig.Interval, "err", err)
			return
		}
		schedule = cron.Schedule{
			Type:    cron.ScheduleEvery,
			EveryMs: duration.Milliseconds(),
		}
	} else {
		slog.Warn("time trigger has no cron or interval", "trigger", t.ID)
		return
	}

	// 构造 Payload（使用 systemEvent 类型）
	payload := cron.Payload{
		Kind:    cron.PayloadSystemEvent,
		Message: fmt.Sprintf("Trigger: %s", t.Name),
		Data: map[string]any{
			"trigger_id": t.ID,
			"tenant_id":  t.TenantID,
		},
	}

	// 注册到 cron.Manager
	_, err := m.cronMgr.Add(t.ID, schedule, payload)
	if err != nil {
		slog.Warn("failed to register time trigger", "trigger", t.ID, "err", err)
	} else {
		slog.Info("registered time trigger", "trigger", t.ID, "cron", schedule.CronExpr, "every_ms", schedule.EveryMs)
	}
}

// ──────────────────────────────────────────────
// 条件触发器检查循环
// ──────────────────────────────────────────────

func (m *Manager) conditionLoop() {
	for {
		select {
		case <-m.conditionTicker.C:
			m.checkConditions()
		case <-m.stopCh:
			return
		}
	}
}

func (m *Manager) checkConditions() {
	if m.conditionEvaluator == nil {
		return
	}

	triggers := m.store.List("", func(t *TriggerDef) bool {
		return t.Type == TriggerTypeCondition && t.Status == TriggerStatusActive
	})

	ctx := context.Background()
	for _, t := range triggers {
		if t.ConditionConfig == nil {
			continue
		}

		// 评估条件
		met, err := m.conditionEvaluator(ctx, t.ConditionConfig)
		if err != nil {
			slog.Warn("condition evaluation failed", "trigger", t.ID, "err", err)
			continue
		}

		if met {
			// 条件满足，执行触发器
			go func(trigger *TriggerDef) {
				payload := EventPayload{
					Event:     EventCustom,
					Text:      fmt.Sprintf("Condition met: %s %s %s", trigger.ConditionConfig.CheckType, trigger.ConditionConfig.Operator, trigger.ConditionConfig.Value),
					TenantID:  trigger.TenantID,
					Timestamp: time.Now(),
				}
				_, err := m.executor.Execute(ctx, trigger, &payload)
				if err != nil {
					slog.Warn("condition trigger execution failed", "trigger", trigger.ID, "err", err)
				}
			}(t)
		}
	}
}

// ──────────────────────────────────────────────
// 认知触发器（由 Reverie/Emotion 调用）
// ──────────────────────────────────────────────

// EmitCognitive 发送认知事件（Reverie insight / emotion shift）
func (m *Manager) EmitCognitive(ctx context.Context, sourceType string, data map[string]any) {
	triggers := m.store.List("", func(t *TriggerDef) bool {
		if t.Status != TriggerStatusActive {
			return false
		}
		if t.Type != TriggerTypeCognitive {
			return false
		}
		if t.CognitiveConfig == nil {
			return false
		}
		// 匹配源类型
		if t.CognitiveConfig.SourceType != sourceType {
			return false
		}
		return true
	})

	// 执行匹配的触发器
	for _, t := range triggers {
		go func(trigger *TriggerDef) {
			payload := EventPayload{
				Event:     EventReverieInsight, // 或 EventEmotionShift
				Data:      data,
				TenantID:  trigger.TenantID,
				Timestamp: time.Now(),
			}
			_, err := m.executor.Execute(ctx, trigger, &payload)
			if err != nil {
				slog.Warn("cognitive trigger execution failed", "trigger", trigger.ID, "err", err)
			}
		}(t)
	}

	if len(triggers) > 0 {
		slog.Info("cognitive event triggered", "source_type", sourceType, "matched_triggers", len(triggers))
	}
}

// ──────────────────────────────────────────────
// 统计和查询
// ──────────────────────────────────────────────

// GetRuns 获取执行记录
func (m *Manager) GetRuns(triggerID string, limit int) []*TriggerRun {
	return m.store.ListRuns(triggerID, limit)
}

// GetEvents 获取事件日志
func (m *Manager) GetEvents(triggerID string, limit int) []TriggerEvent {
	return m.store.ListEvents(triggerID, limit)
}
