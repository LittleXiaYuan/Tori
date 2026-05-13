from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_under_test_airi", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_under_test_airi"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class AiriNamespaceTest(unittest.TestCase):
    def test_airi_namespace_delegates_bridge_routes(self) -> None:
        calls: list[tuple] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            if path == "/v1/ext/airi/status":
                return {"plugin": "airi", "connected": False}
            if path == "/v1/ext/airi/models":
                return {"object": "list", "data": [{"id": "yunque-airi"}]}
            if path == "/v1/ext/airi/chat/completions":
                return {"choices": [{"message": {"role": "assistant", "content": "hi"}}]}
            raise AssertionError(f"unexpected call: {method} {path}")

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertEqual(yunque.airi.status()["plugin"], "airi")
            self.assertEqual(yunque.airi.models()["data"][0]["id"], "yunque-airi")
            reply = yunque.airi.chat_completions([{"role": "user", "content": "hi"}], model="yunque-airi")
            self.assertEqual(reply["choices"][0]["message"]["content"], "hi")

        self.assertEqual(calls[2], ("POST", "/v1/ext/airi/chat/completions", {"model": "yunque-airi", "messages": [{"role": "user", "content": "hi"}], "stream": False}))

    def test_airi_stream_helpers_and_agent_kit(self) -> None:
        request = yunque.airi.stream_request([{"role": "user", "content": "hi"}])
        self.assertTrue(request["stream"])
        items = yunque.airi.parse_stream('data: {"choices":[{"delta":{"content":"hi"}}]}\n\ndata: [DONE]\n\n')
        self.assertEqual(items[0]["kind"], "chunk")
        self.assertEqual(items[1]["kind"], "done")
        self.assertIs(yunque.create_agent_kit().airi, yunque.airi)


if __name__ == "__main__":
    unittest.main()
