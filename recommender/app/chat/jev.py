"""Small, fail-closed adapter for TypeSafe System One."""

from __future__ import annotations

import json
import re
import urllib.error
import urllib.request
from typing import Any


class JevClient:
    def __init__(self, api_key: str, *, endpoint: str = "https://api.typesafe.ai/v1/systemone", model: str = "jev-1.14.0", timeout: float = 3.0, spend_ledger: Any | None = None):
        self.api_key, self.endpoint, self.model, self.timeout = api_key, endpoint, model, timeout
        self.spend_ledger = spend_ledger

    def evaluate(self, state: str, questions: dict[str, Any]) -> dict[str, Any]:
        if not self.api_key or not questions:
            return {}
        reservation_id = self.spend_ledger.reserve_call("typesafe") if self.spend_ledger is not None else None
        body = json.dumps({"model": self.model, "state": state[:6000], "questions": questions}, separators=(",", ":")).encode()
        request = urllib.request.Request(self.endpoint, body, {"Authorization": f"Bearer {self.api_key}", "Content-Type": "application/json"}, method="POST")
        try:
            with urllib.request.urlopen(request, timeout=self.timeout) as response:
                payload = json.loads(response.read(64 * 1024))
            if reservation_id:
                self.spend_ledger.settle_call_cost(reservation_id, rate_env="AI_TYPESAFE_CALL_VND")
            return payload if isinstance(payload, dict) else {}
        except urllib.error.HTTPError as exc:
            if reservation_id and 400 <= exc.code < 500:
                self.spend_ledger.release_call(reservation_id)
            return {}
        except (urllib.error.URLError, TimeoutError, ValueError):
            return {}

    def route(self, text: str, history: list[dict[str, Any]] | None = None) -> tuple[str | None, float, float]:
        safe = _safe_router_state(text, history or [])
        result = self.evaluate(safe, {"route": {"type": "choice", "instructions": "Choose the single best GoshopX shopping assistant route for the latest user request.", "criteria": {
            "discovery": "Find products from store catalog, understand needs, ask clarifying questions",
            "recommendation": "Compare or explain tradeoffs between products using evidence",
            "cart": "Suggest cart additions, accessories, coupon or checkout preview; never execute purchase",
            "multimodal": "Find catalog products similar to an uploaded product image",
        }}})
        answer = result.get("answers", {}).get("route", {}) if isinstance(result.get("answers"), dict) else {}
        probs = answer.get("probabilities", {}) if isinstance(answer, dict) else {}
        if not isinstance(probs, dict):
            return None, 0.0, 0.0
        ranked = sorted(((str(key), float(value)) for key, value in probs.items() if _prob(value)), key=lambda pair: pair[1], reverse=True)
        if not ranked:
            return None, 0.0, 0.0
        gap = ranked[0][1] - (ranked[1][1] if len(ranked) > 1 else 0.0)
        return ranked[0][0], ranked[0][1], gap

    def rerank(self, query: str, products: list[dict[str, Any]]) -> list[dict[str, Any]]:
        shortlist = products[:10]
        if len(shortlist) < 2:
            return shortlist
        state = json.dumps({"query": query[:600], "candidates": [
            {"id": str(p.get("id", "")), "name": str(p.get("name", ""))[:120], "brand": str(p.get("brand", ""))[:80], "category": str(p.get("category", ""))[:80], "description": str(p.get("description", ""))[:240], "price": p.get("price")}
            for p in shortlist
        ]}, ensure_ascii=False, separators=(",", ":"))
        questions = {f"p{i}": {"type": "score", "instructions": f"Relevance of candidate {p.get('id')} to the user query. Score semantic match only; do not infer missing facts.", "criteria": ["unrelated", "weak", "partial", "good", "excellent"]} for i, p in enumerate(shortlist)}
        result = self.evaluate(state, questions)
        answers = result.get("answers", {}) if isinstance(result.get("answers"), dict) else {}
        scores = {str(p.get("id")): float(answers.get(f"p{i}", {}).get("score", -1)) for i, p in enumerate(shortlist)
                  if isinstance(answers.get(f"p{i}"), dict) and _prob(answers.get(f"p{i}", {}).get("score"))}
        if len(scores) != len(shortlist):
            return shortlist
        # Preserve deterministic tie ordering and all application-side eligibility filters.
        return sorted(shortlist, key=lambda p: (-scores[str(p["id"])], -float(p.get("retrievalScore", 0) or 0), str(p["id"])))


def _prob(value: Any) -> bool:
    try:
        return 0.0 <= float(value) <= 5.0
    except (TypeError, ValueError):
        return False


def _safe_router_state(text: str, history: list[dict[str, Any]]) -> str:
    def redact(value: str) -> str:
        value = re.sub(r"[\w.+-]+@[\w.-]+\.[A-Za-z]{2,}", "[email]", value)
        value = re.sub(r"(?<!\w)(?:\+?\d[\d ()-]{7,}\d)(?!\w)", "[phone]", value)
        value = re.sub(r"(?i)(api[_ -]?key|password|token)\s*[:=]\s*\S+", r"\1=[redacted]", value)
        return value[:1000]
    summary = [item for item in history if item.get("summary") is True]
    recent = [f"{item.get('role', 'assistant')}: {redact(str(item.get('content', '')))}" for item in [*summary, *history[-6:]]]
    return "\n".join([*recent, f"user: {redact(text)}"])[-6000:]
