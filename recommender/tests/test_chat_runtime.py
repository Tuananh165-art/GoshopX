import unittest
from unittest.mock import Mock, patch

from app.chat import runtime
from psycopg.rows import dict_row


class ChatRuntimeTests(unittest.TestCase):
    def test_enabled_memory_fails_closed_without_database_configuration(self):
        with patch.object(runtime, "DATABASE_URL", ""), patch.object(runtime, "CHAT_SESSION_SIGNING_KEY", ""):
            with self.assertRaisesRegex(RuntimeError, "database URL or signing key is missing"):
                runtime._build_memory()

    def test_postgres_checkpointer_pool_uses_autocommit_and_dict_rows(self):
        checkpointer = Mock()
        with (
            patch.object(runtime, "DATABASE_URL", "postgresql://user:pass@localhost/db"),
            patch.object(runtime, "CHAT_SESSION_SIGNING_KEY", "test-signing-key"),
            patch("psycopg_pool.ConnectionPool") as pool_factory,
            patch("langgraph.checkpoint.postgres.PostgresSaver", return_value=checkpointer),
        ):
            runtime._build_memory()

        pool_factory.assert_called_once_with(
            "postgresql://user:pass@localhost/db",
            min_size=1,
            max_size=4,
            kwargs={"autocommit": True, "prepare_threshold": 0, "row_factory": dict_row},
            open=True,
        )
        checkpointer.setup.assert_called_once_with()


if __name__ == "__main__":
    unittest.main()
