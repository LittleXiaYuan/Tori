from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_system_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_system_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class SystemTest(unittest.TestCase):
    def test_system_namespace_delegates_health_metadata_and_metrics(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            return {"path": path, "status": "ok", "modules": []}

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertEqual(yunque.system.health()["path"], "/healthz")
            self.assertEqual(yunque.system.livez()["path"], "/livez")
            self.assertEqual(yunque.system.readyz()["path"], "/readyz")
            self.assertEqual(yunque.system.cognitive_health()["path"], "/healthz/cognitive")
            self.assertEqual(yunque.system.version()["path"], "/v1/version")
            self.assertEqual(yunque.system.info()["path"], "/v1/system/info")
            self.assertEqual(yunque.system.stats()["path"], "/v1/system/stats")
            self.assertEqual(yunque.system.metrics()["path"], "/v1/metrics")
            self.assertEqual(yunque.system.cache_stats()["path"], "/v1/cache/stats")
            self.assertEqual(yunque.system.modules()["path"], "/v1/modules")
            self.assertEqual(yunque.system.sbom()["path"], "/sbom")

        self.assertEqual(calls[0], ("GET", "/healthz", None))
        self.assertEqual(calls[-1], ("GET", "/sbom", None))

    def test_system_prometheus_raw_and_agent_kit_exposes_system(self) -> None:
        with patch.object(yunque, "_api_call_raw", return_value="yunque_requests_total 12\n") as raw:
            self.assertIn("yunque_requests_total", yunque.system.metrics_prometheus())
        raw.assert_called_once_with("GET", "/v1/metrics/prometheus")
        self.assertIs(yunque.create_agent_kit().system, yunque.system)


if __name__ == "__main__":
    unittest.main()
