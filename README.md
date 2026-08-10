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
- `admin`: Kafka-backed audit/reporting read model with dashboard and audit gRPC queries
- `graphql`: public GraphQL API gateway
- `recommender`: Python recommendation services and consumers

Admin governance is implemented as RBAC inside `account` for v1. Roles are carried in new JWTs, privileged operations are exposed through GraphQL, account gRPC rechecks policy, and audit facts use `admin_events`.

## Runtime Stack

- PostgreSQL: `account_db`, `order_db`, `payment_db`, `inventory_db`, `notification_db`, `admin_db`, `recommender_db`
- Kafka: `kafka`
- Elasticsearch: `product_db`
- Redis: `redis`
- MinIO: `minio`

### Local infrastructure dashboards

After `docker compose up -d`:

- Kafka UI: http://localhost:8088
- RedisInsight: http://localhost:5540
- MinIO Console: http://localhost:9001
- MinIO API: http://localhost:9000
- Kibana: http://localhost:5601

Kafka UI is configured read-only for local inspection. RedisInsight persists its own connection metadata in `redisinsight_data`; add the Redis connection as `redis:6379` from inside the Compose network or `localhost:6379` from the host. MinIO Console is included in the official MinIO image and uses the same `minio_data` volume as the object API. Kibana is pinned to `6.2.4` to match Elasticsearch OSS `6.2.4`; in Kibana, create an index pattern such as `catalog*` and select `@timestamp` only if the index contains that field.

### Data ownership and synchronization strategy

Do not write every UI field into every storage system. Each system has one owner:

- PostgreSQL: transactional source of truth for accounts, orders, payments, inventory, reservations, notifications, admin read models, and recommender state.
- Elasticsearch: Product catalog/search document source for product content, catalog rating/reviews, and read-optimized search. DummyJSON is only a repeatable seed input; it is not the checkout/payment source of truth.
- Redis: short-lived active cart state and reservation-aware cart reads. Cart data is JSON with TTL; it is not the durable order/payment database.
- Kafka: durable business facts and integration boundary (`order_events`, `payment_events`, `inventory_events`, `cart_events`, `product_events`, `admin_events`, and `interaction_events`). Kafka is not queried by the browser and is not a replacement for transactional databases.
- MinIO: binary object storage for product media. PostgreSQL/Elasticsearch stores metadata and public object references; image bytes are not duplicated into GraphQL or Elasticsearch documents.

The safe write pattern is: commit the owning transactional store first, publish an idempotent Kafka event/outbox record, update read models/search indexes asynchronously, and invalidate/re-read Redis caches. The browser reads only through GraphQL and must never write directly to PostgreSQL, Kafka, Redis, MinIO, or Elasticsearch.

Core commerce topics:

- `product_events`
- `interaction_events`
- `order_events`
- `payment_events`
- `inventory_events`
- `cart_events`
- `admin_events`

## Configuration

Runtime defaults now live in:

- `.env`: local developer values
- `.env.example`: shareable template

Key groups:

