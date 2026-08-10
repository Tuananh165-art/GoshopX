# Admin Commerce Implementation Status

Date: 2026-07-30

## Completed In This Slice

- Added `admin/models` normalized order/payment facts and dashboard result contract.
- Added deterministic dashboard aggregation for GMV, revenue, order count, AOV, payment success rate, and top products.
- Added unit coverage for paid/pending filtering, multi-line order counting, and top-product ranking.
- Added PostgreSQL reporting repository with idempotent `processed_events`, audit events, daily metrics, and product metrics.
- Added Kafka envelope handler and Redis dashboard cache with TTL and safe wildcard invalidation.
- Added repository idempotency integration coverage using SQLite.
- Added `admin` gRPC service/client for dashboard and audit queries.
- Added GraphQL `adminDashboard` and `adminAuditEvents`, gated by admin roles.
- Added `admin_db`, `admin` Docker image/service, Kafka topic wiring, Redis dependency, and runtime environment variables.
- Standardized account audit publishing onto the versioned shared event envelope so the admin projection can deduplicate it.
- Added Catalog commands backed by Product-owned Elasticsearch documents: category creation/listing, product publish/moderation state, and ordered media metadata.
- Added trusted internal admin gRPC operations for catalog moderation, media attachment, category creation, and owner-bypass product editing/deletion policy; GraphQL exposes the category, moderation, and media commands to `operations_admin` and `platform_admin` only.
- Public catalog list/search now returns only products that are both `published` and `approved`; seller-created products start as `draft` and `pending` moderation.
- Regenerated Product protobuf and GraphQL generated code; focused `go test -mod=mod ./product/... ./graphql/...` passed using workspace-local Go caches.

## Pending Integration

- Added MinIO-backed GraphQL multipart `Upload` for product images (JPG/JPEG/PNG/WebP, 10 MiB maximum). The GraphQL gateway authorizes the operation and Product retains only object URL metadata.
- Added admin Order list/filter/detail/cancel, Payment transaction list/refund request/reconciliation read, and Inventory adjustment/low-stock/reservation-monitoring gRPC and GraphQL command paths.
- Added admin Kafka quarantine persistence for malformed, unsupported-version, and projection-failing messages. Replay reuses the projection and processed-event deduplication path.

## Remaining Runtime Evidence

- `docker compose config --quiet` passed on 2026-07-30.
- Focused `go test -mod=mod ./graphql/... ./order/... ./payment/... ./inventory/... ./admin/... ./product/...` passed on 2026-07-30 using workspace-local Go caches.
- Runtime was attempted with `docker compose up --build -d`; Docker Desktop Buildx timed out, then returned `no such job` during parallel build and hung during sequential build with Bake disabled. The existing `product_db` Elasticsearch container was found stopped (`Exited (255)`) and was successfully restarted; it is now `Up`.
- New images and authenticated multipart/E2E are not yet verified because Docker Desktop did not complete image builds. Restart Docker Desktop/Buildx, then run `docker compose build`, `docker compose up -d`, and `go test ./tests/e2e`.
- A real Dodo refund remains intentionally excluded from local E2E because it requires a configured provider account and signed webhook callback; unit/contract coverage verifies the request path without external money movement.
