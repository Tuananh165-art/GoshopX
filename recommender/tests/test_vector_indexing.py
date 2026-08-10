import unittest

from app.vector.documents import build_product_document
from app.vector.milvus_store import ProductVectorRecord, build_collection_schema


class VectorIndexingTests(unittest.TestCase):
    def test_product_document_contains_searchable_business_fields_and_images(self):
        document = build_product_document(
            {
                "id": "p1",
                "name": "Laptop",
                "description": "Laptop cho công việc",
                "price": 1000,
                "brand": "Dell",
                "category": "laptops",
                "tags": ["office"],
                "thumbnail": "https://cdn/p1.jpg",
                "images": ["https://cdn/p1.jpg", "https://cdn/p1-2.jpg"],
            }
        )

        self.assertIn("Laptop", document.text)
        self.assertIn("1000", document.text)
        self.assertEqual(document.image_urls[0], "https://cdn/p1.jpg")
        self.assertEqual(document.metadata["product_id"], "p1")

    def test_collection_schema_has_text_image_sparse_and_business_fields(self):
        fields = {field["name"] for field in build_collection_schema(384, 512)}

        self.assertTrue({"product_id", "text_vector", "image_vector", "text_sparse"}.issubset(fields))
        self.assertTrue({"publish_status", "moderation_status", "price", "stock"}.issubset(fields))

    def test_record_rejects_vectors_with_wrong_dimensions(self):
        with self.assertRaises(ValueError):
            ProductVectorRecord(product_id="p1", text_vector=[0.1], image_vector=[0.2, 0.3], metadata={}).validate(384, 512)


if __name__ == "__main__":
    unittest.main()
