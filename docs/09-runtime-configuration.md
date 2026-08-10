# Runtime Configuration and Local Stack

This document captures the runtime and configuration updates added with the core commerce rollout on July 29, 2026.

## Purpose

- Keep local runtime defaults visible and repeatable.
- Make `.env` and `.env.example` the first place to review when stack startup fails.
- Document the extra dependencies introduced by `inventory`, `cart`, and `notification`.

## Source Files

- [`../.env`](../.env): local developer defaults
- [`../.env.example`](../.env.example): template for new environments
- [`../docker-compose.yaml`](../docker-compose.yaml): runtime wiring and container topology

## New Runtime Dependencies

- `inventory_db`: PostgreSQL owned by `inventory`
- `notification_db`: PostgreSQL owned by `notification`
- `redis`: Redis owned by `cart`

## Important Environment Variables

### Databases

- `ACCOUNT_DATABASE_URL`
- `ORDER_DATABASE_URL`
- `PAYMENT_DATABASE_URL`
- `INVENTORY_DATABASE_URL`
- `NOTIFICATION_DATABASE_URL`
- `RECOMMENDER_DATABASE_URL`
- `PRODUCT_DATABASE_URL`

### Internal Service URLs

- `ACCOUNT_SERVICE_URL`
- `PRODUCT_SERVICE_URL`
- `ORDER_SERVICE_URL`
- `PAYMENT_SERVICE_URL`
- `RECOMMENDER_SERVICE_URL`
- `INVENTORY_SERVICE_URL`
- `CART_SERVICE_URL`
- `NOTIFICATION_SERVICE_URL`

### Messaging and Cache

- `KAFKA_BOOTSTRAP_SERVERS`
- `ADMIN_EVENTS_TOPIC`
- `ADMIN_DATABASE_URL`
- `ADMIN_SERVICE_URL`
- `PRODUCT_EVENTS_TOPIC`
- `INTERACTION_EVENTS_TOPIC`
- `ORDER_EVENTS_TOPIC`
- `PAYMENT_EVENTS_TOPIC`
- `INVENTORY_EVENTS_TOPIC`
- `CART_EVENTS_TOPIC`
- `REDIS_URL`

### Auth and Payment

- `SECRET_KEY`
- `ISSUER`
- `DODO_API_KEY`
- `DODO_WEBHOOK_SECRET`
- `DODO_CHECKOUT_URL`
- `DODO_TEST_MODE`

### Local Gateway Management

- Kong proxy: `http://localhost:8080`
- Kong Admin API (loopback only): `http://localhost:8001`
- Kong Manager (loopback only): `http://localhost:8002`

## Compose Behavior

`docker-compose.yaml` now reads configuration from environment variables with local-safe defaults. This means:

- local startup still works without editing every value,
- `.env` can override DB credentials, JWT settings, Kafka topics, and Redis endpoints,
- `account` and `graphql` now explicitly receive `SECRET_KEY` and `ISSUER`,
- `payment`, `inventory`, `cart`, and `notification` now read the new service and topic settings from environment variables.

## Local Volume Recovery

If you change Postgres credentials after a previous `docker compose up`, Docker volumes may still hold the old user/password state. Symptoms include:

- `password authentication failed`
- services repeatedly restarting against PostgreSQL

Local recovery example:

```bash
docker compose rm -sf account order payment graphql account_db order_db payment_db
docker volume rm goshopx_account_db_data goshopx_order_db_data goshopx_payment_db_data
docker compose up -d account_db order_db payment_db account order payment graphql
```

Only run volume removal when you accept losing local dev data in those databases.

## Validation Commands

```bash
docker compose config --quiet
docker compose up --build -d
docker compose ps
```

GraphQL checks:

- `http://localhost:8080/health`
- `http://localhost:8080/playground`

Gateway management checks:

- `http://localhost:8001`
- `http://localhost:8002`
