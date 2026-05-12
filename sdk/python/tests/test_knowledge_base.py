from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_knowledge_base_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_knowledge_base_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class KnowledgeBaseNamespaceTest(unittest.TestCase):
    def test_stats_search_sources_and_mutations(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            if path == "/v1/knowledge/stats":
                return {"sources": 2, "chunks": 8}
            if path == "/v1/knowledge/sources":
                return {"sources": [{"id": "src_1", "name": "README.md"}]}
            if path == "/v1/knowledge/search?q=SDK&n=3&lang=go":
                return {"chunks": [{"id": "c1", "content": "SDK slice"}], "count": 1}
            if path == "/v1/knowledge/ingest":
                self.assertEqual(body, {"content": "hello", "name": "inline"})
                return {"source": {"id": "src_2", "name": "inline"}, "stats": {"sources": 3}}
            if path == "/v1/knowledge/source/update":
                self.assertEqual(body, {"id": "src_2", "name": "updated"})
                return {"source": {"id": "src_2", "name": "updated"}}
            if path == "/v1/knowledge/source?id=src_2":
                return {"deleted": "src_2"}
            if path == "/v1/knowledge/import-url":
                self.assertEqual(body, {"url": "https://example.test", "max_pages": 2})
                return {"sources": [{"id": "src_url"}], "stats": {"sources": 3}}
            if path == "/v1/knowledge/import-repo":
                self.assertEqual(body, {"path": ".", "max_files": 10})
                return {"source": {"id": "src_repo"}}
            raise AssertionError(f"unexpected call: {method} {path}")

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertEqual(yunque.knowledge_base.stats()["chunks"], 8)
            self.assertEqual(len(yunque.knowledge_base.sources()["sources"]), 1)
            self.assertEqual(yunque.knowledge_base.search("SDK", limit=3, lang="go")["count"], 1)
            self.assertEqual(yunque.knowledge_base.ingest("hello", name="inline")["source"]["id"], "src_2")
            self.assertEqual(yunque.knowledge_base.update_source("src_2", name="updated")["source"]["name"], "updated")
            self.assertEqual(yunque.knowledge_base.delete_source("src_2")["deleted"], "src_2")
            self.assertEqual(len(yunque.knowledge_base.import_url("https://example.test", max_pages=2)["sources"]), 1)
            self.assertEqual(yunque.knowledge_base.import_repo(".", max_files=10)["source"]["id"], "src_repo")

        self.assertEqual(len(calls), 8)


if __name__ == "__main__":
    unittest.main()
