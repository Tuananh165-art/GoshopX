import os
import unittest
from unittest.mock import patch

from app.shared.spend import _rate, token_cost


class SpendCostTests(unittest.TestCase):
    def test_token_cost_rounds_up_to_whole_vnd(self):
        self.assertEqual(token_cost(1, 1, 1, 1), 1)

    def test_token_cost_uses_per_million_rates(self):
        self.assertEqual(token_cost(1_000_000, 250_000, 10_000, 20_000), 15_000)

    def test_token_cost_clamps_negative_usage(self):
        self.assertEqual(token_cost(-10, 5, 100, 100), 1)
        self.assertEqual(token_cost(-10, -5, 100, 100), 0)

    def test_invalid_rate_is_unpriced(self):
        with patch.dict(os.environ, {"AI_DEEPSEEK_INPUT_VND_PER_MILLION_TOKENS": "unknown"}, clear=False):
            self.assertIsNone(_rate("deepseek", "INPUT_VND_PER_MILLION_TOKENS"))


if __name__ == "__main__":
    unittest.main()
