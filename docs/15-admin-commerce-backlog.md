# Admin Commerce Backlog

Status: Sprint proposal
Date: 2026-07-30

## P0 Stories

- `AC-01` As an operations admin, I can list and filter all products by publish/moderation state.
- `AC-02` As an operations admin, I can create/update/delete product metadata and category assignments.
- `AC-03` As a moderator, I can publish, unpublish, approve, or reject a product with a reason.
- `AC-04` As a catalog operator, I can attach and remove product media metadata with validation.
- `AC-05` As support, I can list and inspect orders by account, status, date, and payment status.
- `AC-06` As support, I can request a cancellation or refund with an idempotency key.
- `AC-07` As finance, I can list transactions and reconcile provider status against local state.
- `AC-08` As operations, I can adjust stock and inspect low-stock and active reservations.
- `AC-09` As leadership, I can view daily/monthly revenue, GMV, AOV, order count, top products, and payment success rate.
- `AC-10` As security, I can search immutable admin audit events by actor, action, target, and time range.

## Definition of Ready

The story names its owning service, GraphQL field, gRPC request, Kafka fact, persistence owner, authorization role, idempotency rule, failure mapping, and test level. Payment/refund stories also identify provider behavior and reconciliation fallback.

## Definition of Done

Unit, repository, gRPC contract, Kafka idempotency, authorization, and relevant E2E tests pass. Generated code is refreshed. Compose/env/README/docs are updated. Metrics are calculated from versioned facts rather than client input. Rollback and replay instructions exist.

## Dependencies

Catalog precedes dashboard top-products. Order/payment facts precede revenue metrics. Inventory adjustments precede stock KPIs. Audit event envelope precedes admin audit search.

