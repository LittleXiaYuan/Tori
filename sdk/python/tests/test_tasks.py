from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"
spec = importlib.util.spec_from_file_location("yunque_tasks_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_tasks_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class TasksTest(unittest.TestCase):
    def test_tasks_namespace_delegates_crud_and_lifecycle(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            if path == "/v1/tasks":
                if method == "GET":
                    return [{"id": "task-1"}]
                return {"id": "task-2", "description": body["description"]}
            if path == "/v1/tasks?id=task-1":
                return {"id": "task-1", "status": "running"} if method == "GET" else {"deleted": "task-1"}
            if path == "/v1/tasks/run":
                return {"status": "accepted", "task_id": body["id"]}
            raise AssertionError(f"unexpected call: {method} {path}")

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertEqual(yunque.tasks.list()[0]["id"], "task-1")
            self.assertEqual(yunque.tasks.get("task-1")["status"], "running")
            self.assertEqual(yunque.tasks.create("ship SDK", title="SDK", constraints={"max_steps": 3})["id"], "task-2")
            self.assertEqual(yunque.tasks.run("task-1")["status"], "accepted")
            self.assertEqual(yunque.tasks.delete("task-1")["deleted"], "task-1")

        self.assertEqual(calls[0], ("GET", "/v1/tasks", None))
        self.assertEqual(calls[2], ("POST", "/v1/tasks", {"description": "ship SDK", "title": "SDK", "constraints": {"max_steps": 3}}))
        self.assertEqual(calls[3], ("POST", "/v1/tasks/run", {"id": "task-1"}))


if __name__ == "__main__":
    unittest.main()