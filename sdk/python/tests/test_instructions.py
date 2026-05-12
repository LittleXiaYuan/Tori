from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_instructions_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_instructions_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class InstructionsTest(unittest.TestCase):
    def test_instructions_namespace_routes_crud_and_reorder(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            if method == "GET":
                return {"instructions": [{"instruction_id": "ins-1", "content": "保持简洁"}], "total": 1}
            if method == "POST" and path == "/v1/instructions":
                return {"instruction_id": "ins-1", "content": "保持简洁"}
            return {"status": "updated"}

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertEqual(yunque.instructions.list(category="style")["total"], 1)
            self.assertEqual(yunque.instructions.create({"category": "style", "content": "保持简洁"})["instruction_id"], "ins-1")
            self.assertEqual(yunque.instructions.update({"instruction_id": "ins-1", "content": "更新"})["status"], "updated")
            self.assertEqual(yunque.instructions.delete("ins-1")["status"], "updated")
            self.assertEqual(yunque.instructions.reorder(["ins-2", "ins-1"])["status"], "updated")

        self.assertEqual(calls[0], ("GET", "/v1/instructions?category=style", None))
        self.assertEqual(calls[1], ("POST", "/v1/instructions", {"category": "style", "content": "保持简洁"}))
        self.assertEqual(calls[2], ("PUT", "/v1/instructions", {"instruction_id": "ins-1", "content": "更新"}))
        self.assertEqual(calls[3], ("DELETE", "/v1/instructions?id=ins-1", None))
        self.assertEqual(calls[4], ("POST", "/v1/instructions/reorder", {"ids": ["ins-2", "ins-1"]}))


if __name__ == "__main__":
    unittest.main()
