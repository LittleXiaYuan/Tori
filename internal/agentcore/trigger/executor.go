package trigger

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// ──────────────────────────────────────────────
// Executor — 触发器动作执行引擎
//
// 核心职责：
// 1. 执行 5 类动作：创建任务/继续任务/发送消息/调用技能/写记忆
// 2. 记录执行结果和成本
// 3. 预算控制
// 4. 错误处理和重试
// ──────────────────────────────────────────────

// Executor 触发器执行引擎
type Executor struct {
	store *Store

	// 动作执行回调（由 main.go 注入）
	createTask    func(ctx context.Context, tenantID, title, desc string) (taskID string, err error)
	continueTask  func(ctx context.Context, taskID, message string) error
	sendMessage   func(ctx context.Context, channelID, threadID, message string) (messageID string, err error)
	callSkill     func(ctx context.Context, skillName string, args map[string]any) (result string, cost float64, err error)
	writeMemory   func(ctx context.Context, tenantID, content string) error
	updateProfile func(ctx context.Context, tenantID, key, value string) error
}

// NewExecutor 创建执行引擎
func NewExecutor(store *Store) *Executor {
	return &Executor{
		store: store,
	}
}

// ──────────────────────────────────────────────
// 回调注入（由 main.go 调用）
// ──────────────────────────────────────────────

func (e *Executor) SetCreateTask(fn func(ctx context.Context, tenantID, title, desc string) (string, error)) {
	e.createTask = fn
}

func (e *Executor) SetContinueTask(fn func(ctx context.Context, taskID, message string) error) {
	e.continueTask = fn
}

func (e *Executor) SetSendMessage(fn func(ctx context.Context, channelID, threadID, message string) (string, error)) {
	e.sendMessage = fn
}

func (e *Executor) SetCallSkill(fn func(ctx context.Context, skillName string, args map[string]any) (string, float64, error)) {
	e.callSkill = fn
}

func (e *Executor) SetWriteMemory(fn func(ctx context.Context, tenantID, content string) error) {
	e.writeMemory = fn
}

func (e *Executor) SetUpdateProfile(fn func(ctx context.Context, tenantID, key, value string) error) {
	e.updateProfile = fn
}

// ──────────────────────────────────────────────
// 核心执行方法
// ──────────────────────────────────────────────

// Execute 执行触发器
func (e *Executor) Execute(ctx context.Context, trigger *TriggerDef, payload *EventPayload) (*TriggerRun, error) {
	// 1. 检查预算
	if trigger.Budget != nil {
		allowed, reason := trigger.Budget.CheckBudget(time.Now())
		if !allowed {
			run := &TriggerRun{
				TriggerID:     trigger.ID,
				TenantID:      trigger.TenantID,
				Status:        RunStatusSkipped,
				TriggerType:   trigger.Type,
				TriggerSource: e.buildTriggerSource(trigger, payload),
				EventPayload:  payload,
				StartedAt:     time.Now(),
				Error:         fmt.Sprintf("budget exceeded: %s", reason),
			}
			now := time.Now()
			run.FinishedAt = &now
			e.store.CreateRun(run)
			return run, fmt.Errorf("budget exceeded: %s", reason)
		}
	}

	// 2. 创建执行记录
	run := &TriggerRun{
		TriggerID:     trigger.ID,
		TenantID:      trigger.TenantID,
		Status:        RunStatusRunning,
		TriggerType:   trigger.Type,
		TriggerSource: e.buildTriggerSource(trigger, payload),
		EventPayload:  payload,
		StartedAt:     time.Now(),
		ActionResults: make([]ActionResult, 0, len(trigger.Actions)),
	}
	e.store.CreateRun(run)

	// 3. 执行所有动作
	var totalCost float64
	for _, action := range trigger.Actions {
		result := e.executeAction(ctx, trigger, &action, payload)
		run.ActionResults = append(run.ActionResults, result)
		run.ActionsExecuted++

		if result.Status == "success" {
			run.ActionsSucceeded++
		} else {
			run.ActionsFailed++
		}

		totalCost += result.Cost
	}

	// 4. 更新执行记录
	run.TotalCost = totalCost
	now := time.Now()
	run.FinishedAt = &now
	run.Duration = fmt.Sprintf("%.2fs", now.Sub(run.StartedAt).Seconds())

	if run.ActionsFailed > 0 {
		run.Status = RunStatusFailed
	} else {
		run.Status = RunStatusCompleted
	}

	e.store.UpdateRun(run)

	// 5. 更新触发器统计
	trigger.RunCount++
	trigger.LastRunAt = &now
	if run.Status == RunStatusFailed {
		trigger.FailCount++
		trigger.LastError = fmt.Sprintf("%d/%d actions failed", run.ActionsFailed, run.ActionsExecuted)
	}
	e.store.Update(trigger)

	// 6. 记录事件
	e.store.logEvent(TriggerEvent{
		TriggerID: trigger.ID,
		TenantID:  trigger.TenantID,
		EventType: EventTypeExecuted,
		Message:   fmt.Sprintf("Executed %d actions, %d succeeded, %d failed", run.ActionsExecuted, run.ActionsSucceeded, run.ActionsFailed),
		RunID:     run.ID,
	})

	return run, nil
}

// ──────────────────────────────────────────────
// 单个动作执行
// ──────────────────────────────────────────────

