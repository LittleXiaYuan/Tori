import unittest
from unittest.mock import patch

import sdk.python.yunque as yunque


class IdentitySDKTests(unittest.TestCase):
    def test_resolve_and_profiles_delegate_to_identity_routes(self):
        with patch.object(yunque, "_api_call", return_value={"ok": True}) as api_call:
            yunque.identity.resolve("wechat", "u1", "小云")
            yunque.identity.profiles()

        calls = [call.args for call in api_call.call_args_list]
        self.assertEqual(calls[0], ("POST", "/v1/identity/resolve", {"channel": "wechat", "user_id": "u1", "display_name": "小云"}))
        self.assertEqual(calls[1], ("GET", "/v1/identity/profiles"))

    def test_agent_kit_exposes_identity_namespace(self):
        self.assertIs(yunque.create_agent_kit().identity, yunque.identity)


if __name__ == "__main__":
    unittest.main()
