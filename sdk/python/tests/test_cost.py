from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_cost_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_cost_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class CostNamespaceTest(unittest.TestCase):
    def test_cost_helpers_delegate_to_runtime_routes(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            if path == "/v1/cost/summary":
                return {"today_cost": 0.12}
            if path == "/v1/cost/budget":
                return {"ok": True}
            if path == "/v1/cost/task?id=task%2F1":
                return {"total_cost_usd": 0.2}
            if path == "/v1/cost/task/timeline?id=task%2F1":
                return {"records": []}
            if path == "/v1/cost/breakdown":
                return {"by_provider": []}
            if path == "/v1/cost/history?page=2&limit=25&model=gpt-test":
                return {"records": [], "page": 2}
            if path == "/v1/cost/alerts":
                return {"alerts": []}
            if path == "/v1/usage":
                return {"tenant_id": "tenant-1"}
            if path == "/v1/quota":
                return {"status": "ok"}
            raise AssertionError(f"unexpected call: {method} {path}")

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertEqual(yunque.cost.summary()["today_cost"], 0.12)
            self.assertTrue(yunque.cost.set_budget({"daily_limit_usd": 1})["ok"])
            self.assertEqual(yunque.cost.task("task/1")["total_cost_usd"], 0.2)
            self.assertEqual(yunque.cost.task_timeline("task/1")["records"], [])
            self.assertEqual(yunque.cost.breakdown()["by_provider"], [])
            self.assertEqual(yunque.cost.history(page=2, limit=25, model="gpt-test")["page"], 2)
            self.assertEqual(yunque.cost.alerts()["alerts"], [])
            self.assertEqual(yunque.cost.usage()["tenant_id"], "tenant-1")
            self.assertEqual(yunque.cost.set_quota({"max_chat_calls": 10}, tenant_id="tenant-1")["status"], "ok")

        self.assertEqual(calls[1], ("POST", "/v1/cost/budget", {"daily_limit_usd": 1}))
        self.assertEqual(calls[-1], ("POST", "/v1/quota", {"quota": {"max_chat_calls": 10}, "tenant_id": "tenant-1"}))


if __name__ == "__main__":
    unittest.main()
