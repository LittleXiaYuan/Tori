# Yunque Go SDK

Lightweight Go SDK for Yunque Agent plugins, sidecars, CLIs, and automation scripts.
It intentionally exposes small typed namespaces instead of forcing external code to
link against the full platform internals.

## Environment

The SDK reads the same environment variables used by plugin processes:

```powershell
$env:YUNQUE_API_BASE = "http://localhost:9090"
$env:YUNQUE_PLUGIN_TOKEN = "<plugin-or-api-token>"
$env:YUNQUE_PLUGIN_NAME = "my-state-sidecar"
```

`YUNQUE_API_BASE` defaults to `http://localhost:9090` when omitted.

## Agent Kit bundle

Use `yunque.NewAgentKit()` when an external Go sidecar, CLI, or automation
binary wants the common lightweight surfaces from one object: State Kernel,
Reflection Experience, Mission Parse, Scheduler, Cron System, Triggers, Memory Kernel,
Knowledge Graph, Knowledge Base, LoRA, Workflow, Connector, Notify, Cost, Providers, Cognis, Trace, Heartbeat, Events, Reverie, and Plugin API Runtime. It reuses the same small
namespaces and does not require a generated full OpenAPI client.

```go
kit := yunque.NewAgentKit()

focus, err := kit.State.Focus(ctx)
strategies, err := kit.Reflect.StrategiesWithOptions(ctx, yunque.ReflectStrategyOptions{
    Tag:   "sdk",
    Limit: 5,
})
mission, err := kit.Missions.Parse(ctx, "每天八点总结昨天的任务")
schedulerJobs, err := kit.Scheduler.Jobs(ctx)
connectorList, err := kit.Connectors.List(ctx)
notifyChannels, err := kit.Notify.Channels(ctx)
results, err := kit.Plugin.Search(ctx, "incremental SDK package", 5)
err = kit.Memory.Set(ctx, "last_focus", focus)

fmt.Println(focus, strategies, mission.Type, schedulerJobs.Count, len(connectorList.Connectors), len(notifyChannels.Channels), len(results))
```

## Mission Parse helpers

Use `yunque.Missions.Parse` when a page, plugin, CLI, or automation binary needs
to turn natural-language intent into a task/workflow/cron/trigger draft without
depending on platform internals.

```go
mission, err := yunque.Missions.Parse(ctx, "每天八点总结昨天的任务")
fmt.Println(mission.Type, mission.Name, mission.Config["cron_expr"])
```

## Scheduler helpers

Use `yunque.Scheduler` when an external Go sidecar, CLI, or automation binary
needs to list, add, or remove prompt-based recurring jobs.

```go
jobs, err := yunque.Scheduler.Jobs(ctx)
job, err := yunque.Scheduler.Add(ctx, "hourly", "检查任务", "1h")
removed, err := yunque.Scheduler.Remove(ctx, job.ID)
fmt.Println(jobs.Count, job.ID, removed.Status)
```

## State Kernel incremental helpers

Use `yunque.State` when an external project only needs the agent's current
state layer: goals, tracked resources, focus, recent actions, and capability
summary.

```go
package main

import (
    "context"
    "fmt"
    "log"

    "yunque-agent/sdk/go/yunque"
)

func main() {
    ctx := context.Background()

    snap, err := yunque.State.Snapshot(ctx)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("focus:", snap.Focus)
    fmt.Println("goals:", len(snap.Goals))
    fmt.Println("resources:", len(snap.Resources))
    fmt.Println("recent actions:", len(snap.RecentActions))
    fmt.Println("skills:", snap.Capabilities.TotalSkills)
}
```

Focused helpers are available when callers want smaller intent-revealing access:

```go
goals, _ := yunque.State.Goals(ctx)
actions, _ := yunque.State.Actions(ctx)
caps, _ := yunque.State.Capabilities(ctx)
focus, _ := yunque.State.Focus(ctx)
resources, _ := yunque.State.Resources(ctx)
```

