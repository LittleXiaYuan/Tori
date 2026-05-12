from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_workflows_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_workflows_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class WorkflowsNamespaceTest(unittest.TestCase):
    def test_workflow_helpers(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            if path == "/v1/workflows":
                if method == "POST":
                    return {"id": "wf_1", "name": body["name"]}
                return {"workflows": [{"id": "wf_1", "name": "SDK flow"}], "total": 1}
            if path == "/v1/workflows?id=wf_1":
                return {"id": "wf_1", "name": "SDK flow"} if method == "GET" else {"deleted": "wf_1"}
            if path == "/v1/workflows/run":
                return {"status": "accepted", "instance_id": "inst_1", "instance": {"id": "inst_1", "definition_id": body["definition_id"], "status": "pending"}}
            if path == "/v1/workflows/instances":
                return {"instances": [{"id": "inst_1", "definition_id": "wf_1", "status": "running"}], "total": 1}
            if path == "/v1/workflows/instances?id=inst_1":
                return {"id": "inst_1", "definition_id": "wf_1", "status": "running"}
            if path == "/v1/workflows/cancel":
                return {"status": "cancelling", "instance_id": body["instance_id"]}
            raise AssertionError(f"unexpected call: {method} {path}")

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertEqual(yunque.workflows.list()["total"], 1)
            self.assertEqual(yunque.workflows.get("wf_1")["name"], "SDK flow")
            self.assertEqual(yunque.workflows.save({"name": "SDK flow"})["id"], "wf_1")
            self.assertEqual(yunque.workflows.run("wf_1", {"topic": "sdk"})["instance_id"], "inst_1")
            self.assertEqual(yunque.workflows.instances()["total"], 1)
            self.assertEqual(yunque.workflows.get_instance("inst_1")["status"], "running")
            self.assertEqual(yunque.workflows.cancel("inst_1")["status"], "cancelling")
            self.assertEqual(yunque.workflows.delete("wf_1")["deleted"], "wf_1")

        self.assertEqual(calls[3], ("POST", "/v1/workflows/run", {"definition_id": "wf_1", "variables": {"topic": "sdk"}}))


if __name__ == "__main__":
    unittest.main()
