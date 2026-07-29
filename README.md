# GoshopX

GoshopX is a GraphQL-first e-commerce microservices stack built with Go and Python. Public client traffic goes through GraphQL, internal synchronous calls use gRPC, asynchronous business facts move through Kafka, product search stays in Elasticsearch, and active cart state now uses Redis.

## Services

- `account`: account registration, login, JWT issuance
- `product`: product CRUD and Elasticsearch-backed catalog/search
- `order`: order creation and order event publishing
- `payment`: payment workflow, webhook handling, payment event publishing
- `inventory`: stock ownership, reservations, low-stock signals
- `cart`: Redis-backed active cart with reservation coordination
- `notification`: in-app notification feed backed by PostgreSQL
- `graphql`: public GraphQL API gateway
- `recommender`: Python recommendation services and consumers

## Runtime Stack

- PostgreSQL: `account_db`, `order_db`, `payment_db`, `inventory_db`, `notification_db`, `recommender_db`
- Kafka: `kafka`
- Elasticsearch: `product_db`
- Redis: `redis`

Core commerce topics:

- `product_events`
- `interaction_events`
- `order_events`
- `payment_events`
- `inventory_events`
- `cart_events`

## Configuration

Runtime defaults now live in:

- `.env`: local developer values
- `.env.example`: shareable template

Key groups:

- Database URLs: account, order, payment, inventory, notification, recommender
- Internal service URLs: account, product, order, payment, recommender, inventory, cart, notification
- Infra: Kafka, Redis
- Auth: `SECRET_KEY`, `ISSUER`
- Payments: `DODO_API_KEY`, `DODO_WEBHOOK_SECRET`, `DODO_CHECKOUT_URL`, `DODO_TEST_MODE`

`docker-compose.yaml` reads these values with local-safe defaults, so you can override only what you need.

## Getting Started

1. Review `.env.example` and update `.env` for your machine.
2. Validate Compose:

```bash
docker compose config --quiet
```

3. Build and start the stack:

```bash
docker compose up --build -d
```

4. Open GraphQL:

- Playground: `http://localhost:8080/playground`
- GraphQL endpoint: `http://localhost:8080/graphql`
- Health: `http://localhost:8080/health`

## Verification

Suggested checks:

```bash
go test -race -count=1 ./account/... ./product/... ./order/... ./payment/... ./graphql/... ./pkg/... ./inventory/... ./cart/... ./notification/...
go test ./tests/e2e
docker compose config --quiet
docker compose up --build -d
```

## Local Runtime Notes

- If Postgres credentials change after a previous run, old Docker volumes can keep stale users/passwords.
- For this repo, the most common local reset targets are:
  - `goshopx_account_db_data`
  - `goshopx_order_db_data`
  - `goshopx_payment_db_data`
- Redis is required for `cart`.
- `inventory`, `cart`, and `notification` are required for the new core commerce flow.

## Documentation

Start with [docs/00-index.md](./docs/00-index.md). The docs set includes BMAD process, Scrum delivery, architecture decisions, core-commerce specs, and runtime configuration notes.
