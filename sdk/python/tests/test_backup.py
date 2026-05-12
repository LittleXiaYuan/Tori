from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_backup_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_backup_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class BackupTest(unittest.TestCase):
    def test_backup_namespace_delegates_info_export_and_import(self) -> None:
        with patch.object(yunque, "_api_call", return_value={"file_count": 1}) as api_call:
            self.assertEqual(yunque.backup.info()["file_count"], 1)
        api_call.assert_called_once_with("GET", "/v1/backup/info")

        with patch.object(yunque, "_api_call_bytes", return_value=b"zipdata") as raw:
            self.assertEqual(yunque.backup.export(), b"zipdata")
        raw.assert_called_once_with("GET", "/v1/backup/export")

        with patch.object(yunque, "_api_call_multipart", return_value={"success": True, "restored": 2}) as upload:
            self.assertEqual(yunque.backup.import_zip(b"zipdata", "restore.zip")["restored"], 2)
        upload.assert_called_once_with("POST", "/v1/backup/import", "backup", "restore.zip", b"zipdata")

    def test_agent_kit_exposes_backup(self) -> None:
        self.assertIs(yunque.create_agent_kit().backup, yunque.backup)


if __name__ == "__main__":
    unittest.main()
