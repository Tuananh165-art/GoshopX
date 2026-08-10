#!/usr/bin/env python3
"""Probe the configured OpenAI-compatible LLM without exposing credentials."""

from __future__ import annotations

import json
import os
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path


REQUIRED = ("AI_ENDPOINT", "AI_API_KEY", "AI_MODEL")


def load_env(path: Path) -> dict[str, str]:
    values: dict[str, str] = {}
    for raw_line in path.read_text(encoding="utf-8").splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        values[key.strip()] = value.strip()
    return values


def main() -> int:
    env_path = Path(__file__).resolve().parents[1] / ".env"
    if not env_path.is_file():
        print("probe_result=config_error env_file_missing")
        return 2

    config = load_env(env_path)
    missing = [name for name in REQUIRED if not config.get(name)]
    if missing:
        print(f"probe_result=config_error missing={','.join(missing)}")
        return 2

    retries = max(0, min(int(config.get("AI_MAX_RETRIES", "2")), 5))
    timeout = max(1.0, float(config.get("AI_TIMEOUT_SECONDS", "30")))
    payload = {
        "model": config["AI_MODEL"],
        "messages": [{"role": "user", "content": "Reply exactly: OK"}],
        "temperature": 0,
        "max_tokens": 8,
    }
    request = urllib.request.Request(
        config["AI_ENDPOINT"],
        data=json.dumps(payload).encode("utf-8"),
        headers={
            "Content-Type": "application/json",
            "Accept": "application/json",
            "Authorization": f"Bearer {config['AI_API_KEY']}",
            "User-Agent": "GoshopX-Recommender/1.0",
        },
        method="POST",
    )

    for attempt in range(1, retries + 2):
        try:
            with urllib.request.urlopen(request, timeout=timeout) as response:
                status = int(getattr(response, "status", 200))
            print(f"probe_result=http_{status} model={config['AI_MODEL']} attempt={attempt}")
            return 0 if status == 200 else 1
        except urllib.error.HTTPError as error:
            print(f"probe_result=http_{error.code} model={config['AI_MODEL']} attempt={attempt}")
            if error.code not in (408, 429) and not 500 <= error.code < 600:
                return 1
        except (urllib.error.URLError, TimeoutError) as error:
            print(f"probe_result=network_{type(error).__name__} model={config['AI_MODEL']} attempt={attempt}")
        if attempt <= retries:
            time.sleep(min(attempt, 3))

    return 1


if __name__ == "__main__":
    sys.exit(main())
