from __future__ import annotations

import hashlib
import json
import uuid
from dataclasses import dataclass

from sqlalchemy import func

from shared.db.models import Product, VectorIndexGeneration
from shared.db.session import ReplicaSession


@dataclass(frozen=True)
class Generation:
    id: str
    text_collection: str
    image_collection: str
    status: str
    model: str
    dimension: int


def _value(row: VectorIndexGeneration) -> Generation:
    return Generation(row.id, row.text_collection, row.image_collection, row.status, row.model, row.dimension)


def active_generation() -> Generation | None:
    with ReplicaSession() as session:
        row = session.query(VectorIndexGeneration).filter_by(status="active").first()
        return _value(row) if row else None


def write_generations() -> list[Generation]:
    with ReplicaSession() as session:
        rows = (session.query(VectorIndexGeneration)
                .filter(VectorIndexGeneration.status.in_(("active", "building")))
                .order_by(VectorIndexGeneration.created_at).all())
        return [_value(row) for row in rows]


def create_or_resume_build(model: str, dimension: int) -> Generation:
    with ReplicaSession() as session:
        if session.bind.dialect.name == "postgresql":
            session.execute(func.pg_advisory_xact_lock(719240119))
        row = (session.query(VectorIndexGeneration).filter_by(status="building")
               .order_by(VectorIndexGeneration.created_at).first())
        if row is not None and (row.model != model or row.dimension != dimension):
            row.status = "failed"
            session.flush()
            row = None
        if row is None:
            generation_id = str(uuid.uuid4())
            prefix = f"goshopx_catalog_{generation_id.replace('-', '')[:16]}"
            row = VectorIndexGeneration(id=generation_id, text_collection=prefix,
                                        image_collection=f"{prefix}_images", status="building",
                                        model=model, dimension=dimension)
            session.add(row)
            session.commit()
        else:
            session.commit()
        return _value(row)


def record_manifest(generation_id: str, document_count: int, product_hashes: list[tuple[str, str]]) -> str:
    manifest = hashlib.sha256("\n".join(f"{product_id}:{content_hash}" for product_id, content_hash in sorted(product_hashes)).encode()).hexdigest()
    with ReplicaSession() as session:
        row = session.get(VectorIndexGeneration, generation_id)
        if row is None or row.status != "building":
            raise ValueError("vector generation is not in building state")
        row.document_count = document_count
        row.manifest_hash = manifest
        session.commit()
    return manifest


def activate_generation(generation_id: str, expected_count: int, expected_manifest: str) -> None:
    with ReplicaSession() as session:
        if session.bind.dialect.name == "postgresql":
            session.execute(func.pg_advisory_xact_lock(719240120))
        candidate = session.get(VectorIndexGeneration, generation_id)
        if (candidate is None or candidate.status != "building" or candidate.document_count != expected_count
                or candidate.manifest_hash != expected_manifest or expected_count < 0 or not expected_manifest):
            raise ValueError("vector generation validation failed; active generation is unchanged")
        session.query(VectorIndexGeneration).filter_by(status="active").update({"status": "retired"}, synchronize_session=False)
        candidate.status = "active"
        candidate.activated_at = func.now()
        session.commit()


def rollback_generation(generation_id: str) -> None:
    with ReplicaSession() as session:
        if session.bind.dialect.name == "postgresql":
            session.execute(func.pg_advisory_xact_lock(719240120))
        previous = session.get(VectorIndexGeneration, generation_id)
        if previous is None or previous.status != "retired":
            raise ValueError("rollback target must be a retained generation")
        session.query(VectorIndexGeneration).filter_by(status="active").update({"status": "retired"}, synchronize_session=False)
        previous.status = "active"
        previous.activated_at = func.now()
        session.commit()


def catalog_snapshot(batch_size: int = 200):
    with ReplicaSession() as session:
        last_id = ""
        while True:
            products = (session.query(Product).filter(Product.id > last_id)
                        .order_by(Product.id).limit(batch_size).all())
            if not products:
                break
            for product in products:
                last_id = product.id
                yield {
                    "id": product.id, "product_id": product.id, "name": product.name,
                    "description": product.description, "price": product.price,
                    "accountID": product.account_id, "category": product.category,
                    "brand": product.brand, "tags": json.loads(product.tags_json or "[]"),
                    "thumbnail": product.thumbnail, "images": json.loads(product.images_json or "[]"),
                    "publishStatus": product.publish_status, "moderationStatus": product.moderation_status,
                    "stock": product.stock, "rating": product.rating,
                    "discountPercentage": product.discount_percentage, "sku": product.sku,
                    "warrantyInformation": product.warranty_information,
                    "shippingInformation": product.shipping_information,
                    "returnPolicy": product.return_policy, "availabilityStatus": product.availability_status,
                    "_content_hash": product.source_hash,
                }


def catalog_manifest() -> tuple[int, list[tuple[str, str]]]:
    with ReplicaSession() as session:
        rows = (session.query(Product.id, Product.source_hash)
                .filter(Product.publish_status == "published", Product.moderation_status == "approved")
                .order_by(Product.id).all())
    return len(rows), [(str(product_id), str(content_hash or "")) for product_id, content_hash in rows]


def validate_catalog_snapshot(expected: list[tuple[str, str]], indexed: list[tuple[str, str]],
                              current: list[tuple[str, str]]) -> None:
    if sorted(expected) != sorted(indexed) or sorted(expected) != sorted(current):
        raise RuntimeError("catalog changed during index build; candidate remains inactive, rerun to rebuild consistently")
