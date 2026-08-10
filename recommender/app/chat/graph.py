from __future__ import annotations

from typing import Any, TypedDict

from langgraph.graph import END, START, StateGraph

from .guardrails import GuardrailDecision


class ChatGraphState(TypedDict, total=False):
    request: Any
    authenticated: bool
    decision: GuardrailDecision
    response: Any


def build_chat_graph(service: Any):
    """Build the LangGraph workflow around the LLM+RAG service core.

    The graph is intentionally short: the guardrail only rejects prompt-
    injection attempts. Every other request is routed to the RAG respond
    node, which always invokes the LLM with the hybrid retrieval context.
    """
    def guard(state: ChatGraphState) -> ChatGraphState:
        request = state["request"]
        decision = service._guardrail.inspect_user_input(
            request.text, authenticated=state.get("authenticated", False)
        )
        return {"decision": decision}  # type: ignore[return-value]

    def route(state: ChatGraphState) -> str:
        return "refuse" if state["decision"] is GuardrailDecision.REFUSE else "respond"

    def refuse(state: ChatGraphState) -> ChatGraphState:
        return {"response": service._refusal_response(state["decision"])}

    def respond(state: ChatGraphState) -> ChatGraphState:
        return {
            "response": service._reply_direct(
                state["request"],
                authenticated=state.get("authenticated", False),
                skip_guardrail=True,
            )
        }

    graph = StateGraph(ChatGraphState)
    graph.add_node("guard", guard)
    graph.add_node("respond", respond)
    graph.add_node("refuse", refuse)
    graph.add_edge(START, "guard")
    graph.add_conditional_edges("guard", route, {"respond": "respond", "refuse": "refuse"})
    graph.add_edge("respond", END)
    graph.add_edge("refuse", END)
    return graph.compile()
