from __future__ import annotations

import hashlib
import json
import os
import socket
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid

from shared.db.session import ReplicaSession
from shared.spend import ProviderSpendLedger
from shared.config.settings import GEMINI_API_KEY, GEMINI_EMBEDDING_DIMENSION, GEMINI_EMBEDDING_MODEL, MILVUS_URI
from ingestion.parser import parse_document
from ingestion.repository import IngestionRepository
from vector.embeddings import GeminiEmbedding
from ingestion.parser import DocumentParseError

LEASE_RENEW_INTERVAL_SECONDS = 30


class _LeaseHeartbeat:
    def __init__(self, repository: IngestionRepository, job_id: str, worker_id: str):
        self.repository = repository
        self.job_id = job_id
        self.worker_id = worker_id
        self.lost = threading.Event()
        self.stop = threading.Event()
        self.thread = threading.Thread(target=self._renew, daemon=True)

    def _renew(self) -> None:
        while not self.stop.wait(LEASE_RENEW_INTERVAL_SECONDS):
            try:
                if not self.repository.heartbeat(self.job_id, self.worker_id):
                    self.lost.set()
                    return
            except Exception:
                continue

    def __enter__(self):
        self.thread.start()
        return self.lost

    def __exit__(self, *_exc):
        self.stop.set()
        self.thread.join(timeout=1)


def read_gcs_object(uri: str, *, max_bytes: int = 20 * 1024 * 1024) -> bytes:
    bucket = os.getenv("GCS_MEDIA_BUCKET", "")
    parsed = urllib.parse.urlparse(uri)
    if parsed.scheme != "gs" or parsed.netloc != bucket or not bucket:
        raise ValueError("gcs_source_not_allowed")
    object_name = parsed.path.lstrip("/")
    if not object_name.startswith("knowledge-upload/") or ".." in object_name.split("/"):
        raise ValueError("gcs_source_not_allowed")
    token_request = urllib.request.Request(
        "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token",
        headers={"Metadata-Flavor": "Google"},
    )
    try:
        with urllib.request.urlopen(token_request, timeout=3) as response:
            token = json.loads(response.read(16 * 1024)).get("access_token", "")
        if not token:
            raise RuntimeError("gcs_identity_unavailable")
        url = ("https://storage.googleapis.com/download/storage/v1/b/" + urllib.parse.quote(bucket, safe="")
               + "/o/" + urllib.parse.quote(object_name, safe="") + "?alt=media")
        request = urllib.request.Request(url, headers={"Authorization": f"Bearer {token}"})
        with urllib.request.urlopen(request, timeout=20) as response:
            data = response.read(max_bytes + 1)
    except urllib.error.HTTPError as exc:
        raise RuntimeError(f"gcs_read_http_{exc.code}") from None
    except (urllib.error.URLError, TimeoutError, json.JSONDecodeError) as exc:
        raise RuntimeError(f"gcs_read_{type(exc).__name__.lower()}") from None
    if len(data) > max_bytes:
        raise ValueError("document_size_invalid")
    return data


class IngestionWorker:
    def __init__(self, repository: IngestionRepository, worker_id: str, *, object_reader=read_gcs_object,
                 indexer=None, sessions=ReplicaSession):
        self.repository = repository
        self.worker_id = worker_id
        self.object_reader = object_reader
        self.indexer = indexer
        self.sessions = sessions

    def process_one(self) -> bool:
        job = self.repository.claim(self.worker_id)
        if job is None:
            return False
        try:
            with _LeaseHeartbeat(self.repository, job.id, self.worker_id) as lease_lost:
                with self.sessions() as session:
                    from sqlalchemy import select
                    from shared.db.models import KnowledgeSource
                    source = session.scalar(select(KnowledgeSource).where(KnowledgeSource.id == job.source_id))
                    if source is None:
                        raise ValueError("source_missing")
                    source_data = (source.id, source.version, source.name, source.content_type, source.object_uri,
                                   source.linked_product_id)
                source_id, version, name, content_type, object_uri, linked_product_id = source_data
                data = self.object_reader(object_uri)
                with self.sessions() as session:
                    from sqlalchemy import select
                    from shared.db.models import KnowledgeSource
                    source = session.scalar(select(KnowledgeSource).where(KnowledgeSource.id == source_id))
                    if source is None or hashlib.sha256(data).hexdigest() != source.sha256:
                        raise ValueError("source_hash_mismatch")
                chunks = parse_document(name, content_type, data)
                chunks = [(content, {**metadata, "product_id": linked_product_id}) for content, metadata in chunks]
                if lease_lost.is_set():
                    raise RuntimeError("job_lease_lost")
                self.repository.save_chunks(source_id, version, chunks)
                if self.indexer:
                    if lease_lost.is_set():
                        raise RuntimeError("job_lease_lost")
                    self.indexer(source_id, version, chunks)
                if lease_lost.is_set() or not self.repository.complete(job.id, self.worker_id):
                    raise RuntimeError("job_lease_lost")
        except Exception as exc:
            # Persist only a bounded error code; never save file contents or provider payloads.
            code = str(exc)[:64] if isinstance(exc, DocumentParseError) else f"processing_{type(exc).__name__.lower()}"
            self.repository.fail(job.id, self.worker_id, code)
        return True


def _build_milvus_indexer():
    if not GEMINI_API_KEY:
        raise RuntimeError("gemini_api_key_missing")
    from langchain_core.documents import Document
    from vector.langchain_milvus import LangChainMilvusIndexes

    collection = os.getenv("AI_KNOWLEDGE_COLLECTION", "goshopx_knowledge_gemini_v1")
    embeddings = GeminiEmbedding(GEMINI_API_KEY, model=GEMINI_EMBEDDING_MODEL,
                                 dimension=GEMINI_EMBEDDING_DIMENSION,
                                 spend_ledger=ProviderSpendLedger())
    indexes = LangChainMilvusIndexes.connect(uri=MILVUS_URI, text_collection=collection,
                                             image_collection=None, text_embeddings=embeddings)

    def index_chunks(source_id: str, version: int, chunks: list[tuple[str, dict]]) -> None:
        documents = [Document(page_content=content, metadata={**metadata, "source_id": source_id,
                      "source_version": version, "visibility": "draft"}) for content, metadata in chunks]
        ids = [f"{source_id}:{version}:{index}" for index in range(len(documents))]
        if documents:
            indexes.text.add_documents(documents, ids=ids)

    return index_chunks


def run_worker() -> None:
    worker_id = f"{socket.gethostname()}-{uuid.uuid4().hex[:12]}"
    worker = IngestionWorker(IngestionRepository(ReplicaSession), worker_id, indexer=_build_milvus_indexer(),
                             sessions=ReplicaSession)
    while True:
        if not worker.process_one():
            time.sleep(3)
