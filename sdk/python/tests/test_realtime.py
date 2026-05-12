from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_realtime_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_realtime_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class RealtimeTest(unittest.TestCase):
    def test_ws_url_uses_ws_scheme_and_api_key(self) -> None:
        with patch.object(yunque, "_API_BASE", "https://agent.example/"):
            url = yunque.realtime.ws_url(api_key="key-1", query={"tenant": "t1", "empty": ""})
        self.assertEqual(url, "wss://agent.example/v1/ws?tenant=t1&api_key=key-1")

    def test_ws_url_preserves_explicit_token_query(self) -> None:
        with patch.object(yunque, "_API_BASE", "http://localhost:9090"), patch.object(yunque, "_TOKEN", "runtime-token"):
            url = yunque.realtime.ws_url(query={"access_token": "manual"})
        self.assertEqual(url, "ws://localhost:9090/v1/ws?access_token=manual")

    def test_message_helpers_serialize_and_parse(self) -> None:
        ping = yunque.realtime.ping(trace_id="tr-1")
        chat = yunque.realtime.chat("你好", session="s1", locale="zh-CN")
        self.assertEqual(ping, {"type": "ping", "trace_id": "tr-1"})
        self.assertEqual(chat["type"], "chat")
        self.assertEqual(chat["content"], "你好")
        self.assertEqual(chat["session"], "s1")
        encoded = yunque.realtime.serialize(chat)
        self.assertEqual(yunque.realtime.parse(encoded), chat)
        with self.assertRaises(ValueError):
            yunque.realtime.parse("[]")


if __name__ == "__main__":
    unittest.main()
