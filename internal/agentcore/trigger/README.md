# Unified Trigger System — P1 设计文档

## 概述

统一触发器系统是 Yunque Agent 的核心自主行为引擎，让 Agent 能够在各种条件下主动执行动作，而不仅仅是被动响应用户消息。

## 核心对象

### 1. Trigger（触发器定义）

```go
type TriggerDef struct {
    ID          string
    Name        string
    Type        TriggerType  // time/event/condition/cognitive
    Status      TriggerStatus // active/paused/disabled
    TenantID    string       // 租户隔离
    ThreadID    string       // 关联对话线程
    ChannelID   string       // 关联渠道
    
    // 触发配置（根据 Type 选择）
    TimeConfig      *TimeConfig
    EventConfig     *EventConfig
    ConditionConfig *ConditionConfig
    CognitiveConfig *CognitiveConfig
    
    // 动作列表
    Actions []TriggerAction
    
    // 预算控制
    Budget *BudgetConfig
}
```

### 2. TriggerRun（执行记录）

```go
type TriggerRun struct {
    ID              string
    TriggerID       string
    TenantID        string
    Status          RunStatus  // running/completed/failed/skipped
    TriggerType     TriggerType
    TriggerSource   string     // "cron:0 9 * * *", "event:task_failed"
    EventPayload    *EventPayload
    ActionsExecuted int
    ActionsSucceeded int
    ActionsFailed   int
    ActionResults   []ActionResult
    TotalCost       float64
}
```

### 3. TriggerAction（动作定义）

```go
type TriggerAction struct {
    Type ActionType  // create_task/continue_task/send_message/call_skill/write_memory
    
    // 动作参数（根据 Type 选择）
    TaskTitle       string
    TaskDescription string
    TaskID          string
    Message         string
    SkillName       string
    SkillArgs       map[string]any
    MemoryContent   string
    ProfileKey      string
    ProfileValue    string
}
```

### 4. TriggerEvent（事件日志）

```go
type TriggerEvent struct {
    ID        string
    TriggerID string
    TenantID  string
    EventType EventType  // triggered/executed/failed/skipped/budget_exceeded
    Message   string
    Data      map[string]any
    RunID     string
}
```

### 5. BudgetConfig（预算控制）

```go
type BudgetConfig struct {
    MaxRunsPerDay   int
    MaxRunsPerWeek  int
    MaxCostPerRun   float64
    MaxTotalCost    float64
    CurrentDayCost  float64
    CurrentWeekCost float64
}
```

## 4 类触发器

### 1. 时间触发（Time Trigger）

```go
TimeConfig{
    CronExpr: "0 9 * * *",  // 每天 9 点
    Interval: "1h",          // 或每小时
    Timezone: "Asia/Shanghai",
}
```

**用例**：
- 每天早上 9 点发送日报
- 每周一创建周计划任务
- 每小时检查系统状态

### 2. 事件触发（Event Trigger）

```go
EventConfig{
    EventType: "task_failed",
    SourceID:  "task-123",  // 可选：仅监听特定任务
    Filter:    map[string]string{"error_type": "timeout"},
}
```

**用例**：
- 任务失败时自动创建修复任务
- 任务完成时发送通知
- 知识库更新时触发 Reverie 思考

### 3. 条件触发（Condition Trigger）

```go
ConditionConfig{
    CheckType:     "cost_threshold",
    Operator:      "gt",
    Value:         "10",
    CheckInterval: "5m",
}
```

**用例**：
- 成本超过阈值时发送警告
- 任务状态变化时执行动作
- 记忆数量达到上限时清理

### 4. 认知触发（Cognitive Trigger）

```go
CognitiveConfig{
    SourceType:        "reverie_insight",
    MinSignificance:   0.8,
    ThoughtCategories: []string{"concern", "idea"},
}
```

**用例**：
- Reverie 产生高显著度洞察时创建任务
- 用户情绪剧烈变化时发送关怀消息
- Agent 发现重要模式时更新画像

## 5 类动作

### 1. 创建任务（create_task）

```go
TriggerAction{
    Type:            ActionCreateTask,
    TaskTitle:       "Follow-up Task",
    TaskDescription: "Created by trigger",
}
```

### 2. 继续任务（continue_task）

```go
TriggerAction{
    Type:    ActionContinueTask,
    TaskID:  "task-123",
    Message: "Please continue with next step",
}
```

### 3. 发送消息（send_message）

```go
TriggerAction{
    Type:    ActionSendMessage,
    Message: "Daily report is ready",
}
```

### 4. 调用技能（call_skill）

```go
TriggerAction{
    Type:      ActionCallSkill,
    SkillName: "web_search",
    SkillArgs: map[string]any{"query": "AI news"},
}
```

### 5. 写记忆/更新画像（write_memory）

```go
TriggerAction{
    Type:          ActionWriteMemory,
    MemoryContent: "User prefers morning meetings",
}

// 或更新画像
TriggerAction{
    Type:         ActionWriteMemory,
    ProfileKey:   "work_hours",
    ProfileValue: "9am-6pm",
}
```

