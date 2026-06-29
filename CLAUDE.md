# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

云雀 Agent (Yunque Agent) — a local-first, belief-driven Agent runtime built around **Cogni**. A single Go binary embeds the Next.js frontend and exposes a 90+ endpoint control plane. The product thesis is "信念驱动，非指令驱动" (belief-driven, not instruction-driven): a declarative `Cogni` unit describes *when* an agent activates, *what* context it injects, *which* tools it exposes, and *how* it evolves — see `README.md` and `internal/cognicore` / `internal/cognikernel`.

## Language convention (enforced by review, not tooling)

- Code comments, godoc, and architecture docs: **English**.
- Log messages, system prompts, and user-facing strings: **Chinese** (primary target market).

When editing, match the surrounding file. A Go comment stays English even in a file whose log strings are Chinese.

## Common commands

All `make` targets assume a POSIX shell; on Windows use the Bash tool (Git Bash) for them. Go/npm commands run fine in PowerShell.

```bash
# Build
make build           # Go binary + placeholder frontend (dev; fast, no Node needed)
make build-full      # Frontend (npm ci && npm run build) + Go binary — release-shaped
make release         # Cross-compile 6 platforms with frontend
go build -o yunque-agent ./cmd/agent   # raw dev build, skips frontend entirely

# Run
go run ./cmd/agent   # starts gateway on :9090, opens browser to /dashboard
go run ./cmd/setup   # web-based .env setup wizard (also reachable at /setup once running)

# Test
make test                              # go test ./... -count=1  (web-ensure first)
go test ./internal/agentcore/... -run TestName   # single Go package / test
make test-web                          # cd apps/web && npm test && npm run typecheck
make test-all                          # Go + web
cd apps/web && npm test                # sdk-boundaries check THEN vitest run
cd apps/web && npx vitest run path/to/file.test.tsx   # single web test

# Lint / vet
make lint     # golangci-lint run ./...  +  web tsc --noEmit
make vet      # go vet ./...  (lightweight, no golangci-lint install needed)
make check    # lint + test (pre-commit gate)

# Other gates
make openapi              # regenerate docs/openapi.yaml from gateway routes, then verify
make check-pack-usability # audit official packs for user-visible usefulness
```

Frontend dev server runs on **:3001** (`npm run dev`, webpack); the Go gateway runs on **:9090**.

## How the binary is assembled (read before touching build/embed)

1. The frontend is embedded via `apps/web/embed.go` (`package webui`, `//go:embed all:out`). The Go build always compiles; if `apps/web/out/_next` is absent it embeds a placeholder and the gateway falls back to the pure-HTML dashboard (`internal/controlplane/gateway/dashboard.go`). `webui.HasContent()` distinguishes a real build from the placeholder. This is why `make build` works without Node but `make build-full`/`release` hard-fail if the Next.js export is missing.
2. `cmd/agent/main.go` → `loadConfig()` → `newApp(cfg)` → `initGateway(app)`. The agent **starts even with no `.env`** — configuration is driven from the web UI (`/setup`, `/v1/setup/*`), never a blocking CLI prompt.
3. `newApp` in `cmd/agent/bootstrap.go` runs **10 ordered init phases** (storage → LLM → memory → plugins → channels → planner → browser → intelligence → tasks → extensions → training → modules). Each phase lives in its own `cmd/agent/init_*.go`. Add wiring to the matching phase; respect the dependency order (e.g. memory before planner).
4. Phase 10 (`registerModules`) mounts **hot-pluggable modules gated by `AGENT_PROFILE`** (default `standard`, see `internal/config/config.go`).

## Gateway & routing

