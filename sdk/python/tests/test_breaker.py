from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_under_test_breaker", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_under_test_breaker"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class BreakerNamespaceTest(unittest.TestCase):
    def test_breaker_delegates_to_provider_reset(self) -> None:
        with patch.object(yunque.providers, "reset_breakers", return_value={"ok": True, "reset_count": 2}) as reset:
            self.assertEqual(yunque.breaker.reset()["reset_count"], 2)
        reset.assert_called_once_with()

    def test_agent_kit_exposes_breaker_namespace(self) -> None:
        self.assertIs(yunque.create_agent_kit().breaker, yunque.breaker)


if __name__ == "__main__":
    unittest.main()
