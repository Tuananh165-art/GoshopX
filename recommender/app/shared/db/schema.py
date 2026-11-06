from sqlalchemy import text
from loguru import logger
from sqlalchemy.exc import IntegrityError, OperationalError

from shared.db.models import Base
from shared.db.session import replica_engine


SERVICE = "recommender"
VERSION = "007_vector_index_generations"


def ensure_schema() -> None:
    """Apply the recommender baseline migration once and tolerate concurrent startup."""
    try:
        with replica_engine.begin() as connection:
            if connection.dialect.name == "postgresql":
                connection.execute(text("SELECT pg_advisory_xact_lock(hashtext('goshopx-recommender-schema'))"))
            Base.metadata.create_all(connection, checkfirst=True)

        with replica_engine.begin() as connection:
            connection.execute(text("""
                CREATE TABLE IF NOT EXISTS schema_migrations (
                    service VARCHAR(100) NOT NULL,
                    version VARCHAR(100) NOT NULL,
                    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
                    PRIMARY KEY (service, version)
                )
            """))
            applied = connection.execute(
                text("SELECT 1 FROM schema_migrations WHERE service = :service AND version = :version"),
                {"service": SERVICE, "version": VERSION},
            ).scalar()

        if applied is None:
            Base.metadata.create_all(replica_engine, checkfirst=True)
            if replica_engine.dialect.name == "postgresql":
                for statement in (
                    "ALTER TABLE products ADD COLUMN IF NOT EXISTS category VARCHAR DEFAULT ''",
                    "ALTER TABLE products ADD COLUMN IF NOT EXISTS brand VARCHAR DEFAULT ''",
                    "ALTER TABLE products ADD COLUMN IF NOT EXISTS tags_json VARCHAR DEFAULT '[]'",
                    "ALTER TABLE products ADD COLUMN IF NOT EXISTS thumbnail VARCHAR DEFAULT ''",
                    "ALTER TABLE products ADD COLUMN IF NOT EXISTS images_json VARCHAR DEFAULT '[]'",
                    "ALTER TABLE products ADD COLUMN IF NOT EXISTS publish_status VARCHAR DEFAULT 'published'",
                    "ALTER TABLE products ADD COLUMN IF NOT EXISTS moderation_status VARCHAR DEFAULT 'approved'",
                    "ALTER TABLE products ADD COLUMN IF NOT EXISTS stock INTEGER DEFAULT 1",
                    "ALTER TABLE products ADD COLUMN IF NOT EXISTS rating FLOAT DEFAULT 0",
                    "ALTER TABLE products ADD COLUMN IF NOT EXISTS discount_percentage FLOAT DEFAULT 0",
                    "ALTER TABLE products ADD COLUMN IF NOT EXISTS sku VARCHAR DEFAULT ''",
                    "ALTER TABLE products ADD COLUMN IF NOT EXISTS warranty_information VARCHAR DEFAULT ''",
                    "ALTER TABLE products ADD COLUMN IF NOT EXISTS shipping_information VARCHAR DEFAULT ''",
                    "ALTER TABLE products ADD COLUMN IF NOT EXISTS return_policy VARCHAR DEFAULT ''",
                    "ALTER TABLE products ADD COLUMN IF NOT EXISTS availability_status VARCHAR DEFAULT ''",
                    "ALTER TABLE products ADD COLUMN IF NOT EXISTS source_hash VARCHAR DEFAULT ''",
                    "ALTER TABLE products ADD COLUMN IF NOT EXISTS indexed_hash VARCHAR DEFAULT ''",
                    "ALTER TABLE ai_knowledge_sources ADD COLUMN IF NOT EXISTS linked_product_id VARCHAR(128) NOT NULL DEFAULT ''",
                    "CREATE TABLE IF NOT EXISTS ai_spend_entries (id VARCHAR(36) PRIMARY KEY, provider VARCHAR(32) NOT NULL, reserved_vnd BIGINT NOT NULL, input_tokens BIGINT, output_tokens BIGINT, actual_vnd BIGINT, status VARCHAR(24) NOT NULL DEFAULT 'reserved', created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)",
                    "ALTER TABLE ai_spend_entries ADD COLUMN IF NOT EXISTS input_tokens BIGINT",
                    "ALTER TABLE ai_spend_entries ADD COLUMN IF NOT EXISTS output_tokens BIGINT",
                    "ALTER TABLE ai_spend_entries ADD COLUMN IF NOT EXISTS actual_vnd BIGINT",
                    "ALTER TABLE ai_spend_entries ADD COLUMN IF NOT EXISTS status VARCHAR(24) NOT NULL DEFAULT 'reserved'",
                ):
                    with replica_engine.begin() as migration_connection:
                        migration_connection.execute(text(statement))
            with replica_engine.begin() as connection:
                connection.execute(
                    text("""
                        INSERT INTO schema_migrations(service, version)
                        VALUES (:service, :version)
                        ON CONFLICT (service, version) DO NOTHING
                    """),
                    {"service": SERVICE, "version": VERSION},
                )
    except (IntegrityError, OperationalError) as exc:
        logger.info("Database schema already initialized by another process")
        logger.debug("Schema init detail: {}", exc)
        replica_engine.dispose()
