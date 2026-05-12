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


## Agent Kit bundle

Use `create_agent_kit()` when an external script wants the common lightweight
surfaces from one object: State Kernel, Reflection Experience, and Plugin API
Runtime plus Mission Parse and Scheduler. It reuses the same module-level helpers and does not pull in a
generated OpenAPI client.

```python
import yunque

kit = yunque.create_agent_kit()

focus = kit.state.focus()
strategies = kit.reflect.strategies(tag="sdk", limit=5)
mission = kit.missions.parse("每天八点总结昨天的任务")
scheduler_jobs = kit.scheduler.jobs()
results = kit.plugin.search("incremental SDK package", limit=5)

kit.memory.set("last_focus", focus)
print(focus, strategies, mission["type"], scheduler_jobs["count"], len(results))
```

## Mission Parse helpers

Use `yunque.missions.parse()` when an external script, plugin, or page wants to
turn natural-language intent into a task/workflow/cron/trigger draft without
importing the full platform client.

```python
mission = yunque.missions.parse("每天八点总结昨天的任务")
print(mission["type"], mission["name"], mission.get("config", {}))
```

## Scheduler helpers

Use `yunque.scheduler` when an external script, plugin, or sidecar needs to
list, add, or remove prompt-based recurring jobs.

```python
jobs = yunque.scheduler.jobs()
job = yunque.scheduler.add("hourly", "检查任务", "1h")
removed = yunque.scheduler.remove(job["id"])
print(jobs["count"], job["id"], removed["status"])
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

## Reflection Experience helpers

```python
import yunque

experiences = yunque.reflect.experiences(
    q="code review",
    tag="quality:9",
    limit=5,
)

stats = yunque.reflect.stats(tag="quality:9")
strategies = yunque.reflect.strategies(tag="quality:9", limit=3)
print(stats["total"])
print(strategies)
```

This slice is intended for automation scripts and plugins that want to reuse
agent lessons / strategy hints without depending on the full platform client.

## Runnable state snapshot example

Start `yunque-agent` first so `/v1/state` is reachable, then run from the repo
root:

```powershell
python sdk/python/examples/state_snapshot.py
python sdk/python/examples/reflect_strategies.py
```


## Triggers 触发器自动化切片

Python 插件脚本或自动化任务可以用 `yunque.triggers` 管理 Triggers v2 定义、触发事件并读取运行/事件记录。

```python
defs = yunque.triggers.list(status="enabled")
created = yunque.triggers.create({
    "name": "review done",
    "tenant_id": "default",
    "type": "event",
    "actions": [{"kind": "notify"}],
})
yunque.triggers.emit("review.done", data={"task_id": "task_1"})
runs = yunque.triggers.runs(trigger_id=created["id"], limit=10)
print(defs["total"], runs["total"])
```
