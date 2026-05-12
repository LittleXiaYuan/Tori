from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest
from unittest.mock import patch

SDK_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = SDK_ROOT / "yunque" / "__init__.py"

spec = importlib.util.spec_from_file_location("yunque_federation_under_test", MODULE_PATH)
yunque = importlib.util.module_from_spec(spec)
sys.modules["yunque_federation_under_test"] = yunque
assert spec.loader is not None
spec.loader.exec_module(yunque)


class FederationTest(unittest.TestCase):
    def test_federation_namespace_delegates_a2a_helpers(self) -> None:
        with patch.object(yunque, "_api_call", side_effect=[
            {"local_id": "agent-local", "peers": [{"id": "peer-a"}]},
            {"peers": 1, "messages": 2},
            {"local": {"agent_id": "agent-a"}, "peers": []},
            {"status": "updated"},
            {"results": [{"peer_id": "p1"}], "count": 1},
            {"status": "delegated", "result": {"task_id": "t1"}},
            {"configured": True, "peers": 2},
            {"status": "broadcasted"},
        ]) as api_call:
            self.assertEqual(yunque.federation.peers()["local_id"], "agent-local")
            self.assertEqual(yunque.federation.stats()["messages"], 2)
            self.assertEqual(yunque.federation.capabilities()["local"]["agent_id"], "agent-a")
            self.assertEqual(yunque.federation.update_capabilities({"agent_id": "agent-a"})["status"], "updated")
            self.assertEqual(yunque.federation.discover({"feature": "browser"})["count"], 1)
            self.assertEqual(yunque.federation.delegate({"peer_id": "p1"})["status"], "delegated")
            self.assertTrue(yunque.federation.bridge_stats()["configured"])
            self.assertEqual(yunque.federation.broadcast()["status"], "broadcasted")

        self.assertEqual(api_call.call_args_list[0].args, ("GET", "/v1/federation/peers"))
        self.assertEqual(api_call.call_args_list[1].args, ("GET", "/v1/federation/stats"))
        self.assertEqual(api_call.call_args_list[2].args, ("GET", "/v1/federation/capabilities"))
        self.assertEqual(api_call.call_args_list[3].args, ("POST", "/v1/federation/capabilities", {"agent_id": "agent-a"}))
        self.assertEqual(api_call.call_args_list[4].args, ("POST", "/v1/federation/discover", {"feature": "browser"}))
        self.assertEqual(api_call.call_args_list[5].args, ("POST", "/v1/federation/delegate", {"peer_id": "p1"}))
        self.assertEqual(api_call.call_args_list[6].args, ("GET", "/v1/federation/bridge/stats"))
        self.assertEqual(api_call.call_args_list[7].args, ("POST", "/v1/federation/broadcast", {}))

    def test_agent_kit_exposes_federation(self) -> None:
        self.assertIs(yunque.create_agent_kit().federation, yunque.federation)


if __name__ == "__main__":
    unittest.main()
