from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_cron_system_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_cron_system_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class CronSystemNamespaceTest(unittest.TestCase):
    def test_list_add_remove_and_run(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            if path == "/v1/cron/list":
                return {"jobs": [{"id": "job_1", "name": "daily"}]}
            if path == "/v1/cron/add":
                return {"job": {"id": "job_2", "name": body["name"], "schedule": body["schedule"], "payload": body["payload"]}}
            if path == "/v1/cron/remove?id=job_1":
                return {"deleted": "job_1"}
            if path == "/v1/cron/run?id=job_1":
                return {"run": {"job_id": "job_1", "run_id": "run_1", "status": "success"}}
            raise AssertionError(f"unexpected call: {method} {path} {body}")

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            jobs = yunque.cron_system.list()
            added = yunque.cron_system.add("nightly", {"type": "cron", "cron_expr": "0 2 * * *"}, {"kind": "systemEvent"})
            removed = yunque.cron_system.remove("job_1")
            run = yunque.cron_system.run("job_1")

        self.assertEqual(jobs["jobs"][0]["id"], "job_1")
        self.assertEqual(added["job"]["id"], "job_2")
        self.assertEqual(removed["deleted"], "job_1")
        self.assertEqual(run["run"]["status"], "success")
        self.assertEqual(calls[1], ("POST", "/v1/cron/add", {"name": "nightly", "schedule": {"type": "cron", "cron_expr": "0 2 * * *"}, "payload": {"kind": "systemEvent"}}))

    def test_agent_kit_exposes_cron_system_namespace(self) -> None:
        kit = yunque.create_agent_kit()
        self.assertIs(kit.cron_system, yunque.cron_system)


if __name__ == "__main__":
    unittest.main()
