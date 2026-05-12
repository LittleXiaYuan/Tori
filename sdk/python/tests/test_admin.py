from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_admin_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_admin_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class AdminTest(unittest.TestCase):
    def test_admin_namespace_delegates_operator_helpers(self) -> None:
        with patch.object(yunque, "_api_call", side_effect=[
            {"console_hidden": False},
            {"console_hidden": True},
            {"autostart_enabled": False},
            {"autostart_enabled": True},
            {"tenants": [{"id": "t1"}], "count": 1},
            {"id": "t2", "name": "team"},
            {"status": "ok", "executed": False},
            {"status": "ok", "executed": True},
        ]) as api_call:
            self.assertFalse(yunque.admin.console_status()["console_hidden"])
            self.assertTrue(yunque.admin.toggle_console()["console_hidden"])
            self.assertFalse(yunque.admin.autostart_status()["autostart_enabled"])
            self.assertTrue(yunque.admin.toggle_autostart()["autostart_enabled"])
            self.assertEqual(yunque.admin.list_tenants()["count"], 1)
            self.assertEqual(yunque.admin.create_tenant("team")["name"], "team")
            self.assertFalse(yunque.admin.nl_config_translate("切换到 qwen")["executed"])
            self.assertTrue(yunque.admin.nl_config("切换到 qwen", execute=True)["executed"])

        self.assertEqual(api_call.call_args_list[0].args, ("GET", "/v1/desktop/console"))
        self.assertEqual(api_call.call_args_list[1].args, ("POST", "/v1/desktop/console", {}))
        self.assertEqual(api_call.call_args_list[2].args, ("GET", "/v1/desktop/autostart"))
        self.assertEqual(api_call.call_args_list[3].args, ("POST", "/v1/desktop/autostart", {}))
        self.assertEqual(api_call.call_args_list[4].args, ("GET", "/v1/tenants"))
        self.assertEqual(api_call.call_args_list[5].args, ("POST", "/v1/tenants", {"name": "team"}))
        self.assertEqual(api_call.call_args_list[6].args, ("POST", "/v1/nl-config/translate", {"text": "切换到 qwen", "execute": False}))
        self.assertEqual(api_call.call_args_list[7].args, ("POST", "/v1/nl-config", {"text": "切换到 qwen", "execute": True}))

    def test_agent_kit_exposes_admin(self) -> None:
        self.assertIs(yunque.create_agent_kit().admin, yunque.admin)


if __name__ == "__main__":
    unittest.main()
