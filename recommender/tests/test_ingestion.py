import hashlib
import io
import json
import os
import unittest
import uuid
import zipfile
from types import SimpleNamespace
from unittest.mock import patch

from sqlalchemy import create_engine, select
from sqlalchemy.orm import sessionmaker

os.environ.setdefault("DATABASE_URL", "sqlite://")

from ingestion.parser import DocumentParseError, parse_document
from ingestion.repository import IngestionConflict, IngestionRepository
from ingestion.worker import IngestionWorker, read_gcs_object
from entry.grpc_server import RecommenderServiceServicer
from generated.pb import recommender_pb2
from shared.db.models import Base, IngestionJob, KnowledgeChunk, KnowledgeSource


class IngestionTests(unittest.TestCase):
    def setUp(self):
        self.engine = create_engine("sqlite:///:memory:")
        Base.metadata.create_all(self.engine)
        self.sessions = sessionmaker(bind=self.engine)
        self.repository = IngestionRepository(self.sessions)

    def tearDown(self):
        self.engine.dispose()

    def test_admin_document_preview_returns_bounded_chunks_and_estimated_cost(self):
        class Context:
            code = None
            details = None
            def set_code(self, value): self.code = value
            def set_details(self, value): self.details = value

        context = Context()
        request = recommender_pb2.PreviewKnowledgeDocumentRequest(
            name="policy.txt", content_type="text/plain", content=b"Warranty details")
        with patch.dict(os.environ, {"AI_GEMINI_PROMPT_VND_PER_MILLION_TOKENS": "1000000"}):
            result = RecommenderServiceServicer(None).PreviewKnowledgeDocument(request, context)
        self.assertIsNone(context.code)
        self.assertEqual((result.chunk_count, result.total_characters), (1, 16))
        self.assertEqual(result.estimated_embedding_tokens, 4)
        self.assertEqual(result.estimated_embedding_cost_vnd, 4)
        self.assertTrue(result.cost_estimate_available)
        self.assertEqual(result.samples[0].text, "Warranty details")

    def test_admin_document_preview_rejects_invalid_source_without_persisting(self):
        class Context:
            code = None
            details = None
            def set_code(self, value): self.code = value
            def set_details(self, value): self.details = value

        context = Context()
        request = recommender_pb2.PreviewKnowledgeDocumentRequest(
            name="scan.txt", content_type="text/plain", content=b"\xff")
        result = RecommenderServiceServicer(None).PreviewKnowledgeDocument(request, context)
        self.assertEqual(context.code, __import__("grpc").StatusCode.INVALID_ARGUMENT)
        self.assertEqual(context.details, "text_encoding_unsupported")
        self.assertEqual(result.chunk_count, 0)

    def test_chunks_keep_overlap_and_source_position(self):
        text = "x" * 1500
        chunks = parse_document("notes.txt", "text/plain", text.encode())
        self.assertEqual([len(item[0]) for item in chunks], [800, 800, 140])
        self.assertEqual(chunks[0][0][-120:], chunks[1][0][:120])
        self.assertEqual(chunks[1][1]["start"], 680)

    def test_json_csv_and_docx_are_parsed_without_external_document_code(self):
        self.assertIn('"name": "camera"', parse_document("catalog.json", "application/json", b'{"name":"camera"}')[0][0])
        csv_chunks = parse_document("catalog.csv", "text/csv", b"sku,name\nA1,Camera\n")
        self.assertIn('"sku": "A1"', csv_chunks[0][0])
        data = io.BytesIO()
        with zipfile.ZipFile(data, "w") as archive:
            archive.writestr("word/document.xml", '<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Policy text</w:t></w:r></w:p></w:body></w:document>')
        self.assertEqual(parse_document("policy.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", data.getvalue())[0][0], "Policy text")

    def test_pdf_chunks_keep_page_provenance_and_reject_scanned_files(self):
        fake_reader = SimpleNamespace(PdfReader=lambda *_args, **_kwargs: SimpleNamespace(
            is_encrypted=False, pages=[SimpleNamespace(extract_text=lambda: "Warranty details")]))
        with patch.dict("sys.modules", {"pypdf": fake_reader}):
            chunks = parse_document("guide.pdf", "application/pdf", b"%PDF-1.7")
            self.assertEqual(chunks[0][0], "Warranty details")
            self.assertEqual(chunks[0][1]["page"], 1)

            fake_reader.PdfReader = lambda *_args, **_kwargs: SimpleNamespace(
                is_encrypted=False, pages=[SimpleNamespace(extract_text=lambda: "")])
            with self.assertRaisesRegex(DocumentParseError, "pdf_requires_ocr"):
                parse_document("scan.pdf", "application/pdf", b"%PDF-1.7")

    def test_parser_rejects_mismatched_type_and_oversized_source(self):
        with self.assertRaisesRegex(DocumentParseError, "document_type_mismatch"):
            parse_document("guide.txt", "application/pdf", b"text")
        with self.assertRaisesRegex(DocumentParseError, "document_size_invalid"):
            parse_document("guide.txt", "text/plain", b"x" * (20 * 1024 * 1024 + 1))
        with self.assertRaisesRegex(DocumentParseError, "docx_xml_entities_rejected"):
            parse_document("unsafe.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
                           b"<!DOCTYPE x [<!ENTITY y 'z'>]>")

    def test_gcs_reader_restricts_sources_to_knowledge_prefix(self):
        os.environ["GCS_MEDIA_BUCKET"] = "private-bucket"
        with self.assertRaisesRegex(ValueError, "gcs_source_not_allowed"):
            read_gcs_object("gs://private-bucket/chat-upload/user-image.png")
        with self.assertRaisesRegex(ValueError, "gcs_source_not_allowed"):
            read_gcs_object("gs://private-bucket/knowledge-upload/../chat-upload/image.png")

    def test_enqueue_is_idempotent_and_rejects_key_reuse(self):
        digest = hashlib.sha256(b"hello").hexdigest()
        source_id = str(uuid.uuid4())
        source, job = self.repository.enqueue(source_id=source_id, name="guide.txt", object_uri="gs://bucket/a",
            content_type="text/plain", sha256=digest, owner_account_id=7, idempotency_key="request-1")
        source_again, job_again = self.repository.enqueue(source_id=str(uuid.uuid4()), name="guide.txt", object_uri="gs://bucket/a",
            content_type="text/plain", sha256=digest, owner_account_id=7, idempotency_key="request-1")
        self.assertEqual((source.id, job.id), (source_again.id, job_again.id))
        with self.assertRaises(IngestionConflict):
            self.repository.enqueue(source_id=str(uuid.uuid4()), name="other.txt", object_uri="gs://bucket/b",
                content_type="text/plain", sha256=hashlib.sha256(b"other").hexdigest(), owner_account_id=7,
                idempotency_key="request-1")

    def test_admin_status_list_and_cancel_are_owner_scoped(self):
        digest = hashlib.sha256(b"hello").hexdigest()
        source, job = self.repository.enqueue(source_id=str(uuid.uuid4()), name="guide.txt", object_uri="gs://bucket/status",
            content_type="text/plain", sha256=digest, owner_account_id=7, idempotency_key="status-request")
        status = self.repository.get_job(job.id)
        self.assertEqual((status[0].id, status[1], status[2]), (job.id, "guide.txt", 0))
        self.assertEqual(len(self.repository.list_jobs(50)), 1)
        self.assertFalse(self.repository.cancel(job.id, 8))
        self.assertTrue(self.repository.cancel(job.id, 7))
        self.assertEqual(self.repository.get_job(job.id)[0].status, "cancelled")
        events = self.repository.list_audit_events()
        self.assertEqual([event.action for event in events], ["knowledge.ingestion.cancel", "knowledge.ingestion.enqueue"])
        self.assertEqual(events[0].actor_account_id, 7)

    def test_publish_requires_completed_chunks_and_chat_search_hides_drafts(self):
        payload = b"Returns policy for laptops and electronics."
        source, job = self.repository.enqueue(source_id=str(uuid.uuid4()), name="returns.txt",
            object_uri="gs://bucket/returns", content_type="text/plain", sha256=hashlib.sha256(payload).hexdigest(),
            owner_account_id=7, idempotency_key="publish-source")
        self.assertFalse(self.repository.set_source_status(source.id, "published", 7))
        worker = IngestionWorker(self.repository, "worker", object_reader=lambda _: payload, sessions=self.sessions)
        self.assertTrue(worker.process_one())
        self.assertTrue(self.repository.set_source_status(source.id, "published", 9))
        results = self.repository.search_published("returns policy")
        self.assertEqual(results[0]["source_id"], source.id)
        self.assertFalse(self.repository.set_source_status(source.id, "unknown", 9))
        self.assertTrue(self.repository.set_source_status(source.id, "revoked", 9))
        audit = self.repository.list_audit_events()
        actions = {event.action for event in audit}
        self.assertTrue({"knowledge.source.revoked", "knowledge.source.published", "knowledge.ingestion.enqueue"}.issubset(actions))
        self.assertTrue(all(event.actor_account_id in {7, 9} for event in audit))
        self.assertEqual(self.repository.search_published("returns policy"), [])

    def test_linked_knowledge_is_returned_only_for_its_shortlisted_product(self):
        payload = b"Battery warranty details for this catalog item."
        source, _ = self.repository.enqueue(source_id=str(uuid.uuid4()), name="battery.txt",
            object_uri="gs://bucket/battery", content_type="text/plain",
            sha256=hashlib.sha256(payload).hexdigest(), owner_account_id=7,
            idempotency_key="linked-source", linked_product_id="phone-42")
        worker = IngestionWorker(self.repository, "worker", object_reader=lambda _: payload, sessions=self.sessions)
        self.assertTrue(worker.process_one())
        self.assertTrue(self.repository.set_source_status(source.id, "published", 9))
        self.assertEqual(self.repository.search_published("battery warranty"), [])
        result = self.repository.search_published("battery warranty", product_ids=["phone-42"])
        self.assertEqual(result[0]["product_id"], "phone-42")
        with self.sessions() as session:
            chunk = session.scalar(select(KnowledgeChunk).where(KnowledgeChunk.source_id == source.id))
            self.assertEqual(json.loads(chunk.metadata_json)["product_id"], "phone-42")

    def test_failed_ingestion_can_be_admin_requeued(self):
        source, job = self.repository.enqueue(source_id=str(uuid.uuid4()), name="retry.txt",
            object_uri="gs://bucket/retry", content_type="text/plain", sha256=hashlib.sha256(b"x").hexdigest(),
            owner_account_id=7, idempotency_key="retry-source")
        for attempt in range(3):
            claimed = self.repository.claim(f"worker-{attempt}")
            self.assertEqual(claimed.id, job.id)
            self.assertTrue(self.repository.fail(job.id, f"worker-{attempt}", "failed"))
        self.assertTrue(self.repository.retry(job.id, 9))
        self.assertEqual(self.repository.get_job(job.id)[0].status, "queued")
        self.assertEqual(self.repository.get_job(job.id)[0].attempts, 0)

    def test_worker_persists_chunks_and_completes_retryably(self):
        payload = b"Product support policy and warranty details."
        digest = hashlib.sha256(payload).hexdigest()
        source, job = self.repository.enqueue(source_id=str(uuid.uuid4()), name="policy.txt", object_uri="gs://bucket/policy",
            content_type="text/plain", sha256=digest, owner_account_id=7, idempotency_key="request-2")
        worker = IngestionWorker(self.repository, "worker-1", object_reader=lambda uri: payload, sessions=self.sessions)
        self.assertTrue(worker.process_one())
        with self.sessions() as session:
            self.assertEqual(session.get(IngestionJob, job.id).status, "completed")
            chunks = session.scalars(select(KnowledgeChunk).where(KnowledgeChunk.source_id == source.id)).all()
            self.assertEqual(len(chunks), 1)
            self.assertEqual(json.loads(chunks[0].metadata_json)["format"], "txt")

    def test_worker_does_not_persist_chunks_for_hash_mismatch(self):
        source, job = self.repository.enqueue(source_id=str(uuid.uuid4()), name="policy.txt", object_uri="gs://bucket/policy",
            content_type="text/plain", sha256=hashlib.sha256(b"expected").hexdigest(), owner_account_id=7,
            idempotency_key="request-3")
        worker = IngestionWorker(self.repository, "worker-1", object_reader=lambda uri: b"different", sessions=self.sessions)
        self.assertTrue(worker.process_one())
        with self.sessions() as session:
            self.assertEqual(session.get(IngestionJob, job.id).status, "queued")
            self.assertEqual(session.scalars(select(KnowledgeChunk).where(KnowledgeChunk.source_id == source.id)).all(), [])

    def test_failed_jobs_retry_three_times_then_stop(self):
        source, job = self.repository.enqueue(source_id=str(uuid.uuid4()), name="policy.txt", object_uri="gs://bucket/policy",
            content_type="text/plain", sha256=hashlib.sha256(b"x").hexdigest(), owner_account_id=7,
            idempotency_key="request-4")
        for attempt in range(1, 4):
            claimed = self.repository.claim(f"worker-{attempt}")
            self.assertEqual(claimed.id, job.id)
            self.assertTrue(self.repository.fail(job.id, f"worker-{attempt}", "temporary_error"))
            with self.sessions() as session:
                stored = session.get(IngestionJob, job.id)
                self.assertEqual(stored.status, "failed" if attempt == 3 else "queued")
                self.assertEqual(stored.attempts, attempt)
        self.assertIsNone(self.repository.claim("worker-final"))


if __name__ == "__main__":
    unittest.main()