- `internal/controlplane/gateway` is the HTTP control plane. Core routes are registered by `register*Routes()` methods spread across `routes.go`, `routes_system.go`, and sibling files (chat, task, memory, knowledge, plugin, trigger, system, governance, provider, browser, approval, rbac, reverie, plus MCPDispatch / Orchestrator / Pack / Project / Queue / SSE / Setup / Trace). The header comment in `routes.go` is a useful map but lags the code — `grep -rhoE 'func \(g \*Gateway\) register[A-Za-z]+Routes'` over the gateway package is the authoritative list. Many feature routes (fork, subagent, bots, persona, emotion, lora, …) are *not* here at all — they are owned by packs and mounted via `gw.RegisterModule` (see below).
- Handlers are wired through middleware chains: `g.requireAuth(g.limiter.Middleware(g.guardNoOfflineRole(handler)))`. `guardNoOfflineRole` hard-blocks (403) front-stage requests targeting the offline background engine so its latency never leaks into user-facing paths.
- Auth: every `/v1/*` and `/api/*` route needs `X-API-Key` or `Authorization: Bearer <jwt>`.
- After changing routes, run `make openapi` — `docs/openapi.yaml` is generated from the route table and its regeneration is test-verified.

## Packs (capability packs)

Packs are the "optional capability" layer (`README.md` 能力分层 table). The split matters:

- **Backend** lives in `internal/packs/<name>` (≈65 packages), each a self-contained handler set. Packs mount onto the gateway via `gw.RegisterModule` (called from `cmd/agent/init_*.go`), *not* by adding routes to `routes.go`. The gateway only gates and mounts; business logic stays in the pack package. Routes migrated out of the monolith leave a breadcrumb comment in `routes.go` pointing to the owning pack.
- **Manifests**: `packs/official/<name>-pack/` (≈67), validated against `packs/pack.schema.json`. Pack IDs match `^yunque\.pack\.[a-z0-9-]+$`.
- **Frontend**: pages under `apps/web/src/app/packs/`.

Per the project memory: pack backends are real implementations (not stubs); open work is frontend UX and runtime wiring, not backend gaps.

## SDK boundary (will fail `npm test`)

`packages/yunque-client` is the TypeScript SDK and the **single source of truth** for the frontend↔backend contract. `apps/web/scripts/check-sdk-boundaries.mjs` enforces that specific web adapter/consumer files import from the SDK subpath (e.g. `src/lib/packs-client.ts` must consume `yunque-client/packs`). This check runs first in `npm test` and in `npm run check:sdk-boundaries`. When adding a pack client adapter, register it in that script's `sdkBackedAdapters` / `directSdkConsumers` lists.

## Cogni runtime (the core abstraction)

`Cogni` is not an optional workload — it is how the agent organizes capability. Key locations:

- `pkg/cogni`, `pkg/cognisdk`, `pkg/belief` — public contracts/types.
- `internal/cognicore` — react / eval / curiosity / causal / metacog / trait / taskdistill.
- `internal/cognikernel` — the **three-loop runtime**: `active_loop.go` (current task), `reflective_loop.go` (post-task experience extraction), `dreaming_loop.go` (idle replay/exploration/skill growth). `kernel.go` is the entry; `immune_bridge.go` and `events.go` connect it to the rest.

Recent commits consolidated `cognisdk` belief into a unified `CogniRuntime` and unified the injection path — when working here, prefer the unified runtime over the older split paths.

## Storage

Embedded SQLite (`modernc.org/sqlite`, pure-Go, no CGO) is the default — data at `data/yunque.db`, plus a Ledger KV with ~25 namespaces (`internal/ledger`, `internal/ledgercore`). Setting `DATABASE_URL` switches to PostgreSQL + pgvector. Migrations in `migrations/`.

## Conventions

- **Commits**: Conventional Commits, enforced by `.githooks/commit-msg`. Enable with `git config core.hooksPath .githooks`. Scopes: `cogni|planner|gateway|session|task|approval|storage|sdk|ui|api|cli|deps|ci`. Subject in present tense, not capitalized, no trailing period. See `.gitmessage`.
- **Security fail-closed**: `cmd/agent/bootstrap.go` refuses to start in production-like deployments (`YUNQUE_ENV=production` or bound to a non-loopback address) when `JWT_SECRET` is a known placeholder/weak. Override for dev only with `YUNQUE_ALLOW_WEAK_SECRETS=true`.
- **No external web framework** — backend is Go stdlib `net/http` only.

## Desktop gotcha

If the Tauri desktop build fails with a `云雀` ghost path, delete `apps/desktop/src-tauri/target/` and rebuild (stale target artifact).
