import grpc
from concurrent import futures
from loguru import logger
from datetime import timezone
import math
import os

from generated.pb import recommender_pb2, recommender_pb2_grpc
from recommendations.load import load_service
from chat.runtime import load_chat_service
from chat.service import ChatRequest
from shared.config.settings import GRPC_PORT
from shared.db.repo import get_products_by_ids
from shared.db.session import get_session
from ingestion.repository import IngestionConflict, IngestionRepository
from ingestion.parser import DocumentParseError, MAX_DOCUMENT_BYTES, parse_document
from shared.db.session import ReplicaSession


class RecommenderServiceServicer(recommender_pb2_grpc.RecommenderServiceServicer):
    def __init__(self, service, chat_service=None):
        self._service = service
        self._chat_service = chat_service

    def GetRecommendations(self, request, context):
        try:
            product_ids = self._service.for_user(
                user_id=request.user_id,
                skip=request.skip or 0,
                take=request.take or 5,
            )
            return _to_response(product_ids)
        except Exception as exc:
            logger.exception("Failed to get recommendations for user {}", request.user_id)
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(exc))
            return recommender_pb2.RecommendationResponse()

    def GetRecommendationsBasedOnViewed(self, request, context):
        try:
            product_ids = self._service.for_viewed_products(
                product_ids=list(request.ids),
                skip=request.skip or 0,
                take=request.take or 5,
            )
            return _to_response(product_ids)
        except Exception as exc:
            logger.exception("Failed to get viewed-based recommendations")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(exc))
            return recommender_pb2.RecommendationResponse()

    def ChatRecommend(self, request, context):
        if self._chat_service is None:
            context.set_code(grpc.StatusCode.UNAVAILABLE)
            context.set_details("chat recommendation is not configured")
            return recommender_pb2.ChatRecommendationResponse(request_id=request.request_id)
        try:
            response = self._chat_service.reply(
                ChatRequest(
                    text=request.text,
                    image_url=request.image_url or None,
                    user_id=request.user_id or None,
                    session_id=request.session_id or None,
                    viewed_product_ids=tuple(request.viewed_product_ids),
                    limit=request.limit or 5,
                ),
                authenticated=bool(request.user_id and not request.user_id.startswith("guest:")),
            )
            return recommender_pb2.ChatRecommendationResponse(
                answer=response.answer,
                products=[_to_chat_product(product) for product in response.products],
                next_suggestions=response.next_suggestions,
                warnings=response.warnings,
                request_id=request.request_id,
                session_id=response.session_id,
                sources=[recommender_pb2.ChatSource(title=source.get("title", ""), url=source.get("url", ""), published_at=source.get("publishedAt", "")) for source in response.sources],
            )
        except Exception:
            logger.exception("Failed to handle chat recommendation")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details("chat recommendation failed")
            return recommender_pb2.ChatRecommendationResponse(request_id=request.request_id, warnings=["internal_error"])

    def DeleteChatHistory(self, request, context):
        if self._chat_service is None or not request.user_id or not request.session_id:
            context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
            context.set_details("chat owner and session are required")
            return recommender_pb2.DeleteChatHistoryResponse()
        try:
            deleted = self._chat_service.delete_session(
                request.user_id,
                request.session_id,
                authenticated=not request.user_id.startswith("guest:"),
            )
            return recommender_pb2.DeleteChatHistoryResponse(deleted=deleted)
        except Exception:
            logger.exception("Failed to delete chat history")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details("chat history deletion failed")
            return recommender_pb2.DeleteChatHistoryResponse()

    def EnqueueKnowledgeIngestion(self, request, context):
        if not all((request.name, request.object_uri, request.content_type, request.sha256,
                    request.owner_account_id > 0, request.idempotency_key)):
            context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
            context.set_details("invalid ingestion request")
            return recommender_pb2.KnowledgeIngestionJob()
        try:
            source, job = IngestionRepository(ReplicaSession).enqueue(
                source_id=request.source_id, name=request.name, object_uri=request.object_uri,
                content_type=request.content_type, sha256=request.sha256,
                owner_account_id=request.owner_account_id, idempotency_key=request.idempotency_key,
                linked_product_id=request.linked_product_id,
            )
            return _ingestion_job(job, source.name, 0, source.status)
        except ValueError:
            context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
            context.set_details("invalid ingestion source")
            return recommender_pb2.KnowledgeIngestionJob()
        except IngestionConflict:
            context.set_code(grpc.StatusCode.ALREADY_EXISTS)
            context.set_details("ingestion idempotency conflict")
            return recommender_pb2.KnowledgeIngestionJob()
        except Exception:
            logger.exception("Failed to enqueue knowledge ingestion")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details("knowledge ingestion enqueue failed")
            return recommender_pb2.KnowledgeIngestionJob()

    def GetKnowledgeIngestionJob(self, request, context):
        row = IngestionRepository(ReplicaSession).get_job(request.job_id)
        if row is None:
            context.set_code(grpc.StatusCode.NOT_FOUND)
            context.set_details("ingestion job not found")
            return recommender_pb2.KnowledgeIngestionJob()
        return _ingestion_job(*row)

    def ListKnowledgeIngestionJobs(self, request, context):
        rows = IngestionRepository(ReplicaSession).list_jobs(request.limit or 50)
        return recommender_pb2.KnowledgeIngestionJobList(jobs=[_ingestion_job(*row) for row in rows])

    def CancelKnowledgeIngestionJob(self, request, context):
        cancelled = IngestionRepository(ReplicaSession).cancel(request.job_id, request.owner_account_id)
        return recommender_pb2.CancelKnowledgeIngestionJobResponse(cancelled=cancelled)

    def RetryKnowledgeIngestionJob(self, request, context):
        queued = IngestionRepository(ReplicaSession).retry(request.job_id, request.actor_account_id)
        return recommender_pb2.RetryKnowledgeIngestionJobResponse(queued=queued)

    def SetKnowledgeSourceStatus(self, request, context):
        updated = IngestionRepository(ReplicaSession).set_source_status(
            request.source_id, request.status, request.actor_account_id)
        return recommender_pb2.SetKnowledgeSourceStatusResponse(updated=updated)

    def ListIngestionAuditEvents(self, request, context):
        events = IngestionRepository(ReplicaSession).list_audit_events(request.limit or 100)
        return recommender_pb2.IngestionAuditEventList(events=[recommender_pb2.IngestionAuditEvent(
            id=event.id, actor_account_id=event.actor_account_id, action=event.action,
            source_id=event.source_id, job_id=event.job_id, outcome=event.outcome,
            occurred_at_unix=int(event.occurred_at.replace(tzinfo=timezone.utc).timestamp())
            if event.occurred_at and event.occurred_at.tzinfo is None else int(event.occurred_at.timestamp())
            if event.occurred_at else 0,
        ) for event in events])

    def PreviewKnowledgeDocument(self, request, context):
        if not request.name or len(request.content) > MAX_DOCUMENT_BYTES:
            context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
            context.set_details("invalid document preview request")
            return recommender_pb2.KnowledgeDocumentPreview()
        try:
            chunks = parse_document(request.name, request.content_type, request.content)
            characters = sum(len(text) for text, _ in chunks)
            tokens = math.ceil(characters / 4)
            rate = os.getenv("AI_GEMINI_PROMPT_VND_PER_MILLION_TOKENS", "").strip()
            cost_available = rate.isdigit()
            cost = math.ceil(tokens * int(rate) / 1_000_000) if cost_available else 0
            return recommender_pb2.KnowledgeDocumentPreview(
                chunk_count=len(chunks), total_characters=characters,
                estimated_embedding_tokens=tokens, estimated_embedding_cost_vnd=cost,
                cost_estimate_available=cost_available,
                samples=[recommender_pb2.KnowledgeDocumentChunkPreview(
                    index=index, text=text[:500], characters=len(text))
                    for index, (text, _) in enumerate(chunks[:3])],
            )
        except DocumentParseError as exc:
            context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
            context.set_details(str(exc))
            return recommender_pb2.KnowledgeDocumentPreview()
        except Exception:
            logger.exception("Failed to preview knowledge document")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details("knowledge document preview failed")
            return recommender_pb2.KnowledgeDocumentPreview()