## 核心模块

### Store（存储）

- `Create/Get/Update/Delete/List` — 触发器 CRUD
- `CreateRun/GetRun/UpdateRun/ListRuns` — 执行记录管理
- `logEvent/ListEvents` — 事件日志
- 持久化到 `data/triggers/triggers.json`

### Executor（执行引擎）

- `Execute(trigger, payload)` — 执行触发器
- `executeAction(action)` — 执行单个动作
- 预算检查
- 成本统计
- 错误处理

### Manager（管理器）

- `Start/Stop` — 启动/停止管理器
- `Emit(payload)` — 发送系统事件
- `EmitCognitive(sourceType, data)` — 发送认知事件
- `registerTimeTriggers()` — 注册时间触发器到 cron
- `conditionLoop()` — 条件检查循环

## 打通核心实体

### 1. Task（任务）

```go
// 事件触发：任务完成时创建后续任务
EventConfig{EventType: "task_completed", SourceID: "task-123"}
Actions: [{Type: ActionCreateTask, TaskTitle: "Next Step"}]

// 条件触发：任务失败时发送警告
ConditionConfig{CheckType: "task_status", TargetID: "task-123", Operator: "eq", Value: "failed"}
Actions: [{Type: ActionSendMessage, Message: "Task failed!"}]
```

### 2. Thread（对话线程）

```go
// 触发器关联线程
TriggerDef{
    ThreadID: "thread-456",
    Actions: [{Type: ActionSendMessage, Message: "Daily summary"}],
}
```

### 3. Tenant（租户）

```go
// 所有触发器都有 TenantID，实现多租户隔离
TriggerDef{TenantID: "tenant-1"}
```

### 4. Channel（渠道）

```go
// 触发器关联渠道，用于发送消息
TriggerDef{
    ChannelID: "telegram",
    Actions: [{Type: ActionSendMessage, Message: "Alert!"}],
}
```

### 5. Budget（预算）

```go
// 每个触发器可以设置预算限制
TriggerDef{
    Budget: &BudgetConfig{
        MaxRunsPerDay: 10,
        MaxTotalCost:  1.0,
    },
}
```

## 使用示例

### 示例 1：每天早上 9 点创建日报任务

```go
trigger := &TriggerDef{
    Name:     "Daily Report",
    Type:     TriggerTypeTime,
    Status:   TriggerStatusActive,
    TenantID: "tenant-1",
    TimeConfig: &TimeConfig{
        CronExpr: "0 9 * * *",
        Timezone: "Asia/Shanghai",
    },
    Actions: []TriggerAction{
        {
            Type:            ActionCreateTask,
            TaskTitle:       "Daily Report",
            TaskDescription: "Generate daily report",
        },
    },
}
```

### 示例 2：任务失败时自动重试

```go
trigger := &TriggerDef{
    Name:     "Auto Retry",
    Type:     TriggerTypeEvent,
    Status:   TriggerStatusActive,
    TenantID: "tenant-1",
    EventConfig: &EventConfig{
        EventType: "task_failed",
    },
    Actions: []TriggerAction{
        {
            Type:    ActionContinueTask,
            TaskID:  "{event.task_id}",  // 从事件中获取
            Message: "Retrying failed task",
        },
    },
}
```

### 示例 3：成本超限时发送警告

```go
trigger := &TriggerDef{
    Name:     "Cost Alert",
    Type:     TriggerTypeCondition,
    Status:   TriggerStatusActive,
    TenantID: "tenant-1",
    ChannelID: "telegram",
    ConditionConfig: &ConditionConfig{
        CheckType:     "cost_threshold",
        Operator:      "gt",
        Value:         "10",
        CheckInterval: "5m",
    },
    Actions: []TriggerAction{
        {
            Type:    ActionSendMessage,
            Message: "⚠️ Daily cost exceeded $10",
        },
    },
}
```

### 示例 4：Reverie 洞察触发任务创建

```go
trigger := &TriggerDef{
    Name:     "Insight to Task",
    Type:     TriggerTypeCognitive,
    Status:   TriggerStatusActive,
    TenantID: "tenant-1",
    CognitiveConfig: &CognitiveConfig{
        SourceType:        "reverie_insight",
        MinSignificance:   0.8,
        ThoughtCategories: []string{"idea", "concern"},
    },
    Actions: []TriggerAction{
        {
            Type:            ActionCreateTask,
            TaskTitle:       "Follow up on insight",
            TaskDescription: "{thought.content}",  // 从思考中获取
        },
    },
}
```

## 下一步（P2）

1. **前端可视化**：创建触发器管理页面
2. **条件评估器**：实现完整的条件表达式引擎
3. **模板系统**：支持动作参数模板（如 `{event.task_id}`）
4. **批量操作**：支持批量启用/禁用/删除触发器
5. **统计分析**：触发器执行统计、成本分析、成功率趋势

