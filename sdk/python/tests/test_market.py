from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_market_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_market_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class SkillMarketNamespaceTest(unittest.TestCase):
    def test_market_helpers(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            if path == "/v1/market/search?q=docx":
                return {"skills": [{"name": "doc_parse", "version": "1.0.0"}], "count": 1}
            if path == "/v1/market/search":
                return {"skills": [{"name": "web_search", "version": "1.0.0"}]}
            if path == "/v1/market/top?n=3&by=rating":
                return {"skills": [{"name": "code_gen", "rating": 4.8}]}
            if path == "/v1/market/stats":
                return {"total": 3, "categories": {"coding": 1}}
            raise AssertionError(f"unexpected call: {method} {path}")

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertEqual(yunque.market.search("docx")["skills"][0]["name"], "doc_parse")
            self.assertEqual(yunque.market.search()["skills"][0]["name"], "web_search")
            self.assertEqual(yunque.market.top(n=3, by="rating")["skills"][0]["name"], "code_gen")
            self.assertEqual(yunque.market.stats()["total"], 3)

        self.assertEqual(calls[0], ("GET", "/v1/market/search?q=docx", None))
        self.assertEqual(calls[2], ("GET", "/v1/market/top?n=3&by=rating", None))
        self.assertIs(yunque.create_agent_kit().market, yunque.market)


if __name__ == "__main__":
    unittest.main()
