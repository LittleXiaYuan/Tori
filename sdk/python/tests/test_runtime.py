from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_runtime_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_runtime_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class RuntimeTest(unittest.TestCase):
    def test_runtime_helpers_delegate_to_api(self) -> None:
        with patch.object(yunque, "_api_call", return_value={"ok": True}) as api_call:
            yunque.runtime.queues()
            yunque.runtime.session_queue("session/1")
            yunque.runtime.cancel_queued_task("session/1", "task-1")

        self.assertEqual(api_call.call_args_list[0].args, ("GET", "/v1/sessions/queue"))
        self.assertEqual(api_call.call_args_list[1].args, ("GET", "/v1/sessions/queue?id=session%2F1"))
        self.assertEqual(api_call.call_args_list[2].args, ("POST", "/v1/sessions/queue/cancel", {"session_id": "session/1", "task_id": "task-1"}))
        self.assertTrue(yunque.runtime.events_url().endswith("/v1/events/stream"))
        self.assertEqual(yunque.runtime.event_headers()["Accept"], "text/event-stream")


if __name__ == "__main__":
    unittest.main()
