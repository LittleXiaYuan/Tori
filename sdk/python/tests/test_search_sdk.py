import unittest
from unittest.mock import patch

import sdk.python.yunque as yunque


class SearchSDKTests(unittest.TestCase):
    def test_query_and_providers_delegate_to_search_routes(self):
        with patch.object(yunque, "_api_call", return_value={"ok": True}) as api_call:
            yunque.search_sdk.query("agent", 3, "local")
            yunque.search_sdk.providers()

        calls = [call.args for call in api_call.call_args_list]
        self.assertEqual(calls[0], ("GET", "/v1/search?q=agent&limit=3&provider=local"))
        self.assertEqual(calls[1], ("GET", "/v1/search/providers"))

    def test_agent_kit_exposes_search_namespace(self):
        self.assertIs(yunque.create_agent_kit().search_sdk, yunque.search_sdk)


if __name__ == "__main__":
    unittest.main()
