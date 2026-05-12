from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_providers_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_providers_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class ProvidersNamespaceTest(unittest.TestCase):
    def test_provider_helpers_delegate_to_runtime_routes(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            return {"ok": True, "providers": [{"id": "deepseek"}], "models": [{"id": "m1"}], "mode": "hybrid", "exec_provider": "deepseek"}

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertEqual(yunque.providers.models()["models"][0]["id"], "m1")
            yunque.providers.add_model({"id": "m1", "model_id": "deepseek-chat"})
            yunque.providers.delete_model("m1")
            self.assertEqual(yunque.providers.list()["providers"][0]["id"], "deepseek")
            yunque.providers.test("deepseek")
            yunque.providers.enable("deepseek")
            yunque.providers.disable("deepseek")
            yunque.providers.switch_model("deepseek", "deepseek-chat")
            yunque.providers.set_session("s1", "deepseek")
            self.assertEqual(yunque.providers.mode()["mode"], "hybrid")
            yunque.providers.set_mode("hybrid")
            yunque.providers.presets()
            yunque.providers.register({"preset_id": "deepseek"})
            yunque.providers.delete("deepseek")
            yunque.providers.discover_local("http://127.0.0.1:11434")
            yunque.providers.register_local("http://127.0.0.1:11434", model="qwen", backend="ollama")
            yunque.providers.discover_tori(auto_register=True)
            self.assertEqual(yunque.providers.exec()["exec_provider"], "deepseek")
            yunque.providers.set_exec("deepseek")
            yunque.providers.reset_breakers()

        self.assertIn(("GET", "/api/providers", None), calls)
        self.assertIn(("POST", "/api/providers/switch-model", {"id": "deepseek", "model": "deepseek-chat"}), calls)
        self.assertIn(("POST", "/api/providers/tori/discover?auto_register=true", None), calls)
        self.assertEqual(calls[-1], ("POST", "/api/breaker/reset", None))


if __name__ == "__main__":
    unittest.main()
