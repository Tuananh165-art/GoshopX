from __future__ import annotations

import threading

from shared.config.settings import (AI_API_KEY, AI_ENDPOINT, AI_MODEL, GEMINI_API_KEY,
                                    GEMINI_EMBEDDING_MODEL, MILVUS_COLLECTION, MILVUS_URI,
                                    TEXT_EMBEDDING_DIMENSION, AI_TIMEOUT_SECONDS, AI_MAX_RETRIES)
from shared.spend import ProviderSpendLedger
from vector.generations import active_generation, write_generations
from vector.indexer import ProductIndexer, build_product_indexer
from chat.llm_client import LLMClient

_lock = threading.Lock()
_indexes: dict[str, ProductIndexer] = {}


def serving_indexer(ledger: ProviderSpendLedger) -> ProductIndexer | None:
    generation = active_generation()
    return _get_indexer(generation, ledger)


def write_indexers(ledger: ProviderSpendLedger) -> list[ProductIndexer]:
    generations = write_generations()
    configured = None if any(generation.status == "active" for generation in generations) else _get_indexer(None, ledger)
    result = [configured] if configured else []
    result.extend(indexer for indexer in (_get_indexer(generation, ledger) for generation in generations) if indexer)
    return list({id(indexer): indexer for indexer in result}.values())


def _get_indexer(generation, ledger: ProviderSpendLedger) -> ProductIndexer | None:
    collection = generation.text_collection if generation else MILVUS_COLLECTION
    image_collection = generation.image_collection if generation else f"{MILVUS_COLLECTION}_images"
    key = f"{collection}|{image_collection}"
    if not GEMINI_API_KEY and collection.startswith("product_catalog_gemini_"):
        return None
    if key not in _indexes:
        with _lock:
            if key not in _indexes:
                llm = LLMClient(AI_ENDPOINT, AI_API_KEY, AI_MODEL, timeout_seconds=AI_TIMEOUT_SECONDS,
                                max_retries=AI_MAX_RETRIES, spend_ledger=ledger) if AI_API_KEY else None
                _indexes[key] = build_product_indexer(
                    uri=MILVUS_URI, collection=collection, image_collection=image_collection,
                    dimension=generation.dimension if generation else TEXT_EMBEDDING_DIMENSION,
                    vision_llm=llm, gemini_api_key=GEMINI_API_KEY,
                    gemini_model=generation.model if generation else GEMINI_EMBEDDING_MODEL,
                    spend_ledger=ledger,
                    native_bm25=generation is not None,
                )
    return _indexes[key]
