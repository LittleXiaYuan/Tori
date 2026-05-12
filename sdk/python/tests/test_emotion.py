from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_emotion_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_emotion_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class EmotionTest(unittest.TestCase):
    def test_emotion_namespace_routes_history_and_stickers(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            if path.startswith("/v1/emotion/history"):
                return {"entries": [{"emotion": "happy"}], "total": 1}
            if method == "GET" and path == "/v1/emotion/stickers":
                return {"happy": {"wechat": [{"package_id": "p1", "sticker_id": "s1"}]}}
            return {"status": "ok"}

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertEqual(yunque.emotion.history(session_id="s1", limit=5)["total"], 1)
            self.assertEqual(yunque.emotion.stickers()["happy"]["wechat"][0]["sticker_id"], "s1")
            self.assertEqual(yunque.emotion.register_stickers("wechat", "happy", [{"package_id": "p1", "sticker_id": "s1"}])["status"], "ok")
            self.assertEqual(yunque.emotion.clear_stickers("wechat", "happy")["status"], "ok")

        self.assertEqual(calls[0], ("GET", "/v1/emotion/history?session_id=s1&limit=5", None))
        self.assertEqual(calls[1], ("GET", "/v1/emotion/stickers", None))
        self.assertEqual(calls[2], ("PUT", "/v1/emotion/stickers", {"platform": "wechat", "emotion": "happy", "stickers": [{"package_id": "p1", "sticker_id": "s1"}]}))
        self.assertEqual(calls[3], ("DELETE", "/v1/emotion/stickers", {"platform": "wechat", "emotion": "happy"}))


if __name__ == "__main__":
    unittest.main()
