import unittest
from unittest.mock import patch

import sdk.python.yunque as yunque


class RouterSDKTests(unittest.TestCase):
    def test_stats_reads_router_stats(self):
        with patch.object(yunque, "_api_call", return_value={"stats": {"routed": 7}}) as api_call:
            self.assertEqual(yunque.router.stats(), {"stats": {"routed": 7}})
            api_call.assert_called_once_with("GET", "/v1/router/stats")

    def test_agent_kit_exposes_router_namespace(self):
        self.assertIs(yunque.create_agent_kit().router, yunque.router)


if __name__ == "__main__":
    unittest.main()
