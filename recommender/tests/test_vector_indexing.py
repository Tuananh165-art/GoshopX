import unittest
from unittest.mock import patch

from app.vector.documents import build_product_document
from app.vector.milvus_store import ProductVectorRecord, build_collection_schema
from app.vector.embeddings import GeminiEmbedding, _decode_image_data_url
from app.vector.indexer import build_product_indexer


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

    def test_image_query_rejects_non_image_data(self):
        with self.assertRaises(ValueError):
            _decode_image_data_url("data:image/png;base64,bm90LWltYWdl")

    def test_gemini_image_embedding_uses_rest_camel_case_fields(self):
        client = GeminiEmbedding("test-key", dimension=768)
        with patch.object(client, "_post", return_value={"embedding": {"values": [0.0] * 768}}) as post:
            client.embed_image("image/png", b"\x89PNG\r\n\x1a\nfixture")

        payload = post.call_args.args[1]
        part = payload["content"]["parts"][0]
        self.assertEqual(part["inlineData"]["mimeType"], "image/png")
        self.assertNotIn("inline_data", part)

    def test_hash_fallback_cannot_write_to_gemini_collection(self):
        with self.assertRaisesRegex(ValueError, "Gemini API key is required"):
            build_product_indexer(
                uri="http://localhost:19530",
                collection="product_catalog_gemini_v2",
                image_collection="product_catalog_gemini_v2_images",
                dimension=768,
            )


if __name__ == "__main__":
    unittest.main()
