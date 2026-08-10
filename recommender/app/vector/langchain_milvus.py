from __future__ import annotations

import logging
from dataclasses import dataclass
from typing import Any, Protocol

from retrieval.hybrid import Candidate
from vector.documents import ProductDocument

logger = logging.getLogger(__name__)


class EmbeddingProvider(Protocol):
    def embed_documents(self, texts: list[str]) -> list[list[float]]: ...
    def embed_query(self, text: str) -> list[float]: ...


@dataclass
class LangChainMilvusIndexes:
    text: Any
    image: Any | None

    @classmethod
    def connect(
        cls,
        *,
        uri: str,
        text_collection: str,
        image_collection: str | None,
        text_embeddings: EmbeddingProvider,
        image_embeddings: EmbeddingProvider | None = None,
    ) -> "LangChainMilvusIndexes":
        try:
            from pymilvus import MilvusClient, connections
            from langchain_milvus import Milvus
        except ImportError as exc:
            raise RuntimeError("langchain-milvus is required for Milvus indexes") from exc

        # Register the dynamic alias that langchain-milvus creates internally so
        # utility.has_collection() resolves the same handler.
        probe_client = MilvusClient(uri=uri)
        connections.connect(alias=probe_client._using, uri=uri)

        # Let LangChain Milvus own collection creation. It builds a schema
        # with a "text" field for page_content plus dynamic metadata; trying
        # to pre-create the collection ourselves with a minimal schema
        # breaks the read path because LangChain expects to read the
        # original page_content back from the "text" field.
        index_params = {
            "metric_type": "COSINE",
            "index_type": "HNSW",
            "params": {"M": 16, "efConstruction": 64},
        }

        connection_args = {"uri": uri}
        text_store = Milvus(
            embedding_function=text_embeddings,
            collection_name=text_collection,
            connection_args=connection_args,
            auto_id=False,
            index_params=index_params,
            enable_dynamic_field=False,
            metadata_field="metadata",
        )
        image_store = None
        if image_collection and image_embeddings:
            image_store = Milvus(
                embedding_function=image_embeddings,
                collection_name=image_collection,
                connection_args=connection_args,
                auto_id=False,
                index_params=index_params,
                enable_dynamic_field=False,
                metadata_field="metadata",
            )
        return cls(text=text_store, image=image_store)

    def upsert(self, documents: list[ProductDocument]) -> None:
        from langchain_core.documents import Document

        if not documents:
            return
        text_docs = [Document(page_content=document.text, metadata=document.metadata) for document in documents]
        ids = [document.product_id for document in documents]
        self.text.add_documents(text_docs, ids=ids)
        if self.image:
            image_docs = []
            image_ids = []
            for document in documents:
                descriptions = document.image_descriptions or (document.text,)
                for index, url in enumerate(document.image_urls):
                    description = descriptions[index] if index < len(descriptions) else document.text
                    image_docs.append(Document(page_content=description, metadata={**document.metadata, "image_url": url}))
                    image_ids.append(f"{document.product_id}:{index}")
            if image_docs:
                self.image.add_documents(image_docs, ids=image_ids)

    def dense_search(self, query: str, *, limit: int = 20) -> list[Candidate]:
        try:
            return [Candidate(str(doc.metadata["product_id"]), float(score)) for doc, score in self.text.similarity_search_with_relevance_scores(query, k=limit)]
        except ValueError:
            return [Candidate(str(doc.metadata["product_id"]), float(score)) for doc, score in self.text.similarity_search_with_score(query, k=limit)]

    def image_search(self, query: str, *, limit: int = 20) -> list[Candidate]:
        if not self.image:
            return []
        scores: dict[str, float] = {}
        try:
            for doc, score in self.image.similarity_search_with_relevance_scores(query, k=limit):
                product_id = str(doc.metadata["product_id"])
                scores[product_id] = max(scores.get(product_id, 0.0), float(score))
        except ValueError:
            for doc, score in self.image.similarity_search_with_score(query, k=limit):
                product_id = str(doc.metadata["product_id"])
                scores[product_id] = max(scores.get(product_id, 0.0), float(score))
        return [Candidate(product_id, score) for product_id, score in sorted(scores.items(), key=lambda item: -item[1])]
