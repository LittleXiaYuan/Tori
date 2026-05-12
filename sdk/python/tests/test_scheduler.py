from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_scheduler_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_scheduler_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class SchedulerNamespaceTest(unittest.TestCase):
    def test_jobs_add_and_remove(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            if path == "/v1/scheduler/jobs":
                return {"jobs": [{"id": "job_1", "name": "daily"}], "count": 1}
            if path == "/v1/scheduler/add":
                return {"id": "job_2", "name": body["name"], "prompt": body["prompt"], "interval": 3600000000000}
            if path == "/v1/scheduler/remove":
                return {"status": "removed"}
            raise AssertionError(f"unexpected call: {method} {path}")

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            jobs = yunque.scheduler.jobs()
            added = yunque.scheduler.add("hourly", "检查任务", "1h")
            removed = yunque.scheduler.remove("job_1")

        self.assertEqual(jobs["count"], 1)
        self.assertEqual(added["id"], "job_2")
        self.assertEqual(removed["status"], "removed")
        self.assertEqual(calls, [
            ("GET", "/v1/scheduler/jobs", None),
            ("POST", "/v1/scheduler/add", {"name": "hourly", "prompt": "检查任务", "interval": "1h"}),
            ("POST", "/v1/scheduler/remove", {"id": "job_1"}),
        ])

    def test_agent_kit_exposes_scheduler_namespace(self) -> None:
        kit = yunque.create_agent_kit()
        self.assertIs(kit.scheduler, yunque.scheduler)


if __name__ == "__main__":
    unittest.main()
