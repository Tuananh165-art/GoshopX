# Admin Commerce Epic

Status: Proposed for implementation
Date: 2026-07-30

## Problem

The current admin capability governs accounts only. A production ecommerce operation also needs catalog moderation, order support, payment reconciliation, inventory controls, audit search, and revenue reporting. These capabilities require cross-service read models but must not break service data ownership.

## Decision Direction

Keep command ownership in Product, Order, Payment, and Inventory. Add an `admin` read/orchestration boundary only for cross-domain audit and reporting projections. GraphQL remains the public boundary; gRPC is used for synchronous commands; Kafka facts feed PostgreSQL aggregate tables; Redis caches dashboard responses; Elasticsearch remains Product-owned for catalog search.

## Scope

- Admin Catalog: all-product CRUD, categories, publish state, moderation, media metadata.
- Admin Order: filtered list, detail, status transition, cancellation, refund request.
- Admin Payment: transaction history, status, refund, reconciliation exceptions.
- Admin Inventory: stock adjustment, low-stock list, reservation monitoring.
- Admin Dashboard: daily/monthly revenue, orders, GMV, AOV, top products, payment success rate.
- Admin Audit: paginated, filterable, immutable admin action history.
- Reporting/Analytics: Kafka consumers, idempotent PostgreSQL projections, Redis dashboard cache.

## Non-goals

- No direct cross-service database queries.
- No refund without payment-provider validation and a persisted idempotency key.
- No image binary in PostgreSQL; media uses object storage/local artifact abstraction with metadata ownership in Product.
- No replacing Elasticsearch as the product search source.

## Release Slices

1. Catalog metadata and moderation contract.
2. Admin order/payment support commands and history queries.
3. Inventory adjustment and reservation operations.
4. Kafka reporting projections and dashboard queries.
5. Audit search, cache invalidation, security review, and E2E release flow.

## Business Outcomes

Operators can moderate the catalog, resolve order/payment incidents, correct stock through controlled commands, and see trusted revenue metrics with traceable audit evidence.

