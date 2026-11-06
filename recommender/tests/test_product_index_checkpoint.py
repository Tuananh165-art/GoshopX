import unittest

from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker

from shared.db.models import Base
from shared.db.repo import create_or_update_product, mark_product_indexed


class ProductIndexCheckpointTests(unittest.TestCase):
    def test_failed_index_remains_pending_and_success_is_idempotent(self):
        engine = create_engine("sqlite:///:memory:")
        Base.metadata.create_all(engine)
        session_factory = sessionmaker(bind=engine)
        data = {
            "product_id": "p-1",
            "name": "Laptop",
            "description": "Portable computer",
            "price": 100,
            "accountID": 0,
        }

        with session_factory() as session:
            self.assertTrue(create_or_update_product(session, data))
            # Simulate provider/index failure: the source projection exists but is not checkpointed.
            self.assertTrue(create_or_update_product(session, data))
            mark_product_indexed(session, "p-1")
            self.assertFalse(create_or_update_product(session, data))

            data["description"] = "Updated description"
            self.assertTrue(create_or_update_product(session, data))
            self.assertTrue(create_or_update_product(session, data))
            mark_product_indexed(session, "p-1")
            self.assertFalse(create_or_update_product(session, data))

        engine.dispose()


if __name__ == "__main__":
    unittest.main()
