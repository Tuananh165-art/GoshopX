from __future__ import annotations

from collections.abc import Iterable
import json
import logging
import threading
import time

from chat.llm_client import LLMClient, LLMConfigurationError
from chat.service import ChatService
from retrieval.hybrid import Candidate, HybridRetriever
from shared.config.settings import AI_API_KEY, AI_ENDPOINT, AI_MAX_RETRIES, AI_MODEL, AI_TIMEOUT_SECONDS, MILVUS_COLLECTION, MILVUS_URI, TEXT_EMBEDDING_DIMENSION
from shared.db.models import Product
from shared.db.session import get_session
from vector.indexer import ProductIndexer, build_product_indexer

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
            }
            for product in products
        }

    catalog = read_catalog()
    catalog_lock = threading.Lock()
    last_catalog_refresh = 0.0

    def refresh_catalog() -> None:
        nonlocal last_catalog_refresh
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
            last_catalog_refresh = time.monotonic()

    def search(plan) -> Iterable[Candidate]:
        refresh_catalog()
        tokens = set(getattr(plan, "keywords", ()))
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
            text = " ".join(str(value) for value in product.values()).casefold()
            score = sum(1 for token in tokens if token.casefold() in text)
            if score:
                scored.append(Candidate(product_id, float(score)))
        return sorted(scored, key=lambda candidate: (-candidate.score, candidate.product_id))

    try:
        llm = LLMClient(
            endpoint=AI_ENDPOINT,
            api_key=AI_API_KEY,
            model=AI_MODEL,
            timeout_seconds=AI_TIMEOUT_SECONDS,
            max_retries=AI_MAX_RETRIES,
        ) if AI_API_KEY else None
        if llm is not None:
            logger.info("LLM client configured: endpoint=%s model=%s api_key=present", AI_ENDPOINT, AI_MODEL)
        else:
            logger.error("LLM client disabled: AI_API_KEY is missing")
    except LLMConfigurationError:
        logger.exception("LLM client configuration is invalid")
        llm = None

    indexer: ProductIndexer | None = None
    try:
        indexer = build_product_indexer(
            uri=MILVUS_URI,
            collection=MILVUS_COLLECTION,
            image_collection=f"{MILVUS_COLLECTION}_images",
            dimension=TEXT_EMBEDDING_DIMENSION,
            vision_llm=llm,
        )
    except Exception:
        logger.exception("Milvus unavailable; using lexical fallback")

    def dense_search(plan) -> Iterable[Candidate]:
        fallback = list(search(plan))
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

    retriever = HybridRetriever(dense_search=dense_search, lexical_search=search, catalog=catalog)
    return ChatService(retriever=retriever, llm=llm)
