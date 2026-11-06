import unittest
import os
from unittest.mock import patch

os.environ.setdefault("DATABASE_URL", "sqlite://")
from app.entry import kafka_consumer


class KafkaConsumerSpendTests(unittest.TestCase):
    def test_consumer_start_does_not_run_paid_catalog_bootstrap(self):
        with patch.object(kafka_consumer, "KafkaConsumer", return_value=[]) as consumer, \
             patch.object(kafka_consumer, "_bootstrap_catalog_from_product_api") as bootstrap:
            kafka_consumer.start_kafka_consumer()
        consumer.assert_called_once()
        bootstrap.assert_not_called()

    def test_catalog_indexer_provider_calls_share_the_spend_ledger(self):
        from vector import runtime_index

        with patch.object(runtime_index, "_indexes", {}), \
             patch.object(runtime_index, "write_generations", return_value=[]), \
             patch.object(runtime_index, "AI_API_KEY", "configured"), \
             patch.object(runtime_index, "GEMINI_API_KEY", ""), \
             patch.object(runtime_index, "build_product_indexer", return_value=object()) as build:
            indexes = kafka_consumer._get_indexers()
        self.assertEqual(len(indexes), 1)
        options = build.call_args.kwargs
        self.assertIs(options["spend_ledger"], kafka_consumer._spend_ledger)
        self.assertIs(options["vision_llm"].spend_ledger, kafka_consumer._spend_ledger)


if __name__ == "__main__":
    unittest.main()
