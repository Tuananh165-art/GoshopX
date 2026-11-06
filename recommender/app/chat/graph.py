from __future__ import annotations

from typing import Any, TypedDict

from langgraph.graph import END, START, StateGraph

from .guardrails import GuardrailDecision


class ChatGraphState(TypedDict, total=False):
    request: Any
    authenticated: bool
    decision: GuardrailDecision
    response: Any
    agent: str
    history: list[dict[str, Any]]


def build_chat_graph(service: Any, checkpointer: Any | None = None):
    """Build the LangGraph workflow around the LLM+RAG service core.

    Four specialist routes share one bounded, catalog-grounded RAG executor.
    Cart intent is advisory only; commerce mutations remain explicit backend calls.
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

    def choose_agent(state: ChatGraphState) -> str:
        return service.choose_agent(state["request"], state.get("history", []))

    def selected_agent(state: ChatGraphState) -> str:
        return state["agent"]

    def respond(state: ChatGraphState, agent: str) -> ChatGraphState:
        response = service._reply_direct(
            state["request"],
            authenticated=state.get("authenticated", False),
            skip_guardrail=True,
            agent=agent,
            history=state.get("history", []),
        )
        history = [*state.get("history", []),
                   {"role": "user", "content": str(state["request"].text or "[product image]")[:1000]},
                   {"role": "assistant", "content": response.answer[:2000],
                    "product_ids": [str(product.get("id")) for product in response.products if product.get("id")] }]
        return {"response": response, "history": history[-16:], "agent": agent}

    graph = StateGraph(ChatGraphState)
    graph.add_node("guard", guard)
    graph.add_node("route_agent", lambda state: {"agent": choose_agent(state)})
    for agent_name in ("discovery", "recommendation", "cart", "multimodal"):
        graph.add_node(agent_name, lambda state, agent_name=agent_name: respond(state, agent_name))
    graph.add_node("refuse", refuse)
    graph.add_edge(START, "guard")
    graph.add_conditional_edges("guard", route, {"respond": "route_agent", "refuse": "refuse"})
    graph.add_conditional_edges("route_agent", selected_agent, {name: name for name in ("discovery", "recommendation", "cart", "multimodal")})
    for agent_name in ("discovery", "recommendation", "cart", "multimodal"):
        graph.add_edge(agent_name, END)
    graph.add_edge("refuse", END)
    return graph.compile(checkpointer=checkpointer)
