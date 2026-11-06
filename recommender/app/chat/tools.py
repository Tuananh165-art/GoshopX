from __future__ import annotations

import json
import time
import urllib.error
import urllib.parse
import urllib.request
from typing import Any


class TavilySearch:
    def __init__(self, api_key: str, *, timeout: float = 6.0, spend_ledger: Any | None = None):
        self._api_key = api_key
        self._timeout = timeout
        self._cache: dict[str, tuple[float, list[dict[str, str]]]] = {}
        self._spend_ledger = spend_ledger

    def search(self, query: str) -> list[dict[str, str]]:
        if not self._api_key:
            return []
        query = " ".join(query.split())[:500]
        if not query:
            return []
        cached = self._cache.get(query)
        if cached and cached[0] > time.monotonic():
            return list(cached[1])
        reservation_id = self._spend_ledger.reserve_call("tavily") if self._spend_ledger is not None else None
        body = json.dumps({"query": query, "max_results": 3,
                           "search_depth": "basic", "topic": "general", "include_answer": False,
                           "include_raw_content": False, "include_images": False}).encode("utf-8")
        request = urllib.request.Request("https://api.tavily.com/search", data=body,
                                         headers={"Authorization": f"Bearer {self._api_key}", "Content-Type": "application/json"}, method="POST")
        try:
            with urllib.request.urlopen(request, timeout=self._timeout) as response:
                payload = json.loads(response.read())
        except urllib.error.HTTPError as exc:
            if reservation_id and 400 <= exc.code < 500:
                self._spend_ledger.release_call(reservation_id)
            return []
        except (urllib.error.URLError, TimeoutError, json.JSONDecodeError):
            return []
        if reservation_id:
            self._spend_ledger.settle_call_cost(reservation_id, rate_env="AI_TAVILY_CALL_VND")
        results = payload.get("results", []) if isinstance(payload, dict) else []
        safe = []
        for item in results[:3]:
            if not isinstance(item, dict):
                continue
            url = str(item.get("url", ""))[:1000]
            parsed = urllib.parse.urlsplit(url)
            if parsed.scheme != "https" or not parsed.hostname or parsed.username or parsed.password:
                continue
            safe.append({"title": str(item.get("title", ""))[:200], "url": url,
                         "content": str(item.get("content", ""))[:1200]})
        self._cache[query] = (time.monotonic() + 300, safe)
        if len(self._cache) > 128:
            self._cache = {key: value for key, value in self._cache.items() if value[0] > time.monotonic()}
        return list(safe)


def parse_tool_arguments(value: Any) -> dict[str, Any]:
    if isinstance(value, dict):
        return value
    if isinstance(value, str):
        parsed = json.loads(value)
        if isinstance(parsed, dict):
            return parsed
    raise ValueError("tool arguments must be a JSON object")
