from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_ide_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_ide_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class IDETest(unittest.TestCase):
    def test_ide_namespace_delegates_status_review_and_helpers(self) -> None:
        with patch.object(yunque, "_api_call", side_effect=[
            {"connected": True, "capabilities": ["review"]},
            {"summary": "ok", "issues": [], "score": 9},
            {"summary": "diff", "issues": [], "score": 8},
            {"summary": "quick", "issues": [], "score": 7},
            {"summary": "full", "issues": [], "score": 9},
        ]) as api_call:
            self.assertTrue(yunque.ide.status()["connected"])
            self.assertEqual(yunque.ide.review({"file_path": "main.go", "content": "package main", "mode": "full"})["score"], 9)
            self.assertEqual(yunque.ide.review_diff("+fmt.Println(1)", file_path="main.go", language="go")["summary"], "diff")
            self.assertEqual(yunque.ide.review_quick("console.log(1)", file_path="main.ts", language="ts")["summary"], "quick")
            self.assertEqual(yunque.ide.review_full("print(1)", file_path="main.py", language="py")["summary"], "full")

        self.assertEqual(api_call.call_args_list[0].args, ("GET", "/v1/ide/status"))
        self.assertEqual(api_call.call_args_list[1].args, ("POST", "/v1/ide/review", {"file_path": "main.go", "content": "package main", "mode": "full"}))
        self.assertEqual(api_call.call_args_list[2].args, ("POST", "/v1/ide/review", {"diff": "+fmt.Println(1)", "mode": "diff", "file_path": "main.go", "language": "go"}))
        self.assertEqual(api_call.call_args_list[3].args, ("POST", "/v1/ide/review", {"content": "console.log(1)", "mode": "quick", "file_path": "main.ts", "language": "ts"}))
        self.assertEqual(api_call.call_args_list[4].args, ("POST", "/v1/ide/review", {"content": "print(1)", "mode": "full", "file_path": "main.py", "language": "py"}))

    def test_agent_kit_exposes_ide(self) -> None:
        self.assertIs(yunque.create_agent_kit().ide, yunque.ide)


if __name__ == "__main__":
    unittest.main()
