# yunque Python plugin SDK

Lightweight Python SDK for Yunque Agent plugins, sidecars, CLIs, and automation
scripts. Use this `yunque` package when you want small direct helpers instead of
the generated `yunque_client` full OpenAPI surface.

## Environment

```powershell
$env:YUNQUE_API_BASE = "http://localhost:9090"
$env:YUNQUE_PLUGIN_TOKEN = "<plugin-or-api-token>"
$env:YUNQUE_PLUGIN_NAME = "my-python-sidecar"
```


## Agent Kit bundle

Use `create_agent_kit()` when an external script wants the common lightweight
surfaces from one object: State Kernel, Reflection Experience, Mission Parse,
Scheduler, Cron System, Triggers, Memory Kernel, Knowledge Graph, Knowledge Base,
LoRA, Workflow, Connector, Notify, Cost, Providers, Cognis, Trace, Heartbeat, Events, Reverie, and Plugin API Runtime. It reuses the same module-level helpers and does not pull in a
generated OpenAPI client.

```python
import yunque

kit = yunque.create_agent_kit()

focus = kit.state.focus()
strategies = kit.reflect.strategies(tag="sdk", limit=5)
mission = kit.missions.parse("每天八点总结昨天的任务")
scheduler_jobs = kit.scheduler.jobs()
connector_list = kit.connectors.list()
notify_channels = kit.notify.channels()
results = kit.plugin.search("incremental SDK package", limit=5)

kit.memory.set("last_focus", focus)
print(focus, strategies, mission["type"], scheduler_jobs["count"], len(connector_list["connectors"]), len(notify_channels["channels"]), len(results))
```

## Mission Parse helpers

Use `yunque.missions.parse()` when an external script, plugin, or page wants to
turn natural-language intent into a task/workflow/cron/trigger draft without
importing the full platform client.

```python
mission = yunque.missions.parse("每天八点总结昨天的任务")
print(mission["type"], mission["name"], mission.get("config", {}))
```

## Scheduler helpers

Use `yunque.scheduler` when an external script, plugin, or sidecar needs to
list, add, or remove prompt-based recurring jobs.

```python
jobs = yunque.scheduler.jobs()
job = yunque.scheduler.add("hourly", "检查任务", "1h")
removed = yunque.scheduler.remove(job["id"])
print(jobs["count"], job["id"], removed["status"])
```

## State Kernel helpers

```python
import yunque

snapshot = yunque.state.snapshot()
print(snapshot["focus"])

for goal in yunque.state.goals():
    print(goal["title"])

print(yunque.state.actions())
print(yunque.state.capabilities())
print(yunque.state.focus())
print(yunque.state.resources())

saved = yunque.state.save_goal({
    "title": "Ship a small Python SDK state slice",
    "priority": 2,
})
```

## Reflection Experience helpers

```python
import yunque

experiences = yunque.reflect.experiences(
    q="code review",
    tag="quality:9",
    limit=5,
)

stats = yunque.reflect.stats(tag="quality:9")
strategies = yunque.reflect.strategies(tag="quality:9", limit=3)
print(stats["total"])
print(strategies)
```

This slice is intended for automation scripts and plugins that want to reuse
agent lessons / strategy hints without depending on the full platform client.

## Runnable state snapshot example

Start `yunque-agent` first so `/v1/state` is reachable, then run from the repo
root:

```powershell
python sdk/python/examples/state_snapshot.py
python sdk/python/examples/reflect_strategies.py
```



## Cron System 主机计划任务切片

Python 插件脚本或自动化任务可以用 `yunque.cron_system` 管理主机级 `/v1/cron/*` 计划任务。它不同于 `yunque.cron` 的插件自有定时任务。

```python
jobs = yunque.cron_system.list()
added = yunque.cron_system.add(
    "nightly",
    {"type": "cron", "cron_expr": "0 2 * * *", "timezone": "Asia/Shanghai"},
    {"kind": "systemEvent", "data": {"event": "nightly"}},
)
run = yunque.cron_system.run(added["job"]["id"])
print(len(jobs["jobs"]), run["run"]["status"])
```


### Memory Kernel 宿主回忆记忆切片

Python 插件脚本或自动化任务可以用 `yunque.memory_core` 访问宿主 `/v1/memory/*` 回忆记忆层。它不同于 `yunque.memory` 的插件私有 KV。

```python
stats = yunque.memory_core.stats()
found = yunque.memory_core.search("用户偏好", limit=3)
added = yunque.memory_core.remember("用户偏好中文回复", layer="mid", source="sdk")
print(stats.get("mid"), found["count"], added["status"])
```


### Knowledge Graph 知识图谱切片

Python 插件脚本或自动化任务可以用 `yunque.graph` 访问宿主 `/v1/graph/*` 知识图谱层，读取/维护实体、关系和图谱上下文。

```python
entities = yunque.graph.entities("云雀")
entity = yunque.graph.put_entity({"name": "云雀", "type": "agent"})
context = yunque.graph.context_by_entity_id(entity["id"])
print(len(entities["entities"]), context["context"])
```


### Knowledge Base 宿主 RAG 知识库切片

Python 插件脚本或自动化任务可以用 `yunque.knowledge_base` 访问宿主 `/v1/knowledge/*` RAG 知识库。它不同于 `yunque.knowledge` 的插件运行时 knowledge helper。

```python
stats = yunque.knowledge_base.stats()
found = yunque.knowledge_base.search("增量 SDK", limit=3)
ingested = yunque.knowledge_base.ingest("外部项目可直接调用 Knowledge Base", name="sdk-note")
print(stats.get("sources"), found["count"], ingested["source"]["id"])
```


