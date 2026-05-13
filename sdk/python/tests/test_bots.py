
from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_bots_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_bots_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class BotsTest(unittest.TestCase):
    def test_bots_helpers_delegate_to_api(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            return {"ok": True}

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            yunque.bots.list()
            yunque.bots.create("planner", "计划", {"model": "deepseek"})
            yunque.bots.get("bot/1")
            yunque.bots.update("bot/1", active=False)
            yunque.bots.set_active("bot/1", True)
            yunque.bots.delete("bot/1")
            yunque.bots.inbox(True)
            yunque.bots.push_inbox("ping", source="webhook", action="trigger")
            yunque.bots.delete_inbox("in-1")
            yunque.bots.mark_inbox_read(["in-1", "in-2"])
            yunque.bots.mark_all_inbox_read()
            yunque.bots.channel_groups("telegram")

        self.assertEqual(calls[0], ("GET", "/v1/bots", None))
        self.assertEqual(calls[1], ("POST", "/v1/bots", {"name": "planner", "description": "计划", "config": {"model": "deepseek"}}))
        self.assertEqual(calls[2], ("GET", "/v1/bots/detail?id=bot%2F1", None))
        self.assertEqual(calls[3], ("PUT", "/v1/bots/detail?id=bot%2F1", {"active": False}))
        self.assertEqual(calls[4], ("PUT", "/v1/bots/detail?id=bot%2F1", {"active": True}))
        self.assertEqual(calls[5], ("DELETE", "/v1/bots/detail?id=bot%2F1", None))
        self.assertEqual(calls[6], ("GET", "/v1/inbox?unread=true", None))
        self.assertEqual(calls[7], ("POST", "/v1/inbox", {"source": "webhook", "content": "ping", "action": "trigger", "header": {}}))
        self.assertEqual(calls[8], ("DELETE", "/v1/inbox", {"id": "in-1"}))
        self.assertEqual(calls[9], ("POST", "/v1/inbox/read", {"ids": ["in-1", "in-2"], "all": False}))
        self.assertEqual(calls[10], ("POST", "/v1/inbox/read", {"all": True}))
        self.assertEqual(calls[11], ("GET", "/v1/channels/groups?type=telegram", None))

    def test_agent_kit_exposes_bots_namespace(self) -> None:
        self.assertIs(yunque.create_agent_kit().bots, yunque.bots)


if __name__ == "__main__":
    unittest.main()
