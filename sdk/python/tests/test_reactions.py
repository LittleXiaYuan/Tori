from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_reactions_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_reactions_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class ReactionsTest(unittest.TestCase):
    def test_reactions_namespace_routes_react_and_send_sticker(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            return {"status": "ok" if path == "/v1/react" else "sent"}

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertEqual(yunque.reactions.react("wechat", "u1", "m1", emoji="👍")["status"], "ok")
            self.assertEqual(yunque.reactions.send_sticker("wechat", "u1", emoji="🌟")["status"], "sent")

        self.assertEqual(calls[0], ("POST", "/v1/react", {"channel_type": "wechat", "target": "u1", "message_id": "m1", "emoji": "👍"}))
        self.assertEqual(calls[1], ("POST", "/v1/sticker/send", {"channel_type": "wechat", "target": "u1", "emoji": "🌟"}))


if __name__ == "__main__":
    unittest.main()
