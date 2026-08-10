from sqlalchemy import text
from loguru import logger
from sqlalchemy.exc import IntegrityError, OperationalError

from shared.db.models import Base
from shared.db.session import replica_engine


SERVICE = "recommender"
VERSION = "002_product_search_fields"


def ensure_schema() -> None:
    """Apply the recommender baseline migration once and tolerate concurrent startup."""
    try:
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
