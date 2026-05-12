# Yunque Go SDK

Lightweight Go SDK for Yunque Agent plugins, sidecars, CLIs, and automation scripts.
It intentionally exposes small typed namespaces instead of forcing external code to
link against the full platform internals.

## Environment

The SDK reads the same environment variables used by plugin processes:

```powershell
$env:YUNQUE_API_BASE = "http://localhost:9090"
$env:YUNQUE_PLUGIN_TOKEN = "<plugin-or-api-token>"
$env:YUNQUE_PLUGIN_NAME = "my-state-sidecar"
```

`YUNQUE_API_BASE` defaults to `http://localhost:9090` when omitted.

## State Kernel incremental helpers

Use `yunque.State` when an external project only needs the agent's current
state layer: goals, tracked resources, focus, recent actions, and capability
summary.

```go
package main

import (
    "context"
    "fmt"
    "log"

    "yunque-agent/sdk/go/yunque"
)

func main() {
    ctx := context.Background()

    snap, err := yunque.State.Snapshot(ctx)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("focus:", snap.Focus)
    fmt.Println("goals:", len(snap.Goals))
    fmt.Println("resources:", len(snap.Resources))
    fmt.Println("recent actions:", len(snap.RecentActions))
    fmt.Println("skills:", snap.Capabilities.TotalSkills)
}
```

Focused helpers are available when callers want smaller intent-revealing access:

```go
goals, _ := yunque.State.Goals(ctx)
actions, _ := yunque.State.Actions(ctx)
caps, _ := yunque.State.Capabilities(ctx)
focus, _ := yunque.State.Focus(ctx)
resources, _ := yunque.State.Resources(ctx)
```

Goal mutation is typed but still narrow:

```go
saved, err := yunque.State.SaveGoal(ctx, yunque.StateGoal{
    Title:    "Ship a small SDK slice",
    Priority: 2,
})
```

## Runnable state snapshot example

Start `yunque-agent` first so `/v1/state` is reachable, then run from the repo root:

```powershell
go run ./sdk/go/examples/state_snapshot
```

The example prints a concise state summary as JSON, which is suitable for shell
scripts, dashboards, and CI smoke checks.

## Reflection Experience helpers

Use `yunque.Reflect` when a plugin, sidecar, or automation tool wants to reuse
agent lessons and strategy hints without importing platform internals.

```go
experiences, err := yunque.Reflect.Experiences(ctx, yunque.ReflectExperienceOptions{
    Query: "code review",
    Tag:   "quality:9",
    Limit: 5,
})

stats, err := yunque.Reflect.Stats(ctx, yunque.ReflectExperienceOptions{
    Tag: "quality:9",
})

strategies, err := yunque.Reflect.StrategiesWithOptions(ctx, yunque.ReflectStrategyOptions{
    Tag:   "quality:9",
    Limit: 3,
})
```