### LoRA lifecycle 宿主训练进化切片

Python 脚本、插件处理器或自动化任务可以用 `yunque.lora` 访问宿主 `/v1/lora/*` 本地脑训练生命周期能力。

```python
status = yunque.lora.status()
preview = yunque.lora.preview("default")
triggered = yunque.lora.trigger("default")
print(status.get("active_model"), preview["preview"], triggered["status"])
```


### Workflow orchestration 工作流编排切片

Python 脚本、插件处理器或自动化任务可以用 `yunque.workflows` 调用宿主 `/v1/workflows*`。

```python
defs = yunque.workflows.list()
run = yunque.workflows.run("wf_1", {"topic": "sdk"})
instances = yunque.workflows.instances()
print(defs["total"], run["instance_id"], instances["total"])
```

### Connectors runtime 连接器运行时切片

Python 脚本、插件处理器或自动化任务可以用 `yunque.connectors` 读取连接器目录、连接/断开外部服务并执行连接器动作。

```python
catalog = yunque.connectors.list()
detail = yunque.connectors.detail("github")
result = yunque.connectors.execute("github", "create_issue", {"title": "SDK"})
print(len(catalog["connectors"]), detail["status"], result["ok"])
```

### Notify runtime 通知运行时切片

Python 脚本、插件处理器或自动化任务可以用 `yunque.notify` 管理通知渠道、发送测试通知并分享任务/会话产物。

```python
channels = yunque.notify.channels()
shared = yunque.notify.share("feishu-main", message="任务完成", task_id="task_1")
print(len(channels["channels"]), shared["ok"])
```

## Triggers 触发器自动化切片

Python 插件脚本或自动化任务可以用 `yunque.triggers` 管理 Triggers v2 定义、触发事件并读取运行/事件记录。

```python
defs = yunque.triggers.list(status="enabled")
created = yunque.triggers.create({
    "name": "review done",
    "tenant_id": "default",
    "type": "event",
    "actions": [{"kind": "notify"}],
})
yunque.triggers.emit("review.done", data={"task_id": "task_1"})
runs = yunque.triggers.runs(trigger_id=created["id"], limit=10)
print(defs["total"], runs["total"])
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

`yunque.iterate` exposes the self-iteration proposal review loop as a small incremental SDK slice: `proposals()`, `pending_proposals()`, `approve(id)`, `reject(id)`, `trigger()`, and `status()` call `/api/iterate/proposals`, `/api/iterate/approve`, `/api/iterate/reject`, `/api/iterate/trigger`, and `/api/iterate/status`. `create_agent_kit().iterate` reuses the same helpers for scripts and plugins.
### Persona SDK

`yunque.persona` exposes persona identity, soul, skills, and presets as a small incremental SDK slice. Use `get()`, `update()`, `skills()`, `add_skill()`, `delete_skill()`, `presets()`, `switch_preset()`, `add_custom_preset()`, `delete_custom_preset()`, and `update_preset_features()` over `/v1/persona*`; `create_agent_kit().persona` reuses the same helpers.




### Tasks SDK

The lightweight Tasks SDK exposes task CRUD and lifecycle helpers for external plugin UIs, front-end task pages, CLIs, sidecars, and automation scripts. Use it to list, read, create, run, pause, resume, restart, cancel, and delete `/v1/tasks` records, plus list/get/create/delete/instantiate task templates, inspect/resolve task gaps, read task working memory, interact with task threads, and inspect task trace events, without importing the full platform client or coupling to the backend console.
### Permissions SDK

The lightweight Permissions SDK exposes permission checks and current-role reads for external plugin UIs, front-end pages, CLIs, sidecars, and automation guard scripts. Use it to call `/v1/rbac/check` and `/v1/rbac/my-roles` without pulling in the broader RBAC governance client.

### Reactions SDK

The lightweight Reactions SDK exposes emoji reactions and sticker sending for external plugin UIs, front-end pages, CLIs, and automation scripts. Use it to call `/v1/react` and `/v1/sticker/send` without pulling in the full platform backend.

### Instructions SDK

The lightweight Instructions SDK exposes user instructions and instruction CRUD for external plugin UIs, front-end admin pages, CLIs, and automation scripts. Use it to list, create, update, delete, and reorder `/v1/instructions` records without pulling in the full platform backend.

### Emotion SDK

The lightweight Emotion SDK exposes emotion history and emotion stickers for external plugin UIs, front-end admin pages, CLIs, and automation scripts. Use it to read `/v1/emotion/history`, export `/v1/emotion/stickers`, register sticker mappings, or clear stale mappings without pulling in the full platform backend.


### System SDK

The lightweight System SDK exposes public health/readiness probes, version/SBOM metadata, authenticated system info/stats, metrics, cache stats, and module observability for deployment monitors, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform client. It maps directly to `/healthz`, `/livez`, `/readyz`, `/healthz/cognitive`, `/v1/version`, `/v1/system/info`, `/v1/system/stats`, `/v1/metrics`, `/v1/metrics/prometheus`, `/v1/cache/stats`, `/v1/modules`, and `/sbom`.

### Auth SDK

The lightweight Auth SDK exposes auth status, password login/setup, API-key-to-JWT token exchange, and Tori OAuth start URL helpers for external plugin UIs, front-end setup pages, CLIs, sidecars, and automation scripts without importing the full platform client. It maps directly to `/v1/auth/status`, `/v1/auth/login`, `/v1/auth/set-password`, `/v1/token`, and `/v1/auth/oauth/tori`.
