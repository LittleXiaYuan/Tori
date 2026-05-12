from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_graph_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_graph_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class GraphNamespaceTest(unittest.TestCase):
    def test_graph_read_write_helpers(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            if path == "/v1/graph/entities?q=%E4%BA%91%E9%9B%80":
                return {"entities": [{"id": "e1", "name": "云雀"}]}
            if path == "/v1/graph/entities" and method == "POST":
                self.assertEqual(body, {"name": "云雀", "type": "agent"})
                return {"id": "e1", "name": "云雀", "type": "agent"}
            if path == "/v1/graph/relations?entity_id=e1":
                return {"relations": [{"id": "r1", "from_id": "e1", "to_id": "e2", "type": "uses"}]}
            if path == "/v1/graph/relations" and method == "POST":
                self.assertEqual(body, {"from_id": "e1", "to_id": "e2", "type": "uses"})
                return {"id": "r1", "from_id": "e1", "to_id": "e2", "type": "uses"}
            if path == "/v1/graph/context?entity_id=e1":
                return {"context": "云雀 -> SDK", "neighbors": [{"id": "e2"}]}
            if path == "/v1/graph/context?name=%E4%BA%91%E9%9B%80":
                return {"context": "云雀 by name"}
            if path == "/v1/graph/stats":
                return {"entities": 2, "relations": 1}
            if path == "/v1/graph/entities?id=e1" and method == "DELETE":
                return {"ok": True}
            raise AssertionError(f"unexpected call: {method} {path}")

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            self.assertEqual(len(yunque.graph.entities("云雀")["entities"]), 1)
            self.assertEqual(yunque.graph.put_entity({"name": "云雀", "type": "agent"})["id"], "e1")
            self.assertEqual(len(yunque.graph.relations("e1")["relations"]), 1)
            self.assertEqual(yunque.graph.put_relation({"from_id": "e1", "to_id": "e2", "type": "uses"})["id"], "r1")
            self.assertIn("SDK", yunque.graph.context_by_entity_id("e1")["context"])
            self.assertIn("name", yunque.graph.context_by_name("云雀")["context"])
            self.assertEqual(yunque.graph.stats()["entities"], 2)
            self.assertTrue(yunque.graph.delete_entity("e1")["ok"])

        self.assertEqual(len(calls), 8)


if __name__ == "__main__":
    unittest.main()
