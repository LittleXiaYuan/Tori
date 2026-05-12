from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_conversations_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_conversations_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class ConversationsTest(unittest.TestCase):
    def test_session_message_manage_and_replay_helpers(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            return {"ok": True, "path": path}

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            yunque.conversations.list(archived=True)
            yunque.conversations.messages("s1")
            yunque.conversations.delete_messages("s1")
            yunque.conversations.rename("s1", "新的会话")
            yunque.conversations.pin("s1")
            yunque.conversations.archive("s1", False)
            yunque.conversations.replay("s1", raw=True, limit=10, offset=2)

        self.assertEqual(calls[0], ("GET", "/v1/conversations?archived=true", None))
        self.assertEqual(calls[1], ("GET", "/v1/conversations/messages?session_id=s1", None))
        self.assertEqual(calls[2], ("DELETE", "/v1/conversations/messages?session_id=s1", None))
        self.assertEqual(calls[3], ("PUT", "/v1/conversations/manage", {"session_id": "s1", "name": "新的会话"}))
        self.assertEqual(calls[4], ("PUT", "/v1/conversations/manage", {"session_id": "s1", "pinned": True}))
        self.assertEqual(calls[5], ("PUT", "/v1/conversations/manage", {"session_id": "s1", "archive": False}))
        self.assertEqual(calls[6], ("GET", "/v1/conversations/replay?session_id=s1&raw=true&limit=10&offset=2", None))


if __name__ == "__main__":
    unittest.main()
