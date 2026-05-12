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



## Cron System 主机计划任务切片

Python 插件脚本或自动化任务可以用 `yunque.cron_system` 管理主机级 `/v1/cron/*` 计划任务。它不同于 `yunque.cron` 的插件自有定时任务。

```python
jobs = yunque.cron_system.list()
added = yunque.cron_system.add(
    "nightly",
    {"type": "cron", "cron_expr": "0 2 * * *", "timezone": "Asia/Shanghai"},
    {"kind": "systemEvent", "data": {"event": "nightly"}},
)
run = yunque.cron_system.run(added["job"]["id"])
print(len(jobs["jobs"]), run["run"]["status"])
```


### Memory Kernel 宿主回忆记忆切片

Python 插件脚本或自动化任务可以用 `yunque.memory_core` 访问宿主 `/v1/memory/*` 回忆记忆层。它不同于 `yunque.memory` 的插件私有 KV。

```python
stats = yunque.memory_core.stats()
found = yunque.memory_core.search("用户偏好", limit=3)
added = yunque.memory_core.remember("用户偏好中文回复", layer="mid", source="sdk")
print(stats.get("mid"), found["count"], added["status"])
```


### Knowledge Graph 知识图谱切片

Python 插件脚本或自动化任务可以用 `yunque.graph` 访问宿主 `/v1/graph/*` 知识图谱层，读取/维护实体、关系和图谱上下文。

```python
entities = yunque.graph.entities("云雀")
entity = yunque.graph.put_entity({"name": "云雀", "type": "agent"})
context = yunque.graph.context_by_entity_id(entity["id"])
print(len(entities["entities"]), context["context"])
```


### Knowledge Base 宿主 RAG 知识库切片

Python 插件脚本或自动化任务可以用 `yunque.knowledge_base` 访问宿主 `/v1/knowledge/*` RAG 知识库。它不同于 `yunque.knowledge` 的插件运行时 knowledge helper。

```python
stats = yunque.knowledge_base.stats()
found = yunque.knowledge_base.search("增量 SDK", limit=3)
ingested = yunque.knowledge_base.ingest("外部项目可直接调用 Knowledge Base", name="sdk-note")
print(stats.get("sources"), found["count"], ingested["source"]["id"])
```


### LoRA lifecycle 宿主训练进化切片

Python 脚本、插件处理器或自动化任务可以用 `yunque.lora` 访问宿主 `/v1/lora/*` 本地脑训练生命周期能力。

```python
status = yunque.lora.status()
preview = yunque.lora.preview("default")
triggered = yunque.lora.trigger("default")
print(status.get("active_model"), preview["preview"], triggered["status"])
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
