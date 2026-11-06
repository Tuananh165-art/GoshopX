import json
import unittest
from unittest.mock import MagicMock, patch

from app.chat.tools import TavilySearch


class TavilySearchTests(unittest.TestCase):
    @patch("app.chat.tools.urllib.request.urlopen")
    def test_search_returns_bounded_https_sources_and_caches_result(self, urlopen):
        response = MagicMock()
        response.__enter__.return_value.read.return_value = json.dumps({"results": [
            {"title": "Guide", "url": "https://example.com/guide", "content": "A" * 1500},
            {"title": "Credentials", "url": "https://user:secret@example.com/private", "content": "unsafe"},
            {"title": "Script", "url": "javascript:alert(1)", "content": "unsafe"},
            {"title": "Fourth", "url": "https://example.com/fourth", "content": "not returned"},
        ]}).encode()
        urlopen.return_value = response
        search = TavilySearch("test-key")

        results = search.search("  laptop   warranty  ")

        self.assertEqual(len(results), 1)
        self.assertEqual(results[0]["url"], "https://example.com/guide")
        self.assertEqual(len(results[0]["content"]), 1200)
        self.assertEqual(urlopen.call_count, 1)
        self.assertEqual(search.search("laptop warranty"), results)
        self.assertEqual(urlopen.call_count, 1)

    @patch("app.chat.tools.urllib.request.urlopen")
    def test_empty_query_does_not_call_provider(self, urlopen):
        self.assertEqual(TavilySearch("test-key").search("  \n "), [])
        urlopen.assert_not_called()


if __name__ == "__main__":
    unittest.main()