func (e *Executor) executeAction(ctx context.Context, trigger *TriggerDef, action *TriggerAction, payload *EventPayload) ActionResult {
	start := time.Now()
	result := ActionResult{
		ActionType: action.Type,
		Status:     "success",
	}

	var err error
	switch action.Type {
	case ActionCreateTask:
		result.Result, err = e.execCreateTask(ctx, trigger, action)
	case ActionContinueTask:
		err = e.execContinueTask(ctx, action)
	case ActionSendMessage:
		result.Result, err = e.execSendMessage(ctx, trigger, action)
	case ActionCallSkill:
		result.Result, result.Cost, err = e.execCallSkill(ctx, action)
	case ActionWriteMemory:
		err = e.execWriteMemory(ctx, trigger, action)
	default:
		err = fmt.Errorf("unknown action type: %s", action.Type)
	}

	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		slog.Warn("trigger action failed", "trigger", trigger.ID, "action", action.Type, "err", err)
	}

	result.Duration = fmt.Sprintf("%.2fs", time.Since(start).Seconds())
	return result
}

func (e *Executor) execCreateTask(ctx context.Context, trigger *TriggerDef, action *TriggerAction) (string, error) {
	if e.createTask == nil {
		return "", fmt.Errorf("createTask callback not configured")
	}
	if action.TaskTitle == "" {
		return "", fmt.Errorf("task_title is required")
	}

	taskID, err := e.createTask(ctx, trigger.TenantID, action.TaskTitle, action.TaskDescription)
	if err != nil {
		return "", err
	}

	slog.Info("trigger created task", "trigger", trigger.ID, "task_id", taskID, "title", action.TaskTitle)
	return taskID, nil
}

func (e *Executor) execContinueTask(ctx context.Context, action *TriggerAction) error {
	if e.continueTask == nil {
		return fmt.Errorf("continueTask callback not configured")
	}
	if action.TaskID == "" {
		return fmt.Errorf("task_id is required")
	}

	err := e.continueTask(ctx, action.TaskID, action.Message)
	if err != nil {
		return err
	}

	slog.Info("trigger continued task", "task_id", action.TaskID)
	return nil
}

func (e *Executor) execSendMessage(ctx context.Context, trigger *TriggerDef, action *TriggerAction) (string, error) {
	if e.sendMessage == nil {
		return "", fmt.Errorf("sendMessage callback not configured")
	}
	if action.Message == "" {
		return "", fmt.Errorf("message is required")
	}

	channelID := trigger.ChannelID
	threadID := trigger.ThreadID

	messageID, err := e.sendMessage(ctx, channelID, threadID, action.Message)
	if err != nil {
		return "", err
	}

	slog.Info("trigger sent message", "trigger", trigger.ID, "channel", channelID, "message_id", messageID)
	return messageID, nil
}

func (e *Executor) execCallSkill(ctx context.Context, action *TriggerAction) (string, float64, error) {
	if e.callSkill == nil {
		return "", 0, fmt.Errorf("callSkill callback not configured")
	}
	if action.SkillName == "" {
		return "", 0, fmt.Errorf("skill_name is required")
	}

	result, cost, err := e.callSkill(ctx, action.SkillName, action.SkillArgs)
	if err != nil {
		return "", cost, err
	}

	slog.Info("trigger called skill", "skill", action.SkillName, "cost", cost)
	return result, cost, nil
}

func (e *Executor) execWriteMemory(ctx context.Context, trigger *TriggerDef, action *TriggerAction) error {
	// 写记忆或更新画像
	if action.ProfileKey != "" {
		// 更新画像
		if e.updateProfile == nil {
			return fmt.Errorf("updateProfile callback not configured")
		}
		err := e.updateProfile(ctx, trigger.TenantID, action.ProfileKey, action.ProfileValue)
		if err != nil {
			return err
		}
		slog.Info("trigger updated profile", "trigger", trigger.ID, "key", action.ProfileKey)
		return nil
	}

	// 写记忆
	if e.writeMemory == nil {
		return fmt.Errorf("writeMemory callback not configured")
	}
	if action.MemoryContent == "" {
		return fmt.Errorf("memory_content is required")
	}

	err := e.writeMemory(ctx, trigger.TenantID, action.MemoryContent)
	if err != nil {
		return err
	}

	slog.Info("trigger wrote memory", "trigger", trigger.ID)
	return nil
}

func (e *Executor) buildTriggerSource(trigger *TriggerDef, payload *EventPayload) string {
	switch trigger.Type {
	case TriggerTypeTime:
		if trigger.TimeConfig != nil && trigger.TimeConfig.CronExpr != "" {
			return fmt.Sprintf("cron:%s", trigger.TimeConfig.CronExpr)
		}
		return "time"
	case TriggerTypeEvent:
		if payload != nil {
			return fmt.Sprintf("event:%s", payload.Event)
		}
		return "event"
	case TriggerTypeCondition:
		if trigger.ConditionConfig != nil {
			return fmt.Sprintf("condition:%s", trigger.ConditionConfig.CheckType)
		}
		return "condition"
	case TriggerTypeCognitive:
		if trigger.CognitiveConfig != nil {
			return fmt.Sprintf("cognitive:%s", trigger.CognitiveConfig.SourceType)
		}
		return "cognitive"
	default:
		return "unknown"
	}
}
