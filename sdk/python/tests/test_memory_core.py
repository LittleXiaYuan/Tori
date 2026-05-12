from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_memory_core_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_memory_core_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class MemoryCoreNamespaceTest(unittest.TestCase):
    def test_stats_search_add_and_compact(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            if path == "/v1/memory/stats":
                return {"short": 1, "mid": 2, "long": 3}
            if path == "/v1/memory/search":
                self.assertEqual(body, {"query": "偏好", "limit": 2, "layer": "long"})
                return {"results": [{"key": "pref", "value": "喜欢中文"}], "count": 1}
            if path == "/v1/memory/add":
                self.assertEqual(body, {"value": "喜欢短回答", "layer": "mid", "source": "sdk", "tags": ["preference"]})
                return {"status": "ok"}
            if path == "/v1/memory/compact":
                self.assertEqual(body, {"target_count": 10})
                return {"status": "compacted"}
            raise AssertionError(f"unexpected call: {method} {path}")

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertEqual(yunque.memory_core.stats()["long"], 3)
            self.assertEqual(yunque.memory_core.search("偏好", limit=2, layer="long")["count"], 1)
            self.assertEqual(yunque.memory_core.remember("喜欢短回答", source="sdk", tags=["preference"])["status"], "ok")
            self.assertEqual(yunque.memory_core.compact(target_count=10)["status"], "compacted")

        self.assertEqual(len(calls), 4)

    def test_add_accepts_full_body_and_normalizes_content(self) -> None:
        with patch.object(yunque, "_api_call", return_value={"status": "ok"}) as api_call:
            yunque.memory_core.add({"content": "保留完整 body", "layer": "long"})
        api_call.assert_called_once_with(
            "POST",
            "/v1/memory/add",
            {"content": "保留完整 body", "layer": "long", "value": "保留完整 body"},
        )


if __name__ == "__main__":
    unittest.main()
