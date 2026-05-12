from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_audit_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_audit_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class AuditTest(unittest.TestCase):
    def test_audit_helpers_delegate_to_api(self) -> None:
        with patch.object(yunque, "_api_call", return_value={"ok": True}) as api_call:
            yunque.audit.tail(n=10, type="system event", actor="tenant")
            yunque.audit.verify()
            yunque.audit.stats()
            yunque.audit.trail(date="2026-05-11", type="nl_config")

        self.assertEqual(api_call.call_args_list[0].args, ("GET", "/v1/audit/tail?n=10&type=system+event&actor=tenant"))
        self.assertEqual(api_call.call_args_list[1].args, ("GET", "/v1/audit/verify"))
        self.assertEqual(api_call.call_args_list[2].args, ("GET", "/v1/audit/stats"))
        self.assertEqual(api_call.call_args_list[3].args, ("GET", "/api/audit/trail?date=2026-05-11&type=nl_config"))


if __name__ == "__main__":
    unittest.main()
