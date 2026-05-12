from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_agent_kit_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_agent_kit_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class AgentKitTest(unittest.TestCase):
    def test_agent_kit_groups_state_reflect_and_plugin_runtime(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            if path == "/v1/state/focus":
                return {"focus": "sdk"}
            if path.startswith("/v1/reflect/strategies"):
                return {"strategies": "- keep SDK slices small"}
            if path == "/v1/cron/list":
                return {"jobs": [{"id": "job_1"}]}
            if path == "/v1/triggers/v2?status=enabled":
                return {"triggers": [{"id": "tr_1"}], "total": 1}
            if path == "/v1/memory/search":
                return {"results": [{"key": "pref", "value": "喜欢中文"}], "count": 1}
            if path == "/v1/graph/stats":
                return {"entities": 2, "relations": 1}
            if path == "/v1/knowledge/stats":
                return {"sources": 2, "chunks": 8}
            if path == "/v1/lora/status":
                return {"active_model": "adapter-a", "rolling_success_rate": 0.8}
            if path == "/v1/workflows":
                return {"workflows": [{"id": "wf_1", "name": "SDK flow"}], "total": 1}
            if path == "/api/connectors":
                return {"connectors": [{"id": "github", "name": "GitHub", "supported": True, "status": "connected"}]}
            if path == "/api/notify/channels":
                return {"channels": [{"id": "feishu-main", "type": "feishu", "name": "Feishu", "enabled": True}]}
            if path == "/v1/projects":
                return {"projects": [{"id": "p1", "name": "云雀", "repo_path": "C:/repo"}]}
            if path == "/v1/market/stats":
                return {"total": 3, "categories": {"coding": 1}}
            if path == "/v1/workers":
                return {"workers": [{"id": "w1", "type": "cursor"}], "count": 1}
            if path == "/v1/orchestrator/status":
                return {"running": True, "adapters": ["cursor"], "active_sessions": 1}
            if path == "/v1/fork/list?session_id=s1":
                return {"forks": [{"id": "fork_1", "session_id": "s1", "messages": [], "created_at": "2026-05-12T00:00:00Z"}]}
            if path == "/v1/cost/summary":
                return {"today_cost": 0.12, "month_cost": 1.5}
            if path == "/api/providers":
                return {"providers": [{"id": "deepseek", "model": "deepseek-chat"}], "mode": "hybrid"}
            if path == "/v1/cognis":
                return {"cognis": [{"id": "reviewer", "name": "Code Reviewer"}], "count": 1}
            if path == "/v1/trace/recent?limit=1":
                return {"events": [{"trace_id": "tr-1"}], "count": 1}
            if path == "/v1/heartbeat":
                return {"running": True}
            if path == "/v1/reverie/stats":
                return {"total": 2}
            if path == "/v1/chat":
                return {"reply": "hello from chat"}
            if path == "/v1/conversations":
                return {"sessions": [{"id": "s1"}], "count": 1}
            if path == "/v1/approvals?status=pending":
                return {"approvals": [{"id": "ap1", "status": "pending"}], "total": 1}
            if path == "/v1/rbac/my-roles":
                return {"subject_id": "u1", "roles": [{"id": "operator", "name": "Operator", "permissions": []}], "total": 1}
            if path == "/api/files?path=artifacts":
                return {"files": [{"name": "report.md", "path": "artifacts/report.md", "size": 12, "is_dir": False}]}
            if path == "/v1/browser/status":
                return {"connected": True, "state": "extension"}
            if path == "/v1/sessions/queue":
                return {"queues": {"s1": 1}}
            if path == "/v1/subagent?parent_id=task-1":
                return {"subagents": [{"id": "sa-1", "name": "reviewer"}]}
            if path == "/v1/tools/list":
                return {"sessions": [{"id": "tool-1", "command": "npm test", "state": "running"}]}
            if path == "/v1/audit/verify":
                return {"valid": True, "checked": 1}
            if path == "/api/trust/scores":
                return {"scores": {"shell": {"score": 80}}, "count": 1}
            if path == "/api/iterate/proposals?status=pending":
                return {"proposals": [{"id": "it-1", "status": "pending"}], "count": 1}
            if path == "/v1/persona":
                return {"identity": "Tori", "soul": "careful", "skills": [{"name": "review"}]}
            if path == "/v1/plugin-api/search":
                return {"results": [{"title": "Agent Kit"}]}
            if path == "/v1/plugin-api/memory/set":
                return {"ok": True}
            raise AssertionError(f"unexpected call: {method} {path}")

        kit = yunque.create_agent_kit()
        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertEqual(kit.state.focus(), "sdk")
            self.assertIn("SDK slices", kit.reflect.strategies(tag="sdk"))
            self.assertEqual(len(kit.cron_system.list()["jobs"]), 1)
            self.assertEqual(kit.triggers.list(status="enabled")["total"], 1)
            self.assertEqual(kit.memory_core.search("中文", limit=1)["count"], 1)
            self.assertEqual(kit.graph.stats()["entities"], 2)
            self.assertEqual(kit.knowledge_base.stats()["sources"], 2)
            self.assertEqual(kit.lora.status()["active_model"], "adapter-a")
            self.assertEqual(kit.workflows.list()["total"], 1)
            self.assertEqual(kit.connectors.list()["connectors"][0]["id"], "github")
            self.assertEqual(kit.notify.channels()["channels"][0]["id"], "feishu-main")
            self.assertEqual(kit.projects.list()["projects"][0]["id"], "p1")
            self.assertEqual(kit.market.stats()["total"], 3)
            self.assertEqual(kit.dispatch.workers()["count"], 1)
            self.assertTrue(kit.orchestrator.status()["running"])
            self.assertEqual(kit.fork.list("s1")["forks"][0]["id"], "fork_1")
            self.assertEqual(kit.cost.summary()["today_cost"], 0.12)
            self.assertEqual(kit.providers.list()["providers"][0]["id"], "deepseek")
            self.assertEqual(kit.cognis.list()["cognis"][0]["id"], "reviewer")
            self.assertEqual(kit.trace.recent(limit=1)["events"][0]["trace_id"], "tr-1")
            self.assertTrue(kit.heartbeat.status()["running"])
            self.assertEqual(kit.events.parse("event: connected\ndata: {\"ok\":true}\n\n")[0]["data"]["ok"], True)
            self.assertEqual(kit.reverie.stats()["total"], 2)
            self.assertEqual(kit.realtime.parse(kit.realtime.serialize(kit.realtime.chat("你好", session="s1")))["session"], "s1")
            self.assertEqual(kit.chat.send([{"role": "user", "content": "hi"}])["reply"], "hello from chat")
            self.assertEqual(kit.conversations.list()["sessions"][0]["id"], "s1")
            self.assertEqual(kit.approvals.pending()["approvals"][0]["id"], "ap1")
            self.assertEqual(kit.rbac.my_roles()["roles"][0]["id"], "operator")
            self.assertEqual(kit.files.list("artifacts")["files"][0]["name"], "report.md")
            self.assertTrue(kit.browser.status()["connected"])
            self.assertEqual(kit.runtime.queues()["queues"]["s1"], 1)
            self.assertEqual(kit.subagents.list("task-1")["subagents"][0]["id"], "sa-1")
            self.assertEqual(kit.tools.list()["sessions"][0]["id"], "tool-1")
            self.assertTrue(kit.audit.verify()["valid"])
            self.assertEqual(kit.trust.scores()["scores"]["shell"]["score"], 80)
            self.assertEqual(kit.iterate.pending_proposals()["proposals"][0]["id"], "it-1")
            self.assertEqual(kit.persona.get()["identity"], "Tori")
            self.assertEqual(kit.plugin.search("agent kit", limit=2)[0]["title"], "Agent Kit")
            kit.memory.set("last", "ok")

        self.assertIs(kit.state, yunque.state)
        self.assertIs(kit.reflect, yunque.reflect)
        self.assertIs(kit.cron_system, yunque.cron_system)
        self.assertIs(kit.triggers, yunque.triggers)
        self.assertIs(kit.memory_core, yunque.memory_core)
        self.assertIs(kit.graph, yunque.graph)
        self.assertIs(kit.knowledge_base, yunque.knowledge_base)
        self.assertIs(kit.lora, yunque.lora)
        self.assertIs(kit.workflows, yunque.workflows)
        self.assertIs(kit.connectors, yunque.connectors)
        self.assertIs(kit.notify, yunque.notify)
        self.assertIs(kit.projects, yunque.projects)
        self.assertIs(kit.market, yunque.market)
        self.assertIs(kit.dispatch, yunque.dispatch)
        self.assertIs(kit.orchestrator, yunque.orchestrator)
        self.assertIs(kit.fork, yunque.fork)
        self.assertIs(kit.cost, yunque.cost)
        self.assertIs(kit.providers, yunque.providers)
        self.assertIs(kit.cognis, yunque.cognis)
        self.assertIs(kit.trace, yunque.trace)
        self.assertIs(kit.heartbeat, yunque.heartbeat)
        self.assertIs(kit.events, yunque.events)
        self.assertIs(kit.reverie, yunque.reverie)
        self.assertIs(kit.realtime, yunque.realtime)
        self.assertIs(kit.chat, yunque.chat_sdk)
        self.assertIs(kit.conversations, yunque.conversations)
        self.assertIs(kit.approvals, yunque.approvals)
        self.assertIs(kit.rbac, yunque.rbac)
        self.assertIs(kit.files, yunque.files)
        self.assertIs(kit.browser, yunque.browser)
        self.assertIs(kit.runtime, yunque.runtime)
        self.assertIs(kit.subagents, yunque.subagents)
        self.assertIs(kit.tools, yunque.tools)
        self.assertIs(kit.audit, yunque.audit)
        self.assertIs(kit.trust, yunque.trust)
        self.assertIs(kit.iterate, yunque.iterate)
        self.assertIs(kit.persona, yunque.persona)
        self.assertIs(kit.plugin, yunque.plugin)
        self.assertIs(kit.memory, yunque.memory)
        self.assertEqual(calls[21], ("GET", "/v1/reverie/stats", None))
        self.assertEqual(calls[22], ("POST", "/v1/chat", {"messages": [{"role": "user", "content": "hi"}]}))
        self.assertEqual(calls[23], ("GET", "/v1/conversations", None))
        self.assertEqual(calls[24], ("GET", "/v1/approvals?status=pending", None))
        self.assertEqual(calls[25], ("GET", "/v1/rbac/my-roles", None))
        self.assertEqual(calls[26], ("GET", "/api/files?path=artifacts", None))
        self.assertEqual(calls[27], ("GET", "/v1/browser/status", None))
        self.assertEqual(calls[28], ("GET", "/v1/sessions/queue", None))
        self.assertEqual(calls[29], ("GET", "/v1/subagent?parent_id=task-1", None))
        self.assertEqual(calls[30], ("GET", "/v1/tools/list", None))
        self.assertEqual(calls[31], ("GET", "/v1/audit/verify", None))
        self.assertEqual(calls[32], ("GET", "/api/trust/scores", None))
        self.assertEqual(calls[33], ("GET", "/api/iterate/proposals?status=pending", None))
        self.assertEqual(calls[34], ("GET", "/v1/persona", None))
        self.assertEqual(calls[35], ("POST", "/v1/plugin-api/search", {"query": "agent kit", "limit": 2}))

    def test_plugin_runtime_namespace_delegates_extension_registration(self) -> None:
        with patch.object(yunque, "_api_call", return_value={"ok": True, "provider_id": "local"}) as api_call:
            result = yunque.plugin.register_provider("local", "http://localhost:11434/v1", "llama3")

        self.assertEqual(result["provider_id"], "local")
        api_call.assert_called_once_with(
            "POST",
            "/v1/plugin-api/register/provider",
            {"id": "local", "base_url": "http://localhost:11434/v1", "model": "llama3", "type": "chat"},
        )


if __name__ == "__main__":
    unittest.main()
