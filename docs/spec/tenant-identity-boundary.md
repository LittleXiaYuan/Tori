# Tenant / Identity / `system` Boundary

Status: living note. Scope: how `tenantID` is used across Yunque memory + recall,
why the `"system"` namespace exists, and the read/write convention that keeps
"it remembers me" working per person.

## TL;DR

- `tenantID` is **overloaded with three meanings** depending on entry point.
- `"system"` is the **shared/global namespace** for background + plugin writes.
- **Recall convention: every per-user recall = `currentTenant ∪ "system"`.**
  Enforced in both stores (ledger recall bridge + in-memory orchestrator).
- Do **not** remove tenant: in channels it is the per-person isolation key.

## `tenantID` has three identities

| Entry point | What `tenantID` actually is | Source | Value today |
|---|---|---|---|
| Web / Desktop dashboard | account / API-key tenant | `tenantFromCtx` (API key → tenant, JWT `claims.TenantID`, desktop loopback → `default`) | single-user → always `default` |
| **Channels (TG / Feishu / group chat / …)** | **end-user unified identity** (`identity.Resolver` → `profile.UnifiedID`, e.g. `u_12345_te`) | `cmd/agent/init_gateway_handler.go` (`tenantID = profile.UnifiedID`) | **load-bearing**: per-person memory isolation + cross-channel roaming |
| Future enterprise (Tori) | org / workspace tenant | control plane | not in community binary |

Implication: in a channel deployment, **different people → different `tenantID` →
isolated memories**; the same person on TG + Feishu resolves to the same
`UnifiedID` → one shared memory. Deleting tenant collapses everyone into one
memory pool (the bot mixes people up).

## The `"system"` namespace

`"system"` is the global/background scope. Writers that have no per-user request
context use it:

- Reverie inner-monologue writes — `MemManager.AddMid(ctx, "system", …)`
- Plugin agent-memory — `handleAgentMemAdd` writes `"system"`, `handleAgentMemSearch` reads `"system"` (self-consistent, plugin-global by design)
- Distilled global rules — `CompileContext(ctx, "system", …)` in `init_extensions.go`
- `MemoryBridge` fallback when a task has no tenant

## Read / write convention (the口径)

- **Background / global writes → `"system"`.**
- **Per-user writes → the active `tenantID`** (identity in channels, `default` on web).
- **Every per-user recall = `currentTenant ∪ "system"`** so global memories surface
  for everyone without leaking one user's private memories into another's.

Enforced at:

- `internal/ledger/recall_bridge.go` — `QueryTenant(ctx, tenantID, query)` unions
  `tenantID` with `"system"` (`mergeRecall`). Wired into the planner graph layer
  via `Planner.SetGraphContextForTenant` (tenant-aware).
- `internal/agentcore/memory/orchestrator.go` — `CompileContext` unions
  `tenantID` with `"system"` (`mergeRecallItems`).

## Historical bug (fixed)

The ledger recall bridge used to be pinned to `"system"`
(`NewRecallBridge(ldg, "system")` + `SetGraphContext(recallBridge.Query)`), but
task experiences are written under the active tenant. For **every channel user
and the default web user**, the graph-layer "历史经验 (Ledger Recall)" block was
therefore always empty. Fixed by the tenant-aware + `∪ system` recall above.

## Bug-class scan (write-real / read-fixed)

| Site | Verdict |
|---|---|
| ledger recall bridge (planner graph) | FIXED (tenant-aware ∪ system) |
| in-memory `CompileContext` vs reverie `"system"` writes | FIXED (∪ system) |
| plugin agent memory (`handleAgentMem*`) | OK — read/write both `"system"` |
| distilled-rules `CompileContext("system", …)` | OK — rules are intentionally global |
| reverie `SetRecall` → `CompileContext("system", …)` | Intentional — reverie is bot-global, not per-user |

## Over-sharing leak — contained (full scoping = backlog)

The in-memory **knowledge graph (`KnGraph`) and editable-memory blocks have no
`TenantID` at the data model** (`Entity` / `Block` are global). Left unguarded,
`CompileContext` would surface one channel user's graph entities / editable
blocks to another (tenant = identity).

Containment (current): `CompileContext` injects the global layers only when
`tenantSeesGlobalLayers(tenantID)` is true — i.e. the empty/primary tenant
(`OrchestratorConfig.PrimaryTenant`, set from `DEFAULT_TENANT_ID`, default
`default`). `"system"` and channel identity tenants are excluded, and global-layer
items are filtered out of the system-union, so there is no cross-person leak.

Full fix (backlog): add `TenantID` to `Entity` + `Block`, scope all writes /
reads / persistence, then drop the containment gate.

## Cleanup backlog (non-blocking)

- Rename/clarify the channel path so `identity → tenant` is explicit (the word
  "tenant" doing triple duty is the real smell, not the mechanism).
- Full per-tenant scoping of `KnGraph` + editable memory (then remove the
  `tenantSeesGlobalLayers` containment gate).