- Database URLs: account, order, payment, inventory, notification, recommender
- Internal service URLs: account, product, order, payment, recommender, inventory, cart, notification
- Admin reporting: `ADMIN_DATABASE_URL`, `ADMIN_SERVICE_URL`, `ADMIN_EVENTS_TOPIC`
- Infra: Kafka, Redis, MinIO (`MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, `MINIO_BUCKET`, `MINIO_PUBLIC_URL`)
- Auth: `SECRET_KEY`, `ISSUER`
- Payments: `DODO_API_KEY`, `DODO_WEBHOOK_SECRET`, `DODO_CHECKOUT_URL`, `DODO_TEST_MODE`
- DummyJSON catalog seed: `DUMMYJSON_BASE_URL`, `DUMMYJSON_SEED_ENABLED`, `DUMMYJSON_SEED_LIMIT`
- Admin governance: `ADMIN_EVENTS_TOPIC`; roles and account status are owned by `account`

`docker-compose.yaml` reads these values with local-safe defaults, so you can override only what you need.

Google Login and Gmail notification setup is documented in [docs/21-google-login-gmail-notifications.md](./docs/21-google-login-gmail-notifications.md). Google Login uses a Web OAuth client ID; Gmail delivery uses SMTP with a Gmail App Password.

### DummyJSON product catalog

The Product service maps DummyJSON's complete product payload into the local Elasticsearch catalog: title/description, category, brand, price/discount, rating, stock, SKU, dimensions, shipping/warranty, reviews, metadata, thumbnail, and all images. Existing GraphQL product queries support search, category filtering, and pagination.

To seed the catalog on Product service startup:

```bash
DUMMYJSON_SEED_ENABLED=true DUMMYJSON_SEED_LIMIT=0 docker compose up --build -d product
```

`DUMMYJSON_SEED_LIMIT=0` imports all products. The seed preserves DummyJSON product IDs, is repeatable, and publishes products as `published`/`approved` demo catalog records. DummyJSON remains an external placeholder source; production catalog ownership stays with the Product service and Elasticsearch.

### Connecting to PostgreSQL with TablePlus

For local TablePlus access, Compose publishes PostgreSQL only on loopback with one host port per bounded context:

| Service | Host | Port | Database |
|---|---|---:|---|
| Account | `127.0.0.1` | `5433` | value of `POSTGRES_DB` |
| Order | `127.0.0.1` | `5434` | value of `POSTGRES_DB` |
| Payment | `127.0.0.1` | `5435` | value of `POSTGRES_DB` |
| Inventory | `127.0.0.1` | `5436` | value of `POSTGRES_DB` |
| Notification | `127.0.0.1` | `5437` | value of `POSTGRES_DB` |
| Admin | `127.0.0.1` | `5438` | value of `POSTGRES_DB` |
| Recommender | `127.0.0.1` | `5439` | value of `POSTGRES_DB` |

In TablePlus select PostgreSQL and enter the host, port, database, user, and rotated password from your local `.env`. Set SSL mode to `Disable` for this local Compose setup. Do not paste credentials into documentation, commits, screenshots, or chat. Product is not PostgreSQL: inspect its Elasticsearch data through Kibana at http://localhost:5601.

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
- Kong Manager: `http://localhost:8002`
- Kong Admin API (local only): `http://localhost:8001`

## Verification

Suggested checks:

```bash
go test -race -count=1 ./account/... ./product/... ./order/... ./payment/... ./graphql/... ./pkg/... ./inventory/... ./cart/... ./notification/...
go test ./tests/e2e
docker compose config --quiet
docker compose up --build -d
```

## Deploy K3s / DevSecOps

Kubernetes uses the same architecture as Compose: Traefik is the K3s ingress
controller, but it forwards only to DB-less Kong. Kong routes public Web,
GraphQL and the narrow Payment webhook; gRPC services and all data systems stay
private `ClusterIP` Services. GitHub Actions builds/scans immutable GHCR images,
promotes the image SHA in Git, and Argo CD reconciles Git into K3s. GitHub
Actions does not receive a cluster kubeconfig.

For Ubuntu 22.04/24.04 VPS deployment, follow the complete Vietnamese guide:

- [Ubuntu K3s step-by-step deployment](./docs/23-ubuntu-k3s-step-by-step-deployment.md)
- [Ubuntu deployment scripts](./scripts/ubuntu/README.md)
- [K3s DevSecOps lab runbook](./docs/22-devsecops-k3s-lab-runbook.md)

The 8 GB / 50 GB profile must be deployed in phases. Start with core commerce;
enable infrastructure, Milvus, recommender training and extra dashboards only
after measuring RAM/disk headroom. Operator UIs are ClusterIP by default; use
`kubectl port-forward` rather than public NodePorts.

## Local Runtime Notes

- If Postgres credentials change after a previous run, old Docker volumes can keep stale users/passwords.
- For this repo, the most common local reset targets are:
  - `goshopx_account_db_data`
  - `goshopx_order_db_data`
  - `goshopx_payment_db_data`
- Redis is required for `cart`.
- `inventory`, `cart`, and `notification` are required for the new core commerce flow.
- MinIO backs the admin GraphQL multipart product-media upload. Product stores only media metadata, never image bytes in Elasticsearch.
- If Docker Desktop Buildx reports `no such job` or stops producing progress, restart Docker Desktop and rerun `docker compose build` followed by `docker compose up -d`.
- Kong management ports are loopback-bound in Compose: `8001` and `8002` are reachable only from the local machine unless you intentionally change the port bindings.

## Documentation

Start with [docs/00-index.md](./docs/00-index.md). The docs set includes BMAD process, Scrum delivery, architecture decisions, core-commerce specs, and runtime configuration notes.