Goal mutation is typed but still narrow:

```go
saved, err := yunque.State.SaveGoal(ctx, yunque.StateGoal{
    Title:    "Ship a small SDK slice",
    Priority: 2,
})
```

## Runnable state snapshot example

Start `yunque-agent` first so `/v1/state` is reachable, then run from the repo root:

```powershell
go run ./sdk/go/examples/state_snapshot
```

The example prints a concise state summary as JSON, which is suitable for shell
scripts, dashboards, and CI smoke checks.

## Reflection Experience helpers

Use `yunque.Reflect` when a plugin, sidecar, or automation tool wants to reuse
agent lessons and strategy hints without importing platform internals.

```go
experiences, err := yunque.Reflect.Experiences(ctx, yunque.ReflectExperienceOptions{
    Query: "code review",
    Tag:   "quality:9",
    Limit: 5,
})

stats, err := yunque.Reflect.Stats(ctx, yunque.ReflectExperienceOptions{
    Tag: "quality:9",
})

strategies, err := yunque.Reflect.StrategiesWithOptions(ctx, yunque.ReflectStrategyOptions{
    Tag:   "quality:9",
    Limit: 3,
})
```



### Cron System 主机计划任务切片

Go 插件、sidecar、CLI 或自动化二进制可以用 `yunque.CronSystem` 管理主机级 `/v1/cron/*` 计划任务。它不同于 `yunque.Cron` 的插件自有定时任务。

```go
jobs, err := yunque.CronSystem.List(ctx)
added, err := yunque.CronSystem.Add(ctx, yunque.CronAddRequest{
    Name: "nightly",
    Schedule: yunque.CronSchedule{Type: "cron", CronExpr: "0 2 * * *", Timezone: "Asia/Shanghai"},
    Payload: yunque.CronPayload{Kind: "systemEvent", Data: map[string]any{"event": "nightly"}},
})
run, err := yunque.CronSystem.Run(ctx, added.Job.ID)
fmt.Println(len(jobs.Jobs), added.Job.ID, run.Run.Status)
```

#
### Memory Kernel 宿主回忆记忆切片

Go 插件、sidecar、CLI 或自动化二进制可以用 `yunque.MemoryCore` 访问宿主 `/v1/memory/*` 回忆记忆层。它不同于 `yunque.Memory` 的插件私有 KV。

```go
stats, err := yunque.MemoryCore.Stats(ctx)
found, err := yunque.MemoryCore.Search(ctx, yunque.MemorySearchRequest{Query: "用户偏好", Limit: 3})
added, err := yunque.MemoryCore.Add(ctx, yunque.MemoryAddRequest{Value: "用户偏好中文回复", Layer: "mid", Source: "sdk"})
fmt.Println(stats["mid"], found.Count, added.Status)
```


### Knowledge Graph 知识图谱切片

Go 插件、sidecar、CLI 或自动化二进制可以用 `yunque.Graph` 访问宿主 `/v1/graph/*` 知识图谱层，读取/维护实体、关系和图谱上下文。

```go
entities, err := yunque.Graph.Entities(ctx, "云雀")
entity, err := yunque.Graph.PutEntity(ctx, yunque.GraphEntity{Name: "云雀", Type: "agent"})
context, err := yunque.Graph.ContextByEntityID(ctx, entity.ID)
fmt.Println(len(entities.Entities), context.Context)
```


### Knowledge Base 宿主 RAG 知识库切片

Go 插件、sidecar、CLI 或自动化二进制可以用 `yunque.KnowledgeKB` 访问宿主 `/v1/knowledge/*` RAG 知识库。它不同于 `yunque.Knowledge` 的插件运行时 knowledge helper。

```go
stats, err := yunque.KnowledgeKB.Stats(ctx)
found, err := yunque.KnowledgeKB.Search(ctx, yunque.KnowledgeSearchOptions{Query: "增量 SDK", Limit: 3})
ingested, err := yunque.KnowledgeKB.Ingest(ctx, yunque.KnowledgeIngestRequest{Name: "sdk-note", Content: "外部项目可直接调用 Knowledge Base"})
fmt.Println(stats["sources"], found.Count, ingested.Source.ID)
```


