import requests

from shared.config.settings import PRODUCT_API


def normalize_product_data(product_data: dict) -> dict:
    product_id = product_data.get("product_id") or product_data.get("id")
    account_id = product_data.get("accountID") or product_data.get("account_id") or 0
    return {
        "product_id": product_id,
        "name": product_data["name"],
        "description": product_data["description"],
        "price": product_data["price"],
        "accountID": int(account_id or 0),
        "category": product_data.get("category", product_data.get("categoryId", "")),
        "brand": product_data.get("brand", ""),
        "tags": product_data.get("tags", []),
        "thumbnail": product_data.get("thumbnail", ""),
        "images": product_data.get("images", []),
        "publishStatus": product_data.get("publishStatus", "published"),
        "moderationStatus": product_data.get("moderationStatus", "approved"),
        "stock": product_data.get("stock", 1),
    }


def fetch_product_by_id(product_id: str) -> dict:
    response = requests.get(f"{PRODUCT_API}/{product_id}")
    response.raise_for_status()
    return normalize_product_data(response.json())
