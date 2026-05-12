from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_reflect_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_reflect_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class ReflectNamespaceTest(unittest.TestCase):
    def test_experiences_serializes_filters(self) -> None:
        with patch.object(yunque, "_api_call", return_value={"experiences": [{"id": "e1"}], "total": 1}) as api_call:
            resp = yunque.reflect.experiences(
                q="code review",
                source="task",
                outcome="partial",
                tag="quality:9",
                limit=5,
            )

        self.assertEqual(resp["total"], 1)
        api_call.assert_called_once_with(
            "GET",
            "/v1/reflect/experiences?q=code+review&source=task&outcome=partial&tag=quality%3A9&limit=5",
        )

    def test_stats_and_strategies(self) -> None:
        calls: list[tuple[str, str]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path))
            if path.startswith("/v1/reflect/experiences"):
                return {"total": 2, "by_outcome": {"success": 2}, "recent_7d": 1}
            if path.startswith("/v1/reflect/strategies"):
                return {"strategies": "- 推荐: keep slices small"}
            raise AssertionError(f"unexpected call: {method} {path}")

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            stats = yunque.reflect.stats(tag="quality:9")
            strategies = yunque.reflect.strategies(tag="quality:9", limit=3)

        self.assertEqual(stats["recent_7d"], 1)
        self.assertIn("keep slices small", strategies)
        self.assertEqual(calls, [
            ("GET", "/v1/reflect/experiences?tag=quality%3A9&stats=true"),
            ("GET", "/v1/reflect/strategies?tag=quality%3A9&limit=3"),
        ])


if __name__ == "__main__":
    unittest.main()
