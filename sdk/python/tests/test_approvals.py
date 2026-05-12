from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_approvals_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_approvals_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class ApprovalsTest(unittest.TestCase):
    def test_approval_helpers_delegate_to_api(self) -> None:
        with patch.object(yunque, "_api_call", return_value={"ok": True}) as api_call:
            yunque.approvals.list(status="approved")
            yunque.approvals.pending()
            yunque.approvals.history("denied")
            yunque.approvals.approve("ap1")
            yunque.approvals.deny("ap2", "too risky")
            yunque.approvals.decide("ap3", "allow_once")
            yunque.approvals.rules()
            yunque.approvals.add_rule({"id": "r1", "decision": "allow_always"})
            yunque.approvals.delete_rule("r1")

        self.assertEqual(api_call.call_args_list[0].args, ("GET", "/v1/approvals?status=approved"))
        self.assertEqual(api_call.call_args_list[1].args, ("GET", "/v1/approvals?status=pending"))
        self.assertEqual(api_call.call_args_list[2].args, ("GET", "/v1/approvals?status=denied&history=true"))
        self.assertEqual(api_call.call_args_list[3].args, ("POST", "/v1/approvals/approve", {"id": "ap1"}))
        self.assertEqual(api_call.call_args_list[4].args, ("POST", "/v1/approvals/deny", {"id": "ap2", "reason": "too risky"}))
        self.assertEqual(api_call.call_args_list[5].args, ("POST", "/v1/approvals/decide", {"id": "ap3", "decision": "allow_once"}))
        self.assertEqual(api_call.call_args_list[6].args, ("GET", "/v1/approvals/rules"))
        self.assertEqual(api_call.call_args_list[7].args, ("POST", "/v1/approvals/rules", {"id": "r1", "decision": "allow_always"}))
        self.assertEqual(api_call.call_args_list[8].args, ("DELETE", "/v1/approvals/rules?id=r1"))


if __name__ == "__main__":
    unittest.main()
