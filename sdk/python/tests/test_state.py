from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class StateNamespaceTest(unittest.TestCase):
    def test_snapshot_actions_and_capabilities(self) -> None:
        snapshot = {
            "goals": [{"id": "g1", "title": "Expose Python state slice"}],
            "resources": [],
            "focus": "Python SDK state",
            "recent_actions": [{"action": "slice_added", "success": True}],
            "capabilities": {"total_skills": 3, "unresolved_gaps": 1},
        }

        with patch.object(yunque, "_api_call", return_value=snapshot) as api_call:
            self.assertEqual(yunque.state.snapshot()["focus"], "Python SDK state")
            self.assertEqual(yunque.state.actions()[0]["action"], "slice_added")
            self.assertEqual(yunque.state.capabilities()["total_skills"], 3)

        api_call.assert_called_with("GET", "/v1/state")

    def test_focused_helpers(self) -> None:
        calls: list[tuple] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            if path == "/v1/state/goals" and method == "GET":
                return [{"id": "g1", "title": "Keep SDK incremental"}]
            if path == "/v1/state/goals" and method == "POST":
                return {"id": "g2", "status": "created"}
            if path == "/v1/state/goals?id=g1" and method == "DELETE":
                return {"id": "g1", "status": "deleted"}
            if path == "/v1/state/focus" and method == "GET":
                return {"focus": "SDK boundary"}
            if path == "/v1/state/focus" and method == "POST":
                return {"status": "updated"}
            if path == "/v1/state/resources" and method == "GET":
                return [{"id": "r1", "path": "sdk/python/yunque/__init__.py"}]
            if path == "/v1/state/resources" and method == "POST":
                return {"status": "tracked"}
            if path == "/v1/state/resources?id=r1" and method == "DELETE":
                return {"status": "released"}
            raise AssertionError(f"unexpected call: {method} {path}")

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertEqual(yunque.state.goals()[0]["title"], "Keep SDK incremental")
            self.assertEqual(yunque.state.save_goal({"title": "New goal"})["id"], "g2")
            self.assertEqual(yunque.state.delete_goal("g1")["status"], "deleted")
            self.assertEqual(yunque.state.focus(), "SDK boundary")
            self.assertEqual(yunque.state.update_focus("Next SDK", ["state"])["status"], "updated")
            self.assertEqual(yunque.state.resources()[0]["path"], "sdk/python/yunque/__init__.py")
            self.assertEqual(yunque.state.track_resource({"path": "sdk/python"})["status"], "tracked")
            self.assertEqual(yunque.state.release_resource("r1")["status"], "released")

        self.assertEqual(calls, [
            ("GET", "/v1/state/goals", None),
            ("POST", "/v1/state/goals", {"title": "New goal"}),
            ("DELETE", "/v1/state/goals?id=g1", None),
            ("GET", "/v1/state/focus", None),
            ("POST", "/v1/state/focus", {"focus": "Next SDK", "topics": ["state"]}),
            ("GET", "/v1/state/resources", None),
            ("POST", "/v1/state/resources", {"path": "sdk/python"}),
            ("DELETE", "/v1/state/resources?id=r1", None),
        ])

    def test_missing_snapshot_sections_fallback_to_empty(self) -> None:
        with patch.object(yunque, "_api_call", return_value={}):
            self.assertEqual(yunque.state.actions(), [])
            self.assertEqual(yunque.state.capabilities(), {})


if __name__ == "__main__":
    unittest.main()
