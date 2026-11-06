from __future__ import annotations

import json
import logging
import os
import re
import urllib.error
import urllib.request
from dataclasses import dataclass
from typing import Any

logger = logging.getLogger(__name__)


class LLMConfigurationError(RuntimeError):
    pass


class LLMRequestError(RuntimeError):
    pass


@dataclass
class LLMClient:
    endpoint: str
    api_key: str
    model: str
    timeout_seconds: float = 30.0
    max_retries: int = 2
    spend_ledger: Any | None = None

    @classmethod
    def from_environment(cls) -> "LLMClient":
        endpoint = normalize_chat_completions_endpoint(os.getenv("DEEPSEEK_BASE_URL", "https://api.deepseek.com"))
        api_key = os.getenv("DEEPSEEK_API_KEY", "").strip()
        model = os.getenv("DEEPSEEK_MODEL", "deepseek-flash").strip()
        if not endpoint or not model:
            raise LLMConfigurationError("AI_ENDPOINT and AI_MODEL are required")
        if not api_key:
            raise LLMConfigurationError("AI_API_KEY is required at runtime")
        return cls(
            endpoint=endpoint,
            api_key=api_key,
            model=model,
            timeout_seconds=_float_env("AI_TIMEOUT_SECONDS", 30.0),
            max_retries=max(0, min(int(_float_env("AI_MAX_RETRIES", 2)), 5)),
        )

    def complete_json(self, system_prompt: str, user_prompt: str) -> dict[str, Any]:
        payload = self._post_json(
            {
                "model": self.model,
                "messages": [
                    {"role": "system", "content": system_prompt},
                    {"role": "user", "content": user_prompt},
                ],
                "temperature": 0,
                "max_tokens": 4000,
                "response_format": {"type": "json_object"},
            }
        )
        try:
            content = payload["choices"][0]["message"]["content"]
            if isinstance(content, list):
                content = "".join(str(part.get("text", "")) for part in content if isinstance(part, dict))
            content = _strip_code_fence(str(content))
            value = json.loads(content)
        except (KeyError, IndexError, TypeError, json.JSONDecodeError) as exc:
            raise ValueError("LLM returned invalid JSON response") from exc
        if not isinstance(value, dict):
            raise ValueError("LLM JSON response must be an object")
        return value

    def complete_with_tools(self, system_prompt: str, messages: list[dict[str, Any]], tools: list[dict[str, Any]]) -> dict[str, Any]:
        request = {
            "model": self.model,
            "messages": [{"role": "system", "content": system_prompt}, *messages],
            "temperature": 0,
            "max_tokens": 1800,
        }
        if tools:
            request.update(tools=tools, tool_choice="auto")
        else:
            request["tool_choice"] = "none"
        payload = self._post_json(request)
        try:
            message = payload["choices"][0]["message"]
        except (KeyError, IndexError, TypeError) as exc:
            raise ValueError("LLM returned an invalid tool-call response") from exc
        if not isinstance(message, dict):
            raise ValueError("LLM tool-call response must be an object")
        return message

    def describe_image(self, image_url: str) -> str:
        if image_url.startswith("gs://"):
            import base64
            from vector.embeddings import load_image_reference
            mime_type, image_data = load_image_reference(image_url)
            image_url = f"data:{mime_type};base64," + base64.b64encode(image_data).decode("ascii")
        payload = self._post_json(
            {
                "model": self.model,
                "messages": [{
                    "role": "user",
                    "content": [
                        {"type": "text", "text": "Describe only visible product attributes for catalog search: category, brand, color, material, style, shape and use case. Return one concise Vietnamese sentence. Do not identify people or infer private data."},
                        {"type": "image_url", "image_url": {"url": image_url}},
                    ],
                }],
                "temperature": 0,
                "max_tokens": 160,
            }
        )
        try:
            content = payload["choices"][0]["message"]["content"]
            if isinstance(content, list):
                content = "".join(str(part.get("text", "")) for part in content if isinstance(part, dict))
            return str(content).strip()
        except (KeyError, IndexError, TypeError) as exc:
            raise ValueError("LLM returned invalid image description") from exc

    def _post_json(self, payload: dict[str, Any]) -> dict[str, Any]:
        endpoint = self.endpoint if self.endpoint.rstrip("/").endswith("/messages") else normalize_chat_completions_endpoint(self.endpoint)
        is_anthropic = endpoint.rstrip("/").endswith("/messages")
        request_payload = dict(payload)
        headers = {
            "Content-Type": "application/json",
            "Accept": "application/json",
            "User-Agent": "GoshopX-Recommender/1.0",
        }
        if is_anthropic:
            messages = request_payload.pop("messages", [])
            system_messages = [item.get("content", "") for item in messages if item.get("role") == "system"]
            request_payload["messages"] = [item for item in messages if item.get("role") != "system"]
            if system_messages:
                request_payload["system"] = "\n".join(str(item) for item in system_messages)
            request_payload.pop("response_format", None)
            headers["x-api-key"] = self.api_key
            headers["anthropic-version"] = "2023-06-01"
        else:
            headers["Authorization"] = f"Bearer {self.api_key}"
        body = json.dumps(request_payload).encode("utf-8")
        request = urllib.request.Request(
            endpoint,
            data=body,
            headers=headers,
            method="POST",
        )
        last_error: Exception | None = None
        for attempt in range(self.max_retries + 1):
            reservation_id = None
            if self.spend_ledger is not None:
                # A timeout is charged conservatively: the provider may have processed the request.
                reservation_id = self.spend_ledger.reserve_call("deepseek")
            try:
                with urllib.request.urlopen(request, timeout=self.timeout_seconds) as response:
                    status = getattr(response, "status", 200)
                    response_body = response.read()
                if status < 200 or status >= 300:
                    raise LLMRequestError(f"LLM returned HTTP {status}")
                value = json.loads(response_body.decode("utf-8"))
                if not isinstance(value, dict):
                    raise LLMRequestError("LLM response must be a JSON object")
                if is_anthropic and "content" in value and "choices" not in value:
                    text = "".join(str(part.get("text", "")) for part in value["content"] if isinstance(part, dict))
                    usage = value.get("usage", {})
                    value = {"choices": [{"message": {"content": text}}],
                             "usage": {"prompt_tokens": usage.get("input_tokens", 0),
                                       "completion_tokens": usage.get("output_tokens", 0)}}
                usage = value.get("usage", {})
                if reservation_id and isinstance(usage, dict):
                    input_tokens = usage.get("prompt_tokens", usage.get("input_tokens"))
                    output_tokens = usage.get("completion_tokens", usage.get("output_tokens", 0))
                    if isinstance(input_tokens, int) and isinstance(output_tokens, int):
                        self.spend_ledger.settle_tokens(reservation_id, input_tokens=input_tokens, output_tokens=output_tokens)
                return value
            except urllib.error.HTTPError as exc:
                last_error = exc
                logger.warning("LLM provider rejected request: status=%s model=%s attempt=%s/%s", exc.code, self.model, attempt + 1, self.max_retries + 1)
                if reservation_id and 400 <= exc.code < 500:
                    self.spend_ledger.release_call(reservation_id)
                if exc.code == 429 or exc.code >= 500:
                    if attempt < self.max_retries:
                        continue
                else:
                    break
            except (urllib.error.URLError, TimeoutError, json.JSONDecodeError, LLMRequestError) as exc:
                last_error = exc
                logger.warning("LLM request failed: kind=%s model=%s attempt=%s/%s", type(exc).__name__, self.model, attempt + 1, self.max_retries + 1)
                if attempt < self.max_retries:
                    continue
        raise LLMRequestError("LLM request failed after retries") from last_error


def _strip_code_fence(content: str) -> str:
    content = content.strip()
    match = re.fullmatch(r"```(?:json)?\s*(.*?)\s*```", content, flags=re.IGNORECASE | re.DOTALL)
    return match.group(1).strip() if match else content


def _float_env(name: str, default: float) -> float:
    try:
        value = float(os.getenv(name, str(default)))
        return value if value >= 0 else default
    except ValueError:
        return default


def normalize_chat_completions_endpoint(value: str) -> str:
    endpoint = str(value or "").strip().rstrip("/")
    if not endpoint:
        raise LLMConfigurationError("DeepSeek base URL is required")
    return endpoint if endpoint.endswith("/chat/completions") else f"{endpoint}/chat/completions"
