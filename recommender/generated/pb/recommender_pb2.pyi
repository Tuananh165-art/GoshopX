from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class RecommendationRequestForUserId(_message.Message):
    __slots__ = ("user_id", "skip", "take")
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    SKIP_FIELD_NUMBER: _ClassVar[int]
    TAKE_FIELD_NUMBER: _ClassVar[int]
    user_id: str
    skip: int
    take: int
    def __init__(self, user_id: _Optional[str] = ..., skip: _Optional[int] = ..., take: _Optional[int] = ...) -> None: ...

class RecommendationRequestOnViews(_message.Message):
    __slots__ = ("ids", "skip", "take")
    IDS_FIELD_NUMBER: _ClassVar[int]
    SKIP_FIELD_NUMBER: _ClassVar[int]
    TAKE_FIELD_NUMBER: _ClassVar[int]
    ids: _containers.RepeatedScalarFieldContainer[str]
    skip: int
    take: int
    def __init__(self, ids: _Optional[_Iterable[str]] = ..., skip: _Optional[int] = ..., take: _Optional[int] = ...) -> None: ...

class ProductReplica(_message.Message):
    __slots__ = ("id", "name", "description", "price")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    PRICE_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    description: str
    price: float
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., description: _Optional[str] = ..., price: _Optional[float] = ...) -> None: ...

class RecommendationResponse(_message.Message):
    __slots__ = ("recommended_products",)
    RECOMMENDED_PRODUCTS_FIELD_NUMBER: _ClassVar[int]
    recommended_products: _containers.RepeatedCompositeFieldContainer[ProductReplica]
    def __init__(self, recommended_products: _Optional[_Iterable[_Union[ProductReplica, _Mapping]]] = ...) -> None: ...

class ChatRecommendationRequest(_message.Message):
    __slots__ = ("text", "image_url", "user_id", "session_id", "viewed_product_ids", "limit", "request_id")
    TEXT_FIELD_NUMBER: _ClassVar[int]
    IMAGE_URL_FIELD_NUMBER: _ClassVar[int]
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    VIEWED_PRODUCT_IDS_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    REQUEST_ID_FIELD_NUMBER: _ClassVar[int]
    text: str
    image_url: str
    user_id: str
    session_id: str
    viewed_product_ids: _containers.RepeatedScalarFieldContainer[str]
    limit: int
    request_id: str
    def __init__(self, text: _Optional[str] = ..., image_url: _Optional[str] = ..., user_id: _Optional[str] = ..., session_id: _Optional[str] = ..., viewed_product_ids: _Optional[_Iterable[str]] = ..., limit: _Optional[int] = ..., request_id: _Optional[str] = ...) -> None: ...

class ChatProduct(_message.Message):
    __slots__ = ("id", "name", "description", "price", "currency", "thumbnail", "images", "rating", "stock", "retrieval_score", "matched_reasons", "sku", "discount_percentage", "warranty_information", "shipping_information", "return_policy", "availability_status", "brand")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    PRICE_FIELD_NUMBER: _ClassVar[int]
    CURRENCY_FIELD_NUMBER: _ClassVar[int]
    THUMBNAIL_FIELD_NUMBER: _ClassVar[int]
    IMAGES_FIELD_NUMBER: _ClassVar[int]
    RATING_FIELD_NUMBER: _ClassVar[int]
    STOCK_FIELD_NUMBER: _ClassVar[int]
    RETRIEVAL_SCORE_FIELD_NUMBER: _ClassVar[int]
    MATCHED_REASONS_FIELD_NUMBER: _ClassVar[int]
    SKU_FIELD_NUMBER: _ClassVar[int]
    DISCOUNT_PERCENTAGE_FIELD_NUMBER: _ClassVar[int]
    WARRANTY_INFORMATION_FIELD_NUMBER: _ClassVar[int]
    SHIPPING_INFORMATION_FIELD_NUMBER: _ClassVar[int]
    RETURN_POLICY_FIELD_NUMBER: _ClassVar[int]
    AVAILABILITY_STATUS_FIELD_NUMBER: _ClassVar[int]
    BRAND_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    description: str
    price: float
    currency: str
    thumbnail: str
    images: _containers.RepeatedScalarFieldContainer[str]
    rating: float
    stock: int
    retrieval_score: float
    matched_reasons: _containers.RepeatedScalarFieldContainer[str]
    sku: str
    discount_percentage: float
    warranty_information: str
    shipping_information: str
    return_policy: str
    availability_status: str
    brand: str
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., description: _Optional[str] = ..., price: _Optional[float] = ..., currency: _Optional[str] = ..., thumbnail: _Optional[str] = ..., images: _Optional[_Iterable[str]] = ..., rating: _Optional[float] = ..., stock: _Optional[int] = ..., retrieval_score: _Optional[float] = ..., matched_reasons: _Optional[_Iterable[str]] = ..., sku: _Optional[str] = ..., discount_percentage: _Optional[float] = ..., warranty_information: _Optional[str] = ..., shipping_information: _Optional[str] = ..., return_policy: _Optional[str] = ..., availability_status: _Optional[str] = ..., brand: _Optional[str] = ...) -> None: ...

