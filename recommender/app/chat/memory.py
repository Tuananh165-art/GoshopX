from __future__ import annotations

import json
from contextlib import contextmanager
from datetime import datetime, timedelta, timezone

from sqlalchemy import text
from sqlalchemy.orm import Session

from shared.db.models import ChatConversation


class PostgresChatMemory:
    """Small bounded conversation store; thread IDs are owner-scoped HMACs."""

    def __init__(self, checkpointer=None):
        self.checkpointer = checkpointer

    def load(self, thread_id: str) -> list[dict[str, str]]:
        from shared.db.session import replica_engine

        expired = False
        messages = []
        with Session(replica_engine) as session, session.begin():
            row = session.get(ChatConversation, thread_id)
            if row is None:
                return []
            if row.updated_at and row.updated_at.replace(tzinfo=timezone.utc) < datetime.now(timezone.utc) - timedelta(days=7):
                session.delete(row)
                expired = True
            else:
                try:
                    messages = json.loads(row.messages_json)
                except (TypeError, json.JSONDecodeError):
                    messages = []
        if expired and self.checkpointer:
            self.checkpointer.delete_thread(thread_id)
        return _compact_history(_valid_messages(messages))

    def delete(self, thread_id: str) -> None:
        from shared.db.session import replica_engine

        with replica_engine.begin() as connection:
            connection.execute(text("DELETE FROM chat_conversations WHERE thread_id = :thread_id"), {"thread_id": thread_id})
        if self.checkpointer:
            self.checkpointer.delete_thread(thread_id)

    @contextmanager
    def thread_lock(self, thread_id: str):
        """Serialize turns for one conversation while allowing other threads through."""
        from shared.db.session import replica_engine

        with replica_engine.connect() as connection:
            if connection.dialect.name != "postgresql":
                yield
                return
            connection.execute(text("SELECT pg_advisory_lock(hashtext(:thread_id))"), {"thread_id": thread_id})
            try:
                yield
            finally:
                connection.execute(text("SELECT pg_advisory_unlock(hashtext(:thread_id))"), {"thread_id": thread_id})

    def save(self, thread_id: str, messages: list[dict[str, str]]) -> None:
        from shared.db.session import replica_engine

        incoming = _valid_messages(messages)
        with replica_engine.begin() as connection:
            if connection.dialect.name != "postgresql":
                raise RuntimeError("Persistent chat memory requires PostgreSQL")
            # ChatService holds a session advisory lock for the whole turn. Acquiring
            # the same key from this second pooled connection would deadlock.
            current = connection.execute(text(
                "SELECT messages_json FROM chat_conversations WHERE thread_id = :thread_id FOR UPDATE"
            ), {"thread_id": thread_id}).scalar_one_or_none()
            try:
                previous = _valid_messages(json.loads(current)) if current else []
            except (TypeError, json.JSONDecodeError):
                previous = []
            merged = _compact_history(merge_histories(previous, incoming))
            payload = json.dumps(merged, ensure_ascii=False, separators=(",", ":"))
            connection.execute(text("""
                INSERT INTO chat_conversations(thread_id, messages_json, updated_at)
                VALUES (:thread_id, :messages_json, CURRENT_TIMESTAMP)
                ON CONFLICT (thread_id) DO UPDATE SET
                  messages_json = EXCLUDED.messages_json, updated_at = CURRENT_TIMESTAMP
            """), {"thread_id": thread_id, "messages_json": payload})


def _valid_messages(value: object) -> list[dict[str, object]]:
    if not isinstance(value, list):
        return []
    result = []
    for item in value:
        if isinstance(item, dict) and item.get("role") in {"user", "assistant"}:
            message: dict[str, object] = {"role": str(item["role"]), "content": str(item.get("content", ""))[:2000]}
            ids = item.get("product_ids")
            if isinstance(ids, list):
                message["product_ids"] = [str(product_id)[:100] for product_id in ids[:10] if isinstance(product_id, (str, int))]
            if item.get("summary") is True:
                message["summary"] = True
            result.append(message)
    return result


def merge_histories(previous: list[dict[str, str]], incoming: list[dict[str, str]]) -> list[dict[str, str]]:
    """Merge overlapping bounded windows without duplicating old turns."""
    if incoming[:len(previous)] == previous:
        return incoming
    if previous[:len(incoming)] == incoming:
        return previous

    shared_prefix = 0
    while shared_prefix < min(len(previous), len(incoming)) and previous[shared_prefix] == incoming[shared_prefix]:
        shared_prefix += 1
    if shared_prefix:
        return [*previous, *incoming[shared_prefix:]]

    overlap = min(len(previous), len(incoming))
    while overlap and previous[-overlap:] != incoming[:overlap]:
        overlap -= 1
    tail = incoming[overlap:]
    if not tail:
        return previous
    return [*previous, *tail]


def _compact_history(messages: list[dict[str, object]]) -> list[dict[str, object]]:
    summary = next((item for item in messages if item.get("summary") is True), None)
    ordinary = [item for item in messages if item.get("summary") is not True]
    if len(ordinary) <= 16:
        return ([summary] if summary else []) + ordinary[-(15 if summary else 16):]

    old, recent = ordinary[:-14], ordinary[-14:]
    requests = [str(item.get("content", ""))[:120] for item in old if item.get("role") == "user"]
    if summary:
        previous = str(summary.get("content", ""))
        requests = [previous.removeprefix("Conversation summary: "), *requests]
    product_ids = list(dict.fromkeys(
        str(product_id)[:100]
        for item in ([summary] if summary else []) + old
        for product_id in item.get("product_ids", [])
        if isinstance(product_id, (str, int))
    ))[-10:]
    compacted = {
        "role": "assistant",
        "summary": True,
        "content": "Conversation summary: " + " | ".join(requests[-8:])[-1100:],
        "product_ids": product_ids,
    }
    return [compacted, *recent]
