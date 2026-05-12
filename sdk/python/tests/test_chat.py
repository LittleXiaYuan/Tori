from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_chat_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_chat_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class ChatTest(unittest.TestCase):
    def test_send_and_agentic_call_chat_routes(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            return {"reply": path}

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertEqual(yunque.chat_sdk.send([{"role": "user", "content": "hi"}])["reply"], "/v1/chat")
            self.assertEqual(yunque.chat_sdk.agentic([{"role": "user", "content": "plan"}])["reply"], "/v1/chat/agentic")

        self.assertEqual(calls[0], ("POST", "/v1/chat", {"messages": [{"role": "user", "content": "hi"}]}))
        self.assertEqual(calls[1][0:2], ("POST", "/v1/chat/agentic"))

    def test_stream_helpers(self) -> None:
        with patch.object(yunque, "_API_BASE", "http://localhost:9090"):
            self.assertEqual(yunque.chat_sdk.stream_url(), "http://localhost:9090/v1/chat/stream")
        request = yunque.chat_sdk.stream_request([{"role": "user", "content": "hi"}], session_id="s1")
        self.assertTrue(request["stream"])
        self.assertEqual(request["session_id"], "s1")
        items = yunque.chat_sdk.parse_stream('event: message\ndata: {"type":"delta","content":"你"}\n\nevent: error\ndata: {"error":"bad"}\n\n')
        self.assertEqual(items[0]["kind"], "delta")
        self.assertEqual(items[0]["content"], "你")
        self.assertEqual(items[1]["kind"], "error")


if __name__ == "__main__":
    unittest.main()