class ChatRecommendationResponse(_message.Message):
    __slots__ = ("answer", "products", "next_suggestions", "warnings", "request_id", "session_id", "sources")
    ANSWER_FIELD_NUMBER: _ClassVar[int]
    PRODUCTS_FIELD_NUMBER: _ClassVar[int]
    NEXT_SUGGESTIONS_FIELD_NUMBER: _ClassVar[int]
    WARNINGS_FIELD_NUMBER: _ClassVar[int]
    REQUEST_ID_FIELD_NUMBER: _ClassVar[int]
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    SOURCES_FIELD_NUMBER: _ClassVar[int]
    answer: str
    products: _containers.RepeatedCompositeFieldContainer[ChatProduct]
    next_suggestions: _containers.RepeatedScalarFieldContainer[str]
    warnings: _containers.RepeatedScalarFieldContainer[str]
    request_id: str
    session_id: str
    sources: _containers.RepeatedCompositeFieldContainer[ChatSource]
    def __init__(self, answer: _Optional[str] = ..., products: _Optional[_Iterable[_Union[ChatProduct, _Mapping]]] = ..., next_suggestions: _Optional[_Iterable[str]] = ..., warnings: _Optional[_Iterable[str]] = ..., request_id: _Optional[str] = ..., session_id: _Optional[str] = ..., sources: _Optional[_Iterable[_Union[ChatSource, _Mapping]]] = ...) -> None: ...

class ChatSource(_message.Message):
    __slots__ = ("title", "url", "published_at")
    TITLE_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    PUBLISHED_AT_FIELD_NUMBER: _ClassVar[int]
    title: str
    url: str
    published_at: str
    def __init__(self, title: _Optional[str] = ..., url: _Optional[str] = ..., published_at: _Optional[str] = ...) -> None: ...

class DeleteChatHistoryRequest(_message.Message):
    __slots__ = ("user_id", "session_id")
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    user_id: str
    session_id: str
    def __init__(self, user_id: _Optional[str] = ..., session_id: _Optional[str] = ...) -> None: ...

class DeleteChatHistoryResponse(_message.Message):
    __slots__ = ("deleted",)
    DELETED_FIELD_NUMBER: _ClassVar[int]
    deleted: bool
    def __init__(self, deleted: _Optional[bool] = ...) -> None: ...

class EnqueueKnowledgeIngestionRequest(_message.Message):
    __slots__ = ("source_id", "name", "object_uri", "content_type", "sha256", "owner_account_id", "idempotency_key", "linked_product_id")
    SOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    OBJECT_URI_FIELD_NUMBER: _ClassVar[int]
    CONTENT_TYPE_FIELD_NUMBER: _ClassVar[int]
    SHA256_FIELD_NUMBER: _ClassVar[int]
    OWNER_ACCOUNT_ID_FIELD_NUMBER: _ClassVar[int]
    IDEMPOTENCY_KEY_FIELD_NUMBER: _ClassVar[int]
    LINKED_PRODUCT_ID_FIELD_NUMBER: _ClassVar[int]
    source_id: str
    name: str
    object_uri: str
    content_type: str
    sha256: str
    owner_account_id: int
    idempotency_key: str
    linked_product_id: str
    def __init__(self, source_id: _Optional[str] = ..., name: _Optional[str] = ..., object_uri: _Optional[str] = ..., content_type: _Optional[str] = ..., sha256: _Optional[str] = ..., owner_account_id: _Optional[int] = ..., idempotency_key: _Optional[str] = ..., linked_product_id: _Optional[str] = ...) -> None: ...

