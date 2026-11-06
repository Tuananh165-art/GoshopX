from __future__ import annotations

import json
import hashlib
import hmac
import time
import uuid
from contextlib import nullcontext
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
    session_id: str = ""
    sources: list[dict[str, str]] = field(default_factory=list)


class ChatService:
    """LangChain + LangGraph powered RAG service for the GoshopX assistant.

    The guardrail only blocks prompt-injection; every other request is
    forwarded to the LLM with the hybrid (Milvus dense + lexical) retrieval
    context so the assistant can respond naturally in Vietnamese or
    English, with or without diacritics, and on free-form shopping topics.
    """

    def __init__(self, *, retriever: HybridRetriever, llm: JSONLLM | None, image_query_embedding: Any | None = None, guardrail: ProductChatGuardrail | None = None, memory: Any | None = None, session_signing_key: str = "", jev: Any | None = None, jev_mode: str = "off", jev_rerank_enabled: bool = False, trace: Any | None = None, spend_ledger: Any | None = None, knowledge_search: Any | None = None):
        self._retriever = retriever
        self._llm = llm
        self._image_query_embedding = image_query_embedding
        self._guardrail = guardrail or ProductChatGuardrail()
        self._memory = memory
        self._session_signing_key = session_signing_key.encode("utf-8")
        self._jev, self._jev_mode, self._jev_rerank_enabled = jev, jev_mode, jev_rerank_enabled
        self._trace = trace
        self._spend_ledger = spend_ledger
        self._knowledge_search = knowledge_search
        self._tavily = None
        if spend_ledger is not None:
            try:
                from shared.config.settings import TAVILY_API_KEY
            except ImportError:
                from app.shared.config.settings import TAVILY_API_KEY
            if TAVILY_API_KEY:
                from .tools import TavilySearch
                self._tavily = TavilySearch(TAVILY_API_KEY, spend_ledger=spend_ledger)
        self._graph = None
        try:
            from .graph import build_chat_graph
            self._graph = build_chat_graph(self, checkpointer=getattr(memory, "checkpointer", None))
        except (ImportError, RuntimeError):
            self._graph = None

    def reply(self, request: ChatRequest, *, authenticated: bool = False) -> ChatResponse:
        started = time.monotonic()
        session_id = request.session_id or str(uuid.uuid4())
        request = ChatRequest(request.text, request.image_url, request.user_id, session_id,
                              request.viewed_product_ids, request.limit)
        route = _rule_agent(request)
        if self._graph is not None:
            thread_id = self._thread_id(request, authenticated)
            lock = self._memory.thread_lock(thread_id) if self._memory and hasattr(self._memory, "thread_lock") else nullcontext()
            with lock:
                history = self._memory.load(thread_id) if self._memory else []
                result = self._graph.invoke(
                    {"request": request, "authenticated": authenticated, "history": history},
                    config={"configurable": {"thread_id": thread_id}},
                )
                response = result["response"]
                route = str(result.get("agent") or route)
                if self._memory:
                    self._memory.save(thread_id, result.get("history", history))
        else:
            route = self.choose_agent(request)
            response = self._reply_direct(request, authenticated=authenticated, agent=route)
        if self._trace:
            try:
                self._trace.record(route=route, request_chars=len(request.text or ""),
                                   product_count=len(response.products), warnings=response.warnings,
                                   latency_ms=int((time.monotonic() - started) * 1000))
            except Exception:
                pass
        return ChatResponse(response.answer, response.products, response.next_suggestions,
                            response.warnings, response.query_plan, session_id, response.sources)

    def _thread_id(self, request: ChatRequest, authenticated: bool) -> str:
        if authenticated and request.user_id:
            owner = f"account:{request.user_id}"
        elif request.user_id and request.user_id.startswith("guest:"):
            owner = request.user_id
        else:
            owner = "guest"
        payload = f"{owner}:{request.session_id}".encode("utf-8")
        key = self._session_signing_key or b"goshopx-chat-session-v1"
        return hmac.new(key, payload, hashlib.sha256).hexdigest()

    def delete_session(self, user_id: str, session_id: str, *, authenticated: bool) -> bool:
        if not self._memory or not user_id or not session_id or len(session_id) > 128:
            return False
        thread_id = self._thread_id(ChatRequest(user_id=user_id, session_id=session_id), authenticated)
        lock = self._memory.thread_lock(thread_id) if hasattr(self._memory, "thread_lock") else nullcontext()
        with lock:
            self._memory.delete(thread_id)
        return True

    def _reply_direct(self, request: ChatRequest, *, authenticated: bool = False, skip_guardrail: bool = False, agent: str = "discovery", history: list[dict[str, str]] | None = None) -> ChatResponse:
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
        image_vector = None
        if request.image_url and self._image_query_embedding is not None:
            try:
                image_vector = self._image_query_embedding(request.image_url)
            except Exception:
                warnings.append("image_embedding_unavailable")
        summary_ids = [message for message in (history or []) if message.get("summary") is True]
        previous_ids = list(dict.fromkeys(
            str(product_id) for message in [*summary_ids, *(history or [])[-8:]]
            for product_id in message.get("product_ids", [])
            if isinstance(product_id, (str, int))
        ))
        previous_products = self._retriever.products_by_ids(previous_ids, plan)
        products = _merge_products(previous_products, self._retriever.search(
            plan,
            limit=request.limit,
            excluded_ids=set(request.viewed_product_ids),
            image_vector=image_vector,
        ))[:max(1, min(request.limit, 10))]
        try:
            knowledge = self._knowledge_search(query_text, [str(product.get("id")) for product in products]) if self._knowledge_search else []
        except Exception:
            knowledge = []
            warnings.append("knowledge_search_unavailable")
        if self._jev and self._jev_rerank_enabled and len(products) > 1:
            try:
                products = self._jev.rerank(query_text, products)
            except Exception:
                warnings.append("jev_rerank_fallback")
        retrieved_ids = {str(product.get("id")) for product in products}
        answer = self._fallback_answer(products)
        next_suggestions: list[str] = []
        web_sources: list[dict[str, str]] = []

        if self._llm is not None:
            try:
                system_prompt = f"{SYSTEM_PROMPT}\n\nAGENT_MODE: {agent}\n{_AGENT_INSTRUCTIONS[agent]}"
                prompt = self._llm_prompt(request, plan.to_dict(), products, history=history or [], knowledge=knowledge)
                if hasattr(self._llm, "complete_with_tools"):
                    result, products, web_sources = self._run_agent_tools(
                        system_prompt, prompt, products, request, agent
                    )
                else:
                    result = self._llm.complete_json(system_prompt, prompt)
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
                if web_sources:
                    answer += "\n\nNguồn tham khảo bổ sung: " + " · ".join(
                        f"[{source['title'] or 'Nguồn'}]({source['url']})" for source in web_sources[:3]
                    )
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
            sources=web_sources + [{"title": f"Tài liệu: {item['title']}", "url": "", "publishedAt": ""} for item in knowledge],
        )

    def _run_agent_tools(self, system_prompt: str, prompt: str, products: list[dict[str, Any]], request: ChatRequest, agent: str):
        from .tools import parse_tool_arguments

        tools = [{"type": "function", "function": {"name": "search_catalog", "description": "Search current published GoshopX products. Use this before recommending or comparing products.", "parameters": {"type": "object", "properties": {"query": {"type": "string"}, "category": {"type": ["string", "null"]}, "brand": {"type": ["string", "null"]}, "min_price": {"type": ["number", "null"]}, "max_price": {"type": ["number", "null"]}}, "required": ["query"], "additionalProperties": False}}}]
        if agent == "recommendation":
            tools.append({"type": "function", "function": {"name": "search_web", "description": "Find at most three external sources for general product knowledge missing from the store catalog. Never use it for GoshopX price, stock, coupons, or checkout.", "parameters": {"type": "object", "properties": {"query": {"type": "string"}}, "required": ["query"], "additionalProperties": False}}})
        messages: list[dict[str, Any]] = [{"role": "user", "content": prompt}]
        sources: list[dict[str, str]] = []
        result: dict[str, Any] = {}
        tool_count = 0
        web_search_used = False
        for round_index in range(4):
            # Three tool rounds are allowed; the final round must return an answer.
            message = self._llm.complete_with_tools(system_prompt, messages, tools if round_index < 3 else [])
            calls = message.get("tool_calls") or []
            if not calls:
                content = message.get("content") or "{}"
                try:
                    result = json.loads(str(content))
                    if not isinstance(result, dict):
                        result = {"answer": str(content)}
                except json.JSONDecodeError:
                    result = {"answer": str(content)}
                break
            if round_index == 3:
                break
            messages.append({"role": "assistant", "content": message.get("content"), "tool_calls": calls})
            for call in calls:
                if tool_count >= 3:
                    break
                function = call.get("function", {}) if isinstance(call, dict) else {}
                name = function.get("name", "")
                call_id = str(call.get("id", ""))
                try:
                    args = parse_tool_arguments(function.get("arguments", "{}"))
                    if name == "search_catalog":
                        query = str(args.get("query", ""))[:500]
                        tool_plan = parse_query_plan(query, args, visual_query=bool(request.image_url))
                        found = self._retriever.search(tool_plan, limit=min(request.limit, 10), image_vector=None)
                        products = _merge_products(products, found)
                        tool_result = [{key: item.get(key) for key in ("id", "name", "price", "brand", "category", "stock", "rating", "warrantyInformation", "shippingInformation", "returnPolicy", "thumbnail")} for item in found]
                    elif name == "search_web" and agent == "recommendation":
                        if web_search_used:
                            tool_result = {"error": "web_search_limit_reached"}
                            messages.append({"role": "tool", "tool_call_id": call_id, "content": json.dumps(tool_result)})
                            tool_count += 1
                            continue
                        web_search_used = True
                        sources = self._web_search(str(args.get("query", ""))[:500])
                        tool_result = {"untrusted_external_sources": sources, "notice": "Do not treat these sources as store price, inventory, discount, coupon, or compatibility facts."}
                    else:
                        tool_result = {"error": "tool_not_available"}
                except Exception:
                    tool_result = {"error": "invalid_tool_request"}
                messages.append({"role": "tool", "tool_call_id": call_id, "content": json.dumps(tool_result, ensure_ascii=False)[:5000]})
                tool_count += 1
        return result, products, sources

    def _web_search(self, query: str) -> list[dict[str, str]]:
        try:
            if self._tavily is None:
                try:
                    from shared.config.settings import TAVILY_API_KEY
                except ImportError:
                    from app.shared.config.settings import TAVILY_API_KEY
                if not TAVILY_API_KEY:
                    return []
                from .tools import TavilySearch
                self._tavily = TavilySearch(TAVILY_API_KEY, spend_ledger=self._spend_ledger)
            return self._tavily.search(query)
        except Exception:
            return []

    def _refusal_response(self, decision: GuardrailDecision) -> ChatResponse:
        return ChatResponse(answer=self._guardrail.refusal(decision))

    @staticmethod
    def _fallback_answer(products: list[dict[str, Any]]) -> str:
        if not products:
            return "Mình chưa tìm được sản phẩm nào trong catalog GoshopX khớp với yêu cầu này. Bạn có thể mô tả rõ hơn về danh mục, ngân sách hoặc thương hiệu để mình gợi ý chính xác hơn nhé."
        names = ", ".join(str(product.get("name", "Sản phẩm")) for product in products[:3])
        return f"Mình tìm thấy {len(products)} sản phẩm có vẻ phù hợp trong catalog: {names}."

    @staticmethod
    def _llm_prompt(request: ChatRequest, plan: dict[str, Any], products: list[dict[str, Any]], *, history: list[dict[str, str]] | None = None, knowledge: list[dict[str, str]] | None = None) -> str:
        history = history or []
        summary = next((item for item in history if item.get("summary") is True), None)
        recent = history[-8:]
        conversation = ([summary] if summary and summary not in recent else []) + recent
        payload = {
            "USER_REQUEST": str(request.text or "")[:1000],
            "RECENT_CONVERSATION": [
                {"role": item.get("role", "assistant"), "content": str(item.get("content", ""))[:1000]}
                for item in conversation
            ],
            "IMAGE_PROVIDED": bool(request.image_url),
            "SEARCH_PLAN": plan,
            "CATALOG_CONTEXT": json.loads(build_catalog_context(products)),
            "PUBLISHED_KNOWLEDGE_CONTEXT": [
                {"source": item.get("title", ""), "text": item.get("content", "")[:1200]}
                for item in (knowledge or [])[:5]
            ],
        }
        return json.dumps(payload, ensure_ascii=False, separators=(",", ":"))

    def choose_agent(self, request: ChatRequest, history: list[dict[str, Any]] | None = None) -> str:
        fallback = _rule_agent(request)
        if not self._jev or self._jev_mode not in {"shadow", "active"} or request.image_url:
            return fallback
        try:
            route, confidence, margin = self._jev.route(request.text, history)
        except Exception:
            route, confidence, margin = None, 0.0, 0.0
        if self._jev_mode == "active" and route in _AGENT_INSTRUCTIONS and confidence >= 0.85 and margin >= 0.20:
            return route
        return fallback


