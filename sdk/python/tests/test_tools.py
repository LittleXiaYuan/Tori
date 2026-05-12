from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_tools_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_tools_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class ToolsTest(unittest.TestCase):
    def test_tools_helpers_delegate_to_api(self) -> None:
        with patch.object(yunque, "_api_call", return_value={"ok": True}) as api_call:
            yunque.tools.exec("echo ok", cwd="work", background=True, timeout_ms=1000, yield_ms=50, env=["A=B"])
            yunque.tools.list()
            yunque.tools.poll("session/1")
            yunque.tools.kill("session/1")

        self.assertEqual(api_call.call_args_list[0].args, ("POST", "/v1/tools/exec", {"Command": "echo ok", "Cwd": "work", "Background": True, "TimeoutMs": 1000, "YieldMs": 50, "Env": ["A=B"]}))
        self.assertEqual(api_call.call_args_list[1].args, ("GET", "/v1/tools/list"))
        self.assertEqual(api_call.call_args_list[2].args, ("GET", "/v1/tools/poll?id=session%2F1"))
        self.assertEqual(api_call.call_args_list[3].args, ("POST", "/v1/tools/kill?id=session%2F1"))


if __name__ == "__main__":
    unittest.main()
