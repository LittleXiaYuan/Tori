from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_persona_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_persona_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class PersonaTest(unittest.TestCase):
    def test_persona_helpers_delegate_to_api(self) -> None:
        with patch.object(yunque, "_api_call", return_value={"ok": True}) as api_call:
            yunque.persona.get()
            yunque.persona.update(identity="Tori", soul="careful")
            yunque.persona.skills()
            yunque.persona.add_skill("review", "Review", "review code")
            yunque.persona.delete_skill("review")
            yunque.persona.presets()
            yunque.persona.modes(tenant_id="tenant-1", session_id="session-1")
            yunque.persona.current_mode(tenant_id="tenant-1")
            yunque.persona.switch_preset("studio")
            yunque.persona.add_custom_preset({"id": "studio", "name": "Studio"})
            yunque.persona.delete_custom_preset("studio")
            yunque.persona.update_preset_features("studio", {"emotion": True})

        self.assertEqual(api_call.call_args_list[0].args, ("GET", "/v1/persona"))
        self.assertEqual(api_call.call_args_list[1].args, ("PUT", "/v1/persona", {"identity": "Tori", "soul": "careful"}))
        self.assertEqual(api_call.call_args_list[2].args, ("GET", "/v1/persona/skills"))
        self.assertEqual(api_call.call_args_list[3].args, ("POST", "/v1/persona/skills", {"name": "review", "description": "Review", "content": "review code"}))
        self.assertEqual(api_call.call_args_list[4].args, ("DELETE", "/v1/persona/skills", {"name": "review"}))
        self.assertEqual(api_call.call_args_list[5].args, ("GET", "/v1/persona/presets"))
        self.assertEqual(api_call.call_args_list[6].args, ("GET", "/v1/persona/modes?tenant_id=tenant-1&session_id=session-1"))
        self.assertEqual(api_call.call_args_list[7].args, ("GET", "/v1/persona/mode/current?tenant_id=tenant-1"))
        self.assertEqual(api_call.call_args_list[8].args, ("POST", "/v1/persona/presets", {"id": "studio"}))
        self.assertEqual(api_call.call_args_list[9].args, ("POST", "/v1/persona/presets/custom", {"id": "studio", "name": "Studio"}))
        self.assertEqual(api_call.call_args_list[10].args, ("DELETE", "/v1/persona/presets/custom", {"id": "studio"}))
        self.assertEqual(api_call.call_args_list[11].args, ("PUT", "/v1/persona/presets/features", {"id": "studio", "features": {"emotion": True}}))


if __name__ == "__main__":
    unittest.main()
