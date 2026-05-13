# yunque-client (Rust)

Auto-generated Rust client for the Yunque (云雀) Agent HTTP API.

- Source spec: [`docs/openapi.yaml`](../../docs/openapi.yaml)
- Generator: [`progenitor`](https://github.com/oxidecomputer/progenitor) (build-time)
- Runtime: [`reqwest`](https://crates.io/crates/reqwest) with `rustls-tls`
- 425 async methods, ~19000 LOC of generated code

## Add to your project

```toml
[dependencies]
yunque-client = { path = "../yunque-agent/sdk/rust" }
tokio = { version = "1", features = ["rt-multi-thread", "macros"] }
```

(Path-dep for now; once published, use `cargo add yunque-client`.)

## Quick start

```rust
use yunque_client::Client;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let client = Client::new("http://localhost:9090");

    // Every endpoint has a typed async method on the Client.
    // Names follow `<method>_<sanitised_path>`, e.g. `get_v1_cognis`.
    let cognis = client.get_v1_cognis().send().await?;
    println!("{:?}", cognis.into_inner());

    // Cogni operations (curated names from the spec):
    // - generate_cogni / list_cognis / evolve_cogni / run_cogni_workflow
    // - get_cogni_economics / get_cogni_federation_status / ...

    Ok(())
}
```

## Authentication

```rust
let client = Client::new_with_client(
    "http://localhost:9090",
    reqwest::Client::builder()
        .default_headers({
            let mut h = reqwest::header::HeaderMap::new();
            h.insert(
                reqwest::header::AUTHORIZATION,
                "Bearer <your-jwt>".parse()?,
            );
            h
        })
        .build()?,
);
```

## Regenerating

The client is regenerated **automatically** on every `cargo build` —
`build.rs` reads `docs/openapi.yaml`, so any spec change triggers a rebuild.

```bash
# 1. Refresh OpenAPI from gateway routes
cd ../..        # back to repo root
make openapi

# 2. Rebuild the Rust SDK
cd sdk/rust
cargo build
cargo check     # quick verification
```

## Layout

| File | Purpose |
|---|---|
| `Cargo.toml` | Dependencies + build deps (`progenitor`, `openapiv3`, `prettyplease`) |
| `build.rs` | Reads spec, downgrades `openapi: 3.1.0` → `3.0.3` in-memory, runs progenitor |
| `src/lib.rs` | `include!` for the generated `yunque_client.rs` |
| `target/.../out/yunque_client.rs` | The actual generated code (~19000 LOC, not committed) |

## Status & caveats

- **OpenAPI 3.1 → 3.0.3 downgrade**: `progenitor 0.10` only supports 3.0.x
  parsing. We do an in-memory string substitution (`openapi: 3.1.0` →
  `openapi: 3.0.3`) inside `build.rs`. Our spec doesn't use 3.1-only features
  (yet), so this is safe today.
- **Streaming endpoints** (`/v1/chat/stream`, `/v1/events/stream`): generated
  as standard reqwest calls — for real SSE consumption, use
  [`eventsource-stream`](https://crates.io/crates/eventsource-stream) on the
  raw response body.
- **Lint warning**: `elided_named_lifetimes` rename warning comes from
  progenitor's generated output; benign on rustc 1.94+.
- **Body schemas** are mostly `serde_json::Value` placeholders since the spec
  is path-only. Hand-edit `docs/openapi.yaml` request/response bodies, then
  rebuild.

## Lightweight Agent Kit helper

Use `AgentKit` when a Rust CLI, sidecar, plugin runner, or automation binary
wants the common SDK-first surfaces from one object: State Kernel, Reflection
Experience, Mission Parse, Scheduler, Cron System, Triggers, Memory Kernel,
Knowledge Graph, Knowledge Base, LoRA, Workflow, Connector, Notify, Cost, Providers, Cognis, Trace, Heartbeat, Events, Reverie, Tori, Upload, Speech, Setup, and Plugin API Runtime. It composes the hand-written lightweight
clients and does not import the generated all-in-one API surface.

```rust
use yunque_client::{AgentKit, ReflectOptions};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let kit = AgentKit::new_with_plugin_token(
        "http://localhost:9090",
        "<plugin-or-api-token>",
        "<plugin-token>",
    )?;

    let focus = kit.state.focus().await?;
    let strategies = kit.reflect.strategies(&ReflectOptions {
        tag: "sdk".to_string(),
        limit: 5,
        ..ReflectOptions::default()
    }).await?;
    let mission = kit.missions.parse("每天八点总结昨天的任务").await?;
    let jobs = kit.scheduler.jobs().await?;
    let connectors = kit.connectors.list().await?;
    let channels = kit.notify.channels().await?;
    let search = kit.plugin.search("incremental SDK package", 5).await?;

    println!("{} {} {} {} {} {} {}", focus, strategies, mission.r#type, jobs.count, connectors.connectors.len(), channels.channels.len(), search.results.len());
    Ok(())
}
```

## Lightweight Mission Parse helper

Use `MissionsClient` when a Rust CLI, sidecar, plugin runner, or automation
binary only needs to turn natural-language intent into a structured
task/workflow/cron/trigger draft.

```rust
use yunque_client::MissionsClient;

let missions = MissionsClient::new("http://localhost:9090", "<plugin-or-api-token>")?;
let mission = missions.parse("每天八点总结昨天的任务").await?;
println!("{} {}", mission.r#type, mission.name);
```

## Lightweight Scheduler helper

Use `SchedulerClient` when a Rust CLI, sidecar, plugin runner, or automation
binary needs to list, add, or remove prompt-based recurring jobs.

```rust
use yunque_client::{SchedulerAddRequest, SchedulerClient};

let scheduler = SchedulerClient::new("http://localhost:9090", "<plugin-or-api-token>")?;
let jobs = scheduler.jobs().await?;
let job = scheduler.add(&SchedulerAddRequest {
    name: "hourly".to_string(),
    prompt: "检查任务".to_string(),
    interval: "1h".to_string(),
}).await?;
let removed = scheduler.remove(&job.id).await?;
println!("{} {} {}", jobs.count, job.id, removed.status);
```

## Lightweight State Kernel helper

The generated client offers broad OpenAPI coverage. For sidecars, CLIs, plugins,
or dashboards that only need the agent state layer, use the hand-written
`StateClient` instead. It avoids coupling callers to the large generated method
surface and mirrors the incremental helpers in the TypeScript, Go, and Python SDKs.

```rust
use yunque_client::{StateClient, StateFocusUpdateRequest, StateGoal, StateResource};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let state = StateClient::new("http://localhost:9090", "<plugin-or-api-token>")?;

    let snapshot = state.snapshot().await?;
    println!("focus: {}", snapshot.focus);
    println!("goals: {}", snapshot.goals.len());
    println!("skills: {}", snapshot.capabilities.total_skills);

    let actions = state.actions().await?;
    let caps = state.capabilities().await?;
    let focus = state.focus().await?;
    let resources = state.resources().await?;

    let saved = state.save_goal(&StateGoal {
        title: "Ship a Rust SDK state slice".to_string(),
        priority: 2,
        ..StateGoal::default()
    }).await?;

    let _updated = state.update_focus(&StateFocusUpdateRequest {
        focus: "整理 SDK 状态层".to_string(),
        topics: vec!["sdk".to_string(), "state".to_string()],
    }).await?;
    let _deleted = state.delete_goal("goal-1").await?;
    let _tracked = state.track_resource(&StateResource {
        path: "sdk/rust".to_string(),
        r#type: "repo".to_string(),
        ..StateResource::default()
    }).await?;
    let _released = state.release_resource("resource-1").await?;
    let _ = (actions, caps, focus, resources, saved, _updated, _deleted, _tracked, _released);

    Ok(())
}
```

## Lightweight Reflection Experience helper

Use `ReflectClient` when an external Rust CLI, sidecar, or evaluation runner
only needs agent lessons and strategy hints. This keeps reflection reuse as a
small SDK slice instead of requiring callers to depend on the broad generated
client surface.

```rust
use yunque_client::{ReflectClient, ReflectOptions};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let reflect = ReflectClient::new("http://localhost:9090", "<plugin-or-api-token>")?;

    let options = ReflectOptions {
        tag: "quality:9".to_string(),
        limit: 5,
        ..ReflectOptions::default()
    };

    let experiences = reflect.experiences(&options).await?;
    let stats = reflect.stats(&options).await?;
    let strategies = reflect.strategies(&options).await?;

    println!("experiences: {}", experiences.total);
    println!("recent_7d: {}", stats.recent_7d);
    println!("{}", strategies);

    Ok(())
}
```



## Lightweight Cron System helper

Use `CronClient` when a Rust CLI, sidecar, plugin runner, or automation binary needs host `/v1/cron/*` scheduled task access without importing the generated all-in-one API surface. This is separate from `PluginApiClient::cron_*`, which manages plugin-owned runtime cron jobs.

```rust
use yunque_client::{CronAddRequest, CronClient, CronPayload, CronSchedule};

let cron = CronClient::new("http://localhost:9090", "<plugin-or-api-token>")?;
let jobs = cron.list().await?;
let added = cron.add(&CronAddRequest {
    name: "nightly".to_string(),
    schedule: CronSchedule { r#type: "cron".to_string(), cron_expr: "0 2 * * *".to_string(), timezone: "Asia/Shanghai".to_string(), ..Default::default() },
    payload: CronPayload { kind: "systemEvent".to_string(), ..Default::default() },
}).await?;
let run = cron.run(&added.job.id).await?;
println!("{} {} {}", jobs.jobs.len(), added.job.id, run.run.status);
```

## Lightweight Triggers helper

Use `TriggersClient` when a Rust CLI, sidecar, plugin runner, or automation binary needs Triggers v2 definitions, event emission, and recent trigger history without importing the generated all-in-one API surface.

```rust
use yunque_client::{TriggerDef, TriggerHistoryOptions, TriggerListOptions, TriggerPayload, TriggersClient};

let triggers = TriggersClient::new("http://localhost:9090", "<plugin-or-api-token>")?;
let defs = triggers.list(&TriggerListOptions { status: "enabled".to_string(), ..Default::default() }).await?;
let created = triggers.create(&TriggerDef {
    name: "review done".to_string(),
    tenant_id: "default".to_string(),
    r#type: "event".to_string(),
    actions: vec![serde_json::json!({"kind":"notify"})],
    ..Default::default()
}).await?;
let emitted = triggers.emit(&TriggerPayload { event: "review.done".to_string(), ..Default::default() }).await?;
let runs = triggers.runs(&TriggerHistoryOptions { trigger_id: created.id, limit: 10 }).await?;
println!("{} {} {}", defs.total, emitted.status, runs.total);
```

## Lightweight Plugin API helper

Use `PluginApiClient` when a Rust CLI, sidecar, or plugin runner only needs a
focused plugin runtime slice: LLM, search, channel send, plugin-private memory,
shared agent memory, knowledge search/ingest, plugin-owned cron jobs, and
system extension registration. This keeps simple automations away from the
broad generated OpenAPI surface.

```rust
use yunque_client::{PluginApiClient, PluginLLMMessage, PluginLLMRequest};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let plugin = PluginApiClient::new("http://localhost:9090", "<plugin-token>")?;

    let reply = plugin.llm(&PluginLLMRequest {
        messages: vec![PluginLLMMessage {
            role: "user".to_string(),
            content: "Summarize the current SDK status".to_string(),
        }],
        ..PluginLLMRequest::default()
    }).await?;

    let results = plugin.search("Yunque SDK", 5).await?;
    plugin.memory_set("sdk:last_summary", &reply.reply).await?;
    let note = plugin.memory_get("sdk:last_summary").await?;
    let context = plugin.agent_memory_search("SDK roadmap", 5).await?;
    plugin.agent_memory_add("Rust PluginApiClient covered Plugin API Runtime", "sdk-rust").await?;
    let knowledge = plugin.knowledge_search("incremental SDK package", 5).await?;
    let job = plugin.cron_add(
        "weekly-sdk-summary",
        "0 9 * * MON",
        "Summarize SDK manifest drift",
    ).await?;
    let provider = plugin.register_provider(&serde_json::json!({
        "id": "local-llm",
        "base_url": "http://localhost:11434/v1",
        "model": "llama3",
        "type": "chat"
    })).await?;
    let channel = plugin.register_channel(&serde_json::json!({
        "name": "webhook",
        "webhook_url": "http://localhost:8080/hook",
        "send_endpoint": "http://localhost:8080/send"
    })).await?;
    let search = plugin.register_search(&serde_json::json!({
        "name": "docs-search",
        "base_url": "http://localhost:7700"
    })).await?;
    let guardrail = plugin.register_guardrail(&serde_json::json!({
        "name": "internal-policy",
        "description": "Block internal-only snippets",
        "phase": "both",
        "keywords": ["internal-only"]
    })).await?;
    let embedding = plugin.register_embedding(&serde_json::json!({
        "name": "local-embedding",
        "base_url": "http://localhost:11434/v1",
        "model": "nomic-embed-text",
        "dimensions": 768
    })).await?;
    let speech = plugin.register_speech(&serde_json::json!({
        "name": "local-tts",
        "type": "tts",
        "base_url": "http://localhost:5002",
        "model": "default"
    })).await?;
    let extensions = plugin.extensions().await?;
    let sent = plugin.send("telegram", "chat-id", &note.value, "markdown").await?;

    Ok(())
}
```

### Memory Kernel helper

Rust CLI、sidecar、插件运行器或自动化二进制可以用 `MemoryClient` 访问宿主 `/v1/memory/*` 回忆记忆层。它不同于 `PluginApiClient::memory_*` 的插件私有 KV。

```rust
use yunque_client::{MemoryAddRequest, MemoryClient, MemorySearchRequest};

let memory = MemoryClient::new("http://localhost:9090", "<plugin-or-api-token>")?;
let stats = memory.stats().await?;
let found = memory.search(&MemorySearchRequest { query: "用户偏好".to_string(), limit: 3, ..Default::default() }).await?;
let added = memory.add(&MemoryAddRequest { value: "用户偏好中文回复".to_string(), layer: "mid".to_string(), source: "sdk".to_string(), ..Default::default() }).await?;
println!("{:?} {} {}", stats.get("mid"), found.count, added.status);
```

### Knowledge Graph helper

Rust CLI、sidecar、插件运行器或自动化二进制可以用 `GraphClient` 访问宿主 `/v1/graph/*` 知识图谱层，读取/维护实体、关系和图谱上下文。

```rust
use yunque_client::{GraphClient, GraphEntity};

let graph = GraphClient::new("http://localhost:9090", "<plugin-or-api-token>")?;
let entities = graph.entities("云雀").await?;
let entity = graph.put_entity(&GraphEntity { name: "云雀".to_string(), r#type: "agent".to_string(), ..Default::default() }).await?;
let context = graph.context_by_entity_id(&entity.id).await?;
println!("{} {}", entities.entities.len(), context.context);
```

### Knowledge Base helper

Rust CLI、sidecar、插件运行器或自动化二进制可以用 `KnowledgeClient` 访问宿主 `/v1/knowledge/*` RAG 知识库。它不同于 `PluginApiClient::knowledge_*` 的插件运行时 helper。

```rust
use yunque_client::{KnowledgeClient, KnowledgeIngestRequest, KnowledgeSearchOptions};

let knowledge = KnowledgeClient::new("http://localhost:9090", "<plugin-or-api-token>")?;
let stats = knowledge.stats().await?;
let found = knowledge.search(&KnowledgeSearchOptions { query: "增量 SDK".to_string(), limit: 3, ..Default::default() }).await?;
let ingested = knowledge.ingest(&KnowledgeIngestRequest { name: "sdk-note".to_string(), content: "外部项目可直接调用 Knowledge Base".to_string(), ..Default::default() }).await?;
println!("{:?} {} {:?}", stats.get("sources"), found.count, ingested.source.map(|s| s.id));
```

### LoRA lifecycle helper

Rust CLI、sidecar、插件运行器或自动化二进制可以用 `LoRAClient` 访问宿主 `/v1/lora/*` 本地脑训练生命周期能力。

```rust
use yunque_client::{LoRAClient, LoRAPreviewOptions, TriggerLoRARequest};

let lora = LoRAClient::new("http://localhost:9090", "<plugin-or-api-token>")?;
let status = lora.status().await?;
let preview = lora.preview(&LoRAPreviewOptions { tenant_id: "default".to_string() }).await?;
let triggered = lora.trigger(&TriggerLoRARequest { tenant_id: "default".to_string() }).await?;
println!("{:?} {:?} {:?}", status.get("active_model"), preview.get("preview"), triggered.get("status"));
```

### Workflow orchestration helper

Rust CLI、sidecar、插件运行器或自动化二进制可以用 `WorkflowClient` 访问宿主 `/v1/workflows*` 工作流定义与实例能力。

```rust
use yunque_client::{WorkflowClient, WorkflowRunRequest};

let workflows = WorkflowClient::new("http://localhost:9090", "<plugin-or-api-token>")?;
let defs = workflows.list().await?;
let run = workflows.run(&WorkflowRunRequest { definition_id: "wf_1".to_string(), variables: serde_json::Map::new() }).await?;
println!("{} {}", defs.total, run.instance_id);
```

### Connectors runtime helper

Rust CLI、sidecar、插件运行器或自动化二进制可以用 `ConnectorsClient` 访问宿主连接器目录、连接状态和动作执行能力。

```rust
use yunque_client::{ConnectorExecuteRequest, ConnectorsClient};

let connectors = ConnectorsClient::new("http://localhost:9090", "<plugin-or-api-token>")?;
let catalog = connectors.list().await?;
let detail = connectors.detail("github").await?;
let executed = connectors.execute(&ConnectorExecuteRequest {
    connector_id: "github".to_string(),
    action_id: "create_issue".to_string(),
    params: serde_json::Map::new(),
}).await?;
println!("{} {} {}", catalog.connectors.len(), detail.status, executed.ok);
```

### Notify runtime helper

Rust CLI、sidecar、插件运行器或自动化二进制可以用 `NotifyClient` 管理通知渠道、发送测试通知并分享任务/会话产物。

```rust
use yunque_client::{NotifyClient, NotifyShareRequest};

let notify = NotifyClient::new("http://localhost:9090", "<plugin-or-api-token>")?;
let channels = notify.channels().await?;
let shared = notify.share(&NotifyShareRequest {
    channel_id: "feishu-main".to_string(),
    message: "任务完成".to_string(),
    task_id: "task_1".to_string(),
    ..NotifyShareRequest::default()
}).await?;
println!("{} {}", channels.channels.len(), shared.ok);
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

`IterateClient` is the Rust lightweight self-iteration SDK slice. It wraps proposal review and manual cycles through `proposals`, `pending_proposals`, `approve`, `reject`, `trigger`, and `status` over `/api/iterate/proposals`, `/api/iterate/approve`, `/api/iterate/reject`, `/api/iterate/trigger`, and `/api/iterate/status`; `AgentKit::new(...).iterate` composes it with the other incremental clients.
### Persona SDK

`PersonaClient` is the Rust lightweight persona SDK slice. It wraps persona identity reads/updates, persona skills, persona presets, custom preset management, and feature flags through `/v1/persona*`; `AgentKit::new(...).persona` composes it with the other incremental clients.




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


### PluginExtensions SDK

PluginExtensions SDK exposes plugin-contributed system extensions (`registerProvider`, `registerChannel`, `registerSearch`, `registerGuardrail`, `registerEmbedding`, `registerSpeech`, `list`) for plugins, external frontends, CLIs, sidecars, and automation scripts without importing the full platform or broader Plugin API runtime slice. It maps directly to `POST /v1/plugin-api/register/provider` and sibling extension registration routes plus `GET /v1/plugin-api/extensions`; Agent Kit also exposes this surface as `kit.pluginExtensions` / `kit.PluginExtensions` / `kit.plugin_extensions`.

### PluginCron SDK

PluginCron SDK exposes plugin-scoped cron automation (`add`, `remove`, `list`) for plugins, external frontends, CLIs, sidecars, and automation scripts without importing the full platform or broader Plugin API runtime slice. It maps directly to `POST /v1/plugin-api/cron/add`, `POST /v1/plugin-api/cron/remove`, and `GET /v1/plugin-api/cron/list`; Agent Kit also exposes this surface as `kit.pluginCron` / `kit.PluginCron` / `kit.plugin_cron`.

### PluginAgentMemory SDK

PluginAgentMemory SDK exposes shared Agent memory operations (`search`, `add`) for plugins, external frontends, CLIs, sidecars, and automation scripts without importing the full platform or broader Plugin API runtime slice. It maps directly to `POST /v1/plugin-api/agent-memory/search` and `POST /v1/plugin-api/agent-memory/add`; Agent Kit also exposes this surface as `kit.pluginAgentMemory` / `kit.PluginAgentMemory` / `kit.plugin_agent_memory`.

### PluginKnowledge SDK

PluginKnowledge SDK exposes plugin-scoped knowledge/RAG operations (`search`, `ingest`) for plugins, external frontends, CLIs, sidecars, and automation scripts without importing the full platform or broader Plugin API runtime slice. It maps directly to `POST /v1/plugin-api/knowledge/search` and `POST /v1/plugin-api/knowledge/ingest`; Agent Kit also exposes this surface as `kit.pluginKnowledge` / `kit.PluginKnowledge` / `kit.plugin_knowledge`.

### PluginMemory SDK

PluginMemory SDK exposes plugin-private memory (`get`, `set`, `delete`, `list`, `search`) for plugins, external frontends, CLIs, sidecars, and automation scripts without importing the full platform or broader Plugin API runtime slice. It maps directly to `POST /v1/plugin-api/memory/get` and sibling memory routes; Agent Kit also exposes this surface as `kit.pluginMemory` / `kit.PluginMemory` / `kit.plugin_memory`.

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

### Lightweight Airi Bridge helper

`AiriClient` exposes the Airi desktop pet bridge for Rust CLIs, sidecars, and automation binaries without coupling callers to the full generated client. It maps to `/v1/ext/airi/status`, `/v1/ext/airi/models`, and OpenAI-compatible `/v1/ext/airi/chat/completions`; Agent Kit also exposes it as `AgentKit::airi`.

```rust
use yunque_client::{AiriChatCompletionRequest, AiriChatMessage, AiriClient};

let airi = AiriClient::new("http://localhost:9090", "<token>")?;
let status = airi.status().await?;
let models = airi.models().await?;
let reply = airi.chat_completions(&AiriChatCompletionRequest {
    model: "yunque-airi".to_string(),
    messages: vec![AiriChatMessage { role: "user".to_string(), content: "你好".to_string() }],
    ..Default::default()
}).await?;
```

### Lightweight Breaker helper

`BreakerClient` exposes `/api/breaker/reset` for Rust CLIs, sidecars, and automation binaries that need to reset LLM provider circuit breakers without coupling to the full generated client. Agent Kit also exposes it as `AgentKit::breaker`.

```rust
use yunque_client::BreakerClient;

let breaker = BreakerClient::new("http://localhost:9090", "<token>")?;
let _reset = breaker.reset().await?;
```
