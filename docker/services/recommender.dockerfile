FROM python:3.11-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    libpq-dev \
    && rm -rf /var/lib/apt/lists/*

# Install uv from PyPI instead of pulling a second image from GHCR.
RUN python -m pip install --no-cache-dir uv

# Install dependencies first so this layer is cached when only app code changes.
COPY recommender/pyproject.toml recommender/uv.lock ./
ENV UV_COMPILE_BYTECODE=1 \
    UV_LINK_MODE=copy \
    UV_PYTHON_DOWNLOADS=never

RUN uv sync --frozen --no-install-project --no-dev

COPY recommender /app
RUN uv sync --frozen --no-dev

ENV PATH="/app/.venv/bin:${PATH}" \
    PYTHONPATH="/app/app:${PYTHONPATH}"

RUN useradd --uid 10001 --create-home --shell /usr/sbin/nologin goshopx \
    && chown -R goshopx:goshopx /app
USER 10001:10001

EXPOSE 8080

WORKDIR /app
