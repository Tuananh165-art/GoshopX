import unittest
from unittest.mock import patch

from app.chat import runtime


class ChatRuntimeTests(unittest.TestCase):
    def test_enabled_memory_fails_closed_without_database_configuration(self):
        with patch.object(runtime, "DATABASE_URL", ""), patch.object(runtime, "CHAT_SESSION_SIGNING_KEY", ""):
            with self.assertRaisesRegex(RuntimeError, "database URL or signing key is missing"):
                runtime._build_memory()


if __name__ == "__main__":
    unittest.main()
