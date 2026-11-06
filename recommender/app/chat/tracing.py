"""Metadata-only LangSmith tracing. Prompts, outputs and identifiers stay local."""

from __future__ import annotations

import logging
from datetime import datetime, timezone
from typing import Any

logger = logging.getLogger(__name__)


class RedactedLangSmithTrace:
    def __init__(self, api_key: str, endpoint: str, project: str):
        self._client = None
        if api_key:
            try:
                from langsmith import Client
                self._client = Client(api_key=api_key, api_url=endpoint)
            except Exception:
                logger.warning("LangSmith tracing unavailable")
        self._project = project

    def record(self, *, route: str, request_chars: int, product_count: int, warnings: list[str], latency_ms: int) -> None:
        if self._client is None:
            return
        try:
            now = datetime.now(timezone.utc)
            safe_route = route if route in {"discovery", "recommendation", "cart", "multimodal"} else "unknown"
            safe_warnings = [warning[:64] for warning in warnings[:10] if warning.replace("_", "").isalnum()]
            self._client.create_run(
                name="goshopx-chat-turn", run_type="chain", project_name=self._project,
                start_time=now, end_time=now,
                hide_inputs=True, hide_outputs=True,
                inputs={"route": safe_route, "request_chars": max(0, min(request_chars, 1000))},
                outputs={"product_count": max(0, min(product_count, 10)), "warning_codes": safe_warnings},
                extra={"metadata": {"latency_ms": max(0, latency_ms), "redacted": True}},
            )
        except Exception:
            logger.warning("LangSmith metadata trace failed")
