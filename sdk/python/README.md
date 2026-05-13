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
LoRA, Workflow, Connector, Notify, Cost, Providers, Cognis, Trace, Heartbeat, Events, Reverie, Tori, Upload, Speech, Setup, and Plugin API Runtime. It reuses the same module-level helpers and does not pull in a
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

### RuntimeQueue SDK

RuntimeQueue SDK exposes queue-only runtime helpers (`overview`, `session`, `cancel`) for dashboards, plugin UIs, CLIs, sidecars, and automation monitors without importing the full platform or broader Runtime operations slice. It maps directly to `/v1/sessions/queue` and `/v1/sessions/queue/cancel`; Agent Kit also exposes this surface as `kit.runtimeQueue` / `kit.RuntimeQueue`.


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

### SkillGrow SDK

SkillGrow SDK exposes the focused skill-growth pattern reader (`patterns`) for external plugin UIs, admin panels, CLIs, sidecars, and automation scripts without importing the full platform or broader Trust governance slice. It maps directly to `/api/skillgrow/patterns`; Agent Kit also exposes this surface as `kit.skillgrow` / `kit.SkillGrow`.


### Review SDK

Review SDK exposes the focused review-gate status reader (`status`) for frontends, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform or broader Trust governance slice. It maps directly to `/api/review/status`; Agent Kit also exposes this surface as `kit.review` / `kit.Review`.


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




### Setup SDK

The lightweight Setup SDK exposes first-run setup detection, provider health, setup templates, provider connectivity testing, template apply, and optional component installation helpers for external setup pages, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform client. It maps directly to `/v1/setup/detect`, `/v1/setup/health`, `/v1/setup/templates`, `/v1/setup/test-provider`, `/v1/setup/apply`, and `/v1/setup/install-component`.

### Upload SDK

Upload SDK exposes standalone artifact upload helpers (`file` / `upload`) for external frontends, plugin pages, CLIs, sidecars, and automation scripts without importing the full platform or the broader Speech SDK. It maps directly to `/v1/upload` and returns parsed-file metadata plus optional analysis/actions/rich payloads from the gateway. Agent Kit also exposes this surface as `kit.upload` / `kit.Upload`.


### Speech SDK

The lightweight Speech SDK exposes speech TTS, speech STT, voice/provider listing, STT stream URL construction, and file upload helpers for external voice UIs, desktop widgets, plugin pages, CLIs, sidecars, and automation scripts without importing the full platform client. It maps directly to `/v1/speech/tts`, `/v1/speech/stt`, `/v1/speech/voices`, `/v1/speech/stt/stream`, and `/v1/upload`.

### Tori SDK

The lightweight Tori SDK exposes Tori account bind/status/unbind, bound-instance health, and usage-summary helpers for external setup pages, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform client. It maps directly to `/v1/tori/bind`, `/v1/tori/status`, `/v1/tori/unbind`, `/v1/tori/health`, and `/v1/tori/usage`.

### Backup SDK

The lightweight Backup SDK exposes backup archive info, ZIP export, and ZIP import/restore helpers for external operator UIs, CLIs, sidecars, and automation scripts without importing the full platform client. It maps directly to `/v1/backup/info`, `/v1/backup/export`, and `/v1/backup/import`.

### Settings SDK

The lightweight Settings SDK exposes settings schema/config reads, runtime config updates, setup checks, hot config reload, and host directory detection for external setup pages, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform client. It maps directly to `/api/settings/schema`, `/api/settings/config`, `/api/settings/check`, `/v1/config/reload`, and `/api/settings/detect-dirs`.

### System SDK

The lightweight System SDK exposes public health/readiness probes, version/SBOM metadata, authenticated system info/stats, metrics, cache stats, and module observability for deployment monitors, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform client. It maps directly to `/healthz`, `/livez`, `/readyz`, `/healthz/cognitive`, `/v1/version`, `/v1/system/info`, `/v1/system/stats`, `/v1/metrics`, `/v1/metrics/prometheus`, `/v1/cache/stats`, `/v1/modules`, and `/sbom`.

### Auth SDK

The lightweight Auth SDK exposes auth status, password login/setup, API-key-to-JWT token exchange, and Tori OAuth start URL helpers for external plugin UIs, front-end setup pages, CLIs, sidecars, and automation scripts without importing the full platform client. It maps directly to `/v1/auth/status`, `/v1/auth/login`, `/v1/auth/set-password`, `/v1/token`, and `/v1/auth/oauth/tori`.

### Admin SDK

The lightweight Admin SDK exposes operator controls for external admin pages, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform client. It covers desktop console/autostart status and toggles, tenant listing/creation, and natural-language configuration via `/v1/desktop/console`, `/v1/desktop/autostart`, `/v1/tenants`, `/v1/nl-config`, and `/v1/nl-config/translate`. Agent Kit also exposes this surface as `kit.admin` / `kit.Admin` for one-stop operator automation.

