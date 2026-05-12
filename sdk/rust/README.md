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
Knowledge Graph, Knowledge Base, LoRA, Workflow, Connector, Notify, Cost, Providers, Cognis, Trace, Heartbeat, Reverie, and Plugin API Runtime. It composes the hand-written lightweight
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
use yunque_client::{StateClient, StateGoal};

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

### Reverie SDK

Reverie SDK exposes lightweight proactive thought-loop helpers (`journal`, `stats`, `config`, `updateConfig`, `think`, `deleteThought`, `actions`, `targets`) for external operator pages, plugins, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.reverie` / `kit.Reverie` for one-stop proactive reflection and delivery workflows.

### Cost SDK

Cost SDK exposes lightweight cost governance helpers (`summary`, `setBudget`, `task`, `taskTimeline`, `breakdown`, `history`, `alerts`, `usage`, `setQuota`) for external pages, plugins, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.cost` / `kit.Cost` for one-stop budget, usage, quota, and cost observability workflows.

### Fork SDK

Fork SDK exposes lightweight conversation branch helpers (`root`, `get`, `create`, `remove`, `branch`, `list`) for external pages, plugins, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.fork` / `kit.Fork` for one-stop conversation exploration and rollback-safe alternate-path workflows.
