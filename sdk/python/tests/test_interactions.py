import unittest
from unittest.mock import patch

import sdk.python.yunque as yunque


class InteractionsSDKTests(unittest.TestCase):
    def test_interactions_delegate_to_runtime_namespaces(self):
        with patch.object(yunque, "_api_call", return_value={"ok": True}) as api_call:
            yunque.interactions.emotion_history("s1", 2)
            yunque.interactions.stickers()
            yunque.interactions.instructions("style")
            yunque.interactions.react("telegram", "chat1", "m1", "👍")
            yunque.interactions.send_sticker("telegram", "chat1", "pkg", "stk")

        calls = [call.args for call in api_call.call_args_list]
        self.assertEqual(calls[0], ("GET", "/v1/emotion/history?session_id=s1&limit=2"))
        self.assertEqual(calls[1], ("GET", "/v1/emotion/stickers"))
        self.assertEqual(calls[2], ("GET", "/v1/instructions?category=style"))
        self.assertEqual(calls[3], ("POST", "/v1/react", {"channel_type": "telegram", "target": "chat1", "message_id": "m1", "emoji": "👍"}))
        self.assertEqual(calls[4], ("POST", "/v1/sticker/send", {"channel_type": "telegram", "target": "chat1", "package_id": "pkg", "sticker_id": "stk"}))

    def test_agent_kit_exposes_interactions_namespace(self):
        self.assertIs(yunque.create_agent_kit().interactions, yunque.interactions)


if __name__ == "__main__":
    unittest.main()
