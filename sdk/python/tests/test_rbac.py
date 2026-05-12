from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_rbac_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_rbac_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class RBACTest(unittest.TestCase):
    def test_rbac_helpers_delegate_to_api(self) -> None:
        with patch.object(yunque, "_api_call", return_value={"ok": True}) as api_call:
            yunque.rbac.roles()
            yunque.rbac.create_role({"id": "operator", "name": "Operator", "permissions": []})
            yunque.rbac.delete_role("operator")
            yunque.rbac.assign_role("u1", "operator", "t1")
            yunque.rbac.revoke_role("u1", "operator")
            yunque.rbac.check("tasks", "write", subject_id="u1")
            yunque.rbac.my_roles()

        self.assertEqual(api_call.call_args_list[0].args, ("GET", "/v1/rbac/roles"))
        self.assertEqual(api_call.call_args_list[1].args, ("POST", "/v1/rbac/roles", {"id": "operator", "name": "Operator", "permissions": []}))
        self.assertEqual(api_call.call_args_list[2].args, ("DELETE", "/v1/rbac/roles?id=operator"))
        self.assertEqual(api_call.call_args_list[3].args, ("POST", "/v1/rbac/assign", {"subject_id": "u1", "role_id": "operator", "tenant_id": "t1"}))
        self.assertEqual(api_call.call_args_list[4].args, ("POST", "/v1/rbac/revoke", {"subject_id": "u1", "role_id": "operator"}))
        self.assertEqual(api_call.call_args_list[5].args, ("POST", "/v1/rbac/check", {"resource": "tasks", "action": "write", "subject_id": "u1"}))
        self.assertEqual(api_call.call_args_list[6].args, ("GET", "/v1/rbac/my-roles"))


if __name__ == "__main__":
    unittest.main()
