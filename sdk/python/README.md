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

## Runnable state snapshot example

Start `yunque-agent` first so `/v1/state` is reachable, then run from the repo
root:

```powershell
python sdk/python/examples/state_snapshot.py
```
