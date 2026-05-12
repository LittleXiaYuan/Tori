from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_orchestrator_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_orchestrator_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class OrchestratorNamespaceTest(unittest.TestCase):
    def test_orchestrator_helpers(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            if path == "/v1/orchestrator/status":
                return {"running": True, "adapters": ["cursor"], "active_sessions": 1}
            if path == "/v1/orchestrator/toggle":
                return {"status": body["action"] + "ed"}
            if path == "/v1/orchestrator/sessions":
                return {"sessions": [{"session_id": "s1", "adapter": "cursor", "task_id": "t1"}]}
            if path == "/v1/orchestrator/detect":
                return {"ides": [{"name": "Cursor", "available": True}]}
            if path == "/v1/orchestrator/events?limit=2":
                return {"events": [{"id": "e1", "type": "assigned"}], "total": 1}
            if path == "/v1/orchestrator/events/task?task_id=t1":
                return {"task_id": "t1", "events": [{"id": "e1"}]}
            if path == "/v1/orchestrator/policy" and method == "GET":
                return {"allow_auto_launch": False}
            if path == "/v1/orchestrator/policy" and method == "PUT":
                return {"status": "updated", "policy": body}
            if path == "/v1/orchestrator/adapters/add":
                return {"status": "registered", "name": body["adapter_name"], "available": True}
            raise AssertionError(f"unexpected call: {method} {path}")

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertTrue(yunque.orchestrator.status()["running"])
            self.assertEqual(yunque.orchestrator.toggle("start")["status"], "started")
            self.assertEqual(yunque.orchestrator.sessions()["sessions"][0]["adapter"], "cursor")
            self.assertEqual(yunque.orchestrator.detect_ides()["ides"][0]["name"], "Cursor")
            self.assertEqual(yunque.orchestrator.events(limit=2)["total"], 1)
            self.assertEqual(yunque.orchestrator.task_timeline("t1")["task_id"], "t1")
            self.assertFalse(yunque.orchestrator.policy()["allow_auto_launch"])
            self.assertTrue(yunque.orchestrator.update_policy({"allow_auto_launch": True})["policy"]["allow_auto_launch"])
            self.assertEqual(yunque.orchestrator.add_adapter({"adapter_name": "custom", "binary": "worker.exe", "mcp_config_path": "mcp.json"})["name"], "custom")

        self.assertEqual(calls[4], ("GET", "/v1/orchestrator/events?limit=2", None))
        self.assertIs(yunque.create_agent_kit().orchestrator, yunque.orchestrator)


if __name__ == "__main__":
    unittest.main()
