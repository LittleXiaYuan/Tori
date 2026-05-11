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
```

This keeps the SDK usable as an **incremental package**: embedder code can bring
in only `planner-recovery`, `chat`, `memory`, or `tasks` without importing the
generated 500KB+ SDK/types bundle. Add future slices in the same style when
those surfaces need stable, lightweight integration APIs.

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