### LoRA lifecycle 宿主训练进化切片

Go sidecar、CLI 或自动化二进制可以用 `yunque.LoRA` 访问宿主 `/v1/lora/*` 本地脑训练生命周期能力：状态、历史、预览、触发、回滚、进化和配置。

```go
status, err := yunque.LoRA.Status(ctx)
preview, err := yunque.LoRA.Preview(ctx, yunque.LoRAPreviewOptions{TenantID: "default"})
triggered, err := yunque.LoRA.Trigger(ctx, yunque.TriggerLoRARequest{TenantID: "default"})
fmt.Println(status["active_model"], preview["preview"], triggered["status"])
```


### Workflow orchestration 工作流编排切片

Go sidecar、CLI 或自动化二进制可以用 `yunque.Workflows` 访问宿主 `/v1/workflows*` DAG 工作流定义与实例能力。

```go
defs, err := yunque.Workflows.List(ctx)
run, err := yunque.Workflows.Run(ctx, yunque.WorkflowRunRequest{DefinitionID: "wf_1", Variables: map[string]any{"topic": "sdk"}})
instances, err := yunque.Workflows.Instances(ctx)
fmt.Println(defs.Total, run.InstanceID, instances.Total)
```

### Connectors runtime 连接器运行时切片

Go sidecar、CLI 或自动化二进制可以用 `yunque.Connectors` 读取连接器目录、连接/断开外部服务并执行连接器动作。

```go
catalog, err := yunque.Connectors.List(ctx)
detail, err := yunque.Connectors.Detail(ctx, "github")
executed, err := yunque.Connectors.Execute(ctx, yunque.ConnectorExecuteRequest{
    ConnectorID: "github",
    ActionID:    "create_issue",
    Params:      map[string]any{"title": "SDK"},
})
fmt.Println(len(catalog.Connectors), detail.Status, executed.OK)
```

### Notify runtime 通知运行时切片

Go sidecar、CLI 或自动化二进制可以用 `yunque.Notify` 管理通知渠道、发送测试通知并分享任务/会话产物。

```go
channels, err := yunque.Notify.Channels(ctx)
shared, err := yunque.Notify.Share(ctx, yunque.NotifyShareRequest{
    ChannelID: "feishu-main",
    Message:   "任务完成",
    TaskID:    "task_1",
})
fmt.Println(len(channels.Channels), shared.OK)
```

## Triggers 触发器自动化切片

Go 插件、sidecar、CLI 或自动化二进制可以用 `yunque.Triggers` 管理 Triggers v2 定义、触发事件并读取运行/事件记录。

```go
defs, err := yunque.Triggers.List(ctx, yunque.TriggerListOptions{Status: "enabled"})
created, err := yunque.Triggers.Create(ctx, yunque.TriggerDef{
    Name: "review done", TenantID: "default", Type: "event",
    Actions: []any{map[string]any{"kind": "notify"}},
})
emitted, err := yunque.Triggers.Emit(ctx, yunque.TriggerPayload{
    Event: "review.done", Data: map[string]any{"task_id": "task_1"},
})
runs, err := yunque.Triggers.Runs(ctx, yunque.TriggerHistoryOptions{TriggerID: created.ID, Limit: 10})
fmt.Println(defs.Total, emitted.Status, runs.Total)
```


### Projects SDK

Projects SDK exposes lightweight project workspace CRUD helpers (`list`, `create`, `detail`, `update`, `remove`) for external pages, plugins, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.projects` / `kit.Projects` for one-stop automation composition.


### Skill Market SDK

Skill Market SDK exposes lightweight marketplace helpers (`search`, `top`, `stats`) for external pages, plugins, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.market` / `kit.Market` for skill discovery inside one-stop automation composition.


### Dispatch SDK

