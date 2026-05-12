from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_missions_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_missions_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class MissionsNamespaceTest(unittest.TestCase):
    def test_parse_serializes_description(self) -> None:
        with patch.object(
            yunque,
            "_api_call",
            return_value={
                "type": "cron",
                "name": "每日总结",
                "description": "每天总结昨天的任务",
                "config": {"cron_expr": "0 8 * * *", "message": "总结昨天"},
                "confidence": 0.9,
                "explanation": "mentions daily schedule",
            },
        ) as api_call:
            result = yunque.missions.parse("每天八点总结昨天的任务")

        self.assertEqual(result["type"], "cron")
        api_call.assert_called_once_with(
            "POST",
            "/v1/missions/parse",
            {"description": "每天八点总结昨天的任务"},
        )

    def test_agent_kit_exposes_missions_namespace(self) -> None:
        kit = yunque.create_agent_kit()
        self.assertIs(kit.missions, yunque.missions)


if __name__ == "__main__":
    unittest.main()
