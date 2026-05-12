from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_browser_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_browser_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class BrowserTest(unittest.TestCase):
    def test_browser_helpers_delegate_to_api(self) -> None:
        with patch.object(yunque, "_api_call", return_value={"ok": True}) as api_call:
            yunque.browser.status()
            yunque.browser.config()
            yunque.browser.navigate("https://example.test")
            yunque.browser.screenshot()
            yunque.browser.latest_screenshot()
            yunque.browser.ocr()
            yunque.browser.opp_pending()
            yunque.browser.opp_decide("allow_once", problem_id="opp1")
            yunque.browser.extension_status()
            yunque.browser.extension_session()
            yunque.browser.extension_action({"type": "browser_screenshot"})
            yunque.browser.scenarios()
            yunque.browser.run_scenario("open-page")

        expected = [
            ("GET", "/v1/browser/status"),
            ("GET", "/v1/browser/config"),
            ("POST", "/v1/browser/navigate", {"url": "https://example.test"}),
            ("GET", "/v1/browser/screenshot"),
            ("GET", "/v1/browser/screenshot/latest"),
            ("POST", "/v1/browser/ocr", {}),
            ("GET", "/v1/browser/opp/pending"),
            ("POST", "/v1/browser/opp/decide", {"decision": "allow_once", "problem_id": "opp1"}),
            ("GET", "/api/browser/ext/status"),
            ("POST", "/api/browser/ext/session", {}),
            ("POST", "/api/browser/ext/action", {"type": "browser_screenshot"}),
            ("GET", "/api/browser/ext/scenarios"),
            ("POST", "/api/browser/ext/scenarios/run", {"scenario_id": "open-page"}),
        ]
        self.assertEqual([c.args for c in api_call.call_args_list], expected)


if __name__ == "__main__":
    unittest.main()
