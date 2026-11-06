from __future__ import annotations

from collections.abc import Iterable
import json
import logging
import os
import threading
import time

from chat.llm_client import LLMClient, LLMConfigurationError
from chat.service import ChatService
from retrieval.hybrid import Candidate, HybridRetriever
from retrieval.bm25 import BM25Index, tokenize
from shared.config.settings import AI_API_KEY, AI_ENDPOINT, AI_MAX_RETRIES, AI_MODEL, AI_TIMEOUT_SECONDS, CHAT_MEMORY_ENABLED, CHAT_SESSION_SIGNING_KEY, DATABASE_URL, GEMINI_API_KEY, GEMINI_EMBEDDING_MODEL, GEMINI_EMBEDDING_DIMENSION, MILVUS_URI, TEXT_EMBEDDING_DIMENSION, TYPESAFE_API_KEY, TYPESAFE_JEV_ENDPOINT, TYPESAFE_JEV_MODEL, JEV_MODE, JEV_RERANK_ENABLED, LANGSMITH_API_KEY, LANGSMITH_ENDPOINT, LANGSMITH_PROJECT
from shared.db.models import Product
from shared.db.session import ReplicaSession, get_session
from ingestion.repository import IngestionRepository
from vector.indexer import ProductIndexer
from vector.runtime_index import serving_indexer

logger = logging.getLogger(__name__)


def load_chat_service() -> ChatService:
    def read_catalog() -> dict[str, dict]:
        with get_session() as session:
            products = session.query(Product).all()
        return {
            str(product.id): {
                "id": str(product.id),
                "name": product.name,
                "description": product.description,
                "price": product.price,
                "category": product.category,
                "brand": product.brand,
                "tags": json.loads(product.tags_json or "[]"),
                "thumbnail": product.thumbnail,
                "images": json.loads(product.images_json or "[]"),
                "publishStatus": product.publish_status,
                "moderationStatus": product.moderation_status,
                "stock": product.stock,
                "rating": product.rating,
                "discountPercentage": product.discount_percentage,
                "sku": product.sku,
                "warrantyInformation": product.warranty_information,
                "shippingInformation": product.shipping_information,
                "returnPolicy": product.return_policy,
                "availabilityStatus": product.availability_status,
            }
            for product in products
        }

    catalog = read_catalog()
    catalog_lock = threading.Lock()
    last_catalog_refresh = 0.0

    def build_bm25_index() -> BM25Index:
        documents = {
            product_id: tokenize(" ".join(str(product.get(field, "")) for field in
                                           ("name", "description", "brand", "category", "tags", "sku")))
            for product_id, product in catalog.items()
        }
        return BM25Index.build(documents)

    bm25_index = build_bm25_index()

    def refresh_catalog() -> None:
        nonlocal last_catalog_refresh, bm25_index
        # The sync worker may finish after this gRPC server starts. Refreshing
        # the shared dictionary makes the chatbot see seeded/updated products
        # without a server restart, while bounding PostgreSQL reads.
        if time.monotonic() - last_catalog_refresh < 15:
            return
        with catalog_lock:
            if time.monotonic() - last_catalog_refresh < 15:
                return
            catalog.clear()
            catalog.update(read_catalog())
            bm25_index = build_bm25_index()
            last_catalog_refresh = time.monotonic()

    def search(plan) -> Iterable[Candidate]:
        refresh_catalog()
        tokens = set(tokenize(" ".join(getattr(plan, "keywords", ())) + " " + str(getattr(plan, "query", ""))))
        if not tokens:
            return []
        category = str(getattr(plan, "category", "") or "").casefold()
        brand = str(getattr(plan, "brand", "") or "").casefold()
        min_price = getattr(plan, "min_price", None)
        max_price = getattr(plan, "max_price", None)
        scored: list[Candidate] = []
        for product_id, product in catalog.items():
            product_category = str(product.get("category", "")).casefold()
            product_brand = str(product.get("brand", "")).casefold()
            if category and product_category != category:
                # Hard-filter by category when the user pinned one. This
                # stops accessories sharing a brand keyword (e.g. "Apple
                # iPhone Charger") from drowning out real category matches.
                continue
            if brand and brand not in product_brand:
                continue
            try:
                price = float(product.get("price", 0) or 0)
            except (TypeError, ValueError):
                price = 0.0
            if min_price is not None and price < min_price:
                continue
            if max_price is not None and price > max_price:
                continue
            score = bm25_index.score(tokens, product_id) if product_id in bm25_index.documents else 0
            if score:
                scored.append(Candidate(product_id, score))
        return sorted(scored, key=lambda candidate: (-candidate.score, candidate.product_id))

    try:
        from shared.spend import ProviderSpendLedger
        spend_ledger = ProviderSpendLedger()
        llm = LLMClient(
            endpoint=AI_ENDPOINT,
            api_key=AI_API_KEY,
            model=AI_MODEL,
            timeout_seconds=AI_TIMEOUT_SECONDS,
            max_retries=AI_MAX_RETRIES,
            spend_ledger=spend_ledger if AI_API_KEY else None,
        ) if AI_API_KEY else None
        if llm is not None:
            logger.info("LLM client configured: endpoint=%s model=%s api_key=present", AI_ENDPOINT, AI_MODEL)
        else:
            logger.error("LLM client disabled: AI_API_KEY is missing")
    except LLMConfigurationError:
        logger.exception("LLM client configuration is invalid")
        llm = None

    def current_indexer() -> ProductIndexer | None:
        try:
            return serving_indexer(spend_ledger)
        except Exception:
            logger.exception("Active Milvus generation unavailable; using lexical fallback")
            return None

    def dense_search(plan) -> Iterable[Candidate]:
        fallback = list(search(plan))
        indexer = current_indexer()
        if indexer is None:
            return fallback
        try:
            results = indexer.indexes.dense_search(plan.query, limit=30)
            if getattr(plan, "visual_query", False):
                results += indexer.indexes.image_search(plan.query, limit=30)
            return results or fallback
        except Exception:
            logger.exception("Milvus search failed; using lexical fallback")
            return fallback

    def visual_search(vector):
        indexer = current_indexer()
        return indexer.indexes.image_search_by_vector(vector, limit=30) if indexer else []

    def image_query_embedding(data_url):
        indexer = current_indexer()
        if not indexer or not GEMINI_API_KEY:
            raise RuntimeError("Multimodal index is unavailable")
        return indexer.embeddings.embed_image_data_url(data_url)
    retriever = HybridRetriever(dense_search=dense_search, lexical_search=search, catalog=catalog, image_search=visual_search)
    memory = _build_memory() if CHAT_MEMORY_ENABLED else None
    from .jev import JevClient
    from .tracing import RedactedLangSmithTrace
    jev = JevClient(TYPESAFE_API_KEY, endpoint=TYPESAFE_JEV_ENDPOINT, model=TYPESAFE_JEV_MODEL, spend_ledger=spend_ledger) if TYPESAFE_API_KEY else None
    trace = RedactedLangSmithTrace(LANGSMITH_API_KEY, LANGSMITH_ENDPOINT, LANGSMITH_PROJECT) if LANGSMITH_API_KEY else None
    knowledge_search = _build_knowledge_search(spend_ledger)
    return ChatService(
        retriever=retriever,
        llm=llm,
        image_query_embedding=image_query_embedding,
        memory=memory,
        session_signing_key=CHAT_SESSION_SIGNING_KEY,
        spend_ledger=spend_ledger,
        jev=jev,
        jev_mode=JEV_MODE,
        jev_rerank_enabled=JEV_RERANK_ENABLED,
        trace=trace,
        knowledge_search=knowledge_search,
    )