Dispatch SDK exposes lightweight MCP worker and queue helpers (`workers`, `worker`, `removeWorker`, `queue`, `enqueue`, `workerConfig`) for external pages, plugins, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.dispatch` / `kit.Dispatch` for one-stop worker orchestration.


### Orchestrator SDK

Orchestrator SDK exposes lightweight IDE worker daemon helpers (`status`, `toggle`, `sessions`, `detectIDEs`, `events`, `taskTimeline`, `policy`, `updatePolicy`, `addAdapter`) for external pages, plugins, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.orchestrator` / `kit.Orchestrator` for one-stop IDE worker orchestration.


### Providers SDK

Providers SDK exposes lightweight LLM provider and model helpers (`models`, `addModel`, `deleteModel`, `list`, `test`, `enable`, `disable`, `switchModel`, `setSession`, `mode`, `setMode`, `presets`, `register`, `delete`, `discoverLocal`, `registerLocal`, `discoverTori`, `exec`, `setExec`, `resetBreakers`) for external setup pages, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.providers` / `kit.Providers` for one-stop model configuration and runtime routing workflows.

### Cognis SDK

Cognis SDK exposes lightweight Cogni registry, trace, health, experience, evolution, workflow, bundle, and federation helpers (`list`, `create`, `traces`, `experience`, `evolve`, `federation`, `exportBundle`, `importBundle`) for external pages, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.cognis` / `kit.Cognis` for one-stop CogniKernel and multi-cogni automation workflows.

### Trace SDK

Trace SDK exposes lightweight execution/audit trace helpers (`recent`, `byTraceId`, `byTaskId`) for external debugging pages, replay tools, plugins, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.trace` / `kit.Trace` for one-stop observability and replay workflows.

### Heartbeat SDK

Heartbeat SDK exposes lightweight proactive lifecycle helpers (`status`, `update`, `trigger`, `logs`) for external operator pages, plugins, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.heartbeat` / `kit.Heartbeat` for one-stop lifecycle supervision workflows.

### Events SDK

Events SDK exposes lightweight Server-Sent Events helpers (`stream`, `parse`) for external dashboards, plugin UIs, CLIs, sidecars, and automation scripts that need live task/workflow/approval/runtime updates without importing the full platform. Agent Kit also exposes this surface as `kit.events` / `kit.Events` for one-stop live observability workflows.

### Runtime SDK

Runtime SDK exposes lightweight `/v1/sessions/queue` and `/v1/events/stream` helpers (`queues`, `sessionQueue`, `cancelQueuedTask`, `events`) for external runtime dashboards, plugin UIs, CLIs, sidecars, and automation monitors without importing the full platform. Agent Kit also exposes this surface as `kit.runtime` / `kit.Runtime`.

### Subagents SDK

Subagents SDK exposes lightweight `/v1/subagent` and `/v1/subagent/message` helpers (`list`, `get`, `spawn`, `destroy`, `appendMessages`) for external operator pages, plugin UIs, CLIs, sidecars, and automation scripts to orchestrate specialist agents without importing the full platform. Agent Kit also exposes this surface as `kit.subagents` / `kit.Subagents`.

### Tools SDK

Tools SDK exposes lightweight `/v1/tools/*` helpers (`exec`, `list`, `poll`, `kill`) for external operator pages, plugin UIs, CLIs, sidecars, and automation scripts to observe and control server-side tool process sessions through the existing authenticated guardrails. Agent Kit also exposes this surface as `kit.tools` / `kit.Tools`.

### Audit SDK

Audit SDK exposes lightweight `/v1/audit/*` and `/api/audit/trail` helpers (`tail`, `verify`, `stats`, `trail`) for external compliance pages, plugin UIs, CLIs, sidecars, and automation scripts to inspect audit-chain integrity and task audit trails without importing the full platform. Agent Kit also exposes this surface as `kit.audit` / `kit.Audit`.

### Trust SDK

