
from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_documents_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_documents_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class DocumentsTest(unittest.TestCase):
    def test_documents_helpers_delegate_to_api(self) -> None:
        calls: list[tuple[str, str, object]] = []

        def fake_api_call(method: str, path: str, body=None, timeout: int = 30):
            calls.append((method, path, body))
            return {"ok": True}

        with patch.object(yunque, "_api_call", side_effect=fake_api_call):
            yunque.documents.templates()
            yunque.documents.generate("docx", "hello", path="out.docx", title="Report")
            yunque.documents.generate_docx("hello", title="Doc")
            yunque.documents.generate_xlsx("a,b", sheet_name="Data")
            yunque.documents.generate_pptx("slides", path="deck.pptx")
            yunque.documents.generate_html("<p>hi</p>")

        self.assertEqual(calls[0], ("GET", "/v1/documents/templates", None))
        self.assertEqual(calls[1], ("POST", "/v1/documents/generate", {"format": "docx", "content": "hello", "path": "out.docx", "title": "Report"}))
        self.assertEqual(calls[2][2], {"format": "docx", "content": "hello", "title": "Doc"})
        self.assertEqual(calls[3][2], {"format": "xlsx", "content": "a,b", "sheet_name": "Data"})
        self.assertEqual(calls[4][2], {"format": "pptx", "content": "slides", "path": "deck.pptx"})
        self.assertEqual(calls[5][2], {"format": "html", "content": "<p>hi</p>"})

    def test_agent_kit_exposes_documents_namespace(self) -> None:
        self.assertIs(yunque.create_agent_kit().documents, yunque.documents)


if __name__ == "__main__":
    unittest.main()
