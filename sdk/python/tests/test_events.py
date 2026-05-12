from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_events_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_events_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class EventsNamespaceTest(unittest.TestCase):
    def test_events_parse_sse_frames_and_headers(self) -> None:
        parsed = yunque.events.parse(
            "event: connected\n"
            "id: evt-1\n"
            "data: {\"client_id\":\"sse-1\"}\n\n"
            ": ignored\n\n"
            "data: plain\n"
            "retry: 1500\n\n"
        )

        self.assertEqual(yunque.events.stream_url(), "http://localhost:9090/v1/events/stream")
        self.assertEqual(yunque.events.headers()["Accept"], "text/event-stream")
        self.assertEqual(parsed[0]["event"], "connected")
        self.assertEqual(parsed[0]["id"], "evt-1")
        self.assertEqual(parsed[0]["data"]["client_id"], "sse-1")
        self.assertEqual(parsed[1]["event"], "message")
        self.assertEqual(parsed[1]["data"], "plain")
        self.assertEqual(parsed[1]["retry"], 1500)


if __name__ == "__main__":
    unittest.main()
