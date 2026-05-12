from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_settings_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_settings_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class SettingsTest(unittest.TestCase):
    def test_settings_namespace_delegates_config_helpers(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            return {"path": path, "success": True, "values": {"LLM_MODEL": "qwen"}}

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertEqual(yunque.settings.schema()["path"], "/api/settings/schema")
            self.assertEqual(yunque.settings.config()["values"]["LLM_MODEL"], "qwen")
            self.assertTrue(yunque.settings.update_config({"LLM_MODEL": "deepseek"})["success"])
            self.assertEqual(yunque.settings.check()["path"], "/api/settings/check")
            self.assertTrue(yunque.settings.reload()["success"])
            self.assertEqual(yunque.settings.detect_dirs()["path"], "/api/settings/detect-dirs")

        self.assertEqual(calls[0], ("GET", "/api/settings/schema", None))
        self.assertEqual(calls[2], ("PUT", "/api/settings/config", {"values": {"LLM_MODEL": "deepseek"}}))
        self.assertEqual(calls[4], ("POST", "/v1/config/reload", {}))

    def test_agent_kit_exposes_settings(self) -> None:
        self.assertIs(yunque.create_agent_kit().settings, yunque.settings)


if __name__ == "__main__":
    unittest.main()