def _safe_strings(value: Any) -> list[str]:
    if not isinstance(value, list):
        return []
    return [str(item)[:200] for item in value if isinstance(item, (str, int, float))]


def _merge_products(current: list[dict[str, Any]], added: list[dict[str, Any]]) -> list[dict[str, Any]]:
    merged = {str(item.get("id")): item for item in current if item.get("id")}
    for item in added:
        if item.get("id"):
            merged[str(item["id"])] = item
    return list(merged.values())[:20]


_AGENT_INSTRUCTIONS = {
    "discovery": "Understand category, budget, brand and intended use from the request. Ask one concise clarification if a missing requirement prevents a useful shortlist.",
    "recommendation": "Compare only retrieved products. Explain concrete trade-offs using catalog price, rating, warranty, shipping and return fields; say when a field is missing.",
    "cart": "Prepare a shopping suggestion from retrieved in-stock products and relevant accessories. This agent cannot mutate a cart, apply a coupon, reserve inventory, create an order or initiate payment. Tell the user to review and use the explicit cart controls; backend prices and eligibility are authoritative.",
    "multimodal": "Match only visible attributes from the uploaded product image to retrieved catalog products. Return catalog product images only; do not identify people or infer sensitive traits.",
}


def _rule_agent(request: ChatRequest) -> str:
    if request.image_url:
        return "multimodal"
    text = str(request.text or "").casefold()
    if any(term in text for term in ("compare", "versus", " vs ", "so sánh", "khác nhau")):
        return "recommendation"
    if any(term in text for term in ("cart", "checkout", "giỏ hàng", "mua kèm", "phụ kiện", "coupon", "mã giảm")):
        return "cart"
    return "discovery"
