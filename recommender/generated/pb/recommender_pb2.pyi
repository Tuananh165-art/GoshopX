from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

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
    __slots__ = ("id", "name", "description", "price", "currency", "thumbnail", "images", "rating", "stock", "retrieval_score", "matched_reasons")
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
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., description: _Optional[str] = ..., price: _Optional[float] = ..., currency: _Optional[str] = ..., thumbnail: _Optional[str] = ..., images: _Optional[_Iterable[str]] = ..., rating: _Optional[float] = ..., stock: _Optional[int] = ..., retrieval_score: _Optional[float] = ..., matched_reasons: _Optional[_Iterable[str]] = ...) -> None: ...

class ChatRecommendationResponse(_message.Message):
    __slots__ = ("answer", "products", "next_suggestions", "warnings", "request_id")
    ANSWER_FIELD_NUMBER: _ClassVar[int]
    PRODUCTS_FIELD_NUMBER: _ClassVar[int]
    NEXT_SUGGESTIONS_FIELD_NUMBER: _ClassVar[int]
    WARNINGS_FIELD_NUMBER: _ClassVar[int]
    REQUEST_ID_FIELD_NUMBER: _ClassVar[int]
    answer: str
    products: _containers.RepeatedCompositeFieldContainer[ChatProduct]
    next_suggestions: _containers.RepeatedScalarFieldContainer[str]
    warnings: _containers.RepeatedScalarFieldContainer[str]
    request_id: str
    def __init__(self, answer: _Optional[str] = ..., products: _Optional[_Iterable[_Union[ChatProduct, _Mapping]]] = ..., next_suggestions: _Optional[_Iterable[str]] = ..., warnings: _Optional[_Iterable[str]] = ..., request_id: _Optional[str] = ...) -> None: ...
