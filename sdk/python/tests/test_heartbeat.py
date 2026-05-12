from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_heartbeat_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_heartbeat_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class HeartbeatNamespaceTest(unittest.TestCase):
    def test_heartbeat_helpers_delegate_to_lifecycle_routes(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            if path.startswith("/v1/heartbeat/logs"):
                return [{"id": "hb1"}]
            if path == "/v1/heartbeat":
                return {"running": True} if method == "GET" else {"status": "ok"}
            return {"id": "hb1", "summary": "checked"}

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertTrue(yunque.heartbeat.status()["running"])
            self.assertEqual(yunque.heartbeat.update(enabled=True, interval_minutes=30)["status"], "ok")
            self.assertEqual(yunque.heartbeat.trigger()["id"], "hb1")
            self.assertEqual(yunque.heartbeat.logs(limit=2)[0]["id"], "hb1")

        self.assertEqual(calls[0], ("GET", "/v1/heartbeat", None))
        self.assertEqual(calls[1], ("PUT", "/v1/heartbeat", {"enabled": True, "interval_minutes": 30}))
        self.assertEqual(calls[2], ("POST", "/v1/heartbeat/trigger", {}))
        self.assertEqual(calls[3], ("GET", "/v1/heartbeat/logs?limit=2", None))


if __name__ == "__main__":
    unittest.main()
