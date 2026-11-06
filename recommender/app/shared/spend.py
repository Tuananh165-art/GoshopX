"""Conservative per-call AI spend ceiling for the short-lived lab."""

from __future__ import annotations

import os
import uuid
from decimal import Decimal, ROUND_CEILING

from sqlalchemy import text


def token_cost(input_tokens: int, output_tokens: int, input_rate: int, output_rate: int) -> int:
    amount = (Decimal(max(0, input_tokens) * input_rate + max(0, output_tokens) * output_rate)
              / Decimal(1_000_000))
    return int(amount.to_integral_value(rounding=ROUND_CEILING))


class SpendLimitExceeded(RuntimeError):
    pass


class ProviderSpendLedger:
    def __init__(self, *, limit_vnd: int | None = None, reserve_vnd: int | None = None):
        self.limit = limit_vnd if limit_vnd is not None else int(os.getenv("AI_SPEND_LIMIT_VND", "200000"))
        self.reserve = reserve_vnd if reserve_vnd is not None else int(os.getenv("AI_SPEND_RESERVE_PER_CALL_VND", "500"))

    def reserve_call(self, provider: str) -> str:
        from shared.db.session import replica_engine
        key = str(uuid.uuid4())
        with replica_engine.begin() as connection:
            if connection.dialect.name != "postgresql":
                raise SpendLimitExceeded("AI spend ledger requires PostgreSQL")
            connection.execute(text("SELECT pg_advisory_xact_lock(719240118)"))
            spent = int(connection.execute(text("SELECT COALESCE(SUM(CASE WHEN status='released' THEN 0 ELSE COALESCE(actual_vnd,reserved_vnd) END),0) FROM ai_spend_entries")).scalar_one())
            if self.reserve <= 0 or spent + self.reserve > self.limit:
                raise SpendLimitExceeded("configured AI spend ceiling reached")
            connection.execute(text("INSERT INTO ai_spend_entries(id, provider, reserved_vnd) VALUES (:id,:provider,:amount)"),
                               {"id": key, "provider": provider[:32], "amount": self.reserve})
        return key

    def settle_tokens(self, entry_id: str, *, input_tokens: int, output_tokens: int = 0) -> int | None:
        """Settle provider-reported usage when operator-configured rates exist.

        Amounts are VND per million tokens. Missing rates keep the original
        reservation charged instead of silently recording a zero-cost call.
        """
        provider = self._provider_for(entry_id)
        input_rate = _rate(provider, "PROMPT_RATE_VND_PER_MILLION")
        output_rate = _rate(provider, "COMPLETION_RATE_VND_PER_MILLION")
        actual = None if input_rate is None or output_rate is None else token_cost(input_tokens, output_tokens, input_rate, output_rate)
        self._update(entry_id, input_tokens=max(0, input_tokens), output_tokens=max(0, output_tokens), actual_vnd=actual,
                     status="settled" if actual is not None else "usage_unpriced")
        return actual

    def settle_call_cost(self, entry_id: str, *, rate_env: str) -> int | None:
        """Settle a non-token provider call; missing price keeps its reservation."""
        raw = os.getenv(rate_env, "").strip()
        actual = int(raw) if raw.isdigit() else None
        if actual is not None:
            self._update(entry_id, input_tokens=None, output_tokens=None, actual_vnd=actual, status="settled")
        else:
            self._update(entry_id, input_tokens=None, output_tokens=None, actual_vnd=None, status="usage_unpriced")
        return actual

    def release_call(self, entry_id: str) -> None:
        """Release only when the provider response proves it did not process a billable request."""
        self._update(entry_id, input_tokens=None, output_tokens=None, actual_vnd=0, status="released")

    @staticmethod
    def _provider_for(entry_id: str) -> str:
        from shared.db.session import replica_engine
        with replica_engine.connect() as connection:
            row = connection.execute(text("SELECT provider FROM ai_spend_entries WHERE id=:id"), {"id": entry_id}).first()
        if row is None:
            raise ValueError("unknown spend reservation")
        return str(row[0])

    @staticmethod
    def _update(entry_id: str, *, input_tokens: int | None, output_tokens: int | None,
                actual_vnd: int | None, status: str) -> None:
        from shared.db.session import replica_engine
        with replica_engine.begin() as connection:
            connection.execute(text("""UPDATE ai_spend_entries SET input_tokens=:input_tokens,
              output_tokens=:output_tokens, actual_vnd=:actual_vnd, status=:status WHERE id=:id"""),
              {"id": entry_id, "input_tokens": input_tokens, "output_tokens": output_tokens,
               "actual_vnd": actual_vnd, "status": status})


def _rate(provider: str, suffix: str) -> int | None:
    raw = os.getenv(f"AI_{provider.upper()}_{suffix}", "").strip()
    return int(raw) if raw.isdigit() else None
