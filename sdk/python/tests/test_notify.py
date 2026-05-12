from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_notify_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_notify_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class NotifyNamespaceTest(unittest.TestCase):
    def test_notify_helpers(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            if path == "/api/notify/channels":
                return {"channels": [{"id": "feishu-main", "type": "feishu", "name": "Feishu", "enabled": True}]}
            if path == "/api/notify/add":
                return {"ok": True}
            if path == "/api/notify/remove?id=feishu-main":
                return {"ok": True}
            if path == "/api/notify/toggle":
                return {"ok": True}
            if path == "/api/notify/test":
                return {"ok": True}
            if path == "/api/notify/share":
                return {"ok": True, "sent_at": "2026-05-12T00:00:00Z", "share": {"code": "yq_abc"}}
            raise AssertionError(f"unexpected call: {method} {path}")

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertEqual(yunque.notify.channels()["channels"][0]["id"], "feishu-main")
            self.assertTrue(yunque.notify.add_channel({"id": "feishu-main", "type": "feishu", "name": "Feishu"})["ok"])
            self.assertTrue(yunque.notify.remove_channel("feishu-main")["ok"])
            self.assertTrue(yunque.notify.toggle_channel("feishu-main", False)["ok"])
            self.assertTrue(yunque.notify.test_channel("feishu-main")["ok"])
            self.assertEqual(yunque.notify.share("feishu-main", message="done")["share"]["code"], "yq_abc")

        self.assertEqual(calls[3], ("POST", "/api/notify/toggle", {"id": "feishu-main", "enabled": False}))
        self.assertEqual(calls[5], ("POST", "/api/notify/share", {"channel_id": "feishu-main", "title": "", "message": "done", "session_id": "", "task_id": "", "url": "", "files": []}))


if __name__ == "__main__":
    unittest.main()
