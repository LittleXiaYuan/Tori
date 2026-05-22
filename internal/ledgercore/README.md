# Ledger — Agent State Core

Ledger is the lightweight state infrastructure layer for long-running AI agent runtimes. It is the durable source for **what happened**, **what changed**, **what was remembered**, and **what can be replayed or recalled**.

Ledger intentionally stays out of agent decision-making. It records reasoning traces, task transitions, memories, graph links, vector indexes, artifacts, checkpoints, and KV-backed runtime data; orchestration policies such as ReAct execution, Plan-Execute-Reflect, evaluation, curiosity, world-model updates, and causal reasoning live in the host agent runtime (for example `yunque-agent/internal/agentcore`).

See [`BOUNDARY.md`](./BOUNDARY.md) for the data-plane / decision-plane boundary.

## Design boundary

| Layer | Responsibility | Examples |
|---|---|---|
| Ledger data plane | Persist, query, replay, recall, stream state | tasks, events, checkpoints, memory, vector, graph, KV, artifacts |
| Agent decision plane | Decide what to do next | planning, tool choice, ReAct/PER loops, evaluation, curiosity, world model |
| Integration adapters | Connect host runtime to Ledger | yunque `internal/ledger/*`, planner recall bridge, memory bridge, KV migrators |

This keeps Ledger usable as an SDK-like state package instead of a heavy platform dependency.

## Quick Start

```go
import (
    "context"
    "encoding/json"
    "log"

    "yunque-agent/internal/ledgercore"
    lsqlite "yunque-agent/internal/ledgercore/backend/sqlite"
)

func main() {
    // 1. Create backend (zero-config SQLite)
    backend, err := lsqlite.New("./data/ledger.db")
    if err != nil { log.Fatal(err) }

    // 2. Open Ledger (auto-migrates)
    ldg, err := ledger.Open(backend)
    if err != nil { log.Fatal(err) }
    defer ldg.Close()

    // 3. Record task state
    ctx := context.Background()
    task, _ := ldg.Tasks.CreateTask(ctx, "Write a report", ledger.TaskTypeGoal, "tenant-1")
    _ = ldg.Tasks.Transition(ctx, task.ID, ledger.TaskReady, "runtime", nil)
    _ = ldg.Tasks.Transition(ctx, task.ID, ledger.TaskRunning, "runtime", nil)
    _ = ldg.Tasks.Complete(ctx, task.ID, json.RawMessage(`{"result":"done"}`))
}
```

## Public surface

```
ldg.Tasks       — Task lifecycle (Create, Transition, Complete, Fail, Cancel)
ldg.Events      — Append-only event log (Append, List, Query, Replay, EmitTransition, GetReasoningTrace)
ldg.Checkpoints — Execution snapshots (Save, Latest, List, Cleanup)
ldg.Resume      — Crash recovery (Resume from checkpoint + incremental replay)
ldg.Memory      — Structured memory (Put, Get, Search, Delete)
ldg.Recall      — Task-aware recall (metadata → graph → semantic → rerank)
ldg.Vector      — Semantic embedding search (Put, Search, Embed)
ldg.Graph       — Entity relationship graph (Link, Neighbors, FindRelated)
ldg.Lifecycle   — Memory consolidation, decay, and GC
ldg.Artifacts   — Output metadata (Save, Get, List)
ldg.Deps        — Task dependencies (Create, Satisfy, Blockers, IsBlocked)
ldg.Bus         — Real-time event streaming (Subscribe, Unsubscribe, Publish)
ldg.KV          — Namespaced JSON KV store for runtime/config state
ldg.Reasoning() — Reasoning trace recorder (Observe, Think, Decide, Backtrack, Plan, Reflect)
```

Decision-plane modules that previously lived in Ledger have been moved to the host agent runtime. Ledger still keeps protocol/shared types in `types_react.go` so host runtimes can persist traces and results without import cycles.

## Task state machine

```
[*] → created → ready → running → completed
                                 → failed → ready (restart)
                                 → waiting_input → running
                                 → blocked → ready
                                 → retrying → running / failed
                                 → cancelled → ready (reopen)
```

9 states, all transitions validated. Every transition emits an event.

## Reasoning trace recorder

Ledger records reasoning as events; it does not choose the next action.

```go
tracer := ldg.Reasoning(taskID, "planner")
tracer.Observe(ctx, "User wants X", nil)
tracer.Think(ctx, "I should try approach A", nil)
tracer.Decide(ctx, "use_tool_A", "best for this case", 0.8, nil)
tracer.Backtrack(ctx, "tool A failed", "try tool B", nil)
tracer.Reflect(ctx, "Tool B worked, lesson learned", 0.9, nil)

trace, _ := ldg.Events.GetReasoningTrace(ctx, taskID)
fmt.Println(trace.Summary.Thoughts, trace.Summary.Backtracks)
```

