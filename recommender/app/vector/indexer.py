from __future__ import annotations

import logging
from dataclasses import dataclass
from typing import Any

from chat.llm_client import LLMClient
from vector.documents import ProductDocument, build_product_document
from vector.embeddings import HashEmbedding
from vector.langchain_milvus import LangChainMilvusIndexes

logger = logging.getLogger(__name__)


@dataclass
class ProductIndexer:
    indexes: LangChainMilvusIndexes
    embeddings: HashEmbedding
    vision_llm: LLMClient | None = None

    def upsert(self, product: dict[str, Any]) -> None:
        document = build_product_document(product)
        descriptions: list[str] = []
        if self.vision_llm:
            for image_url in document.image_urls:
                try:
                    descriptions.append(self.vision_llm.describe_image(image_url))
                except Exception:
                    logger.warning("image description failed for product %s", document.product_id, exc_info=True)
                    descriptions.append(document.text)
        document = ProductDocument(
            document.product_id,
            document.text,
            document.image_urls,
            document.metadata,
            tuple(descriptions),
        )
        self.indexes.upsert([document])

    def delete(self, product_id: str) -> None:
        self.indexes.text.delete([str(product_id)])
        if self.indexes.image:
            self.indexes.image.delete(expr=f'product_id == "{str(product_id).replace(chr(34), "")}"')


def build_product_indexer(*, uri: str, collection: str, image_collection: str, dimension: int, vision_llm: LLMClient | None = None) -> ProductIndexer:
    embeddings = HashEmbedding(dimension)
    indexes = LangChainMilvusIndexes.connect(
        uri=uri,
        text_collection=collection,
        image_collection=image_collection,
        text_embeddings=embeddings,
        image_embeddings=embeddings,
    )
    return ProductIndexer(indexes=indexes, embeddings=embeddings, vision_llm=vision_llm)
