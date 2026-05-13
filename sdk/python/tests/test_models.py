import unittest
from unittest.mock import patch

import sdk.python.yunque as yunque


class ModelsSDKTests(unittest.TestCase):
    def test_list_add_delete_delegate_to_model_routes(self):
        with patch.object(yunque, "_api_call", return_value={"ok": True}) as api_call:
            yunque.models.list()
            yunque.models.add({"id": "custom", "model_id": "custom-model"})
            yunque.models.delete("custom")

        calls = [call.args for call in api_call.call_args_list]
        self.assertEqual(calls[0], ("GET", "/v1/models"))
        self.assertEqual(calls[1], ("POST", "/v1/models", {"id": "custom", "model_id": "custom-model"}))
        self.assertEqual(calls[2], ("DELETE", "/v1/models?id=custom"))

    def test_agent_kit_exposes_models_namespace(self):
        self.assertIs(yunque.create_agent_kit().models, yunque.models)


if __name__ == "__main__":
    unittest.main()
