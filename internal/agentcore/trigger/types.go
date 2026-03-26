package trigger

import (
	"time"
)

// ──────────────────────────────────────────────
// P1 Core Types — Unified Trigger System
//
// 核心对象：
// 1. Trigger       — 触发器定义
// 2. TriggerRun    — 执行记录
// 3. TriggerAction — 动作定义
// 4. TriggerEvent  — 事件日志
// 5. BudgetConfig  — 预算控制
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// 1. Trigger — 触发器定义（核心对象）
// ──────────────────────────────────────────────

// TriggerDef 触发器定义（完整版）
type TriggerDef struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Type        TriggerType   `json:"type"`
	Status      TriggerStatus `json:"status"`

	// 关联对象（打通核心实体）
	TenantID  string `json:"tenant_id"`            // 租户隔离
	ThreadID  string `json:"thread_id,omitempty"`  // 关联对话线程
	ChannelID string `json:"channel_id,omitempty"` // 关联渠道（用于 send_message）

	// 触发配置（根据 Type 不同，使用不同字段）
	TimeConfig      *TimeConfig      `json:"time_config,omitempty"`      // 时间触发配置
	EventConfig     *EventConfig     `json:"event_config,omitempty"`     // 事件触发配置
	ConditionConfig *ConditionConfig `json:"condition_config,omitempty"` // 条件触发配置
	CognitiveConfig *CognitiveConfig `json:"cognitive_config,omitempty"` // 认知触发配置

	// 动作列表（一个触发器可以执行多个动作）
	Actions []TriggerAction `json:"actions"`

	// 预算控制
	Budget *BudgetConfig `json:"budget,omitempty"`

	// 元数据
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	LastRunAt  *time.Time `json:"last_run_at,omitempty"`
	NextRunAt  *time.Time `json:"next_run_at,omitempty"` // 仅时间触发器有效
	RunCount   int        `json:"run_count"`              // 执行次数
	FailCount  int        `json:"fail_count"`             // 失败次数
	LastError  string     `json:"last_error,omitempty"`
	CreatedBy  string     `json:"created_by,omitempty"` // user/system/reverie
}

// ──────────────────────────────────────────────
// TimeConfig — 时间触发配置
// ──────────────────────────────────────────────

type TimeConfig struct {
	CronExpr string `json:"cron_expr,omitempty"` // cron 表达式，如 "0 9 * * *"（每天 9 点）
	Interval string `json:"interval,omitempty"`  // 简化间隔，如 "1h", "30m", "1d"
	Timezone string `json:"timezone,omitempty"`  // 时区，默认 UTC
}

// ──────────────────────────────────────────────
// EventConfig — 事件触发配置
// ──────────────────────────────────────────────

type EventConfig struct {
	EventType string            `json:"event_type"` // "task_failed", "task_completed", "knowledge_updated"
	SourceID  string            `json:"source_id,omitempty"` // 事件源 ID（如 task_id, knowledge_source_id）
	Filter    map[string]string `json:"filter,omitempty"` // 事件过滤条件
}

// ──────────────────────────────────────────────
// ConditionConfig — 条件触发配置
// ──────────────────────────────────────────────

type ConditionConfig struct {
	CheckType     string `json:"check_type"`               // "task_status", "cost_threshold", "memory_count"
	TargetID      string `json:"target_id,omitempty"`      // 检查目标 ID
	Operator      string `json:"operator"`                 // "eq", "gt", "lt", "gte", "lte", "contains"
	Value         string `json:"value"`                    // 比较值
	CheckInterval string `json:"check_interval,omitempty"` // 检查间隔，如 "5m"
}

// ──────────────────────────────────────────────
// CognitiveConfig — 认知触发配置
// ──────────────────────────────────────────────

type CognitiveConfig struct {
	SourceType        string   `json:"source_type"`                    // "reverie_insight", "emotion_shift"
	MinSignificance   float64  `json:"min_significance,omitempty"`     // Reverie 最低显著度
	EmotionFrom       string   `json:"emotion_from,omitempty"`         // 情绪变化起点
	EmotionTo         string   `json:"emotion_to,omitempty"`           // 情绪变化终点
	ThoughtCategories []string `json:"thought_categories,omitempty"`   // Reverie 思考类别过滤
}

// ──────────────────────────────────────────────
// TriggerAction — 触发器动作
// ──────────────────────────────────────────────

type TriggerAction struct {
	Type ActionType `json:"type"`

	// 动作参数（根据 Type 不同，使用不同字段）
	TaskTitle       string         `json:"task_title,omitempty"`       // create_task
	TaskDescription string         `json:"task_description,omitempty"` // create_task
	TaskID          string         `json:"task_id,omitempty"`          // continue_task
	Message         string         `json:"message,omitempty"`          // send_message
	SkillName       string         `json:"skill_name,omitempty"`       // call_skill
	SkillArgs       map[string]any `json:"skill_args,omitempty"`       // call_skill
	MemoryContent   string         `json:"memory_content,omitempty"`   // write_memory
	ProfileKey      string         `json:"profile_key,omitempty"`      // write_memory (update_profile)
	ProfileValue    string         `json:"profile_value,omitempty"`    // write_memory (update_profile)
}

// ──────────────────────────────────────────────
// BudgetConfig — 预算控制
// ──────────────────────────────────────────────

type BudgetConfig struct {
	MaxRunsPerDay   int     `json:"max_runs_per_day,omitempty"`   // 每天最多执行次数
	MaxRunsPerWeek  int     `json:"max_runs_per_week,omitempty"`  // 每周最多执行次数
	MaxCostPerRun   float64 `json:"max_cost_per_run,omitempty"`   // 单次执行最大成本（USD）
	MaxTotalCost    float64 `json:"max_total_cost,omitempty"`     // 总成本上限（USD）
	CurrentDayCost  float64 `json:"current_day_cost,omitempty"`   // 当天已消耗成本
	CurrentWeekCost float64 `json:"current_week_cost,omitempty"`  // 本周已消耗成本
	CurrentDayRuns  int     `json:"current_day_runs,omitempty"`   // 当天已执行次数
	CurrentWeekRuns int     `json:"current_week_runs,omitempty"`  // 本周已执行次数
	LastResetAt     time.Time `json:"last_reset_at,omitempty"`    // 上次重置时间
}