Trust SDK exposes lightweight `/api/trust/*`, `/api/review/status`, and `/api/skillgrow/patterns` helpers (`scores`, `reset`, `grant`, `grantAll`, `reviewStatus`, `skillGrowPatterns`) for external admin pages, plugin UIs, CLIs, sidecars, and automation scripts to inspect and operate trust governance without importing the full platform. Agent Kit also exposes this surface as `kit.trust` / `kit.Trust`.

### Reverie SDK

Reverie SDK exposes lightweight proactive thought-loop helpers (`journal`, `stats`, `config`, `updateConfig`, `think`, `deleteThought`, `actions`, `targets`) for external operator pages, plugins, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.reverie` / `kit.Reverie` for one-stop proactive reflection and delivery workflows.

### Chat SDK

Chat SDK exposes lightweight `/v1/chat`, `/v1/chat/stream`, and `/v1/chat/agentic` helpers (`send`, `stream`, `agentic`, `parseStream`) for external chat panes, plugin UIs, CLIs, sidecars, desktop widgets, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.chat` / `kit.Chat`.

### Conversations SDK

Conversations SDK exposes lightweight `/v1/conversations` helpers (`list`, `messages`, `deleteMessages`, `manage`, `rename`, `pin`, `archive`, `replay`) for external chat panes, audit/replay tools, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.conversations` / `kit.Conversations`.

### Realtime SDK

Realtime SDK exposes lightweight `/v1/ws` helpers (`wsUrl`, `connect`, `ping`, `chat`, `send`/`serialize`, `parse`) for external chat panes, desktop widgets, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.realtime` / `kit.Realtime`.

### Cost SDK

Cost SDK exposes lightweight cost governance helpers (`summary`, `setBudget`, `task`, `taskTimeline`, `breakdown`, `history`, `alerts`, `usage`, `setQuota`) for external pages, plugins, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.cost` / `kit.Cost` for one-stop budget, usage, quota, and cost observability workflows.

### Fork SDK

Fork SDK exposes lightweight conversation branch helpers (`root`, `get`, `create`, `remove`, `branch`, `list`) for external pages, plugins, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.fork` / `kit.Fork` for one-stop conversation exploration and rollback-safe alternate-path workflows.

### Approvals SDK

Approvals SDK exposes lightweight `/v1/approvals` helpers (`list`, `pending`, `history`, `approve`, `deny`, `decide`, `rules`, `addRule`, `deleteRule`) for external approval desks, plugin UIs, CLIs, sidecars, and automation guard scripts without importing the full platform. Agent Kit also exposes this surface as `kit.approvals` / `kit.Approvals`.

### RBAC SDK

RBAC SDK exposes lightweight `/v1/rbac` helpers (`roles`, `createRole`, `deleteRole`, `assignRole`, `revokeRole`, `check`, `myRoles`) for external admin pages, plugin UIs, CLIs, sidecars, and automation guard scripts without importing the full platform. Agent Kit also exposes this surface as `kit.rbac` / `kit.RBAC`.

### Files SDK

Files SDK exposes lightweight `/api/files` helpers (`list`, `preview`, `download`) for external artifact panes, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.files` / `kit.Files`.

### Browser SDK

Browser SDK exposes lightweight `/v1/browser` and `/api/browser/ext` helpers (`status`, `config`, `navigate`, `screenshot`, `latestScreenshot`, `ocr`, `oppPending`, `oppDecide`, `extensionStatus`, `extensionSession`, `extensionAction`, `scenarios`, `runScenario`) for external browser task panes, plugin UIs, extension bridges, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.browser` / `kit.Browser`.

### Iterate SDK

`yunque.Iterate` keeps self-iteration proposal review available to Go sidecars and CLIs without a generated platform client. Use `Proposals`, `PendingProposals`, `Approve`, `Reject`, `Trigger`, and `Status` for `/api/iterate/proposals`, approval/rejection, manual cycle triggering, and status reads; `NewAgentKit().Iterate` points to the same lightweight namespace.
