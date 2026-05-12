from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_trace_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_trace_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class TraceNamespaceTest(unittest.TestCase):
    def test_trace_helpers_delegate_to_audit_routes(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            return {"events": [{"trace_id": "tr/1", "task_id": "task/1"}], "count": 1}

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertEqual(yunque.trace.recent(limit=10)["count"], 1)
            self.assertEqual(yunque.trace.by_trace_id("tr/1", raw=True)["events"][0]["trace_id"], "tr/1")
            self.assertEqual(yunque.trace.by_task_id("task/1")["events"][0]["task_id"], "task/1")

        self.assertEqual(calls[0], ("GET", "/v1/trace/recent?limit=10", None))
        self.assertEqual(calls[1], ("GET", "/v1/trace/tr%2F1?raw=true", None))
        self.assertEqual(calls[2], ("GET", "/v1/trace/task/task%2F1", None))


if __name__ == "__main__":
    unittest.main()
