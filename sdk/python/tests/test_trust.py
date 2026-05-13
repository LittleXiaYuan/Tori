from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_trust_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_trust_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class TrustTest(unittest.TestCase):
    def test_trust_helpers_delegate_to_api(self) -> None:
        with patch.object(yunque, "_api_call", return_value={"ok": True}) as api_call:
            yunque.trust.scores()
            yunque.trust.reset("shell")
            yunque.trust.grant("shell")
            yunque.trust.grant_all()
            yunque.trust.review_status()
            yunque.trust.skillgrow_patterns()
            yunque.skillgrow.patterns()
            yunque.review.status()

        self.assertEqual(api_call.call_args_list[0].args, ("GET", "/api/trust/scores"))
        self.assertEqual(api_call.call_args_list[1].args, ("POST", "/api/trust/reset", {"slug": "shell"}))
        self.assertEqual(api_call.call_args_list[2].args, ("POST", "/api/trust/grant", {"slug": "shell"}))
        self.assertEqual(api_call.call_args_list[3].args, ("POST", "/api/trust/grant", {"slug": "*"}))
        self.assertEqual(api_call.call_args_list[4].args, ("GET", "/api/review/status"))
        self.assertEqual(api_call.call_args_list[5].args, ("GET", "/api/skillgrow/patterns"))
        self.assertEqual(api_call.call_args_list[6].args, ("GET", "/api/skillgrow/patterns"))
        self.assertEqual(api_call.call_args_list[7].args, ("GET", "/api/review/status"))

    def test_agent_kit_exposes_skillgrow(self) -> None:
        self.assertIs(yunque.create_agent_kit().skillgrow, yunque.skillgrow)

    def test_agent_kit_exposes_review(self) -> None:
        self.assertIs(yunque.create_agent_kit().review, yunque.review)


if __name__ == "__main__":
    unittest.main()
