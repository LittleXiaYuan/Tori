from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_reverie_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_reverie_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class ReverieNamespaceTest(unittest.TestCase):
    def test_reverie_helpers_delegate_to_proactive_routes(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            if path.startswith("/v1/reverie/journal"):
                return {"thoughts": [{"id": "t1"}], "total": 1}
            if path == "/v1/reverie/stats":
                return {"total": 2}
            if path == "/v1/reverie/config":
                return {"config": {"enabled": False}} if method == "PUT" else {"config": {"enabled": True}}
            if path == "/v1/reverie/actions":
                return {"actions": [{"id": "a1"}], "total": 1}
            if path == "/v1/reverie/targets":
                return {"targets": [{"channel": "feishu"}], "count": 1}
            if path.startswith("/v1/reverie/thought"):
                return {"deleted": True, "id": "t1"}
            return {"thought": {"id": "t2"}}

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertEqual(yunque.reverie.journal(category="task", delivered=False, limit=10)["total"], 1)
            self.assertEqual(yunque.reverie.stats()["total"], 2)
            self.assertTrue(yunque.reverie.config()["config"]["enabled"])
            self.assertFalse(yunque.reverie.update_config({"enabled": False})["config"]["enabled"])
            self.assertEqual(yunque.reverie.think(event_type="task_completed", trigger="sdk")["thought"]["id"], "t2")
            self.assertTrue(yunque.reverie.delete_thought("t1")["deleted"])
            self.assertEqual(yunque.reverie.actions()["total"], 1)
            self.assertEqual(yunque.reverie.targets()["count"], 1)

        self.assertEqual(calls[0], ("GET", "/v1/reverie/journal?category=task&delivered=false&limit=10", None))
        self.assertEqual(calls[3], ("PUT", "/v1/reverie/config", {"enabled": False}))
        self.assertEqual(calls[4], ("POST", "/v1/reverie/think", {"event_type": "task_completed", "trigger": "sdk"}))
        self.assertEqual(calls[5], ("DELETE", "/v1/reverie/thought?id=t1", None))


if __name__ == "__main__":
    unittest.main()