def _build_memory():
    if not DATABASE_URL or not CHAT_SESSION_SIGNING_KEY:
        raise RuntimeError("Persistent chat memory is enabled but database URL or signing key is missing")
    try:
        from chat.memory import PostgresChatMemory
        from langgraph.checkpoint.postgres import PostgresSaver
        from psycopg_pool import ConnectionPool

        # LangGraph checkpoints are needed to resume an interrupted graph; the
        # compact conversation table remains the customer-facing history API.
        connection_url = DATABASE_URL.replace("postgresql+psycopg2://", "postgresql://", 1)
        pool = ConnectionPool(connection_url, min_size=1, max_size=4, open=True)
        checkpointer = PostgresSaver(pool)
        checkpointer.setup()
        logger.info("Persistent chat memory enabled")
        return PostgresChatMemory(checkpointer=checkpointer)
    except Exception as exc:
        logger.exception("Persistent chat memory initialization failed")
        raise RuntimeError("Persistent chat memory initialization failed") from exc


def _build_knowledge_search(spend_ledger):
    repository = IngestionRepository(ReplicaSession)
    if not GEMINI_API_KEY:
        return repository.search_published
    try:
        from vector.embeddings import GeminiEmbedding
        from vector.langchain_milvus import LangChainMilvusIndexes

        collection = os.getenv("AI_KNOWLEDGE_COLLECTION", "goshopx_knowledge_gemini_v1")
        embeddings = GeminiEmbedding(GEMINI_API_KEY, model=GEMINI_EMBEDDING_MODEL,
                                     dimension=GEMINI_EMBEDDING_DIMENSION, spend_ledger=spend_ledger)
        indexes = LangChainMilvusIndexes.connect(uri=MILVUS_URI, text_collection=collection,
                                                 image_collection=None, text_embeddings=embeddings)

        def search(query: str, product_ids: list[str] | None = None):
            if not repository.has_published_sources():
                return []
            docs = indexes.text.similarity_search(query[:1000], k=30)
            source_ids = list(dict.fromkeys(str(doc.metadata.get("source_id", "")) for doc in docs
                                             if doc.metadata.get("source_id")))
            return repository.search_published(query, limit=5, dense_source_ids=source_ids,
                                               product_ids=product_ids or [])

        return search
    except Exception:
        logger.exception("Knowledge Milvus is unavailable; using published-source lexical retrieval")
        return repository.search_published
