from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_setup_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_setup_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class SetupTest(unittest.TestCase):
    def test_setup_namespace_delegates_detection_templates_apply_and_install(self) -> None:
        with patch.object(yunque, "_api_call", side_effect=[
            {"has_docker": True},
            {"providers": [{"id": "ollama"}]},
            {"templates": [{"id": "local"}], "count": 1},
            {"ok": True},
            {"status": "applied", "restart_required": True},
            {"success": True},
        ]) as api_call:
            self.assertTrue(yunque.setup.detect()["has_docker"])
            self.assertEqual(yunque.setup.health()["providers"][0]["id"], "ollama")
            self.assertEqual(yunque.setup.templates()["count"], 1)
            self.assertTrue(yunque.setup.test_provider("http://127.0.0.1:11434", model="qwen")["ok"])
            self.assertTrue(yunque.setup.apply("local", base_url="http://127.0.0.1:11434", model="qwen", overrides={"sandbox_tier": "local"})["restart_required"])
            self.assertTrue(yunque.setup.install_component("python_office")["success"])

        self.assertEqual(api_call.call_args_list[0].args, ("GET", "/v1/setup/detect"))
        self.assertEqual(api_call.call_args_list[1].args, ("GET", "/v1/setup/health"))
        self.assertEqual(api_call.call_args_list[2].args, ("GET", "/v1/setup/templates"))
        self.assertEqual(api_call.call_args_list[3].args, ("POST", "/v1/setup/test-provider", {"base_url": "http://127.0.0.1:11434", "model": "qwen"}))
        self.assertEqual(api_call.call_args_list[4].args, ("POST", "/v1/setup/apply", {"template_id": "local", "base_url": "http://127.0.0.1:11434", "model": "qwen", "overrides": {"sandbox_tier": "local"}}))
        self.assertEqual(api_call.call_args_list[5].args, ("POST", "/v1/setup/install-component", {"component_id": "python_office"}))

    def test_agent_kit_exposes_setup(self) -> None:
        self.assertIs(yunque.create_agent_kit().setup, yunque.setup)


if __name__ == "__main__":
    unittest.main()
