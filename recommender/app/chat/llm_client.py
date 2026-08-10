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

    @classmethod
    def from_environment(cls) -> "LLMClient":
        endpoint = os.getenv("AI_ENDPOINT", "").strip()
        api_key = os.getenv("AI_API_KEY", "").strip()
        model = os.getenv("AI_MODEL", "").strip()
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

    def describe_image(self, image_url: str) -> str:
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
        is_anthropic = self.endpoint.rstrip("/").endswith("/messages")
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
            self.endpoint,
            data=body,
            headers=headers,
            method="POST",
        )
        last_error: Exception | None = None
        for attempt in range(self.max_retries + 1):
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
                    value = {"choices": [{"message": {"content": text}}]}
                return value
            except urllib.error.HTTPError as exc:
                last_error = exc
                logger.warning("LLM provider rejected request: status=%s model=%s attempt=%s/%s", exc.code, self.model, attempt + 1, self.max_retries + 1)
                if attempt < self.max_retries:
                    continue
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
