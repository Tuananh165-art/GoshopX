from datetime import datetime, timezone

from sqlalchemy import String, Float, Integer, DateTime, Text, func, ForeignKey, UniqueConstraint
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import Mapped, mapped_column, relationship
from generated.pb import recommender_pb2

Base = declarative_base()

class Product(Base):
    __tablename__ = "products"
    id: Mapped[str] = mapped_column(String, primary_key=True)
    name: Mapped[str] = mapped_column(String)
    description: Mapped[str] = mapped_column(String)
    price: Mapped[float] = mapped_column(Float)
    account_id: Mapped[int] = mapped_column(Integer)
    category: Mapped[str] = mapped_column(String, default="")
    brand: Mapped[str] = mapped_column(String, default="")
    tags_json: Mapped[str] = mapped_column(String, default="[]")
    thumbnail: Mapped[str] = mapped_column(String, default="")
    images_json: Mapped[str] = mapped_column(String, default="[]")
    publish_status: Mapped[str] = mapped_column(String, default="published")
    moderation_status: Mapped[str] = mapped_column(String, default="approved")
    stock: Mapped[int] = mapped_column(Integer, default=1)
    rating: Mapped[float] = mapped_column(Float, default=0)
    discount_percentage: Mapped[float] = mapped_column(Float, default=0)
    sku: Mapped[str] = mapped_column(String, default="")
    warranty_information: Mapped[str] = mapped_column(String, default="")
    shipping_information: Mapped[str] = mapped_column(String, default="")
    return_policy: Mapped[str] = mapped_column(String, default="")
    availability_status: Mapped[str] = mapped_column(String, default="")
    source_hash: Mapped[str] = mapped_column(String, default="")
    indexed_hash: Mapped[str] = mapped_column(String, default="")

    interactions: Mapped[list["Interaction"]] = relationship(
        "Interaction", back_populates="product", cascade="all, delete-orphan")

    def to_grpc_model(self) -> recommender_pb2.ProductReplica:
        return recommender_pb2.ProductReplica(
            id=self.id,
            name=self.name,
            description=self.description,
            price=self.price
        )

class Interaction(Base):
    __tablename__ = "interactions"
    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String)
    product_id: Mapped[str] = mapped_column(String, ForeignKey("products.id"))
    interaction_type: Mapped[str] = mapped_column(String)
    timestamp: Mapped[DateTime] = mapped_column(DateTime, default=func.now())

    product: Mapped["Product"] = relationship(
        "Product", back_populates="interactions")


class ChatConversation(Base):
    __tablename__ = "chat_conversations"
    thread_id: Mapped[str] = mapped_column(String(64), primary_key=True)
    messages_json: Mapped[str] = mapped_column(Text, default="[]")
    updated_at: Mapped[DateTime] = mapped_column(DateTime, default=func.now(), onupdate=func.now(), index=True)


class KnowledgeSource(Base):
    __tablename__ = "ai_knowledge_sources"
    id: Mapped[str] = mapped_column(String(36), primary_key=True)
    name: Mapped[str] = mapped_column(String(255))
    object_uri: Mapped[str] = mapped_column(String(1024), unique=True)
    content_type: Mapped[str] = mapped_column(String(128))
    sha256: Mapped[str] = mapped_column(String(64))
    owner_account_id: Mapped[int] = mapped_column(Integer, index=True)
    linked_product_id: Mapped[str] = mapped_column(String(128), default="", index=True)
    status: Mapped[str] = mapped_column(String(24), default="draft", index=True)
    version: Mapped[int] = mapped_column(Integer, default=1)
    metadata_json: Mapped[str] = mapped_column(Text, default="{}")
    created_at: Mapped[DateTime] = mapped_column(DateTime, default=func.now())
    updated_at: Mapped[DateTime] = mapped_column(DateTime, default=func.now(), onupdate=func.now())


class IngestionJob(Base):
    __tablename__ = "ai_ingestion_jobs"
    __table_args__ = (UniqueConstraint("idempotency_key", name="uq_ai_ingestion_job_idempotency"),)
    id: Mapped[str] = mapped_column(String(36), primary_key=True)
    source_id: Mapped[str] = mapped_column(String(36), ForeignKey("ai_knowledge_sources.id", ondelete="CASCADE"), index=True)
    idempotency_key: Mapped[str] = mapped_column(String(128))
    status: Mapped[str] = mapped_column(String(24), default="queued", index=True)
    attempts: Mapped[int] = mapped_column(Integer, default=0)
    max_attempts: Mapped[int] = mapped_column(Integer, default=3)
    lease_owner: Mapped[str | None] = mapped_column(String(128))
    lease_expires_at: Mapped[DateTime | None] = mapped_column(DateTime, index=True)
    error_code: Mapped[str | None] = mapped_column(String(64))
    created_at: Mapped[DateTime] = mapped_column(DateTime, default=func.now())
    updated_at: Mapped[DateTime] = mapped_column(DateTime, default=func.now(), onupdate=func.now())


class KnowledgeChunk(Base):
    __tablename__ = "ai_knowledge_chunks"
    __table_args__ = (UniqueConstraint("source_id", "source_version", "chunk_index", name="uq_ai_knowledge_chunk_version_index"),)
    id: Mapped[str] = mapped_column(String(36), primary_key=True)
    source_id: Mapped[str] = mapped_column(String(36), ForeignKey("ai_knowledge_sources.id", ondelete="CASCADE"), index=True)
    source_version: Mapped[int] = mapped_column(Integer)
    chunk_index: Mapped[int] = mapped_column(Integer)
    content: Mapped[str] = mapped_column(Text)
    content_hash: Mapped[str] = mapped_column(String(64))
    metadata_json: Mapped[str] = mapped_column(Text, default="{}")
    created_at: Mapped[DateTime] = mapped_column(DateTime, default=func.now())


class IngestionAuditEvent(Base):
    __tablename__ = "ai_ingestion_audit_events"
    id: Mapped[str] = mapped_column(String(36), primary_key=True)
    actor_account_id: Mapped[int] = mapped_column(Integer, index=True)
    action: Mapped[str] = mapped_column(String(64), index=True)
    source_id: Mapped[str] = mapped_column(String(36), index=True)
    job_id: Mapped[str] = mapped_column(String(36), default="")
    outcome: Mapped[str] = mapped_column(String(24))
    occurred_at: Mapped[DateTime] = mapped_column(DateTime, default=lambda: datetime.now(timezone.utc), index=True)


class VectorIndexGeneration(Base):
    __tablename__ = "ai_vector_index_generations"
    id: Mapped[str] = mapped_column(String(36), primary_key=True)
    text_collection: Mapped[str] = mapped_column(String(255), unique=True)
    image_collection: Mapped[str] = mapped_column(String(255), unique=True)
    status: Mapped[str] = mapped_column(String(16), default="building", index=True)
    model: Mapped[str] = mapped_column(String(128))
    dimension: Mapped[int] = mapped_column(Integer)
    document_count: Mapped[int] = mapped_column(Integer, default=0)
    manifest_hash: Mapped[str] = mapped_column(String(64), default="")
    created_at: Mapped[DateTime] = mapped_column(DateTime, default=func.now())
    activated_at: Mapped[DateTime | None] = mapped_column(DateTime)
