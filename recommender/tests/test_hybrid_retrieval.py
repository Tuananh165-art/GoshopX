import unittest

from app.retrieval.hybrid import Candidate, HybridRetriever, reciprocal_rank_fusion


class HybridRetrieverTests(unittest.TestCase):
    def test_reciprocal_rank_fusion_merges_dense_and_lexical_results(self):
        ranked = reciprocal_rank_fusion(
            [["dense-a", "shared"], ["lexical-b", "shared"]], k=60
        )

        self.assertEqual(ranked[0], "shared")
        self.assertIn("dense-a", ranked)
        self.assertIn("lexical-b", ranked)

    def test_filters_ineligible_products_before_ranking(self):
        retriever = HybridRetriever(
            dense_search=lambda plan: [Candidate("sold", 0.99), Candidate("ok", 0.5)],
            lexical_search=lambda plan: [Candidate("ok", 1.0)],
            catalog={
                "sold": {"id": "sold", "publishStatus": "published", "moderationStatus": "approved", "stock": 0},
                "ok": {"id": "ok", "publishStatus": "published", "moderationStatus": "approved", "stock": 3},
            },
        )

        results = retriever.search(object(), limit=5)

        self.assertEqual([item["id"] for item in results], ["ok"])

    def test_excludes_user_seen_products(self):
        retriever = HybridRetriever(
            dense_search=lambda plan: [Candidate("seen", 1.0), Candidate("new", 0.9)],
            lexical_search=lambda plan: [],
            catalog={
                "seen": {"id": "seen", "publishStatus": "published", "moderationStatus": "approved", "stock": 3},
                "new": {"id": "new", "publishStatus": "published", "moderationStatus": "approved", "stock": 3},
            },
        )

        results = retriever.search(object(), limit=5, excluded_ids={"seen"})

        self.assertEqual([item["id"] for item in results], ["new"])


if __name__ == "__main__":
    unittest.main()
