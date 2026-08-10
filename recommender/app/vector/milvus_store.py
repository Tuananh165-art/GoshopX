from __future__ import annotations

from dataclasses import dataclass
from typing import Any


@dataclass(frozen=True)
class ProductVectorRecord:
    product_id: str
    text_vector: list[float]
    image_vector: list[float]
    metadata: dict[str, Any]

    def validate(self, text_dimension: int, image_dimension: int) -> None:
        if not self.product_id:
            raise ValueError("product_id is required")
        if len(self.text_vector) != text_dimension:
            raise ValueError(f"text vector must have dimension {text_dimension}")
        if len(self.image_vector) != image_dimension:
            raise ValueError(f"image vector must have dimension {image_dimension}")


def build_collection_schema(text_dimension: int, image_dimension: int) -> list[dict[str, Any]]:
    if text_dimension <= 0 or image_dimension <= 0:
        raise ValueError("vector dimensions must be positive")
    return [
        {"name": "product_id", "dtype": "VARCHAR", "is_primary": True, "max_length": 128},
        {"name": "text_vector", "dtype": "FLOAT_VECTOR", "dim": text_dimension, "metric_type": "COSINE"},
        {"name": "image_vector", "dtype": "FLOAT_VECTOR", "dim": image_dimension, "metric_type": "COSINE"},
        {"name": "text_sparse", "dtype": "SPARSE_FLOAT_VECTOR", "metric_type": "BM25"},
        {"name": "publish_status", "dtype": "VARCHAR", "max_length": 32},
        {"name": "moderation_status", "dtype": "VARCHAR", "max_length": 32},
        {"name": "category", "dtype": "VARCHAR", "max_length": 128},
        {"name": "brand", "dtype": "VARCHAR", "max_length": 128},
        {"name": "price", "dtype": "DOUBLE"},
        {"name": "stock", "dtype": "INT64"},
        {"name": "rating", "dtype": "DOUBLE"},
        {"name": "thumbnail", "dtype": "VARCHAR", "max_length": 1024},
        {"name": "images_json", "dtype": "VARCHAR", "max_length": 8192},
        {"name": "text_content", "dtype": "VARCHAR", "max_length": 8192},
    ]


class MilvusStore:
    """Small adapter; pymilvus is imported only when a real store is constructed."""

    def __init__(self, uri: str, collection: str):
        self.uri = uri
        self.collection = collection
        try:
            from pymilvus import MilvusClient
        except ImportError as exc:
            raise RuntimeError("pymilvus is required to connect to Milvus") from exc
        self._client = MilvusClient(uri=uri)

    def has_collection(self) -> bool:
        return self._client.has_collection(collection_name=self.collection)

    def delete(self, product_ids: list[str]) -> None:
        if product_ids and self.has_collection():
            self._client.delete(collection_name=self.collection, filter="product_id in [" + ",".join(repr(pid) for pid in product_ids) + "]")
