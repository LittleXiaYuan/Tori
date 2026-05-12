from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_connectors_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_connectors_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class ConnectorsNamespaceTest(unittest.TestCase):
    def test_connector_helpers(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            if path == "/api/connectors":
                return {"connectors": [{"id": "github", "name": "GitHub", "supported": True, "status": "disconnected"}]}
            if path == "/api/connectors/detail?id=github":
                return {"connector": {"id": "github", "actions": [{"id": "create_issue"}]}, "supported": True, "status": "disconnected"}
            if path == "/api/connectors/connect":
                return {"ok": True, "status": "connected", "user_info": "octocat"}
            if path == "/api/connectors/disconnect":
                return {"ok": True}
            if path == "/api/connectors/execute":
                return {"ok": True, "result": {"issue": 1}}
            raise AssertionError(f"unexpected call: {method} {path}")

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertEqual(yunque.connectors.list()["connectors"][0]["id"], "github")
            self.assertEqual(yunque.connectors.detail("github")["connector"]["actions"][0]["id"], "create_issue")
            self.assertEqual(yunque.connectors.connect("github", token="oauth")["status"], "connected")
            self.assertTrue(yunque.connectors.disconnect("github")["ok"])
            self.assertEqual(yunque.connectors.execute("github", "create_issue", {"title": "SDK"})["result"]["issue"], 1)

        self.assertEqual(calls[2], ("POST", "/api/connectors/connect", {"connector_id": "github", "token": "oauth", "api_key": ""}))
        self.assertEqual(calls[4], ("POST", "/api/connectors/execute", {"connector_id": "github", "action_id": "create_issue", "params": {"title": "SDK"}}))


if __name__ == "__main__":
    unittest.main()
