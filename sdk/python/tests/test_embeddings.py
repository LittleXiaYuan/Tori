import unittest
from unittest.mock import patch

import sdk.python.yunque as yunque


class EmbeddingsSDKTests(unittest.TestCase):
    def test_providers_and_embed_delegate_to_embedding_routes(self):
        with patch.object(yunque, "_api_call", return_value={"ok": True}) as api_call:
            yunque.embeddings.providers()
            yunque.embeddings.embed("hello", "local")

        calls = [call.args for call in api_call.call_args_list]
        self.assertEqual(calls[0], ("GET", "/v1/embeddings"))
        self.assertEqual(calls[1], ("POST", "/v1/embeddings", {"text": "hello", "provider": "local"}))

    def test_agent_kit_exposes_embeddings_namespace(self):
        self.assertIs(yunque.create_agent_kit().embeddings, yunque.embeddings)


if __name__ == "__main__":
    unittest.main()
