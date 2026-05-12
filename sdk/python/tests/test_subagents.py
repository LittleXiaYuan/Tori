from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_subagents_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_subagents_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class SubagentsTest(unittest.TestCase):
    def test_subagents_helpers_delegate_to_api(self) -> None:
        with patch.object(yunque, "_api_call", return_value={"ok": True}) as api_call:
            yunque.subagents.list("task/1")
            yunque.subagents.get("sa/1")
            yunque.subagents.spawn("planner", parent_id="task/1", description="计划拆解", skills=["plan"])
            yunque.subagents.append_messages("sa/1", [{"role": "user", "content": "继续"}])
            yunque.subagents.destroy("sa/1")

        self.assertEqual(api_call.call_args_list[0].args, ("GET", "/v1/subagent?parent_id=task%2F1"))
        self.assertEqual(api_call.call_args_list[1].args, ("GET", "/v1/subagent?id=sa%2F1"))
        self.assertEqual(api_call.call_args_list[2].args, ("POST", "/v1/subagent", {"parent_id": "task/1", "name": "planner", "description": "计划拆解", "skills": ["plan"]}))
        self.assertEqual(api_call.call_args_list[3].args, ("POST", "/v1/subagent/message", {"id": "sa/1", "messages": [{"role": "user", "content": "继续"}]}))
        self.assertEqual(api_call.call_args_list[4].args, ("DELETE", "/v1/subagent?id=sa%2F1"))


if __name__ == "__main__":
    unittest.main()
