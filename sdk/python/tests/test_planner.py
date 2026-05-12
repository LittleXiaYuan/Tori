from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_planner_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_planner_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class PlannerTest(unittest.TestCase):
    def test_planner_namespace_delegates_recovery_helpers(self) -> None:
        with patch.object(yunque, "_api_call", side_effect=[
            {"checkpoints": [{"plan_id": "plan-1"}], "count": 1},
            {"action": "retry_failed", "plan_id": "plan-1"},
            {"status": "accepted", "task_id": "task-1"},
            {"status": "accepted", "plan_id": "plan-1", "job_id": "job-1"},
            {"job": {"id": "job-1", "status": "running"}},
            {"plan_id": "plan-1", "next_action": "retry_failed"},
        ]) as api_call:
            self.assertEqual(yunque.planner.list_checkpoints(limit=5, plan_id="plan-1", include_snapshot=True)["count"], 1)
            self.assertEqual(yunque.planner.recover_checkpoint("plan-1", "retry_failed")["action"], "retry_failed")
            self.assertEqual(yunque.planner.resume_checkpoint_task("plan-1", "continue", run=False)["status"], "accepted")
            self.assertEqual(yunque.planner.resume_checkpoint_plan("plan-1", "continue", async_=True)["job_id"], "job-1")
            self.assertEqual(yunque.planner.get_resume_plan_job(job_id="job-1")["job"]["id"], "job-1")
            self.assertEqual(yunque.planner.execution_state("plan-1", "retry_failed")["next_action"], "retry_failed")

        self.assertEqual(api_call.call_args_list[0].args, ("GET", "/v1/planner/checkpoints?limit=5&plan_id=plan-1&include_snapshot=true"))
        self.assertEqual(api_call.call_args_list[1].args, ("POST", "/v1/planner/checkpoints/recover", {"plan_id": "plan-1", "action": "retry_failed"}))
        self.assertEqual(api_call.call_args_list[2].args, ("POST", "/v1/planner/checkpoints/resume", {"plan_id": "plan-1", "action": "continue", "run": False}))
        self.assertEqual(api_call.call_args_list[3].args, ("POST", "/v1/planner/checkpoints/resume-plan", {"plan_id": "plan-1", "action": "continue", "async": True}))
        self.assertEqual(api_call.call_args_list[4].args, ("GET", "/v1/planner/checkpoints/resume-plan/jobs?job_id=job-1"))
        self.assertEqual(api_call.call_args_list[5].args, ("GET", "/v1/planner/execution-state?plan_id=plan-1&action=retry_failed"))

    def test_agent_kit_exposes_planner(self) -> None:
        self.assertIs(yunque.create_agent_kit().planner, yunque.planner)


if __name__ == "__main__":
    unittest.main()
