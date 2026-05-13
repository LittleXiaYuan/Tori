
from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import MagicMock, patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_webchat_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_webchat_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class WebChatTest(unittest.TestCase):
    def test_webchat_builds_widget_url_and_embed_snippet(self) -> None:
        with patch.object(yunque, "_API_BASE", "https://agent.example/"):
            self.assertEqual(yunque.webchat.widget_url(), "https://agent.example/v1/webchat/widget.js")
            snippet = yunque.webchat.embed_snippet("key&1", title='Tori "Chat"', theme="dark")
        self.assertIn('src="https://agent.example/v1/webchat/widget.js"', snippet)
        self.assertIn('data-api-key="key&amp;1"', snippet)
        self.assertIn('data-title="Tori &quot;Chat&quot;"', snippet)
        self.assertIn('data-theme="dark"', snippet)
        with self.assertRaises(ValueError):
            yunque.webchat.embed_snippet("")

    def test_widget_script_fetches_text_with_origin(self) -> None:
        resp = MagicMock()
        resp.__enter__.return_value.read.return_value = b"console.log('ok')"
        with patch.object(yunque, "_API_BASE", "http://localhost:9090"), patch.object(yunque.urllib.request, "urlopen", return_value=resp) as urlopen:
            script = yunque.webchat.widget_script("https://site.example")
        self.assertEqual(script, "console.log('ok')")
        req = urlopen.call_args.args[0]
        self.assertEqual(req.full_url, "http://localhost:9090/v1/webchat/widget.js")
        self.assertEqual(req.get_header("Origin"), "https://site.example")

    def test_agent_kit_exposes_webchat_namespace(self) -> None:
        self.assertIs(yunque.create_agent_kit().webchat, yunque.webchat)


if __name__ == "__main__":
    unittest.main()
