import unittest
from unittest.mock import patch

import sdk.python.yunque as yunque


class ModesSDKTests(unittest.TestCase):
    def test_modes_delegate_to_persona_mode_routes(self):
        with patch.object(yunque, "_api_call", return_value={"ok": True}) as api_call:
            yunque.modes.list("tenant-1", "session-1")
            yunque.modes.current("tenant-1", "session-1")
            yunque.modes.set("coder", "tenant-1", "session-1")

        calls = [call.args for call in api_call.call_args_list]
        self.assertEqual(calls[0], ("GET", "/v1/persona/modes?tenant_id=tenant-1&session_id=session-1"))
        self.assertEqual(calls[1], ("GET", "/v1/persona/mode/current?tenant_id=tenant-1&session_id=session-1"))
        self.assertEqual(calls[2], ("POST", "/v1/persona/mode", {"tenant_id": "tenant-1", "mode": "coder", "session_id": "session-1"}))

    def test_agent_kit_exposes_modes_namespace(self):
        self.assertIs(yunque.create_agent_kit().modes, yunque.modes)


if __name__ == "__main__":
    unittest.main()
