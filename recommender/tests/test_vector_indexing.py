import unittest
from unittest.mock import patch

from app.vector.documents import build_product_document
from app.vector.milvus_store import ProductVectorRecord, build_collection_schema
from app.vector.embeddings import GeminiEmbedding, _decode_image_data_url, load_image_reference
from app.vector.indexer import ProductIndexer, build_product_indexer


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

    def test_gcs_image_reference_is_bucket_and_prefix_scoped(self):
        with patch.dict("os.environ", {"GCS_MEDIA_BUCKET": "private-media"}):
            with self.assertRaises(ValueError):
                load_image_reference("gs://other-bucket/chat-upload/a.jpg")
            with self.assertRaises(ValueError):
                load_image_reference("gs://private-media/products/a.jpg")

    def test_gcs_image_reference_uses_workload_identity_and_validates_bytes(self):
        class Headers:
            def get_content_type(self):
                return "application/json" if self.token else "image/jpeg"

            def __init__(self, token):
                self.token = token

        class Response:
            def __init__(self, body, token=False):
                self.body = body
                self.headers = Headers(token)

            def read(self, limit=-1):
                return self.body[:limit]

            def __enter__(self):
                return self

            def __exit__(self, *_):
                return False

        with patch.dict("os.environ", {"GCS_MEDIA_BUCKET": "private-media"}), patch(
            "urllib.request.urlopen",
            side_effect=[Response(b'{"access_token":"test-token"}', True), Response(b"\xff\xd8\xffimage")],
        ) as open_url:
            self.assertEqual(load_image_reference("gs://private-media/chat-upload/abc.jpg"),
                             ("image/jpeg", b"\xff\xd8\xffimage"))
            self.assertEqual(open_url.call_count, 2)

    def test_gemini_image_embedding_uses_rest_camel_case_fields(self):
        client = GeminiEmbedding("test-key", dimension=768)
        with patch.object(client, "_post", return_value={"embedding": {"values": [0.0] * 768}}) as post:
            client.embed_image("image/png", b"\x89PNG\r\n\x1a\nfixture")

        payload = post.call_args.args[1]
        part = payload["content"]["parts"][0]
        self.assertEqual(part["inlineData"]["mimeType"], "image/png")
        self.assertNotIn("inline_data", part)
        self.assertNotIn("taskType", payload)

    def test_gemini_embedding_2_batches_catalog_images_without_task_type(self):
        client = GeminiEmbedding("test-key", dimension=768)
        vector = {"values": [0.0] * 768}
        with patch("app.vector.embeddings._read_catalog_image", return_value=("image/png", b"\x89PNG\r\n\x1a\nfixture")), \
             patch("app.vector.embeddings.MAX_IMAGE_BATCH_BYTES", 32), \
             patch.object(client, "_post", side_effect=[{"embeddings": [vector, vector]}, {"embeddings": [vector]}]) as post:
            result = client.embed_image_urls(["https://cdn.dummyjson.com/1.png", "https://cdn.dummyjson.com/2.png", "https://cdn.dummyjson.com/3.png"])

        self.assertEqual(len(result), 3)
        self.assertEqual([len(call.args[1]["requests"]) for call in post.call_args_list], [2, 1])
        self.assertTrue(all("taskType" not in item for call in post.call_args_list for item in call.args[1]["requests"]))

    def test_product_indexer_accepts_a_page_batch(self):
        class Indexes:
            def upsert(self, documents):
                self.documents = documents

        indexes = Indexes()
        indexer = ProductIndexer(indexes=indexes, embeddings=None)
        with patch("app.vector.indexer.build_product_document", side_effect=lambda product: product):
            indexer.upsert([{"id": "p1"}, {"id": "p2"}])
        self.assertEqual([item["id"] for item in indexes.documents], ["p1", "p2"])

    def test_gemini_embedding_2_text_payload_uses_task_instruction_not_task_type(self):
        client = GeminiEmbedding("test-key", dimension=768)
        vector = {"values": [0.0] * 768}
        with patch.object(client, "_post", side_effect=[{"embeddings": [vector]}, {"embedding": vector}]) as post:
            client.embed_documents(["Laptop catalog entry"])
            client.embed_query("laptop for school")

        document_request = post.call_args_list[0].args[1]["requests"][0]
        query_request = post.call_args_list[1].args[1]
        self.assertTrue(document_request["content"]["parts"][0]["text"].startswith("task: product catalog document"))
        self.assertTrue(query_request["content"]["parts"][0]["text"].startswith("task: product search query"))
        self.assertNotIn("taskType", document_request)
        self.assertNotIn("taskType", query_request)

    def test_hash_fallback_cannot_write_to_gemini_collection(self):
        with self.assertRaisesRegex(ValueError, "Gemini API key is required"):
            build_product_indexer(
                uri="http://localhost:19530",
                collection="product_catalog_gemini_v2",
                image_collection="product_catalog_gemini_v2_images",
                dimension=768,
            )

    def test_versioned_product_collection_enables_native_milvus_bm25(self):
        class Indexes:
            pass

        with patch("app.vector.indexer.LangChainMilvusIndexes.connect", return_value=Indexes()) as connect:
            build_product_indexer(
                uri="http://localhost:19530", collection="goshopx_catalog_generation",
                image_collection="goshopx_catalog_generation_images", dimension=768,
                gemini_api_key="test-key", native_bm25=True,
            )
        self.assertTrue(connect.call_args.kwargs["native_bm25"])


if __name__ == "__main__":
    unittest.main()
