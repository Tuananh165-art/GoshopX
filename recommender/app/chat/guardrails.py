from __future__ import annotations

import re
import unicodedata
from dataclasses import dataclass
from enum import Enum
from typing import Any

from .prompts import INJECTION_REFUSAL


class GuardrailDecision(str, Enum):
    ALLOW = "allow"
    REFUSE = "refuse"


@dataclass(frozen=True)
class GuardrailResult:
    decision: GuardrailDecision
    reason: str


class ProductChatGuardrail:
    """Minimal safety gate around the LLM.

    The guardrail only blocks clear prompt-injection attempts or requests
    that try to expose secrets. Anything else (greetings, general shopping
    questions, chitchat, ambiguous Vietnamese/English, free-form product
    search) is forwarded to the LLM with the RAG retrieval context, so the
    assistant can produce a natural, grounded reply.
    """

    _INJECTION_PATTERNS = (
        r"ignore\s+(all\s+)?(previous|prior|above)\s+instructions?",
        r"bo\s+qua\s+(system|developer|moi|tat\s+ca)\s+(prompt|huong\s+dan|chi\s+dan)",
        r"reveal\s+(the\s+)?(system\s+prompt|prompt|api\s*key|secret)",
        r"tiet\s+lo\s+(system\s+prompt|prompt|api\s*key|secret|khoa)",
        r"(system|developer)\s*prompt\s*[:=]",
        r"act\s+as\s+(an?\s+)?unrestricted",
        r"dong\s+vai\s+(mot\s+)?tro\s+ly\s+khong\s+gioi\s+han",
        r"jailbreak|dan\s+mode|do\s+anything\s+now",
        r"(print|show|output|send)\s+.*(token|password|credential|secret)",
        r"what\s+(is|are)\s+(your|the)\s+(system|hidden|internal)\s+(prompt|instructions?)",
    )

    def inspect_user_input(self, text: str, *, authenticated: bool) -> GuardrailDecision:
        del authenticated
        normalized = self._normalize(text)
        if self._contains_injection(normalized):
            return GuardrailDecision.REFUSE
        return GuardrailDecision.ALLOW

    def inspect(self, text: str, *, authenticated: bool) -> GuardrailResult:
        decision = self.inspect_user_input(text, authenticated=authenticated)
        reason = {
            GuardrailDecision.ALLOW: "natural_query",
            GuardrailDecision.REFUSE: "prompt_injection_or_secret_request",
        }[decision]
        return GuardrailResult(decision=decision, reason=reason)

    def refusal(self, decision: GuardrailDecision) -> str:
        if decision is GuardrailDecision.REFUSE:
            return INJECTION_REFUSAL
        raise ValueError("refusal() is only valid for a REFUSE decision")

    @classmethod
    def _normalize(cls, text: str) -> str:
        lowered = str(text or "").strip().lower()
        folded = "".join(
            character for character in unicodedata.normalize("NFD", lowered)
            if unicodedata.category(character) != "Mn"
        ).replace("đ", "d")
        return re.sub(r"\s+", " ", folded)

    @classmethod
    def _contains_injection(cls, normalized: str) -> bool:
        return any(re.search(pattern, normalized, flags=re.IGNORECASE) for pattern in cls._INJECTION_PATTERNS)


def grounded_response(
    *, answer: str, products: list[dict[str, Any]], retrieved_product_ids: set[str]
) -> dict[str, Any]:
    """Return a safe response whose product list is a subset of retrieval results.

    The assistant is free to answer conversational queries (greetings, meta
    questions) without products, so we only fail closed if a claimed product
    cannot be matched to the retrieval context.
    """
    warnings: list[str] = []
    safe_products: list[dict[str, Any]] = []
    for product in products:
        if not isinstance(product, dict):
            warnings.append("grounding")
            continue
        product_id = str(product.get("id", ""))
        if product_id not in retrieved_product_ids:
            if "grounding" not in warnings:
                warnings.append("grounding")
            continue
        safe_products.append(product)

    unsafe_answer = re.search(
        r"\b(hack|phishing|api\s*key|password|secret|credential)\b|(?:bỏ qua|tiết lộ).{0,40}(?:prompt|khóa|secret)",
        answer or "",
        flags=re.IGNORECASE,
    )
    if unsafe_answer:
        answer = INJECTION_REFUSAL
        warnings.append("unsafe_answer_replaced")

    return {
        "answer": answer or "Tôi chưa tìm thấy sản phẩm phù hợp trong catalog GoshopX.",
        "products": safe_products,
        "warnings": warnings,
    }
