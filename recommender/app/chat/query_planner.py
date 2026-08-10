from __future__ import annotations

import json
import re
from dataclasses import dataclass, field
from typing import Any


@dataclass(frozen=True)
class QueryPlan:
    query: str
    keywords: tuple[str, ...] = ()
    category: str | None = None
    brand: str | None = None
    min_price: float | None = None
    max_price: float | None = None
    visual_query: bool = False
    filter_expression: str = "published == true && moderation == 'approved'"
    limit: int = 5

    def to_dict(self) -> dict[str, Any]:
        return {
            "query": self.query,
            "keywords": list(self.keywords),
            "category": self.category,
            "brand": self.brand,
            "min_price": self.min_price,
            "max_price": self.max_price,
            "visual_query": self.visual_query,
            "limit": self.limit,
        }


_ALLOWED_CATEGORIES = {
    "laptop": "laptops",
    "laptops": "laptops",
    "máy tính": "laptops",
    "may tinh": "laptops",
    "điện thoại": "smartphones",
    "dien thoai": "smartphones",
    "phone": "smartphones",
    "smartphone": "smartphones",
    "smartphones": "smartphones",
    "iphone": "smartphones",
    "ipad": "tablets",
    "tablet": "tablets",
    "tablets": "tablets",
    "macbook": "laptops",
    "tai nghe": "audio",
    "headphone": "audio",
    "giày": "shoes",
    "áo": "fashion",
    "túi": "bags",
    "đồng hồ": "mens-watches",
    "dong ho": "mens-watches",
    "tivi": "televisions",
    "tv": "televisions",
}
_ALLOWED_BRANDS = ("apple", "samsung", "dell", "lenovo", "asus", "acer", "nike", "adidas", "huawei", "xiaomi", "oppo", "vivo", "realme")
_BRAND_DISPLAY = {brand: brand.title() for brand in _ALLOWED_BRANDS}


def parse_query_plan(text: str, model_plan: dict[str, Any] | None = None, *, visual_query: bool = False) -> QueryPlan:
    """Parse a safe query plan; model output is advisory and never becomes an expression."""
    query = re.sub(r"\s+", " ", str(text or "").strip())[:1000]
    normalized = query.casefold()
    model_plan = model_plan if isinstance(model_plan, dict) else {}

    category = _safe_text(model_plan.get("category")) or next(
        (value for term, value in _ALLOWED_CATEGORIES.items() if term in normalized), None
    )
    brand = _safe_text(model_plan.get("brand")) or next(
        (_BRAND_DISPLAY[brand] for brand in _ALLOWED_BRANDS if brand in normalized), None
    )
    min_price = _safe_number(model_plan.get("min_price"))
    max_price = _safe_number(model_plan.get("max_price"))
    if min_price is None or max_price is None:
        detected_min, detected_max = _prices_from_text(normalized)
        min_price = min_price if min_price is not None else detected_min
        max_price = max_price if max_price is not None else detected_max

    keywords = _keywords(query, category=category, brand=brand)
    return QueryPlan(
        query=query,
        keywords=tuple(keywords),
        category=category,
        brand=brand,
        min_price=min_price if min_price is None or min_price >= 0 else None,
        max_price=max_price if max_price is None or max_price >= 0 else None,
        visual_query=bool(visual_query),
        filter_expression=_safe_filter(category, brand, min_price, max_price),
        limit=min(max(int(_safe_number(model_plan.get("limit")) or 5), 1), 20),
    )


def build_catalog_context(products: list[dict[str, Any]], *, max_products: int = 20, max_chars: int = 12000) -> str:
    """Serialize only safe catalog fields into bounded LLM context."""
    allowed = (
        "id", "name", "description", "price", "currency", "category", "brand", "tags",
        "rating", "stock", "availabilityStatus", "thumbnail", "images", "warrantyInformation",
        "shippingInformation", "returnPolicy",
    )
    safe: list[dict[str, Any]] = []
    for raw in products[:max_products]:
        if not isinstance(raw, dict) or not raw.get("id"):
            continue
        item = {key: raw[key] for key in allowed if key in raw}
        for key in ("description", "warrantyInformation", "shippingInformation", "returnPolicy"):
            if key in item:
                item[key] = str(item[key])[:500]
        if "images" in item and isinstance(item["images"], list):
            item["images"] = [str(url)[:500] for url in item["images"][:8]]
        safe.append(item)
    return json.dumps(safe, ensure_ascii=False, separators=(",", ":"))[:max_chars]


def _safe_text(value: Any) -> str | None:
    if not isinstance(value, str):
        return None
    value = re.sub(r"[^\wÀ-ỹ ._-]", "", value.strip())[:80]
    return value or None


def _safe_number(value: Any) -> float | None:
    try:
        number = float(value)
    except (TypeError, ValueError):
        return None
    return number if number == number and number not in (float("inf"), float("-inf")) else None


def _prices_from_text(text: str) -> tuple[float | None, float | None]:
    matches = re.findall(r"(\d+(?:[.,]\d+)?)\s*(triệu|tr|m|million|k|nghìn)?", text)
    values: list[float] = []
    for raw, suffix in matches:
        value = float(raw.replace(",", "."))
        if suffix in ("triệu", "tr", "m", "million"):
            value *= 1_000_000
        elif suffix in ("k", "nghìn"):
            value *= 1_000
        if value >= 1_000:
            values.append(value)
    if not values:
        return None, None
    if any(word in text for word in ("dưới", "under", "less than", "tối đa", "max")):
        return None, max(values)
    if any(word in text for word in ("trên", "over", "more than", "tối thiểu", "min")):
        return min(values), None
    return None, None


def _keywords(query: str, *, category: str | None, brand: str | None) -> list[str]:
    words = [word for word in re.findall(r"[\wÀ-ỹ-]+", query.casefold()) if len(word) > 2]
    result: list[str] = []
    for phrase in ("màu đen", "màu trắng", "màu xanh", "màu đỏ", "màu hồng"):
        if phrase in query.casefold():
            result.append(phrase)
    for word in words:
        if word not in result and word not in {"dưới", "trên", "triệu", "cho", "với", "tìm", "màu"}:
            result.append(word)
    if category and category not in result:
        result.append(category)
    if brand and brand.casefold() not in result:
        result.append(brand.casefold())
    return result[:20]


def _safe_filter(category: str | None, brand: str | None, minimum: float | None, maximum: float | None) -> str:
    clauses = ["publishStatus == 'published'", "moderationStatus == 'approved'"]
    if category:
        clauses.append(f"category == '{category}'")
    if brand:
        clauses.append(f"brand == '{brand}'")
    if minimum is not None and minimum >= 0:
        clauses.append(f"price >= {minimum:g}")
    if maximum is not None and maximum >= 0:
        clauses.append(f"price <= {maximum:g}")
    return " && ".join(clauses)
