from __future__ import annotations

from shared.config.settings import (GEMINI_API_KEY, GEMINI_EMBEDDING_MODEL,
                                    TEXT_EMBEDDING_DIMENSION, MILVUS_URI)
from shared.spend import ProviderSpendLedger
from vector.generations import (activate_generation, catalog_manifest, catalog_snapshot,
                                create_or_resume_build, record_manifest, validate_catalog_snapshot)
from vector.indexer import build_product_indexer


def rebuild_catalog_index(batch_size: int = 100) -> str:
    if not GEMINI_API_KEY:
        raise RuntimeError("GEMINI_API_KEY is required to build a production catalog generation")
    if batch_size < 1 or batch_size > 500:
        raise ValueError("batch_size must be between 1 and 500")
    ledger = ProviderSpendLedger()
    generation = create_or_resume_build(GEMINI_EMBEDDING_MODEL, TEXT_EMBEDDING_DIMENSION)
    expected_count, expected_hashes = catalog_manifest()
    indexer = build_product_indexer(
        uri=MILVUS_URI, collection=generation.text_collection,
        image_collection=generation.image_collection, dimension=generation.dimension,
        gemini_api_key=GEMINI_API_KEY, gemini_model=generation.model, spend_ledger=ledger,
        native_bm25=True,
    )
    hashes: list[tuple[str, str]] = []
    batch: list[dict] = []
    count = 0
    for product in catalog_snapshot(batch_size):
        if product["publishStatus"] == "published" and product["moderationStatus"] == "approved":
            hashes.append((str(product["id"]), str(product.get("_content_hash", ""))))
            batch.append(product)
            count += 1
            if len(batch) == batch_size:
                indexer.upsert(batch)
                batch.clear()
    if batch:
        indexer.upsert(batch)
    if not count:
        raise RuntimeError("refusing to activate an empty catalog generation")

    # A provider/index failure exits before activation, leaving the previous generation active.
    collection = getattr(indexer.indexes.text, "col", None)
    entity_count = getattr(collection, "num_entities", count)
    entity_count = entity_count() if callable(entity_count) else entity_count
    if int(entity_count) < count:
        raise RuntimeError(f"candidate collection contains {entity_count} rows; expected at least {count}")
    current_count, current_hashes = catalog_manifest()
    if count != expected_count or current_count != expected_count:
        raise RuntimeError("catalog changed during index build; candidate remains inactive, rerun to rebuild consistently")
    validate_catalog_snapshot(expected_hashes, hashes, current_hashes)
    manifest = record_manifest(generation.id, count, hashes)
    activate_generation(generation.id, count, manifest)
    return generation.id
