from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_lora_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_lora_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class LoRANamespaceTest(unittest.TestCase):
    def test_lora_lifecycle_helpers(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            if path == "/v1/lora/status":
                return {"active_model": "adapter-a"}
            if path == "/v1/lora/history":
                return {"records": [{"adapter": "a1"}], "count": 1}
            if path == "/v1/lora/summary":
                return {"summary": {"best_score": 0.9}}
            if path == "/v1/lora/preview?tenant_id=default":
                return {"preview": {"ready": True, "tenant_id": "default"}}
            if path == "/v1/lora/trigger":
                return {"status": "ok", "tenant_id": body.get("tenant_id")}
            if path == "/v1/lora/rollback":
                return {"status": "ok"}
            if path == "/v1/lora/evolution":
                return {"state": {"phase": "eval"}}
            if path == "/v1/lora/config":
                return {"config": {"min_samples": 8}, "status": "updated" if method == "PUT" else "ok"}
            raise AssertionError(f"unexpected call: {method} {path}")

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertEqual(yunque.lora.status()["active_model"], "adapter-a")
            self.assertEqual(yunque.lora.history()["count"], 1)
            self.assertEqual(yunque.lora.summary()["summary"]["best_score"], 0.9)
            self.assertTrue(yunque.lora.preview("default")["preview"]["ready"])
            self.assertEqual(yunque.lora.trigger("default")["tenant_id"], "default")
            self.assertEqual(yunque.lora.rollback()["status"], "ok")
            self.assertEqual(yunque.lora.evolution()["state"]["phase"], "eval")
            self.assertEqual(yunque.lora.config()["config"]["min_samples"], 8)
            self.assertEqual(yunque.lora.update_config({"min_samples": 9})["status"], "updated")

        self.assertEqual(calls[3], ("GET", "/v1/lora/preview?tenant_id=default", None))
        self.assertEqual(calls[4], ("POST", "/v1/lora/trigger", {"tenant_id": "default"}))
        self.assertEqual(calls[8], ("PUT", "/v1/lora/config", {"min_samples": 9}))


if __name__ == "__main__":
    unittest.main()