### Federation SDK

The lightweight Federation SDK exposes model-aware A2A federation helpers for external operator pages, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform client. It covers legacy peer/stat reads, OPP capability reads and updates, peer discovery, task delegation, bridge stats, and capability broadcast through `/v1/federation/peers`, `/v1/federation/stats`, `/v1/federation/capabilities`, `/v1/federation/discover`, `/v1/federation/delegate`, `/v1/federation/bridge/stats`, and `/v1/federation/broadcast`. Agent Kit also exposes this surface as `kit.federation` / `kit.Federation` for one-stop A2A federation automation.

### Planner SDK

The lightweight Planner SDK exposes Planner Recovery helpers for external task recovery pages, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform client. It covers checkpoint listing, recovery prompt generation, task resume, direct resume-plan execution, async resume-plan job lookup, and execution-state inspection through `/v1/planner/checkpoints`, `/v1/planner/checkpoints/recover`, `/v1/planner/checkpoints/resume`, `/v1/planner/checkpoints/resume-plan`, `/v1/planner/checkpoints/resume-plan/jobs`, and `/v1/planner/execution-state`. Agent Kit also exposes this surface as `kit.planner` / `kit.Planner` for one-stop recovery automation.

### IDE SDK

The lightweight IDE SDK exposes IDE supervisor status and structured code review helpers for editor plugins, external review panes, CLIs, sidecars, and automation scripts without importing the full platform client. It covers `/v1/ide/status` and `/v1/ide/review`, including full, quick, and diff review helper methods. Agent Kit also exposes this surface as `kit.ide` / `kit.IDE` for one-stop editor automation.

### Discovery SDK

The lightweight Discovery SDK exposes identity, embeddings, and search as a reusable incremental package surface for external projects, plugin UIs, front-end pages, CLIs, sidecars, and automation scripts. It maps directly to `POST /v1/identity/resolve`, `GET /v1/identity/profiles`, `GET/POST /v1/embeddings`, `GET /v1/search`, and `GET /v1/search/providers`, so callers can resolve identities, list profiles, inspect embedding providers, embed text, search the web, and list search providers without coupling to the full 云雀 platform. Agent Kit also exposes this surface as `kit.discovery` / `kit.Discovery`.

### Persona Modes SDK

The lightweight Persona SDK now also exposes persona mode listing, current-mode reads, and mode switching as reusable incremental helpers for external admin pages, plugin UIs, CLIs, sidecars, and automation scripts. It maps directly to `GET /v1/persona/modes`, `POST /v1/persona/mode`, and `GET /v1/persona/mode/current`, and Agent Kit exposes these helpers as `kit.persona` / `kit.Persona`.
### Bots SDK

The lightweight Bots SDK exposes bot management, inbox operations, and channel group discovery for external bot admin pages, plugin UIs, CLIs, sidecars, front-end pages, and automation scripts without importing the full platform client. It maps directly to `GET/POST /v1/bots`, `GET/PUT/DELETE /v1/bots/detail`, `GET/POST/DELETE /v1/inbox`, `POST /v1/inbox/read`, and `GET /v1/channels/groups`; Agent Kit also exposes this surface as `kit.bots` / `kit.Bots`.
### Documents SDK

The lightweight Documents SDK exposes document generation templates and DOCX/XLSX/PPTX/HTML generation helpers for external authoring pages, plugin UIs, CLIs, sidecars, front-end pages, and automation scripts without importing the full platform client. It maps directly to `GET /v1/documents/templates` and `POST /v1/documents/generate`; Agent Kit also exposes this surface as `kit.documents` / `kit.Documents`.
### WebChat SDK

The lightweight WebChat SDK exposes embeddable widget helpers (`widgetUrl`, `widgetScript`, `embedSnippet`) for external websites, front-end pages, plugin UIs, sidecars, and automation scripts without importing the full platform client. It maps to public `GET /v1/webchat/widget.js` plus local snippet generation; Agent Kit also exposes this surface as `kit.webchat` / `kit.WebChat`.
### Sandbox SDK

The lightweight Sandbox SDK exposes sandbox runtime helpers (`exec`, `probe`, `createDesktop`, `desktopStatus`, `destroyDesktop`) for external operator pages, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform client. It maps directly to `/v1/sandbox/exec`, `/v1/sandbox/probe`, `/v1/sandbox/desktop`, `/v1/sandbox/desktop/status`, and `/v1/sandbox/desktop/destroy`; Agent Kit also exposes this surface as `kit.sandbox` / `kit.Sandbox`.

### Router SDK