class KnowledgeIngestionJobRequest(_message.Message):
    __slots__ = ("job_id",)
    JOB_ID_FIELD_NUMBER: _ClassVar[int]
    job_id: str
    def __init__(self, job_id: _Optional[str] = ...) -> None: ...

class ListKnowledgeIngestionJobsRequest(_message.Message):
    __slots__ = ("limit",)
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    limit: int
    def __init__(self, limit: _Optional[int] = ...) -> None: ...

class CancelKnowledgeIngestionJobRequest(_message.Message):
    __slots__ = ("job_id", "owner_account_id")
    JOB_ID_FIELD_NUMBER: _ClassVar[int]
    OWNER_ACCOUNT_ID_FIELD_NUMBER: _ClassVar[int]
    job_id: str
    owner_account_id: int
    def __init__(self, job_id: _Optional[str] = ..., owner_account_id: _Optional[int] = ...) -> None: ...

class KnowledgeIngestionJob(_message.Message):
    __slots__ = ("id", "source_id", "name", "status", "attempts", "error_code", "created_at_unix", "chunk_count", "source_status")
    ID_FIELD_NUMBER: _ClassVar[int]
    SOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    ATTEMPTS_FIELD_NUMBER: _ClassVar[int]
    ERROR_CODE_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_UNIX_FIELD_NUMBER: _ClassVar[int]
    CHUNK_COUNT_FIELD_NUMBER: _ClassVar[int]
    SOURCE_STATUS_FIELD_NUMBER: _ClassVar[int]
    id: str
    source_id: str
    name: str
    status: str
    attempts: int
    error_code: str
    created_at_unix: int
    chunk_count: int
    source_status: str
    def __init__(self, id: _Optional[str] = ..., source_id: _Optional[str] = ..., name: _Optional[str] = ..., status: _Optional[str] = ..., attempts: _Optional[int] = ..., error_code: _Optional[str] = ..., created_at_unix: _Optional[int] = ..., chunk_count: _Optional[int] = ..., source_status: _Optional[str] = ...) -> None: ...

class KnowledgeIngestionJobList(_message.Message):
    __slots__ = ("jobs",)
    JOBS_FIELD_NUMBER: _ClassVar[int]
    jobs: _containers.RepeatedCompositeFieldContainer[KnowledgeIngestionJob]
    def __init__(self, jobs: _Optional[_Iterable[_Union[KnowledgeIngestionJob, _Mapping]]] = ...) -> None: ...

class CancelKnowledgeIngestionJobResponse(_message.Message):
    __slots__ = ("cancelled",)
    CANCELLED_FIELD_NUMBER: _ClassVar[int]
    cancelled: bool
    def __init__(self, cancelled: _Optional[bool] = ...) -> None: ...

class RetryKnowledgeIngestionJobRequest(_message.Message):
    __slots__ = ("job_id", "actor_account_id")
    JOB_ID_FIELD_NUMBER: _ClassVar[int]
    ACTOR_ACCOUNT_ID_FIELD_NUMBER: _ClassVar[int]
    job_id: str
    actor_account_id: int
    def __init__(self, job_id: _Optional[str] = ..., actor_account_id: _Optional[int] = ...) -> None: ...

class RetryKnowledgeIngestionJobResponse(_message.Message):
    __slots__ = ("queued",)
    QUEUED_FIELD_NUMBER: _ClassVar[int]
    queued: bool
    def __init__(self, queued: _Optional[bool] = ...) -> None: ...

class SetKnowledgeSourceStatusRequest(_message.Message):
    __slots__ = ("source_id", "status", "actor_account_id")
    SOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    ACTOR_ACCOUNT_ID_FIELD_NUMBER: _ClassVar[int]
    source_id: str
    status: str
    actor_account_id: int
    def __init__(self, source_id: _Optional[str] = ..., status: _Optional[str] = ..., actor_account_id: _Optional[int] = ...) -> None: ...

