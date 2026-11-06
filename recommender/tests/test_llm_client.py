import unittest
from unittest.mock import patch
from io import BytesIO

from app.chat.llm_client import LLMClient, LLMConfigurationError, normalize_chat_completions_endpoint


class LLMClientTests(unittest.TestCase):
    def test_requires_api_key_before_network_call(self):
        with patch.dict("os.environ", {
            "AI_ENDPOINT": "https://example.test", "AI_API_KEY": "",
            "DEEPSEEK_API_KEY": "",
        }, clear=False):
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

    def test_normalizes_base_and_full_endpoint(self):
        self.assertEqual(normalize_chat_completions_endpoint("https://api.deepseek.com/"), "https://api.deepseek.com/chat/completions")
        self.assertEqual(normalize_chat_completions_endpoint("https://api.deepseek.com/chat/completions"), "https://api.deepseek.com/chat/completions")

    def test_deepseek_configuration_does_not_reuse_an_unrelated_ai_key(self):
        with patch.dict("os.environ", {
            "AI_API_KEY": "unrelated-provider-key",
            "DEEPSEEK_API_KEY": "",
            "DEEPSEEK_BASE_URL": "https://api.deepseek.com",
            "DEEPSEEK_MODEL": "deepseek-flash",
        }, clear=False):
            with self.assertRaises(LLMConfigurationError):
                LLMClient.from_environment()

    def test_tool_request_disables_tools_on_final_round(self):
        client = LLMClient(endpoint="https://example.test", api_key="secret", model="test")
        with patch.object(client, "_post_json", return_value={"choices": [{"message": {"content": "ok"}}]}) as post:
            client.complete_with_tools("system", [], [])
        request = post.call_args.args[0]
        self.assertEqual(request["tool_choice"], "none")
        self.assertNotIn("tools", request)

    def test_provider_usage_settles_the_reserved_call(self):
        class Ledger:
            def __init__(self):
                self.settled = []

            def reserve_call(self, provider):
                self.assert_provider = provider
                return "reservation-1"

            def settle_tokens(self, entry_id, *, input_tokens, output_tokens):
                self.settled.append((entry_id, input_tokens, output_tokens))

            def release_call(self, entry_id):
                raise AssertionError("successful provider call must not be released")

        class Response:
            status = 200

            def __enter__(self):
                return self

            def __exit__(self, *_):
                return False

            def read(self):
                return BytesIO(b'{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":12,"completion_tokens":7}}').read()

        ledger = Ledger()
        client = LLMClient(endpoint="https://example.test", api_key="secret", model="test", spend_ledger=ledger)
        with patch("app.chat.llm_client.urllib.request.urlopen", return_value=Response()):
            self.assertEqual(client._post_json({"messages": []})["choices"][0]["message"]["content"], "ok")
        self.assertEqual(ledger.assert_provider, "deepseek")
        self.assertEqual(ledger.settled, [("reservation-1", 12, 7)])


if __name__ == "__main__":
    unittest.main()