The lightweight Router SDK exposes smart-router statistics (`stats`) for external operator pages, plugin UIs, CLIs, sidecars, routing dashboards, and automation monitors without importing the full platform client. It maps directly to `GET /v1/router/stats`; Agent Kit also exposes this surface as `kit.router` / `kit.Router`.

### SkillHub SDK

The lightweight SkillHub SDK exposes incremental skill-package catalog, install, update, rollback, version, policy, and analytics helpers for external plugin UIs, operator pages, CLIs, sidecars, and automation scripts without importing the full platform client. It maps directly to `/api/skillhub/*`; Agent Kit also exposes this surface as `kit.skillhub` / `kit.SkillHub`.

### Plugins SDK

The lightweight Plugins SDK exposes plugin catalog, toggle, create, delete, file editing, UI tabs, reload, and open-folder helpers for external plugin UIs, operator pages, CLIs, sidecars, and automation scripts without importing the full platform client. It maps directly to `/v1/plugins*`; Agent Kit also exposes this surface as `kit.plugins` / `kit.Plugins`.


### PluginLLM SDK

PluginLLM SDK exposes plugin-scoped LLM completion (`complete`) for plugins, external frontends, CLIs, sidecars, and automation scripts without importing the full platform or broader Plugin API runtime slice. It maps directly to `POST /v1/plugin-api/llm`; Agent Kit also exposes this surface as `kit.pluginLLM` / `kit.PluginLLM` / `kit.plugin_llm`.

### PluginSend SDK

PluginSend SDK exposes plugin-scoped channel sending (`send`) for plugins, external frontends, CLIs, sidecars, and automation scripts without importing the full platform or broader Plugin API runtime slice. It maps directly to `POST /v1/plugin-api/send`; Agent Kit also exposes this surface as `kit.pluginSend` / `kit.PluginSend` / `kit.plugin_send`.

### PluginSearch SDK

PluginSearch SDK exposes plugin-scoped web search (`search`) for plugins, external frontends, CLIs, sidecars, and automation scripts without importing the full platform or broader Plugin API runtime slice. It maps directly to `POST /v1/plugin-api/search`; Agent Kit also exposes this surface as `kit.pluginSearch` / `kit.PluginSearch` / `kit.plugin_search`.

### PluginFolder SDK

PluginFolder SDK exposes plugin folder opening (`openFolder`) for external plugin editors, marketplaces, admin pages, CLIs, sidecars, and automation scripts without importing the full platform or broader Plugins management slice. It maps directly to `GET /v1/plugins/open-folder`; Agent Kit also exposes this surface as `kit.pluginFolder` / `kit.PluginFolder` / `kit.plugin_folder`.

### PluginFiles SDK

PluginFiles SDK exposes plugin file read/write helpers (`files`, `saveFile`) for external plugin editors, marketplaces, admin pages, CLIs, sidecars, and automation scripts without importing the full platform or broader Plugins management slice. It maps directly to `GET /v1/plugins/files` and `PUT /v1/plugins/files`; Agent Kit also exposes this surface as `kit.pluginFiles` / `kit.PluginFiles` / `kit.plugin_files`.

### PluginReload SDK

PluginReload SDK exposes plugin registry reload control (`reload`) for external plugin marketplaces, admin pages, CLIs, sidecars, and automation scripts without importing the full platform or broader Plugins management slice. It maps directly to `POST /v1/plugins/reload`; Agent Kit also exposes this surface as `kit.pluginReload` / `kit.PluginReload` / `kit.plugin_reload`.

### PluginToggle SDK

PluginToggle SDK exposes plugin enable/disable control (`toggle`) for external plugin marketplaces, admin pages, CLIs, sidecars, and automation scripts without importing the full platform or broader Plugins management slice. It maps directly to `POST /v1/plugins/toggle`; Agent Kit also exposes this surface as `kit.pluginToggle` / `kit.PluginToggle` / `kit.plugin_toggle`.

### PluginUI SDK

PluginUI SDK exposes read-only plugin UI tab discovery (`ui`) for external frontends, plugin marketplaces, operator panels, CLIs, and automation scripts without importing the full platform or broader Plugins management slice. It maps directly to `/v1/plugins/ui`; Agent Kit also exposes this surface as `kit.pluginUI` / `kit.PluginUI`.



### SkillsCatalog SDK

SkillsCatalog SDK exposes read-only runtime skills catalog listing (`list`) for external frontends, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform or broader Skills management slice. It maps directly to `/v1/skills`; Agent Kit also exposes this surface as `kit.skillsCatalog` / `kit.SkillsCatalog` / `kit.skills_catalog`.


### SkillsScan SDK

