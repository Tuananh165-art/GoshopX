"""Hybrid lexical/dense retrieval primitives with deterministic business filtering."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any, Callable, Iterable

from langchain_core.runnables import RunnableLambda, RunnableParallel


@dataclass(frozen=True)
class Candidate:
    product_id: str
    score: float


class HybridRetriever:
    def __init__(self, *, dense_search: Callable[[Any], Iterable[Candidate]], lexical_search: Callable[[Any], Iterable[Candidate]], catalog: dict[str, dict[str, Any]], image_search: Callable[[list[float]], Iterable[Candidate]] | None = None):
        self._dense_search = dense_search
        self._lexical_search = lexical_search
        self._catalog = catalog
        self._image_search = image_search
        self._hybrid_chain = RunnableParallel(
            dense=RunnableLambda(lambda plan: list(self._dense_search(plan))),
            lexical=RunnableLambda(lambda plan: list(self._lexical_search(plan))),
        )

    def search(self, plan: Any, *, limit: int = 5, excluded_ids: set[str] | None = None, image_vector: list[float] | None = None) -> list[dict[str, Any]]:
        excluded_ids = excluded_ids or set()
        candidates = self._hybrid_chain.invoke(plan)
        dense = candidates["dense"]
        lexical = candidates["lexical"]
        visual = list(self._image_search(image_vector)) if image_vector and self._image_search else []
        fused_ids = reciprocal_rank_fusion([[c.product_id for c in dense], [c.product_id for c in lexical], [c.product_id for c in visual]])
        raw_scores: dict[str, float] = {}
        for candidate in [*dense, *lexical, *visual]:
            raw_scores[candidate.product_id] = max(raw_scores.get(candidate.product_id, 0.0), candidate.score)

        result: list[dict[str, Any]] = []
        for rank, product_id in enumerate(fused_ids):
            if product_id in excluded_ids or not self._eligible(product_id, plan):
                continue
            product = dict(self._catalog[product_id])
            product["retrievalScore"] = round((1 / (60 + rank + 1)) + raw_scores.get(product_id, 0.0) * 0.1, 6)
            product["matchedReasons"] = _matched_reasons(product, plan)
            result.append(product)
            if len(result) >= max(1, min(limit, 30)):
                break
        return result

    def products_by_ids(self, product_ids: list[str], plan: Any, *, limit: int = 10) -> list[dict[str, Any]]:
        return [dict(self._catalog[product_id]) for product_id in dict.fromkeys(product_ids)
                if self._eligible(product_id, plan)][:max(1, min(limit, 10))]

    def _eligible(self, product_id: str, plan: Any) -> bool:
        product = self._catalog.get(product_id)
        if not product:
            return False
        if product.get("publishStatus", "") != "published" or product.get("moderationStatus", "") != "approved":
            return False
        if int(product.get("stock", 0) or 0) <= 0:
            return False
        category = str(getattr(plan, "category", None) or "").casefold()
        brand = str(getattr(plan, "brand", None) or "").casefold()
        if category and str(product.get("category", "")).casefold() != category:
            return False
        if brand and brand not in str(product.get("brand", "")).casefold():
            return False
        try:
            price = float(product.get("price", 0) or 0)
        except (TypeError, ValueError):
            return False
        minimum, maximum = getattr(plan, "min_price", None), getattr(plan, "max_price", None)
        return not (minimum is not None and price < minimum or maximum is not None and price > maximum)


def reciprocal_rank_fusion(rankings: list[list[str]], *, k: int = 60) -> list[str]:
    scores: dict[str, float] = {}
    for ranking in rankings:
        seen: set[str] = set()
        for rank, product_id in enumerate(ranking, start=1):
            if product_id in seen:
                continue
            seen.add(product_id)
            scores[product_id] = scores.get(product_id, 0.0) + 1.0 / (k + rank)
    return [product_id for product_id, _ in sorted(scores.items(), key=lambda item: (-item[1], item[0]))]


def _matched_reasons(product: dict[str, Any], plan: Any) -> list[str]:
    reasons: list[str] = []
    keywords = getattr(plan, "keywords", ())
    searchable = " ".join(str(product.get(field, "")) for field in ("name", "description", "brand", "category", "tags")).casefold()
    matched = [keyword for keyword in keywords if keyword.casefold() in searchable]
    if matched:
        reasons.append(f"Khớp từ khóa: {', '.join(matched[:3])}")
    category = getattr(plan, "category", None)
    if category and str(product.get("category", "")).casefold() == category.casefold():
        reasons.append("Đúng danh mục yêu cầu")
    brand = getattr(plan, "brand", None)
    if brand and str(product.get("brand", "")).casefold() == brand.casefold():
        reasons.append("Đúng thương hiệu yêu cầu")
    if isinstance(product.get("price"), (int, float)) and (getattr(plan, "min_price", None) is not None or getattr(plan, "max_price", None) is not None):
        reasons.append("Nằm trong khoảng ngân sách")
    return reasons or ["Phù hợp với kết quả tìm kiếm sản phẩm"]
