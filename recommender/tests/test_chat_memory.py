import unittest

from app.chat.memory import _compact_history, merge_histories


class ChatMemoryTests(unittest.TestCase):
    def test_concurrent_thread_writes_append_non_overlapping_turns(self):
        old = [{"role": "user", "content": "find a laptop"}, {"role": "assistant", "content": "Dell"}]
        first = old + [{"role": "user", "content": "under 20m"}, {"role": "assistant", "content": "Dell A"}]
        second = old + [{"role": "user", "content": "with 16GB RAM"}, {"role": "assistant", "content": "Dell B"}]
        merged = merge_histories(first, second)
        self.assertEqual(len(merged), 6)
        self.assertEqual(merged[:4], first)
        self.assertEqual(merged[4:], second[2:])

    def test_idempotent_retry_does_not_duplicate_current_turn(self):
        turn = [{"role": "user", "content": "find a laptop"}, {"role": "assistant", "content": "Dell"}]
        self.assertEqual(merge_histories(turn, turn), turn)

    def test_compaction_keeps_summary_shortlist_and_recent_turns(self):
        history = []
        for index in range(20):
            history.extend([
                {"role": "user", "content": f"Laptop under {index + 10} million"},
                {"role": "assistant", "content": f"Shortlist {index}", "product_ids": [f"p{index}", f"q{index}"]},
            ])

        compacted = _compact_history(history)
        self.assertEqual(len(compacted), 15)
        self.assertTrue(compacted[0]["summary"])
        self.assertLessEqual(len(str(compacted[0]["content"])), 1200)
        self.assertIn("Laptop under 15 million", str(compacted[0]["content"]))
        self.assertEqual(compacted[-1]["content"], "Shortlist 19")
        self.assertLessEqual(len(compacted[0]["product_ids"]), 10)


if __name__ == "__main__":
    unittest.main()