class SetKnowledgeSourceStatusResponse(_message.Message):
    __slots__ = ("updated",)
    UPDATED_FIELD_NUMBER: _ClassVar[int]
    updated: bool
    def __init__(self, updated: _Optional[bool] = ...) -> None: ...

class ListIngestionAuditEventsRequest(_message.Message):
    __slots__ = ("limit",)
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    limit: int
    def __init__(self, limit: _Optional[int] = ...) -> None: ...

class IngestionAuditEvent(_message.Message):
    __slots__ = ("id", "actor_account_id", "action", "source_id", "job_id", "outcome", "occurred_at_unix")
    ID_FIELD_NUMBER: _ClassVar[int]
    ACTOR_ACCOUNT_ID_FIELD_NUMBER: _ClassVar[int]
    ACTION_FIELD_NUMBER: _ClassVar[int]
    SOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    JOB_ID_FIELD_NUMBER: _ClassVar[int]
    OUTCOME_FIELD_NUMBER: _ClassVar[int]
    OCCURRED_AT_UNIX_FIELD_NUMBER: _ClassVar[int]
    id: str
    actor_account_id: int
    action: str
    source_id: str
    job_id: str
    outcome: str
    occurred_at_unix: int
    def __init__(self, id: _Optional[str] = ..., actor_account_id: _Optional[int] = ..., action: _Optional[str] = ..., source_id: _Optional[str] = ..., job_id: _Optional[str] = ..., outcome: _Optional[str] = ..., occurred_at_unix: _Optional[int] = ...) -> None: ...

class IngestionAuditEventList(_message.Message):
    __slots__ = ("events",)
    EVENTS_FIELD_NUMBER: _ClassVar[int]
    events: _containers.RepeatedCompositeFieldContainer[IngestionAuditEvent]
    def __init__(self, events: _Optional[_Iterable[_Union[IngestionAuditEvent, _Mapping]]] = ...) -> None: ...

class PreviewKnowledgeDocumentRequest(_message.Message):
    __slots__ = ("name", "content_type", "content")
    NAME_FIELD_NUMBER: _ClassVar[int]
    CONTENT_TYPE_FIELD_NUMBER: _ClassVar[int]
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    name: str
    content_type: str
    content: bytes
    def __init__(self, name: _Optional[str] = ..., content_type: _Optional[str] = ..., content: _Optional[bytes] = ...) -> None: ...

class KnowledgeDocumentChunkPreview(_message.Message):
    __slots__ = ("index", "text", "characters")
    INDEX_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    CHARACTERS_FIELD_NUMBER: _ClassVar[int]
    index: int
    text: str
    characters: int
    def __init__(self, index: _Optional[int] = ..., text: _Optional[str] = ..., characters: _Optional[int] = ...) -> None: ...

class KnowledgeDocumentPreview(_message.Message):
    __slots__ = ("chunk_count", "total_characters", "estimated_embedding_tokens", "estimated_embedding_cost_vnd", "samples", "cost_estimate_available")
    CHUNK_COUNT_FIELD_NUMBER: _ClassVar[int]
    TOTAL_CHARACTERS_FIELD_NUMBER: _ClassVar[int]
    ESTIMATED_EMBEDDING_TOKENS_FIELD_NUMBER: _ClassVar[int]
    ESTIMATED_EMBEDDING_COST_VND_FIELD_NUMBER: _ClassVar[int]
    SAMPLES_FIELD_NUMBER: _ClassVar[int]
    COST_ESTIMATE_AVAILABLE_FIELD_NUMBER: _ClassVar[int]
    chunk_count: int
    total_characters: int
    estimated_embedding_tokens: int
    estimated_embedding_cost_vnd: int
    samples: _containers.RepeatedCompositeFieldContainer[KnowledgeDocumentChunkPreview]
    cost_estimate_available: bool
    def __init__(self, chunk_count: _Optional[int] = ..., total_characters: _Optional[int] = ..., estimated_embedding_tokens: _Optional[int] = ..., estimated_embedding_cost_vnd: _Optional[int] = ..., samples: _Optional[_Iterable[_Union[KnowledgeDocumentChunkPreview, _Mapping]]] = ..., cost_estimate_available: _Optional[bool] = ...) -> None: ...
