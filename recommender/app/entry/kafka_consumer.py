import json

import requests
from kafka import KafkaConsumer
from loguru import logger
from sqlalchemy.exc import IntegrityError

from shared.config.settings import (
    AI_API_KEY,
    AI_ENDPOINT,
    AI_MODEL,
    INTERACTION_EVENTS_TOPIC,
    INVENTORY_EVENTS_TOPIC,
    KAFKA_SERVER,
    MILVUS_COLLECTION,
    MILVUS_URI,
    PRODUCT_API,
    TEXT_EMBEDDING_DIMENSION,
    PRODUCT_EVENTS_TOPIC,
)
from chat.llm_client import LLMClient
from vector.indexer import ProductIndexer, build_product_indexer
from shared.db.repo import (
    create_or_update_product,
    create_product,
    delete_product_by_id,
    get_product_by_id,
    record_interaction,
)
from shared.db.session import get_session
from shared.kafka.utils import product_is_created_or_updated, product_is_deleted
from shared.product.utils import fetch_product_by_id, normalize_product_data
from shared.db.stock import update_product_stock

_indexer: ProductIndexer | None = None


def _get_indexer() -> ProductIndexer:
    global _indexer
    if _indexer is None:
        vision_llm = None
        if AI_API_KEY:
            vision_llm = LLMClient(AI_ENDPOINT, AI_API_KEY, AI_MODEL)
        _indexer = build_product_indexer(
            uri=MILVUS_URI,
            collection=MILVUS_COLLECTION,
            image_collection=f"{MILVUS_COLLECTION}_images",
            dimension=TEXT_EMBEDDING_DIMENSION,
            vision_llm=vision_llm,
        )
    return _indexer


def _bootstrap_catalog_from_product_api() -> int:
    if not PRODUCT_API:
        logger.warning("PRODUCT_API not configured; skipping catalog bootstrap")
        return 0
    page_size = 200
    total = 0
    skip = 0
    while True:
        try:
            response = requests.get(PRODUCT_API, params={"skip": skip, "take": page_size}, timeout=60)
            response.raise_for_status()
        except requests.RequestException as exc:
            logger.error("Catalog bootstrap request failed (skip={}): {}", skip, exc)
            break
        try:
            page = response.json()
        except ValueError as exc:
            logger.error("Catalog bootstrap returned invalid JSON (skip={}): {}", skip, exc)
            break
        if not isinstance(page, list) or not page:
            break
        with get_session() as session:
            for raw in page:
                try:
                    normalized = normalize_product_data(raw)
                except (KeyError, TypeError, ValueError) as exc:
                    logger.warning("Skipping malformed product during bootstrap: {}", exc)
                    continue
                try:
                    create_or_update_product(session, normalized)
                except IntegrityError as exc:
                    session.rollback()
                    logger.warning("Skipping product {} due to integrity error: {}", normalized.get("product_id"), exc.orig)
                    continue
                except Exception as exc:  # noqa: BLE001
                    session.rollback()
                    logger.error("Skipping product {} due to error: {}", normalized.get("product_id"), exc)
                    continue
        indexer = _get_indexer()
        for raw in page:
            try:
                normalized = normalize_product_data(raw)
                indexer.upsert(normalized)
            except (KeyError, TypeError, ValueError):
                continue
        total += len(page)
        logger.info("Bootstrap ingested page starting at {} ({} products so far)", skip, total)
        if len(page) < page_size:
            break
        skip += page_size
    logger.info("Catalog bootstrap complete: {} products ingested", total)
    return total


def start_kafka_consumer() -> None:
    _bootstrap_catalog_from_product_api()
    consumer = KafkaConsumer(
        PRODUCT_EVENTS_TOPIC,
        INTERACTION_EVENTS_TOPIC,
        INVENTORY_EVENTS_TOPIC,
        bootstrap_servers=KAFKA_SERVER,
        group_id="recommender-sync",
    )

    for message in consumer:
        event = json.loads(message.value)

        if message.topic == PRODUCT_EVENTS_TOPIC:
            _handle_product_event(event)
        elif message.topic == INTERACTION_EVENTS_TOPIC:
            _handle_interaction_event(event)
        elif message.topic == INVENTORY_EVENTS_TOPIC:
            _handle_inventory_event(event)


def _handle_product_event(event: dict) -> None:
    with get_session() as session:
        if product_is_created_or_updated(event):
            product_data = event["data"]
            logger.info(
                "Processing product event {} for product ID: {}",
                event["type"],
                product_data["product_id"],
            )
            create_or_update_product(session, product_data)
            _get_indexer().upsert(product_data)
        elif product_is_deleted(event):
            product_id = event["data"]["product_id"]
            delete_product_by_id(session, product_id)
            _get_indexer().delete(product_id)


def _handle_interaction_event(event: dict) -> None:
    with get_session() as session:
        product_id = str(event["data"]["product_id"])
        product = get_product_by_id(session, product_id)
        if not product:
            try:
                product_data = fetch_product_by_id(product_id)
                if not product_data:
                    logger.warning("Ignoring interaction for unknown product {}", product_id)
                    return
                create_product(session, product_data)
                session.commit()
            except (requests.RequestException, KeyError, TypeError, ValueError) as exc:
                logger.error(
                    "Failed to fetch product {} for interaction event {}: {}",
                    product_id,
                    event["type"],
                    exc,
                )
                session.rollback()
                return
        try:
            record_interaction(session, event)
        except IntegrityError:
            session.rollback()
            logger.warning("Skipping interaction for product {} because its projection is unavailable", product_id)

def _handle_inventory_event(event: dict) -> None:
    data = event.get("data") or {}
    product_id = data.get("product_id") or data.get("productID")
    available = data.get("available_qty", data.get("availableQuantity"))
    if not product_id or available is None:
        logger.warning("Ignoring inventory event without product and available quantity: {}", event.get("type"))
        return
    with get_session() as session:
        product = update_product_stock(session, str(product_id), int(available))
        if product is None:
            return
        _get_indexer().upsert({
            "id": product.id,
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
        })


if __name__ == "__main__":
    start_kafka_consumer()
