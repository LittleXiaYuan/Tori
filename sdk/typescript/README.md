# yunque-client (TypeScript)

Auto-generated TypeScript client for the Yunque (云雀) Agent HTTP API.

- Source spec: [`docs/openapi.yaml`](../../docs/openapi.yaml)
- Generator: [`@hey-api/openapi-ts`](https://github.com/hey-api/openapi-ts)
- Runtime: [`@hey-api/client-fetch`](https://heyapi.dev/openapi-ts/clients/fetch) (zero-dep, native fetch)

## Install

From the repo root:

```bash
cd sdk/typescript
npm install
```

When/if we publish to npm, install with `npm i yunque-client`.

## Quick start

```ts
import { client } from "./src/client.gen";
import {
  getV1Cognis,
  postV1CognisIdEvolve,
  postV1CognisGenerate,
  getV1ChatStream,
} from "./src/sdk.gen";

client.setConfig({
  baseUrl: "http://localhost:9090",
  headers: { Authorization: "Bearer <your-jwt>" },
});

// List every Cogni
const { data, error } = await getV1Cognis();
if (error) throw error;
console.log(data);

// Self-generate a Cogni
const generated = await postV1CognisGenerate({
  body: { prompt: "Build a code-review cogni" },
});

// Trigger evolution on one cogni
await postV1CognisIdEvolve({ path: { id: "code-reviewer" } });
```

## Incremental imports

The generated `src/sdk.gen.ts` is useful for full API coverage, but it is a
large all-in-one surface. Product integrations that only need Planner recovery
can import the hand-written incremental slice instead:

```ts
import { createPlannerRecoveryClient } from "yunque-client/planner-recovery";
import { createChatClient } from "yunque-client/chat";
import { createMemoryClient } from "yunque-client/memory";
import { createTasksClient } from "yunque-client/tasks";
import { createKnowledgeClient } from "yunque-client/knowledge";
import { createProvidersClient } from "yunque-client/providers";
import { createSetupClient } from "yunque-client/setup";
import { createDocumentsClient } from "yunque-client/documents";
import { createApprovalsClient } from "yunque-client/approvals";
import { createTraceClient } from "yunque-client/trace";
import { createBrowserClient } from "yunque-client/browser";
import { createRuntimeClient } from "yunque-client/runtime";
import { createModesClient } from "yunque-client/modes";
import { createIDEClient } from "yunque-client/ide";
import { createPersonaClient } from "yunque-client/persona";
import { createWorkflowClient } from "yunque-client/workflow";
import { createCostClient } from "yunque-client/cost";
import { createLoRAClient } from "yunque-client/lora";
import { createIterateClient } from "yunque-client/iterate";
import { createTrustClient } from "yunque-client/trust";
import { createAuditClient } from "yunque-client/audit";
import { createHeartbeatClient } from "yunque-client/heartbeat";
import { createReverieClient } from "yunque-client/reverie";
import { createFederationClient } from "yunque-client/federation";
import { createSystemClient } from "yunque-client/system";

const planner = createPlannerRecoveryClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});

const state = await planner.getExecutionState({ plan_id: "plan_123" });
if (state.next_action === "retry_failed") {
  await planner.resumeCheckpointPlan({
    plan_id: "plan_123",
    action: "retry_failed",
    async: true,
  });
}

const chat = createChatClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

const reply = await chat.send({
  messages: [{ role: "user", content: "你好呀" }],
  session_id: "demo-session",
});
console.log(reply.reply);

const memory = createMemoryClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

await memory.add({
  layer: "long",
  content: "用户希望回答更简洁",
  source: "demo-shell",
});

const tasks = createTasksClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

const task = await tasks.create({
  title: "整理恢复现场",
  description: "读取最近 Planner checkpoint 并给出下一步建议",
  constraints: { max_steps: 6, risk_level: "low" },
});
await tasks.run(task.id);

const knowledge = createKnowledgeClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

await knowledge.ingest({
  name: "technical-blueprint.md",
  content: "Planner 恢复、任务编排、记忆与知识库是外部壳的最小闭环。",
});
const matches = await knowledge.search({ query: "Planner 恢复", limit: 5 });
console.log(matches.chunks);

const providers = createProvidersClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

await providers.registerProvider({
  preset_id: "deepseek",
  api_key: "<provider-key>",
  model: "deepseek-chat",
});
await providers.testProvider("deepseek-deepseek-chat");
await providers.setExecProvider("deepseek-deepseek-chat");

const setup = createSetupClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

const templates = await setup.templates();
await setup.testProvider({
  base_url: "https://api.deepseek.com/v1",
  api_key: "<provider-key>",
  model: "deepseek-chat",
});
await setup.apply({
  template_id: templates.templates[0]?.id ?? "hybrid",
  base_url: "https://api.deepseek.com/v1",
  api_key: "<provider-key>",
  model: "deepseek-chat",
});

const documents = createDocumentsClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

await documents.generateDocx({
  title: "技术蓝图摘要",
  content: "# 云雀技术蓝图摘要\n\nPlanner、任务、记忆与知识库已经拆成增量 SDK。",
});

const approvals = createApprovalsClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

const pending = await approvals.pending();
if (pending.approvals[0]) {
  await approvals.decide(pending.approvals[0].id, "allow_once");
}

const trace = createTraceClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

const recentEvents = await trace.recent({ limit: 20 });
console.log(recentEvents.events);

const browser = createBrowserClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

const browserStatus = await browser.status();
if (browserStatus.connected) {
  await browser.navigate("https://example.com");
  const pageText = await browser.ocr();
  console.log(pageText.text);
}

const runtime = createRuntimeClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

const queues = await runtime.queues();
console.log(queues.queues);

const modes = createModesClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

const currentMode = await modes.current({ session_id: "demo-session" });
console.log(currentMode.mode);

const ide = createIDEClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

const ideStatus = await ide.status();
if (ideStatus.connected) {
  await ide.reviewDiff({
    file_path: "src/app.ts",
    language: "ts",
    diff: "+console.log('hello')",
  });
}

const persona = createPersonaClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});

const currentPersona = await persona.get();
await persona.addSkill({
  name: "review-style",
  description: "Review tone and output preference",
  content: "Prefer concise, evidence-first review comments.",
});
console.log(currentPersona.identity);

const workflows = createWorkflowClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

const saved = await workflows.save({
  name: "daily-review",
  nodes: [{ id: "review", name: "Review", type: "llm", position: { x: 0, y: 0 } }],
  edges: [],
});
await workflows.run({ definition_id: saved.id!, variables: { topic: "sdk" } });

const costs = createCostClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
console.log(await costs.summary());
await costs.setQuota({ quota: { max_chat_calls: 100, max_tokens_per_day: 200000 } });

const lora = createLoRAClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

const preview = await lora.preview({ tenant_id: "default" });
if (preview.preview.ready) {
  await lora.trigger({ tenant_id: "default" });
}

const iterate = createIterateClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});

const pending = await iterate.pendingProposals();
if (pending.proposals[0]) {
  await iterate.approve({ id: pending.proposals[0].id });
}

const trust = createTrustClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

const reviewGate = await trust.reviewStatus();
if (reviewGate.trust_enabled) {
  console.log(await trust.scores());
}

const audit = createAuditClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
const integrity = await audit.verify();
console.log(integrity.valid);

const heartbeat = createHeartbeatClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await heartbeat.update({ enabled: true, interval_minutes: 30 });

const reverie = createReverieClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await reverie.think({ event_type: "task_completed", trigger: "sdk-demo" });

const federation = createFederationClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
const matches = await federation.discover({ feature: "browser", intent: "open page" });
console.log(matches.count);

const system = createSystemClient({ baseUrl: "http://localhost:9090" });
const readiness = await system.readyz();
console.log(readiness.status);
```

This keeps the SDK usable as an **incremental package**: embedder code can bring
in only `planner-recovery`, `chat`, `memory`, `tasks`, `knowledge`, or
`providers`/`setup`/`documents`/`approvals`/`trace`/`browser`/`runtime`/`modes`
`/ide`/`persona`/`workflow`/`cost`/`lora`/`iterate`/`trust`/`audit`/`heartbeat`
`/reverie`/`federation`/`system` without importing the generated 500KB+ SDK/types bundle. Add future
slices in the same style when those surfaces need stable, lightweight
integration APIs.

## Regenerating

After spec changes:

```bash
# 1. Refresh OpenAPI from gateway routes
cd ../..        # back to repo root
make openapi

# 2. Regenerate this SDK
cd sdk/typescript
npm run generate
npm run typecheck   # should be silent (0 errors)
```

## Layout

| File / dir | Purpose |
|---|---|
| `src/sdk.gen.ts` | Per-endpoint typed functions (~263 KB) |
| `src/types.gen.ts` | All schemas, request/response types (~295 KB) |
| `src/client.gen.ts` | Default client instance |
| `src/client/` | Fetch runtime (from `@hey-api/client-fetch`) |
| `src/core/` | Internal helpers |
| `src/planner-recovery.ts` | Lightweight hand-written Planner recovery slice for incremental imports |
| `src/chat.ts` | Lightweight hand-written Chat/SSE slice for incremental imports |
| `src/memory.ts` | Lightweight hand-written Memory stats/search/add/compact slice |
| `src/tasks.ts` | Lightweight hand-written Task create/list/lifecycle slice |
| `src/knowledge.ts` | Lightweight hand-written Knowledge search/ingest/import/upload slice |
| `src/providers.ts` | Lightweight hand-written LLM provider/model configuration slice |
| `src/setup.ts` | Lightweight hand-written first-run setup/configuration wizard slice |
| `src/documents.ts` | Lightweight hand-written DOCX/XLSX/PPTX/HTML generation slice |
| `src/approvals.ts` | Lightweight hand-written human-in-the-loop approval queue/rules slice |
| `src/trace.ts` | Lightweight hand-written execution/audit trace inspection slice |
| `src/browser.ts` | Lightweight hand-written browser extension automation and OPP slice |
| `src/runtime.ts` | Lightweight hand-written session queue and events stream slice |
| `src/modes.ts` | Lightweight hand-written persona mode listing/switching slice |
| `src/ide.ts` | Lightweight hand-written IDE status/code-review slice |
| `src/persona.ts` | Lightweight hand-written persona identity/skills/presets slice |
| `src/workflow.ts` | Lightweight hand-written workflow definition/instance execution slice |
| `src/cost.ts` | Lightweight hand-written cost, usage and quota slice |
| `src/lora.ts` | Lightweight hand-written LoRA training and evolution lifecycle slice |
| `src/iterate.ts` | Lightweight hand-written self-iteration proposal approval slice |
| `src/trust.ts` | Lightweight hand-written trust, review-gate and skill-growth slice |
| `src/audit.ts` | Lightweight hand-written audit chain and audit trail inspection slice |
| `src/heartbeat.ts` | Lightweight hand-written proactive heartbeat lifecycle slice |
| `src/reverie.ts` | Lightweight hand-written inner monologue and proactive thought slice |
| `src/federation.ts` | Lightweight hand-written federation peers, capabilities, discovery, delegation, and broadcast slice |
| `src/system.ts` | Lightweight hand-written health, version, metrics, cache, and module observability slice |
| `openapi-ts.config.ts` | Generator configuration |
| `tsconfig.json` | TypeScript compiler config (`DOM.Iterable` required for `Headers.entries`) |

## Status

- 343 endpoints, ~22000 LOC, 100+ schemas
- Hand-curated `cognis` operationIds yield idiomatic names (`postV1CognisGenerate` etc.)
- Auto-generated names follow `<method><PathPascalCase>` pattern
- Streaming (`getV1ChatStream`, `getV1EventsStream`) is stubbed but native fetch
  doesn't expose SSE — use `EventSource` or [`fetch-event-source`](https://www.npmjs.com/package/@microsoft/fetch-event-source) for real SSE.
- Request/response bodies are mostly `unknown` placeholders since the source
  spec is path-only. Hand-edit `docs/openapi.yaml` to add real schemas, then
  regenerate.

## Caveats

- 6 npm audit warnings come from `prettier` / `ruff` dev deps; runtime is clean.
- Client uses ESM (`"type": "module"` in package.json). For CommonJS consumers,
  rebuild with a different tsconfig (`"module": "CommonJS"`).
