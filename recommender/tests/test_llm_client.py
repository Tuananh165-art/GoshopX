import unittest
from unittest.mock import patch

from app.chat.llm_client import LLMClient, LLMConfigurationError


class LLMClientTests(unittest.TestCase):
    def test_requires_api_key_before_network_call(self):
        with patch.dict("os.environ", {"AI_ENDPOINT": "https://example.test", "AI_API_KEY": ""}, clear=False):
            with self.assertRaises(LLMConfigurationError):
                LLMClient.from_environment()

    def test_parses_openai_compatible_message_content(self):
        client = LLMClient(endpoint="https://example.test", api_key="secret", model="test")
        payload = {"choices": [{"message": {"content": '{"ok":true}'}}]}
        with patch.object(client, "_post_json", return_value=payload):
            self.assertEqual(client.complete_json("system", "user"), {"ok": True})

    def test_rejects_non_json_model_output(self):
        client = LLMClient(endpoint="https://example.test", api_key="secret", model="test")
        with patch.object(client, "_post_json", return_value={"choices": [{"message": {"content": "not-json"}}]}):
            with self.assertRaises(ValueError):
                client.complete_json("system", "user")


if __name__ == "__main__":
    unittest.main()
