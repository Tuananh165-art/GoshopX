"""Lightweight embeddings with no Torch/CUDA dependency.

Text and image descriptions are encoded with a deterministic signed hashing
vector. Product images are described by the configured vision-capable chat API;
only the description is embedded locally, so the runtime remains small.
"""

from __future__ import annotations

import hashlib
import re
from typing import Protocol


class ImageDescriber(Protocol):
    def describe_image(self, image_url: str) -> str: ...


class HashEmbedding:
    def __init__(self, dimension: int = 512):
        if dimension < 64:
            raise ValueError("embedding dimension must be at least 64")
        self.dimension = dimension

    def _embed(self, value: str) -> list[float]:
        vector = [0.0] * self.dimension
        normalized = re.sub(r"\s+", " ", value.casefold().strip())
        tokens = re.findall(r"[\wÀ-ỹ]+", normalized)
        features = tokens + [f"{normalized[index:index + 3]}" for index in range(max(0, len(normalized) - 2))]
        if not features:
            return vector
        for feature in features:
            digest = hashlib.blake2b(feature.encode("utf-8"), digest_size=8).digest()
            index = int.from_bytes(digest[:4], "big") % self.dimension
            sign = 1.0 if digest[4] & 1 else -1.0
            vector[index] += sign
        norm = sum(item * item for item in vector) ** 0.5
        return [item / norm for item in vector] if norm else vector

    def embed_documents(self, texts: list[str]) -> list[list[float]]:
        return [self._embed(text) for text in texts]

    def embed_query(self, text: str) -> list[float]:
        return self._embed(text)

    def embed_image_description(self, description: str) -> list[float]:
        return self._embed(f"visual product image: {description}")


def describe_product_image(image_url: str, llm: ImageDescriber) -> str:
    """Ask the configured API for a bounded, catalog-safe visual description."""
    description = llm.describe_image(image_url).strip()
    return description[:2000]
