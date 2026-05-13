
from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_sandbox_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_sandbox_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class SandboxTest(unittest.TestCase):
    def test_sandbox_helpers_delegate_to_api(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            return {"ok": True}

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            yunque.sandbox.exec("python", ["-V"])
            yunque.sandbox.probe()
            yunque.sandbox.create_desktop()
            yunque.sandbox.desktop_status()
            yunque.sandbox.destroy_desktop("DELETE")

        self.assertEqual(calls[0], ("POST", "/v1/sandbox/exec", {"command": "python", "args": ["-V"]}))
        self.assertEqual(calls[1], ("GET", "/v1/sandbox/probe", None))
        self.assertEqual(calls[2], ("POST", "/v1/sandbox/desktop", {}))
        self.assertEqual(calls[3], ("GET", "/v1/sandbox/desktop/status", None))
        self.assertEqual(calls[4], ("DELETE", "/v1/sandbox/desktop/destroy", {}))
        with self.assertRaises(ValueError):
            yunque.sandbox.destroy_desktop("PATCH")

    def test_agent_kit_exposes_sandbox_namespace(self) -> None:
        self.assertIs(yunque.create_agent_kit().sandbox, yunque.sandbox)


if __name__ == "__main__":
    unittest.main()
