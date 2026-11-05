# ADR-20261105: Shopping assistant agent and retrieval boundary

- Status: Partially implemented.
- Context: Product discovery and visual similarity need grounding in the store catalog; LLM output cannot own prices, inventory, coupons or checkout.
- Decision: Use a LangGraph intent router with Discovery, Recommendation, Cart-advice and Multimodal paths, sharing the existing recommender service and hybrid candidate executor. Use catalog product IDs and thumbnail URLs in responses. Gemini `gemini-embedding-2` uses a new 768-dimensional collection; legacy hash vectors remain a separate local/test fallback. Image upload bytes are checked and not persisted in browser history.
- Trust boundary: Retrieval filters published, moderated, in-stock records. LLM may explain only retrieved IDs. Cart path can recommend and direct the user to explicit cart controls; it cannot mutate cart or execute checkout. Backend remains authoritative.
- Consequences: This is not a complete agentic commerce workflow. Server-owned session history/checkpointing, action proposal API and confirmation mutation, Tavily grounding, Jev reranking, LangSmith redacted tracing, live-provider usage accounting, and the production Milvus migration/evaluation gate remain unimplemented.
- Rollback: Set `GEMINI_API_KEY` empty and use a fresh local hash collection; never reuse the old 512-dimensional collection for Gemini vectors.
