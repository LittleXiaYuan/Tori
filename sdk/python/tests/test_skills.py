import unittest
from unittest.mock import patch

import sdk.python.yunque as yunque


class SkillsSDKTests(unittest.TestCase):
    def test_catalog_scan_dynamic_review_and_suggestions_routes(self):
        with patch.object(yunque, "_api_call", return_value={"ok": True}) as api_call:
            yunque.skills.list()
            yunque.skills_catalog.list()
            yunque.skills.scan()
            yunque.skills_scan.scan()
            yunque.skills.dynamic()
            yunque.skills.approve("draft_doc", "use safely")
            yunque.skills.reject("old_skill")
            yunque.skills.suggestions("sess-1")

        calls = [call.args for call in api_call.call_args_list]
        self.assertEqual(calls[0], ("GET", "/v1/skills"))
        self.assertEqual(calls[1], ("GET", "/v1/skills"))
        self.assertEqual(calls[2], ("POST", "/v1/skills/scan"))
        self.assertEqual(calls[3], ("POST", "/v1/skills/scan"))
        self.assertEqual(calls[4], ("GET", "/v1/skills/dynamic"))
        self.assertEqual(calls[5], ("POST", "/v1/skills/approve", {"name": "draft_doc", "instruction": "use safely"}))
        self.assertEqual(calls[6], ("POST", "/v1/skills/reject", {"name": "old_skill"}))
        self.assertEqual(calls[7], ("GET", "/v1/skill-suggestions?session_id=sess-1"))

    def test_agent_kit_exposes_skills_namespace(self):
        self.assertIs(yunque.create_agent_kit().skills, yunque.skills)
        self.assertIs(yunque.create_agent_kit().skills_catalog, yunque.skills_catalog)
        self.assertIs(yunque.create_agent_kit().skills_scan, yunque.skills_scan)


if __name__ == "__main__":
    unittest.main()
