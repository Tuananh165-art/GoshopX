import grpc
from concurrent import futures
from loguru import logger

from generated.pb import recommender_pb2, recommender_pb2_grpc
from recommendations.load import load_service
from chat.runtime import load_chat_service
from chat.service import ChatRequest
from shared.config.settings import GRPC_PORT
from shared.db.repo import get_products_by_ids
from shared.db.session import get_session


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
                authenticated=bool(request.user_id),
            )
            return recommender_pb2.ChatRecommendationResponse(
                answer=response.answer,
                products=[_to_chat_product(product) for product in response.products],
                next_suggestions=response.next_suggestions,
                warnings=response.warnings,
                request_id=request.request_id,
            )
        except Exception:
            logger.exception("Failed to handle chat recommendation")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details("chat recommendation failed")
            return recommender_pb2.ChatRecommendationResponse(request_id=request.request_id, warnings=["internal_error"])


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
    )


def serve() -> None:
    service = load_service()
    chat_service = load_chat_service()
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    recommender_pb2_grpc.add_RecommenderServiceServicer_to_server(
        RecommenderServiceServicer(service, chat_service),
        server,
    )
    server.add_insecure_port(f"[::]:{GRPC_PORT}")
    logger.info("gRPC server started on port {}", GRPC_PORT)
    server.start()
    server.wait_for_termination()
