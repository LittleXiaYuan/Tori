import unittest
from unittest.mock import patch

import sdk.python.yunque as yunque


class SkillHubSDKTests(unittest.TestCase):
    def test_catalog_lifecycle_policy_and_analytics_routes(self):
        with patch.object(yunque, "_api_call", return_value={"ok": True}) as api_call:
            yunque.skillhub.search("browser", limit=5, source="clawhub")
            yunque.skillhub.installed()
            yunque.skillhub.install("browser")
            yunque.skillhub.uninstall("browser")
            yunque.skillhub.trending(limit=3, cursor="n1")
            yunque.skillhub.detail("browser")
            yunque.skillhub.check_updates()
            yunque.skillhub.update("browser")
            yunque.skillhub.rollback("browser", "1.0.0")
            yunque.skillhub.versions("browser")
            yunque.skillhub.policy()
            yunque.skillhub.update_policy({"min_security_score": 80})
            yunque.skillhub.policy_check("browser")
            yunque.skillhub.analytics()

        calls = [call.args for call in api_call.call_args_list]
        self.assertEqual(calls[0], ("GET", "/api/skillhub/search?q=browser&limit=5&source=clawhub"))
        self.assertEqual(calls[1], ("GET", "/api/skillhub/installed"))
        self.assertEqual(calls[2], ("POST", "/api/skillhub/install", {"slug": "browser"}))
        self.assertEqual(calls[3], ("POST", "/api/skillhub/uninstall", {"slug": "browser"}))
        self.assertEqual(calls[4], ("GET", "/api/skillhub/trending?limit=3&cursor=n1"))
        self.assertEqual(calls[5], ("GET", "/api/skillhub/detail?slug=browser"))
        self.assertEqual(calls[6], ("GET", "/api/skillhub/check-updates"))
        self.assertEqual(calls[7], ("POST", "/api/skillhub/update", {"slug": "browser"}))
        self.assertEqual(calls[8], ("POST", "/api/skillhub/rollback", {"slug": "browser", "version": "1.0.0"}))
        self.assertEqual(calls[9], ("GET", "/api/skillhub/versions?slug=browser"))
        self.assertEqual(calls[10], ("GET", "/api/skillhub/policy"))
        self.assertEqual(calls[11], ("POST", "/api/skillhub/policy", {"min_security_score": 80}))
        self.assertEqual(calls[12], ("GET", "/api/skillhub/policy/check?slug=browser"))
        self.assertEqual(calls[13], ("GET", "/api/skillhub/analytics"))

    def test_agent_kit_exposes_skillhub_namespace(self):
        self.assertIs(yunque.create_agent_kit().skillhub, yunque.skillhub)


if __name__ == "__main__":
    unittest.main()
