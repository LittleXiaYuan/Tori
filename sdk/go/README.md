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
Reflection Experience, Mission Parse, Scheduler, Cron System, Triggers, and Plugin API Runtime. It reuses the same small
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
results, err := kit.Plugin.Search(ctx, "incremental SDK package", 5)
err = kit.Memory.Set(ctx, "last_focus", focus)

fmt.Println(focus, strategies, mission.Type, schedulerJobs.Count, len(results))
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
