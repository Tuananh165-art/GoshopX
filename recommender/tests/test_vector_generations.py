import unittest
from unittest.mock import patch

from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker

from shared.db.models import Base
from vector import generations


class VectorGenerationTests(unittest.TestCase):
    def setUp(self):
        self.engine = create_engine("sqlite:///:memory:")
        Base.metadata.create_all(self.engine)
        self.sessions = sessionmaker(bind=self.engine)
        self.session_patch = patch.object(generations, "ReplicaSession", self.sessions)
        self.session_patch.start()

    def tearDown(self):
        self.session_patch.stop()
        self.engine.dispose()

    def test_activate_is_atomic_and_rollback_keeps_previous_generation(self):
        first = generations.create_or_resume_build("gemini-embedding-2", 768)
        digest1 = generations.record_manifest(first.id, 2, [("p1", "a"), ("p2", "b")])
        generations.activate_generation(first.id, 2, digest1)
        self.assertEqual(generations.active_generation().id, first.id)

        second = generations.create_or_resume_build("gemini-embedding-2", 768)
        digest2 = generations.record_manifest(second.id, 2, [("p1", "a"), ("p2", "c")])
        with self.assertRaisesRegex(ValueError, "validation failed"):
            generations.activate_generation(second.id, 3, digest2)
        self.assertEqual(generations.active_generation().id, first.id)

        generations.activate_generation(second.id, 2, digest2)
        self.assertEqual(generations.active_generation().id, second.id)
        generations.rollback_generation(first.id)
        self.assertEqual(generations.active_generation().id, first.id)

    def test_build_reuses_collection_names_for_idempotent_upserts(self):
        first = generations.create_or_resume_build("gemini-embedding-2", 768)
        resumed = generations.create_or_resume_build("gemini-embedding-2", 768)
        self.assertEqual(first.id, resumed.id)
        self.assertEqual(first.text_collection, resumed.text_collection)

    def test_catalog_manifest_tracks_only_publishable_products_in_stable_order(self):
        from shared.db.models import Product
        with self.sessions() as session:
            session.add_all([
                Product(id="b", name="B", description="", price=2, account_id=0,
                        publish_status="published", moderation_status="approved", source_hash="hash-b"),
                Product(id="a", name="A", description="", price=1, account_id=0,
                        publish_status="published", moderation_status="approved", source_hash="hash-a"),
                Product(id="draft", name="Draft", description="", price=3, account_id=0,
                        publish_status="draft", moderation_status="approved", source_hash="hash-draft"),
            ])
            session.commit()
        count, rows = generations.catalog_manifest()
        self.assertEqual(count, 2)
        self.assertEqual(rows, [("a", "hash-a"), ("b", "hash-b")])

    def test_catalog_change_during_build_blocks_candidate_activation(self):
        with self.assertRaisesRegex(RuntimeError, "candidate remains inactive"):
            generations.validate_catalog_snapshot([("p1", "old")], [("p1", "old")], [("p1", "new")])

if __name__ == "__main__":
    unittest.main()
