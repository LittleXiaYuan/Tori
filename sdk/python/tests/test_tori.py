from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_tori_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_tori_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class ToriTest(unittest.TestCase):
    def test_tori_namespace_delegates_bind_status_unbind_health_and_usage(self) -> None:
        with patch.object(yunque, "_api_call", side_effect=[
            {"status": "pending"},
            {"bound": True},
            {"status": "unbound"},
            {"status": "ok"},
            {"total_tokens": 12},
        ]) as api_call:
            self.assertEqual(yunque.tori.bind("https://tori.example")["status"], "pending")
            self.assertTrue(yunque.tori.status()["bound"])
            self.assertEqual(yunque.tori.unbind()["status"], "unbound")
            self.assertEqual(yunque.tori.health()["status"], "ok")
            self.assertEqual(yunque.tori.usage()["total_tokens"], 12)

        self.assertEqual(api_call.call_args_list[0].args, ("POST", "/v1/tori/bind", {"tori_url": "https://tori.example"}))
        self.assertEqual(api_call.call_args_list[1].args, ("GET", "/v1/tori/status"))
        self.assertEqual(api_call.call_args_list[2].args, ("POST", "/v1/tori/unbind", {}))
        self.assertEqual(api_call.call_args_list[3].args, ("GET", "/v1/tori/health"))
        self.assertEqual(api_call.call_args_list[4].args, ("GET", "/v1/tori/usage"))

    def test_agent_kit_exposes_tori(self) -> None:
        self.assertIs(yunque.create_agent_kit().tori, yunque.tori)


if __name__ == "__main__":
    unittest.main()
