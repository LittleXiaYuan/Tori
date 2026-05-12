from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_dispatch_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_dispatch_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class DispatchNamespaceTest(unittest.TestCase):
    def test_dispatch_helpers(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            if path == "/v1/workers":
                return {"workers": [{"id": "w1", "type": "cursor"}], "count": 1}
            if path == "/v1/workers/detail?id=w1":
                return {"id": "w1", "type": "cursor", "capabilities": ["coding"]}
            if path == "/v1/workers/remove":
                return {"status": "removed"}
            if path == "/v1/dispatch/queue":
                return {"message": "dispatch queue (use task system for now)"}
            if path == "/v1/dispatch/enqueue":
                return {"task_id": body["task_id"], "status": "enqueued"}
            if path == "/v1/workers/config?type=cursor":
                return {"type": "cursor", "server_url": "http://localhost:9090/mcp/v1"}
            raise AssertionError(f"unexpected call: {method} {path}")

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertEqual(yunque.dispatch.workers()["count"], 1)
            self.assertEqual(yunque.dispatch.worker("w1")["type"], "cursor")
            self.assertEqual(yunque.dispatch.remove_worker("w1")["status"], "removed")
            self.assertIn("dispatch queue", yunque.dispatch.queue()["message"])
            self.assertEqual(yunque.dispatch.enqueue("t1", capabilities=["coding"], priority=10)["status"], "enqueued")
            self.assertEqual(yunque.dispatch.worker_config("cursor")["type"], "cursor")

        self.assertEqual(calls[4], ("POST", "/v1/dispatch/enqueue", {"task_id": "t1", "capabilities": ["coding"], "priority": 10}))
        self.assertIs(yunque.create_agent_kit().dispatch, yunque.dispatch)


if __name__ == "__main__":
    unittest.main()
