from __future__ import annotations

import json
from dataclasses import dataclass, field
from typing import Any, Protocol

try:
    from retrieval.hybrid import HybridRetriever
except ImportError:  # package import used by unit tests
    from app.retrieval.hybrid import HybridRetriever

from .guardrails import GuardrailDecision, ProductChatGuardrail, grounded_response
from .prompts import INJECTION_REFUSAL, SYSTEM_PROMPT
from .query_planner import build_catalog_context, parse_query_plan


class JSONLLM(Protocol):
    def complete_json(self, system_prompt: str, user_prompt: str) -> dict[str, Any]: ...


@dataclass(frozen=True)
class ChatRequest:
    text: str = ""
    image_url: str | None = None
    user_id: str | None = None
    session_id: str | None = None
    viewed_product_ids: tuple[str, ...] = ()
    limit: int = 5


@dataclass(frozen=True)
class ChatResponse:
    answer: str
    products: list[dict[str, Any]] = field(default_factory=list)
    next_suggestions: list[str] = field(default_factory=list)
    warnings: list[str] = field(default_factory=list)
    query_plan: dict[str, Any] = field(default_factory=dict)


class ChatService:
    """LangChain + LangGraph powered RAG service for the GoshopX assistant.

    The guardrail only blocks prompt-injection; every other request is
    forwarded to the LLM with the hybrid (Milvus dense + lexical) retrieval
    context so the assistant can respond naturally in Vietnamese or
    English, with or without diacritics, and on free-form shopping topics.
    """

    def __init__(self, *, retriever: HybridRetriever, llm: JSONLLM | None, guardrail: ProductChatGuardrail | None = None):
        self._retriever = retriever
        self._llm = llm
        self._guardrail = guardrail or ProductChatGuardrail()
        self._graph = None
        try:
            from .graph import build_chat_graph
            self._graph = build_chat_graph(self)
        except (ImportError, RuntimeError):
            self._graph = None

    def reply(self, request: ChatRequest, *, authenticated: bool = False) -> ChatResponse:
        if self._graph is not None:
            return self._graph.invoke({"request": request, "authenticated": authenticated})["response"]
        return self._reply_direct(request, authenticated=authenticated)

    def _reply_direct(self, request: ChatRequest, *, authenticated: bool = False, skip_guardrail: bool = False) -> ChatResponse:
        warnings: list[str] = []
        if not skip_guardrail:
            decision = self._guardrail.inspect_user_input(request.text, authenticated=authenticated)
            if decision is GuardrailDecision.REFUSE:
                return self._refusal_response(decision)

        query_text = request.text
        if request.image_url and self._llm is not None and hasattr(self._llm, "describe_image"):
            try:
                query_text = f"{query_text} {self._llm.describe_image(request.image_url)}".strip()
            except Exception:
                query_text = request.text
        plan = parse_query_plan(query_text, visual_query=bool(request.image_url))
        products = self._retriever.search(
            plan,
            limit=request.limit,
            excluded_ids=set(request.viewed_product_ids),
        )
        retrieved_ids = {str(product.get("id")) for product in products}
        answer = self._fallback_answer(products)
        next_suggestions: list[str] = []

        if self._llm is not None:
            try:
                result = self._llm.complete_json(
                    SYSTEM_PROMPT,
                    self._llm_prompt(request, plan.to_dict(), products),
                )
                selected_ids = {str(value) for value in result.get("product_ids", []) if value is not None}
                selected = [product for product in products if str(product.get("id")) in selected_ids]
                grounded = grounded_response(
                    answer=str(result.get("answer", answer)),
                    products=selected,
                    retrieved_product_ids=retrieved_ids,
                )
                answer = grounded["answer"]
                products = grounded["products"]
                warnings.extend(grounded["warnings"])
                next_suggestions = _safe_strings(result.get("next_suggestions"))[:3]
            except Exception:
                warnings.append("llm_fallback")
                answer = self._fallback_answer(products)
        else:
            warnings.append("llm_disabled")

        if not products and "no_results" not in warnings and "no_grounded_products" not in warnings:
            warnings.append("no_results")
        return ChatResponse(
            answer=answer,
            products=products,
            next_suggestions=next_suggestions,
            warnings=list(dict.fromkeys(warnings)),
            query_plan=plan.to_dict(),
        )

    def _refusal_response(self, decision: GuardrailDecision) -> ChatResponse:
        return ChatResponse(answer=self._guardrail.refusal(decision))

    @staticmethod
    def _fallback_answer(products: list[dict[str, Any]]) -> str:
        if not products:
            return "Mình chưa tìm được sản phẩm nào trong catalog GoshopX khớp với yêu cầu này. Bạn có thể mô tả rõ hơn về danh mục, ngân sách hoặc thương hiệu để mình gợi ý chính xác hơn nhé."
        names = ", ".join(str(product.get("name", "Sản phẩm")) for product in products[:3])
        return f"Mình tìm thấy {len(products)} sản phẩm có vẻ phù hợp trong catalog: {names}."

    @staticmethod
    def _llm_prompt(request: ChatRequest, plan: dict[str, Any], products: list[dict[str, Any]]) -> str:
        payload = {
            "USER_REQUEST": str(request.text or "")[:1000],
            "IMAGE_PROVIDED": bool(request.image_url),
            "SEARCH_PLAN": plan,
            "CATALOG_CONTEXT": json.loads(build_catalog_context(products)),
        }
        return json.dumps(payload, ensure_ascii=False, separators=(",", ":"))


def _safe_strings(value: Any) -> list[str]:
    if not isinstance(value, list):
        return []
    return [str(item)[:200] for item in value if isinstance(item, (str, int, float))]
