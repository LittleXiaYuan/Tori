from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_files_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_files_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class FilesTest(unittest.TestCase):
    def test_files_helpers_delegate_to_api(self) -> None:
        with patch.object(yunque, "_api_call", return_value={"ok": True}) as api_call:
            yunque.files.list("artifacts")
            yunque.files.preview("artifacts/report.md")
            yunque.files.download("artifacts/report.md")

        self.assertEqual(api_call.call_args_list[0].args, ("GET", "/api/files?path=artifacts"))
        self.assertEqual(api_call.call_args_list[1].args, ("GET", "/api/files/preview?path=artifacts%2Freport.md"))
        self.assertEqual(api_call.call_args_list[2].args, ("GET", "/api/files/download?path=artifacts%2Freport.md"))


if __name__ == "__main__":
    unittest.main()
