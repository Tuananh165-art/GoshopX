FROM ghcr.io/astral-sh/uv:0.12.3 AS uv

FROM python:3.11-slim AS build

WORKDIR /app

COPY --from=uv /uv /uvx /usr/local/bin/

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    libpq-dev \
    && rm -rf /var/lib/apt/lists/*

# Install dependencies first so this layer is cached when only app code changes.
COPY recommender/pyproject.toml recommender/uv.lock ./
ENV UV_COMPILE_BYTECODE=1 \
    UV_LINK_MODE=copy \
    UV_PYTHON_DOWNLOADS=never

RUN uv sync --frozen --no-install-project --no-dev

COPY recommender /app
RUN uv sync --frozen --no-dev

FROM python:3.11-slim

WORKDIR /app

# implicit loads OpenMP-backed native extensions at import time. The builder
# has the compiler toolchain, but the slim runtime must provide libgomp too.
RUN apt-get update && apt-get install -y --no-install-recommends \
    libgomp1 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /app /app

# The application runs from /app/.venv. Remove global packaging tools whose
# vendored dependencies would otherwise be scanned in the runtime image.
RUN rm -rf \
    /usr/local/lib/python3.11/site-packages/pip \
    /usr/local/lib/python3.11/site-packages/pip-*.dist-info \
    /usr/local/lib/python3.11/site-packages/setuptools \
    /usr/local/lib/python3.11/site-packages/setuptools-*.dist-info \
    /usr/local/lib/python3.11/site-packages/wheel \
    /usr/local/lib/python3.11/site-packages/wheel-*.dist-info

ENV PATH="/app/.venv/bin:${PATH}" \
    PYTHONPATH="/app/app:${PYTHONPATH}"

RUN useradd --uid 10001 --create-home --shell /usr/sbin/nologin goshopx \
    && chown -R goshopx:goshopx /app
USER 10001:10001

EXPOSE 8080

WORKDIR /app
