from __future__ import annotations

import logging
from dataclasses import dataclass
from typing import Any

from chat.llm_client import LLMClient
from vector.documents import ProductDocument, build_product_document
from vector.embeddings import GeminiCatalogImageEmbedding, GeminiEmbedding, HashEmbedding
from vector.langchain_milvus import LangChainMilvusIndexes

logger = logging.getLogger(__name__)


@dataclass
class ProductIndexer:
    indexes: LangChainMilvusIndexes
    embeddings: Any
    vision_llm: LLMClient | None = None

    def upsert(self, product: dict[str, Any] | list[dict[str, Any]]) -> None:
        products = product if isinstance(product, list) else [product]
        self.indexes.upsert([build_product_document(item) for item in products])

    def delete(self, product_id: str) -> None:
        self.indexes.text.delete([str(product_id)])
        if self.indexes.image:
            self.indexes.image.delete(expr=f'product_id == "{str(product_id).replace(chr(34), "")}"')


def build_product_indexer(*, uri: str, collection: str, image_collection: str, dimension: int, vision_llm: LLMClient | None = None, gemini_api_key: str = "", gemini_model: str = "gemini-embedding-2", spend_ledger: Any | None = None, native_bm25: bool = False) -> ProductIndexer:
    if not gemini_api_key and collection.startswith("product_catalog_gemini_"):
        raise ValueError("Gemini API key is required for a Gemini vector collection")
    embeddings = GeminiEmbedding(gemini_api_key, model=gemini_model, dimension=dimension, spend_ledger=spend_ledger) if gemini_api_key else HashEmbedding(dimension)
    image_embeddings = GeminiCatalogImageEmbedding(embeddings) if gemini_api_key else embeddings
    indexes = LangChainMilvusIndexes.connect(
        uri=uri,
        text_collection=collection,
        image_collection=image_collection,
        text_embeddings=embeddings,
        image_embeddings=image_embeddings,
        native_bm25=native_bm25,
    )
    return ProductIndexer(indexes=indexes, embeddings=embeddings, vision_llm=vision_llm)