def _to_response(product_ids: list[str]) -> recommender_pb2.RecommendationResponse:
    with get_session() as session:
        products = get_products_by_ids(session, product_ids)
    return recommender_pb2.RecommendationResponse(
        recommended_products=[p.to_grpc_model() for p in products]
    )


def _to_chat_product(product: dict) -> recommender_pb2.ChatProduct:
    return recommender_pb2.ChatProduct(
        id=str(product.get("id", "")),
        name=str(product.get("name", "")),
        description=str(product.get("description", "")),
        price=float(product.get("price", 0) or 0),
        currency=str(product.get("currency", "VND")),
        thumbnail=str(product.get("thumbnail", "")),
        images=[str(url) for url in product.get("images", []) if url],
        rating=float(product.get("rating", 0) or 0),
        stock=int(product.get("stock", 0) or 0),
        retrieval_score=float(product.get("retrievalScore", 0) or 0),
        matched_reasons=[str(reason) for reason in product.get("matchedReasons", [])],
        sku=str(product.get("sku", "") or ""),
        discount_percentage=float(product.get("discountPercentage", 0) or 0),
        warranty_information=str(product.get("warrantyInformation", "") or ""),
        shipping_information=str(product.get("shippingInformation", "") or ""),
        return_policy=str(product.get("returnPolicy", "") or ""),
        availability_status=str(product.get("availabilityStatus", "") or ""),
        brand=str(product.get("brand", "") or ""),
    )


def _ingestion_job(job, name: str, chunk_count: int, source_status: str):
    created_at = job.created_at
    if created_at and created_at.tzinfo is None:
        created_at = created_at.replace(tzinfo=timezone.utc)
    return recommender_pb2.KnowledgeIngestionJob(
        id=job.id, source_id=job.source_id, name=name, status=job.status,
        attempts=job.attempts, error_code=job.error_code or "",
        created_at_unix=int(created_at.timestamp()) if created_at else 0,
        chunk_count=int(chunk_count),
        source_status=source_status,
    )


def serve() -> None:
    service = load_service()
    chat_service = load_chat_service()
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10), options=[("grpc.max_receive_message_length", 21 * 1024 * 1024)])
    recommender_pb2_grpc.add_RecommenderServiceServicer_to_server(
        RecommenderServiceServicer(service, chat_service),
        server,
    )
    server.add_insecure_port(f"[::]:{GRPC_PORT}")
    logger.info("gRPC server started on port {}", GRPC_PORT)
    server.start()
    server.wait_for_termination()
