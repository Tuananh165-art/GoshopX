import os
import sys

APP_DIR = os.path.join(os.path.dirname(__file__), "app")
if APP_DIR not in sys.path:
    sys.path.insert(0, APP_DIR)

from loguru import logger

from entry.grpc_server import serve
from entry.kafka_consumer import _bootstrap_catalog_from_product_api, start_kafka_consumer
from recommendations.train import train_and_save
from shared.db.schema import ensure_schema


def main() -> None:
    if len(sys.argv) < 2:
        logger.error("Usage: python main.py [consumer|server|train|bootstrap-catalog|rebuild-catalog-index|rollback-catalog-index <generation-id>|ingestion-worker]")
        sys.exit(1)

    ensure_schema()

    command = sys.argv[1]
    if command == "consumer":
        start_kafka_consumer()
    elif command == "bootstrap-catalog":
        _bootstrap_catalog_from_product_api()
    elif command == "rebuild-catalog-index":
        from vector.rebuild import rebuild_catalog_index
        logger.info("Activated catalog vector generation {}", rebuild_catalog_index())
    elif command == "rollback-catalog-index":
        if len(sys.argv) != 3:
            logger.error("Usage: python main.py rollback-catalog-index <retired-generation-id>")
            sys.exit(2)
        from vector.generations import rollback_generation
        rollback_generation(sys.argv[2])
        logger.info("Catalog vector generation rolled back to {}", sys.argv[2])
    elif command == "server":
        serve()
    elif command == "train":
        train_and_save()
    elif command == "ingestion-worker":
        from ingestion.worker import run_worker
        run_worker()
    else:
        logger.error("Unknown command: {}", command)
        sys.exit(1)


if __name__ == "__main__":
    main()
