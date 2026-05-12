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
            self.assertEqual(kit.plugin.search("agent kit", limit=2)[0]["title"], "Agent Kit")
            kit.memory.set("last", "ok")

        self.assertIs(kit.state, yunque.state)
        self.assertIs(kit.reflect, yunque.reflect)
        self.assertIs(kit.cron_system, yunque.cron_system)
        self.assertIs(kit.triggers, yunque.triggers)
        self.assertIs(kit.plugin, yunque.plugin)
        self.assertIs(kit.memory, yunque.memory)
        self.assertEqual(calls[4], ("POST", "/v1/plugin-api/search", {"query": "agent kit", "limit": 2}))

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
