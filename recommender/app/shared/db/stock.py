from __future__ import annotations

from shared.db.models import Product


def update_product_stock(session, product_id: str, available_quantity: int) -> Product | None:
    product = session.get(Product, str(product_id))
    if product is None:
        return None
    product.stock = max(0, int(available_quantity))
    session.commit()
    return product
