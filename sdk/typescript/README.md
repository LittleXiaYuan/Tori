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
