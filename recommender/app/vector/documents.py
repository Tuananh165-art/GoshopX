from __future__ import annotations

from dataclasses import dataclass
import hashlib
import json
from typing import Any


@dataclass(frozen=True)
class ProductDocument:
    product_id: str
    text: str
    image_urls: tuple[str, ...]
    metadata: dict[str, Any]
    image_descriptions: tuple[str, ...] = ()


def build_product_document(product: dict[str, Any]) -> ProductDocument:
    product_id = str(product.get("id") or product.get("product_id") or "").strip()
    if not product_id:
        raise ValueError("product id is required")
    fields = [
        str(product.get("name", "")),
        str(product.get("description", "")),
        str(product.get("category", "")),
        str(product.get("brand", "")),
        " ".join(str(tag) for tag in product.get("tags", []) if tag),
        f"price {product.get('price', '')}",
        str(product.get("rating", "")),
        str(product.get("warrantyInformation", "")),
        str(product.get("shippingInformation", "")),
        str(product.get("returnPolicy", "")),
    ]
    image_urls = []
    if product.get("thumbnail"):
        image_urls.append(str(product["thumbnail"]))
    # A single representative image is enough for the initial multimodal index.
    image_urls = list(dict.fromkeys(image_urls))[:1]
    content_hash = product_content_hash(product)
    metadata = {
        "product_id": product_id,
        "publish_status": str(product.get("publishStatus", "")),
        "moderation_status": str(product.get("moderationStatus", "")),
        "category": str(product.get("category", "")),
        "brand": str(product.get("brand", "")),
        "price": float(product.get("price", 0) or 0),
        "stock": int(product.get("stock", 0) or 0),
        "rating": float(product.get("rating", 0) or 0),
        "thumbnail": image_urls[0] if image_urls else "",
        "images": image_urls,
        "content_hash": content_hash,
    }
    return ProductDocument(product_id, " ".join(field for field in fields if field).strip(), tuple(image_urls), metadata)


def product_content_hash(product: dict[str, Any]) -> str:
    content = {
        key: product.get(key)
        for key in ("name", "description", "price", "discountPercentage", "rating", "category", "brand",
                    "tags", "sku", "thumbnail", "images", "warrantyInformation", "shippingInformation",
                    "returnPolicy", "availabilityStatus", "publishStatus", "moderationStatus")
    }
    encoded = json.dumps(content, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(encoded.encode("utf-8")).hexdigest()
