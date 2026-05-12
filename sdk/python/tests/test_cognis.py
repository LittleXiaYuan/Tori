from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_cognis_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_cognis_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class CognisNamespaceTest(unittest.TestCase):
    def test_cognis_helpers_delegate_to_runtime_routes(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            if path == "/v1/cognis":
                if method == "POST":
                    return {"id": "reviewer", "name": "Code Reviewer"}
                return {"cognis": [{"id": "reviewer"}], "count": 1}
            if path.endswith("/experience"):
                return {"enabled": True}
            if path.endswith("/federation/peers"):
                return {"peers": []}
            return {"status": "ok", "id": "reviewer"}

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertEqual(yunque.cognis.list()["count"], 1)
            self.assertEqual(yunque.cognis.create({"id": "reviewer"})["id"], "reviewer")
            self.assertEqual(yunque.cognis.get("reviewer")["status"], "ok")
            self.assertEqual(yunque.cognis.remove("reviewer")["status"], "ok")
            self.assertEqual(yunque.cognis.enable("reviewer")["status"], "ok")
            self.assertEqual(yunque.cognis.disable("reviewer")["status"], "ok")
            self.assertEqual(yunque.cognis.reload()["status"], "ok")
            self.assertEqual(yunque.cognis.traces(limit=5)["status"], "ok")
            self.assertEqual(yunque.cognis.trace("reviewer", limit=2)["status"], "ok")
            self.assertEqual(yunque.cognis.stats()["status"], "ok")
            self.assertEqual(yunque.cognis.health("reviewer")["status"], "ok")
            self.assertEqual(yunque.cognis.verify()["status"], "ok")
            self.assertEqual(yunque.cognis.alerts()["status"], "ok")
            self.assertEqual(yunque.cognis.scan_alerts()["status"], "ok")
            self.assertEqual(yunque.cognis.generate({"prompt": "make cogni"})["status"], "ok")
            self.assertEqual(yunque.cognis.export_bundle()["status"], "ok")
            self.assertEqual(yunque.cognis.import_bundle({"bundle": {}})["status"], "ok")
            self.assertEqual(yunque.cognis.workflows("reviewer")["status"], "ok")
            self.assertEqual(yunque.cognis.run_workflow("reviewer", "summarize", {"input": "x"})["status"], "ok")
            self.assertTrue(yunque.cognis.experience("reviewer")["enabled"])
            self.assertEqual(yunque.cognis.record_experience("reviewer", "fact", {"fact": "x"})["status"], "ok")
            self.assertEqual(yunque.cognis.confirm_experience_pattern("reviewer", "pat-1")["status"], "ok")
            self.assertEqual(yunque.cognis.evolve("reviewer")["status"], "ok")
            self.assertEqual(yunque.cognis.evolution("reviewer")["status"], "ok")
            self.assertEqual(yunque.cognis.federation()["status"], "ok")
            self.assertEqual(yunque.cognis.federation_peers()["peers"], [])
            self.assertEqual(yunque.cognis.discover_federation({"query": "reviewer"})["status"], "ok")
            self.assertEqual(yunque.cognis.expose("reviewer")["status"], "ok")
            self.assertEqual(yunque.cognis.unexpose("reviewer")["status"], "ok")
            self.assertEqual(yunque.cognis.economics()["status"], "ok")

        self.assertIn(("GET", "/v1/cognis/traces?limit=5", None), calls)
        self.assertIn(("GET", "/v1/cognis/reviewer/trace?limit=2", None), calls)
        self.assertIn(("POST", "/v1/cognis/reviewer/experience/record", {"type": "fact", "data": {"fact": "x"}}), calls)


if __name__ == "__main__":
    unittest.main()