Host runtimes can implement ReAct / PER / evaluator / causal / curiosity loops and use `ldg.Reasoning(...)` to persist observability.

## Event streaming

```go
sub := ldg.Bus.Subscribe(ledger.EventFilter{
    TaskIDs:   []string{taskID},
    Reasoning: true,
}, 64)
defer ldg.Bus.Unsubscribe(sub)

for event := range sub.C {
    fmt.Println(event.Kind, event.Payload)
}
```

## Temporal query

```go
events, _ := ldg.Events.Query(ctx, ledger.EventQuery{
    TaskID: taskID,
    Kinds:  []ledger.EventKind{ledger.EventReasoningDecision},
    After:  &startTime,
    Before: &endTime,
    Limit:  100,
})
```

## Event sourcing

All lifecycle mutations produce immutable events. The `tasks` table is a materialized view.

```go
task, _ := ldg.Events.Replay(ctx, taskID)  // reconstruct from events
state, _ := ldg.Resume.Resume(ctx, taskID) // resume from checkpoint
```

## Memory & recall

```go
ldg.Memory.PutFact(ctx, "tenant-1", "user.name", "Alice", "user")
ldg.Memory.PutPreference(ctx, "tenant-1", "lang", "Go")

result, _ := ldg.Recall.Recall(ctx, ledger.RecallQuery{
    TenantID: "tenant-1",
    Query:    "programming language",
    TaskGoal: "Write a Go tutorial",
    TaskType: ledger.TaskTypeGoal,
    Limit:    5,
})
```

## KV state package pattern

Ledger KV is used by `yunque-agent` to replace scattered JSON files with a single SQLite-backed state package:

```go
_ = ldg.KV.Put(ctx, "trust", "scores", trustScores)
var scores TrustScores
found, err := ldg.KV.Get(ctx, "trust", "scores", &scores)
```

This keeps external SDKs, plugins, and automation scripts able to reason about stable state surfaces without coupling to a monolithic platform internals.

## yunque-agent integration map

In `yunque-agent`, Ledger is initialized early and then exposed to narrower adapters:

| yunque path | Role |
|---|---|
| `cmd/agent/init_storage.go` | Initializes SQLite Ledger and registers it in the app runtime |
| `internal/ledger/adapter.go` | Adapts Ledger tasks to the existing task store interface |
| `internal/ledger/memory_bridge.go` | Writes task success/failure experiences into Ledger Memory |
| `internal/ledger/recall_bridge.go` | Feeds Ledger Recall results into the planner context |
| `internal/ledger/kv_migrate.go` | Migrates JSON state files into Ledger KV namespaces |
| `internal/ledger/ledger_orch_persister.go` | Persists orchestrator graph/editable memory via Ledger |

## Storage backends

| Backend | Use Case | Config |
|---|---|---|
| SQLite (default) | Dev, desktop, single-machine deployments | `lsqlite.New("path.db")` |
| Custom backend | Advanced deployments | Implement `ledger.Backend` |

A PostgreSQL backend can be added behind the same `Backend` interface when needed, but current production wiring uses SQLite.

## Project structure

```
ledger/
├── ledger.go          — Open() entry point and subsystem wiring
├── backend.go         — Backend interface
├── types_*.go         — Core domain and shared protocol types
├── task.go            — TaskManager
├── task_fsm.go        — 9-state FSM
├── task_deps.go       — Dependency management
├── event.go           — EventStore (append, list, query, replay)
├── event_apply.go     — Event→state projection
├── eventbus.go        — Real-time pub/sub event streaming
├── checkpoint.go      — CheckpointManager
├── resume.go          — ResumeManager
├── memory.go          — MemoryStore
├── recall.go          — RecallEngine pipeline
├── recall_score.go    — Multi-factor recall scoring
├── recall_adapt.go    — Feedback-driven recall weights
├── memory_conflict.go — Memory conflict detection/resolution helpers
├── reasoning.go       — ReasoningTracer event recorder
├── vector.go          — VectorIndex and ANN adapters
├── graph.go           — ContextGraph relationships
├── lifecycle.go       — Memory decay, GC, consolidation
├── artifact.go        — ArtifactManager
├── kv.go              — Namespaced JSON KV facade
├── sync*.go           — Cross-instance sync and HTTP sync helpers
├── backend/sqlite/    — SQLite implementation
└── internal/ulid/     — Time-ordered IDs
```
