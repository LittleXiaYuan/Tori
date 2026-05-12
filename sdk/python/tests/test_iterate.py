from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_iterate_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_iterate_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class IterateTest(unittest.TestCase):
    def test_iterate_helpers_delegate_to_api(self) -> None:
        with patch.object(yunque, "_api_call", return_value={"ok": True}) as api_call:
            yunque.iterate.proposals()
            yunque.iterate.pending_proposals()
            yunque.iterate.approve("p1")
            yunque.iterate.reject("p2")
            yunque.iterate.trigger()
            yunque.iterate.status()

        self.assertEqual(api_call.call_args_list[0].args, ("GET", "/api/iterate/proposals"))
        self.assertEqual(api_call.call_args_list[1].args, ("GET", "/api/iterate/proposals?status=pending"))
        self.assertEqual(api_call.call_args_list[2].args, ("POST", "/api/iterate/approve", {"id": "p1"}))
        self.assertEqual(api_call.call_args_list[3].args, ("POST", "/api/iterate/reject", {"id": "p2"}))
        self.assertEqual(api_call.call_args_list[4].args, ("POST", "/api/iterate/trigger", {}))
        self.assertEqual(api_call.call_args_list[5].args, ("GET", "/api/iterate/status"))


if __name__ == "__main__":
    unittest.main()
