import json

import requests
from kafka import KafkaConsumer
from loguru import logger
from sqlalchemy.exc import IntegrityError

from shared.config.settings import (
    INTERACTION_EVENTS_TOPIC,
    INVENTORY_EVENTS_TOPIC,
    KAFKA_SERVER,
    PRODUCT_API,
    PRODUCT_EVENTS_TOPIC,
)
from vector.indexer import ProductIndexer
from vector.runtime_index import write_indexers
from shared.db.repo import (
    create_or_update_product,
    create_product,
    delete_product_by_id,
    get_product_by_id,
    mark_product_indexed,
    record_interaction,
)
from shared.db.session import get_session
from shared.spend import ProviderSpendLedger
from shared.kafka.utils import product_is_created_or_updated, product_is_deleted
from shared.product.utils import fetch_product_by_id, normalize_product_data
from shared.db.stock import update_product_stock

_spend_ledger = ProviderSpendLedger()


def _get_indexer() -> ProductIndexer:
    return write_indexers(_spend_ledger)[0]


def _get_indexers() -> list[ProductIndexer]:
    return write_indexers(_spend_ledger)


def _bootstrap_catalog_from_product_api() -> int:
    if not PRODUCT_API:
        logger.warning("PRODUCT_API not configured; skipping catalog bootstrap")
        return 0
    page_size = 200
    total = 0
    skip = 0
    index_errors = 0
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
        pending: list[dict] = []
        with get_session() as session:
            for raw in page:
                try:
                    normalized = normalize_product_data(raw)
                except (KeyError, TypeError, ValueError) as exc:
                    logger.warning("Skipping malformed product during bootstrap: {}", exc)
                    continue
                try:
                    changed = create_or_update_product(session, normalized)
                except IntegrityError as exc:
                    session.rollback()
                    logger.warning("Skipping product {} due to integrity error: {}", normalized.get("product_id"), exc.orig)
                    continue
                except Exception as exc:  # noqa: BLE001
                    session.rollback()
                    logger.error("Skipping product {} due to error: {}", normalized.get("product_id"), exc)
                    continue
                if changed:
                    pending.append(normalized)
        if pending:
            try:
                for indexer in _get_indexers():
                    indexer.upsert(pending)
                with get_session() as session:
                    for product in pending:
                        mark_product_indexed(session, product["product_id"])
            except Exception as exc:  # noqa: BLE001
                index_errors += len(pending)
                logger.error("Index update failed for {} products in page starting at {}: {}", len(pending), skip, exc)
        total += len(page)
        logger.info("Bootstrap ingested page starting at {} ({} products so far)", skip, total)
        if len(page) < page_size:
            break
        skip += page_size
    logger.info("Catalog bootstrap complete: {} products ingested", total)
    if index_errors:
        raise RuntimeError(f"catalog bootstrap left {index_errors} product vectors pending; rerun to retry")
    return total


def start_kafka_consumer() -> None:
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
            changed = create_or_update_product(session, product_data)
            if changed:
                for indexer in _get_indexers():
                    indexer.upsert(product_data)
                mark_product_indexed(session, str(product_data["product_id"]))
        elif product_is_deleted(event):
            product_id = event["data"]["product_id"]
            delete_product_by_id(session, product_id)
            for indexer in _get_indexers():
                indexer.delete(product_id)


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
        # Stock is read from the live PostgreSQL projection and does not affect vectors.


if __name__ == "__main__":
    start_kafka_consumer()