SkillsScan SDK exposes runtime skill scanning (`scan`) for external operator pages, plugin tools, CLIs, sidecars, and automation scripts without importing the full platform or broader Skills management slice. It maps directly to `POST /v1/skills/scan`; Agent Kit also exposes this surface as `kit.skillsScan` / `kit.SkillsScan` / `kit.skills_scan`.


### SkillsSuggestions SDK

SkillsSuggestions SDK exposes session skill suggestions (`suggestions`) for external chat pages, plugin tools, CLIs, sidecars, and automation scripts without importing the full platform or broader Skills management slice. It maps directly to `GET /v1/skill-suggestions`; Agent Kit also exposes this surface as `kit.skillsSuggestions` / `kit.SkillsSuggestions` / `kit.skills_suggestions`.


### SkillsDynamic SDK

SkillsDynamic SDK exposes dynamic skill review (`list`, `approve`, `reject`) for external admin pages, plugin tools, CLIs, sidecars, and automation scripts without importing the full platform or broader Skills management slice. It maps directly to `GET /v1/skills/dynamic`, `POST /v1/skills/approve`, and `POST /v1/skills/reject`; Agent Kit also exposes this surface as `kit.skillsDynamic` / `kit.SkillsDynamic` / `kit.skills_dynamic`.

### Skills SDK

The lightweight Runtime Skills SDK exposes skill catalog, scan, dynamic skill review, approve/reject, and session suggestion helpers for external plugin UIs, operator pages, CLIs, sidecars, and automation scripts without importing the full platform client. It maps directly to `/v1/skills*` and `/v1/skill-suggestions`; Agent Kit also exposes this surface as `kit.skills` / `kit.Skills`.

### Models SDK

The lightweight Models SDK exposes model registry list, add, and delete helpers for external settings pages, operator panels, plugins, CLIs, sidecars, and automation scripts without importing the full platform client. It maps directly to `/v1/models`; Agent Kit also exposes this surface as `kit.models` / `kit.Models`.

### Identity SDK

The lightweight Identity SDK exposes cross-channel identity resolution and profile listing helpers for external channel adapters, plugin UIs, operator pages, CLIs, sidecars, and automation scripts without importing the full platform client. It maps directly to `/v1/identity/resolve` and `/v1/identity/profiles`; Agent Kit also exposes this surface as `kit.identity` / `kit.Identity`.

### Embeddings SDK

The lightweight Embeddings SDK exposes embedding provider listing and text embedding helpers for external knowledge tools, plugin UIs, operator pages, CLIs, sidecars, and automation scripts without importing the full platform client. It maps directly to `/v1/embeddings`; Agent Kit also exposes this surface as `kit.embeddings` / `kit.Embeddings`.

### Search SDK

The lightweight Search SDK exposes web search and search-provider listing helpers for external research tools, plugin UIs, operator pages, CLIs, sidecars, and automation scripts without importing the full platform client. It maps directly to `/v1/search` and `/v1/search/providers`; Agent Kit also exposes this surface as `kit.search` / `kit.WebSearch`.

### Modes SDK

The lightweight Modes SDK exposes persona modes, current-mode reads, and mode switching helpers for external operator pages, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform client. It maps directly to `/v1/persona/modes`, `/v1/persona/mode/current`, and `/v1/persona/mode`; Agent Kit also exposes this surface as `kit.modes` / `kit.Modes`.

### Interactions SDK

The lightweight Interactions SDK exposes emotion history, sticker mappings, user instructions, emoji reactions, and sticker sending helpers for external plugin UIs, front-end admin pages, CLIs, sidecars, and automation scripts without importing the full platform client. It maps directly to `/v1/emotion/*`, `/v1/instructions*`, `/v1/react`, and `/v1/sticker/send`; Agent Kit also exposes this surface as `kit.interactions` / `kit.Interactions`.

### Airi Bridge SDK

The lightweight `yunque.airi` namespace exposes the Airi desktop pet bridge for Python plugins, CLIs, sidecars, and automation scripts without importing the generated full client. It maps to `/v1/ext/airi/status`, `/v1/ext/airi/models`, and the OpenAI-compatible `/v1/ext/airi/chat/completions`; Agent Kit also exposes it as `create_agent_kit().airi`.

```python
import yunque

status = yunque.airi.status()
models = yunque.airi.models()
reply = yunque.airi.chat_completions([{"role": "user", "content": "你好"}], model="yunque-airi")
items = yunque.airi.parse_stream('data: {"choices":[{"delta":{"content":"hi"}}]}\n\ndata: [DONE]\n\n')
```

### Breaker SDK

The lightweight `yunque.breaker` namespace exposes `/api/breaker/reset` for Python plugins, CLIs, sidecars, and automation scripts that need to reset LLM provider circuit breakers without importing the generated full client. Agent Kit also exposes it as `create_agent_kit().breaker`.

```python
import yunque

yunque.breaker.reset()
```
