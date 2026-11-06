from __future__ import annotations

from datetime import datetime, timedelta, timezone
import hashlib
import json
import re
import uuid

from sqlalchemy import func, or_, select
from sqlalchemy.exc import IntegrityError
from sqlalchemy.orm import Session, sessionmaker

from shared.db.models import IngestionAuditEvent, IngestionJob, KnowledgeChunk, KnowledgeSource


class IngestionConflict(RuntimeError):
    pass


class IngestionRepository:
    """Durable PostgreSQL queue; processing is at-least-once and idempotent."""

    def __init__(self, sessions: sessionmaker[Session]):
        self._sessions = sessions

    def enqueue(self, *, source_id: str, name: str, object_uri: str, content_type: str,
                sha256: str, owner_account_id: int, idempotency_key: str,
                linked_product_id: str = "") -> tuple[KnowledgeSource, IngestionJob]:
        if not re.fullmatch(r"[0-9a-f]{64}", sha256) or not idempotency_key or len(idempotency_key) > 128:
            raise ValueError("invalid source hash or idempotency key")
        with self._sessions() as session, session.begin():
            existing = session.scalar(select(IngestionJob).where(IngestionJob.idempotency_key == idempotency_key).with_for_update())
            if existing:
                source = session.get(KnowledgeSource, existing.source_id)
                if source is None or source.sha256 != sha256 or source.owner_account_id != owner_account_id:
                    raise IngestionConflict("idempotency key was already used for another source")
                session.expunge(source)
                session.expunge(existing)
                return source, existing
            source = KnowledgeSource(id=source_id or str(uuid.uuid4()), name=name[:255], object_uri=object_uri,
                                     content_type=content_type, sha256=sha256, owner_account_id=owner_account_id,
                                     linked_product_id=linked_product_id[:128], status="draft", version=1, metadata_json="{}")
            job = IngestionJob(id=str(uuid.uuid4()), source_id=source.id, idempotency_key=idempotency_key,
                               status="queued", attempts=0, max_attempts=3)
            session.add_all([source, job])
            try:
                session.flush()
            except IntegrityError as exc:
                raise IngestionConflict("source object or idempotency key already exists") from exc
            session.add(IngestionAuditEvent(id=str(uuid.uuid4()), actor_account_id=owner_account_id,
                                            action="knowledge.ingestion.enqueue", source_id=source.id,
                                            job_id=job.id, outcome="accepted"))
            session.expunge(source)
            session.expunge(job)
            return source, job

    def claim(self, worker_id: str, *, lease_seconds: int = 300) -> IngestionJob | None:
        now = datetime.now(timezone.utc)
        with self._sessions() as session, session.begin():
            while True:
                query = (select(IngestionJob)
                         .where(or_((IngestionJob.status == "queued") & (IngestionJob.attempts < IngestionJob.max_attempts),
                                    (IngestionJob.status == "running") & (IngestionJob.lease_expires_at < now)))
                         .order_by(IngestionJob.created_at, IngestionJob.id)
                         .limit(1))
                if session.bind and session.bind.dialect.name == "postgresql":
                    query = query.with_for_update(skip_locked=True)
                job = session.scalar(query)
                if job is None:
                    return None
                if job.attempts >= job.max_attempts:
                    job.status = "failed"
                    job.error_code = "lease_expired_after_max_attempts"
                    job.lease_owner = None
                    job.lease_expires_at = None
                    continue
                break
            job.status = "running"
            job.attempts += 1
            job.lease_owner = worker_id[:128]
            job.lease_expires_at = now + timedelta(seconds=max(10, min(lease_seconds, 600)))
            job.error_code = None
            session.flush()
            session.expunge(job)
            return job

    def heartbeat(self, job_id: str, worker_id: str, *, lease_seconds: int = 300) -> bool:
        now = datetime.now(timezone.utc)
        with self._sessions() as session, session.begin():
            job = session.scalar(select(IngestionJob).where(IngestionJob.id == job_id).with_for_update())
            if job is None or job.status != "running" or job.lease_owner != worker_id:
                return False
            job.lease_expires_at = now + timedelta(seconds=max(10, min(lease_seconds, 600)))
            return True

    def complete(self, job_id: str, worker_id: str) -> bool:
        return self._finish(job_id, worker_id, "completed", None)

    def fail(self, job_id: str, worker_id: str, error_code: str) -> bool:
        safe_code = re.sub(r"[^a-zA-Z0-9_.-]", "_", error_code)[:64] or "processing_error"
        return self._finish(job_id, worker_id, "queued", safe_code, retryable=True)

    def _finish(self, job_id: str, worker_id: str, status: str, error_code: str | None, retryable: bool = False) -> bool:
        with self._sessions() as session, session.begin():
            job = session.scalar(select(IngestionJob).where(IngestionJob.id == job_id).with_for_update())
            if job is None or job.status != "running" or job.lease_owner != worker_id:
                return False
            job.status = "queued" if retryable and job.attempts < job.max_attempts else ("failed" if retryable else status)
            job.error_code = error_code
            job.lease_owner = None
            job.lease_expires_at = None
            return True

    def cancel(self, job_id: str, owner_account_id: int) -> bool:
        with self._sessions() as session, session.begin():
            job = session.scalar(select(IngestionJob).where(IngestionJob.id == job_id).with_for_update())
            source = session.get(KnowledgeSource, job.source_id) if job else None
            if job is None or source is None or source.owner_account_id != owner_account_id or job.status != "queued":
                return False
            job.status = "cancelled"
            session.add(IngestionAuditEvent(id=str(uuid.uuid4()), actor_account_id=owner_account_id,
                                            action="knowledge.ingestion.cancel", source_id=source.id,
                                            job_id=job.id, outcome="success"))
            return True

    def retry(self, job_id: str, actor_account_id: int) -> bool:
        if actor_account_id <= 0:
            return False
        with self._sessions() as session, session.begin():
            job = session.scalar(select(IngestionJob).where(IngestionJob.id == job_id).with_for_update())
            if job is None or job.status != "failed":
                return False
            job.status = "queued"
            job.attempts = 0
            job.error_code = None
            session.add(IngestionAuditEvent(id=str(uuid.uuid4()), actor_account_id=actor_account_id,
                                            action="knowledge.ingestion.retry", source_id=job.source_id,
                                            job_id=job.id, outcome="success"))
            return True

    def set_source_status(self, source_id: str, status: str, actor_account_id: int) -> bool:
        if status not in {"published", "revoked"}:
            return False
        if actor_account_id <= 0:
            return False
        with self._sessions() as session, session.begin():
            source = session.scalar(select(KnowledgeSource).where(KnowledgeSource.id == source_id).with_for_update())
            if source is None:
                return False
            if status == "published":
                completed = session.scalar(select(IngestionJob.id).where(
                    IngestionJob.source_id == source_id, IngestionJob.status == "completed"))
                chunks = session.scalar(select(func.count(KnowledgeChunk.id)).where(
                    KnowledgeChunk.source_id == source_id, KnowledgeChunk.source_version == source.version))
                if completed is None or not chunks:
                    return False
            source.status = status
            session.add(IngestionAuditEvent(id=str(uuid.uuid4()), actor_account_id=actor_account_id,
                                            action=f"knowledge.source.{status}", source_id=source.id,
                                            outcome="success"))
            return True

    def list_audit_events(self, limit: int = 100) -> list[IngestionAuditEvent]:
        with self._sessions() as session:
            events = session.scalars(select(IngestionAuditEvent)
                                     .order_by(IngestionAuditEvent.occurred_at.desc(), IngestionAuditEvent.id)
                                     .limit(max(1, min(limit, 200)))).all()
            for event in events:
                session.expunge(event)
            return events

    def search_published(self, query: str, limit: int = 5, dense_source_ids: list[str] | None = None,
                         product_ids: list[str] | None = None) -> list[dict[str, str]]:
        terms = list(dict.fromkeys(re.findall(r"[\w-]{2,}", query.casefold())))[:12]
        if not terms:
            return []
        with self._sessions() as session:
            rows = session.execute(
                select(KnowledgeSource.id, KnowledgeSource.name, KnowledgeChunk.content,
                       KnowledgeSource.linked_product_id)
                .join(KnowledgeChunk, KnowledgeChunk.source_id == KnowledgeSource.id)
                .where(KnowledgeSource.status == "published",
                       KnowledgeChunk.source_version == KnowledgeSource.version)
                .limit(1000)
            ).all()
        allowed_products = set(product_ids or [])
        rows = [row for row in rows if not row[3] or row[3] in allowed_products]
        scored_rows = []
        for source_id, name, content, linked_product_id in rows:
            text = content.casefold()
            score = sum(text.count(term) for term in terms)
            if score:
                scored_rows.append((score, source_id, name, content))
        scored_rows.sort(key=lambda row: (-row[0], row[1]))
        lexical_rank = {source_id: rank for rank, (_, source_id, _, _) in enumerate(scored_rows)}
        dense_rank = {source_id: index for index, source_id in enumerate(dense_source_ids or [])}
        ranked = []
        for source_id, name, content, linked_product_id in rows:
            lexical_position = lexical_rank.get(source_id)
            dense_position = dense_rank.get(source_id)
            if lexical_position is not None or dense_position is not None:
                fused = (1 / (60 + 1 + dense_position) if dense_position is not None else 0) + (1 / (60 + 1 + lexical_position) if lexical_position is not None else 0)
                ranked.append((fused, source_id, name, content, linked_product_id))
        ranked.sort(key=lambda row: (-row[0], row[1]))
        return [{"source_id": source_id, "title": name, "content": content[:1200], "product_id": linked_product_id}
                for _, source_id, name, content, linked_product_id in ranked[:max(1, min(limit, 10))]]

    def has_published_sources(self) -> bool:
        with self._sessions() as session:
            return session.scalar(select(KnowledgeSource.id).where(KnowledgeSource.status == "published").limit(1)) is not None

    def get_job(self, job_id: str):
        with self._sessions() as session:
            return session.execute(
                select(IngestionJob, KnowledgeSource.name,
                       select(func.count(KnowledgeChunk.id)).where(
                           KnowledgeChunk.source_id == KnowledgeSource.id,
                           KnowledgeChunk.source_version == KnowledgeSource.version).scalar_subquery(),
                       KnowledgeSource.status)
                .join(KnowledgeSource, KnowledgeSource.id == IngestionJob.source_id)
                .where(IngestionJob.id == job_id)
            ).first()

    def list_jobs(self, limit: int = 50):
        with self._sessions() as session:
            return session.execute(
                select(IngestionJob, KnowledgeSource.name,
                       select(func.count(KnowledgeChunk.id)).where(
                           KnowledgeChunk.source_id == KnowledgeSource.id,
                           KnowledgeChunk.source_version == KnowledgeSource.version).scalar_subquery(),
                       KnowledgeSource.status)
                .join(KnowledgeSource, KnowledgeSource.id == IngestionJob.source_id)
                .order_by(IngestionJob.created_at.desc(), IngestionJob.id)
                .limit(max(1, min(limit, 100)))
            ).all()

    def save_chunks(self, source_id: str, source_version: int, chunks: list[tuple[str, dict]]) -> None:
        with self._sessions() as session, session.begin():
            source = session.scalar(select(KnowledgeSource).where(KnowledgeSource.id == source_id).with_for_update())
            if source is None or source.version != source_version or source.status == "revoked":
                raise IngestionConflict("source version changed or source is revoked")
            session.query(KnowledgeChunk).filter_by(source_id=source_id, source_version=source_version).delete()
            for index, (content, metadata) in enumerate(chunks):
                chunk_hash = hashlib.sha256(content.encode("utf-8")).hexdigest()
                session.add(KnowledgeChunk(id=str(uuid.uuid4()), source_id=source_id,
                                           source_version=source_version, chunk_index=index,
                                           content=content, content_hash=chunk_hash,
                                           metadata_json=json.dumps(metadata, ensure_ascii=False, sort_keys=True)))
