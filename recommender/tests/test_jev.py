import unittest

from app.chat.jev import JevClient, _safe_router_state
from app.chat.service import ChatRequest, ChatService
from app.retrieval.hybrid import Candidate, HybridRetriever


class JevTests(unittest.TestCase):
    def test_router_state_redacts_identifiers_and_only_keeps_recent_turns(self):
        value = _safe_router_state("Call me at 0912345678 me@example.com", [{"role": "user", "content": "old"}])
        self.assertNotIn("0912345678", value)
        self.assertNotIn("me@example.com", value)
        self.assertIn("[phone]", value)
        self.assertIn("[email]", value)

    def test_route_falls_back_when_confidence_or_margin_is_low(self):
        products = {"a": {"id": "a", "name": "Laptop", "price": 10, "stock": 1, "publishStatus": "published", "moderationStatus": "approved"}}
        retriever = HybridRetriever(dense_search=lambda _: [Candidate("a", 1)], lexical_search=lambda _: [], catalog=products)
        class FakeJev:
            def route(self, *_):
                return "cart", .6, .1
        service = ChatService(retriever=retriever, llm=None, jev=FakeJev(), jev_mode="active")
        self.assertEqual(service.choose_agent(ChatRequest(text="Find a laptop")), "discovery")

    def test_active_route_accepts_only_known_high_confidence_route(self):
        products = {"a": {"id": "a", "name": "Laptop", "price": 10, "stock": 1, "publishStatus": "published", "moderationStatus": "approved"}}
        retriever = HybridRetriever(dense_search=lambda _: [], lexical_search=lambda _: [], catalog=products)
        class FakeJev:
            def route(self, *_):
                return "recommendation", .91, .4
        service = ChatService(retriever=retriever, llm=None, jev=FakeJev(), jev_mode="active")
        self.assertEqual(service.choose_agent(ChatRequest(text="pick one")), "recommendation")

    def test_bad_rerank_result_preserves_fused_order(self):
        client = JevClient("fake")
        client.evaluate = lambda *_: {"answers": {"p0": {"score": 3}}}
        products = [{"id": "a", "retrievalScore": .8}, {"id": "b", "retrievalScore": .7}]
        self.assertEqual(client.rerank("laptop", products), products)


if __name__ == "__main__":
    unittest.main()
