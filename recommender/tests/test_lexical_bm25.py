import unittest
from collections import Counter

from app.retrieval.bm25 import score, tokenize


class BM25Tests(unittest.TestCase):
    def test_prefers_document_matching_more_query_terms(self):
        documents = {
            "laptop": tokenize("Dell laptop 16gb ram"),
            "case": tokenize("Dell laptop case"),
            "phone": tokenize("Samsung phone camera"),
        }
        df = Counter(token for doc in documents.values() for token in set(doc))
        query = set(tokenize("Dell laptop 16gb"))
        scores = {product_id: score(query, documents, df, product_id) for product_id in documents}
        self.assertGreater(scores["laptop"], scores["case"])
        self.assertEqual(scores["phone"], 0)


if __name__ == "__main__":
    unittest.main()
