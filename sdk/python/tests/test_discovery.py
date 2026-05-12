from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_discovery_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_discovery_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class DiscoveryTest(unittest.TestCase):
    def test_discovery_namespace_delegates_identity_embeddings_and_search(self) -> None:
        with patch.object(yunque, "_api_call", side_effect=[
            {"unified_id": "u1", "display_name": "小羽"},
            {"profiles": [{"unified_id": "u1"}]},
            {"providers": ["mock"]},
            {"embedding": [0.1, 0.2], "dimensions": 2},
            {"results": [{"title": "云雀"}]},
            {"enabled": True, "providers": ["bing"]},
        ]) as api_call:
            self.assertEqual(yunque.discovery.resolve_identity("feishu", "42", "小羽")["unified_id"], "u1")
            self.assertEqual(yunque.discovery.identity_profiles()["profiles"][0]["unified_id"], "u1")
            self.assertEqual(yunque.discovery.embedding_providers()["providers"][0], "mock")
            self.assertEqual(yunque.discovery.embed("云雀", "mock")["dimensions"], 2)
            self.assertEqual(yunque.discovery.search("planner", limit=3, provider="bing")["results"][0]["title"], "云雀")
            self.assertTrue(yunque.discovery.search_providers()["enabled"])

        self.assertEqual(api_call.call_args_list[0].args, ("POST", "/v1/identity/resolve", {"channel": "feishu", "user_id": "42", "display_name": "小羽"}))
        self.assertEqual(api_call.call_args_list[1].args, ("GET", "/v1/identity/profiles"))
        self.assertEqual(api_call.call_args_list[2].args, ("GET", "/v1/embeddings"))
        self.assertEqual(api_call.call_args_list[3].args, ("POST", "/v1/embeddings", {"text": "云雀", "provider": "mock"}))
        self.assertEqual(api_call.call_args_list[4].args, ("GET", "/v1/search?q=planner&limit=3&provider=bing"))
        self.assertEqual(api_call.call_args_list[5].args, ("GET", "/v1/search/providers"))

    def test_agent_kit_exposes_discovery(self) -> None:
        self.assertIs(yunque.create_agent_kit().discovery, yunque.discovery)


if __name__ == "__main__":
    unittest.main()
