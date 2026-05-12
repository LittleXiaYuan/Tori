from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_triggers_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_triggers_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class TriggersNamespaceTest(unittest.TestCase):
    def test_list_emit_history_and_control(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            if path == "/v1/triggers/v2?tenant_id=default&type=event&status=enabled":
                return {"triggers": [{"id": "tr_1", "name": "review done"}], "total": 1}
            if method == "GET" and path == "/v1/triggers/v2?id=tr_1":
                return {"id": "tr_1", "name": "review done"}
            if method in {"POST", "PUT"} and path == "/v1/triggers/v2":
                return {"id": "tr_1", **body}
            if method == "DELETE" and path == "/v1/triggers/v2?id=tr_1":
                return {"deleted": "tr_1"}
            if path == "/v1/triggers/v2/emit":
                return {"status": "emitted", "event": body["event"]}
            if path == "/v1/triggers/v2/runs?trigger_id=tr_1&limit=2":
                return {"runs": [{"id": "run_1"}], "total": 1}
            if path == "/v1/triggers/v2/events?limit=3":
                return {"events": [{"event": "review.done"}], "total": 1}
            raise AssertionError(f"unexpected call: {method} {path} {body}")

        definition = {"name": "review done", "tenant_id": "default", "type": "event", "status": "enabled", "actions": [{"kind": "notify"}]}
        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            listed = yunque.triggers.list(tenant_id="default", type="event", status="enabled")
            got = yunque.triggers.get("tr_1")
            created = yunque.triggers.create(definition)
            updated = yunque.triggers.update({"id": "tr_1", **definition})
            deleted = yunque.triggers.delete("tr_1")
            emitted = yunque.triggers.emit("review.done", data={"task_id": "task_1"})
            runs = yunque.triggers.runs(trigger_id="tr_1", limit=2)
            events = yunque.triggers.events(limit=3)

        self.assertEqual(listed["total"], 1)
        self.assertEqual(got["id"], "tr_1")
        self.assertEqual(created["id"], "tr_1")
        self.assertEqual(updated["status"], "enabled")
        self.assertEqual(deleted["deleted"], "tr_1")
        self.assertEqual(emitted["event"], "review.done")
        self.assertEqual(runs["total"], 1)
        self.assertEqual(events["total"], 1)
        self.assertEqual(calls[0], ("GET", "/v1/triggers/v2?tenant_id=default&type=event&status=enabled", None))

    def test_agent_kit_exposes_triggers_namespace(self) -> None:
        kit = yunque.create_agent_kit()
        self.assertIs(kit.triggers, yunque.triggers)


if __name__ == "__main__":
    unittest.main()
