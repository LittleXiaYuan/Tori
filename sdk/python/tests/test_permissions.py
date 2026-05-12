from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_permissions_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_permissions_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class PermissionsTest(unittest.TestCase):
    def test_permissions_namespace_delegates_check_and_my_roles(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            if path == "/v1/rbac/check":
                return {"allowed": True, "subject_id": "u1"}
            return {"subject_id": "u1", "roles": [{"id": "viewer"}], "total": 1}

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertTrue(yunque.permissions.check("knowledge", "read", subject_id="u1")["allowed"])
            self.assertEqual(yunque.permissions.my_roles()["roles"][0]["id"], "viewer")

        self.assertEqual(calls[0], ("POST", "/v1/rbac/check", {"resource": "knowledge", "action": "read", "subject_id": "u1"}))
        self.assertEqual(calls[1], ("GET", "/v1/rbac/my-roles", None))


if __name__ == "__main__":
    unittest.main()
