from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_fork_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_fork_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class ForkNamespaceTest(unittest.TestCase):
    def test_fork_helpers(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            fork = {"id": "fork_1", "session_id": "s1", "messages": [], "created_at": "2026-05-12T00:00:00Z"}
            if path == "/v1/fork?session_id=s1":
                return fork
            if path == "/v1/fork?id=fork_1" and method == "GET":
                return fork
            if path == "/v1/fork" and method == "POST":
                return {**fork, "messages": body.get("messages", [])}
            if path == "/v1/fork?id=fork_1" and method == "DELETE":
                return {"deleted": True}
            if path == "/v1/fork/branch":
                return {**fork, "id": "fork_2", "parent_id": body["fork_id"], "label": body.get("label", "")}
            if path == "/v1/fork/list?session_id=s1":
                return {"forks": [fork]}
            raise AssertionError(f"unexpected call: {method} {path}")

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertEqual(yunque.fork.root("s1")["id"], "fork_1")
            self.assertEqual(yunque.fork.get("fork_1")["session_id"], "s1")
            self.assertEqual(len(yunque.fork.create("s1", messages=[{"role": "user", "content": "hi"}])["messages"]), 1)
            self.assertTrue(yunque.fork.remove("fork_1")["deleted"])
            self.assertEqual(yunque.fork.branch("fork_1", at_index=0, label="alt")["parent_id"], "fork_1")
            self.assertEqual(yunque.fork.list("s1")["forks"][0]["id"], "fork_1")

        self.assertEqual(calls[2], ("POST", "/v1/fork", {"session_id": "s1", "messages": [{"role": "user", "content": "hi"}]}))
        self.assertIs(yunque.create_agent_kit().fork, yunque.fork)


if __name__ == "__main__":
    unittest.main()
