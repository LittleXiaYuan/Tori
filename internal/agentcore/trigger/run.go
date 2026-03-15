package trigger

import (
	"time"
)

// ──────────────────────────────────────────────
// TriggerRun — 执行记录
// ──────────────────────────────────────────────

// TriggerRun 触发器执行记录
type TriggerRun struct {
	ID         string    `json:"id"`
	TriggerID  string    `json:"trigger_id"`
	TenantID   string    `json:"tenant_id"`
	Status     RunStatus `json:"status"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Duration   string    `json:"duration,omitempty"` // 执行时长（如 "1.2s"）

	// 触发上下文
	TriggerType   TriggerType `json:"trigger_type"`   // 触发类型
	TriggerSource string      `json:"trigger_source"` // 触发来源（如 "cron", "event:task_failed", "condition:cost>10"）
	EventPayload  *EventPayload `json:"event_payload,omitempty"` // 事件触发时的 payload

	// 执行结果
	ActionsExecuted int      `json:"actions_executed"` // 执行的动作数
	ActionsSucceeded int     `json:"actions_succeeded"` // 成功的动作数
	ActionsFailed   int      `json:"actions_failed"`    // 失败的动作数
	ActionResults   []ActionResult `json:"action_results"` // 每个动作的执行结果

	// 成本统计
	TotalCost float64 `json:"total_cost,omitempty"` // 本次执行总成本（USD）

	// 错误信息
	Error string `json:"error,omitempty"`
}

// RunStatus 执行状态
type RunStatus string

const (
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusSkipped   RunStatus = "skipped" // 因预算限制等原因跳过
)

// ActionResult 单个动作的执行结果
type ActionResult struct {
	ActionType ActionType `json:"action_type"`
	Status     string     `json:"status"` // "success", "failed", "skipped"
	Result     string     `json:"result,omitempty"` // 执行结果（如 task_id, message_id）
	Error      string     `json:"error,omitempty"`
	Cost       float64    `json:"cost,omitempty"` // 该动作的成本
	Duration   string     `json:"duration,omitempty"`
}

// ──────────────────────────────────────────────
// TriggerEvent — 事件日志
// ──────────────────────────────────────────────

// TriggerEvent 触发器事件日志（用于审计和调试）
type TriggerEvent struct {
	ID        string    `json:"id"`
	TriggerID string    `json:"trigger_id"`
	TenantID  string    `json:"tenant_id"`
	EventType EventType `json:"event_type"`
	Timestamp time.Time `json:"timestamp"`

	// 事件详情
	Message string         `json:"message"`
	Data    map[string]any `json:"data,omitempty"`

	// 关联对象
	RunID  string `json:"run_id,omitempty"`  // 关联的执行记录 ID
	TaskID string `json:"task_id,omitempty"` // 关联的任务 ID
}

// EventType 事件类型
type EventType string

const (
	EventTypeTriggered      EventType = "triggered"       // 触发器被触发
	EventTypeExecuted       EventType = "executed"        // 执行完成
	EventTypeFailed         EventType = "failed"          // 执行失败
	EventTypeSkipped        EventType = "skipped"         // 跳过执行
	EventTypeBudgetExceeded EventType = "budget_exceeded" // 预算超限
	EventTypeEnabled        EventType = "enabled"         // 触发器启用
	EventTypeDisabled       EventType = "disabled"        // 触发器禁用
	EventTypeUpdated        EventType = "updated"         // 触发器更新
	EventTypeDeleted        EventType = "deleted"         // 触发器删除
)

// ──────────────────────────────────────────────
// EventPayload — 系统事件载荷（扩展版）
// ──────────────────────────────────────────────

// EventPayload 系统事件载荷
type EventPayload struct {
	Event EventName      `json:"event"`
	Data  map[string]any `json:"data,omitempty"`
	Text  string         `json:"text,omitempty"` // human-readable description

	// 关联对象（打通核心实体）
	TenantID  string `json:"tenant_id,omitempty"`
	TaskID    string `json:"task_id,omitempty"`
	ThreadID  string `json:"thread_id,omitempty"`
	ChannelID string `json:"channel_id,omitempty"`

	// 时间戳
	Timestamp time.Time `json:"timestamp"`
}

// ──────────────────────────────────────────────
// 辅助方法
// ──────────────────────────────────────────────

// IsTerminal 判断执行状态是否为终态
func (r *TriggerRun) IsTerminal() bool {
	return r.Status == RunStatusCompleted || r.Status == RunStatusFailed || r.Status == RunStatusSkipped
}

// SuccessRate 计算成功率
func (r *TriggerRun) SuccessRate() float64 {
	if r.ActionsExecuted == 0 {
		return 0
	}
	return float64(r.ActionsSucceeded) / float64(r.ActionsExecuted)
}

// CheckBudget 检查预算是否允许执行
func (b *BudgetConfig) CheckBudget(now time.Time) (allowed bool, reason string) {
	if b == nil {
		return true, ""
	}

	// 检查每日次数限制
	if b.MaxRunsPerDay > 0 {
		// TODO: 需要从 Store 查询今日执行次数
		// 这里先简化处理
	}

	// 检查每周次数限制
	if b.MaxRunsPerWeek > 0 {
		// TODO: 需要从 Store 查询本周执行次数
	}

	// 检查成本限制
	if b.MaxTotalCost > 0 && b.CurrentDayCost >= b.MaxTotalCost {
		return false, "daily cost limit exceeded"
	}

	return true, ""
}

