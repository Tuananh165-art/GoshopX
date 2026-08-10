# Admin Commerce Specification

Status: Proposed
Date: 2026-07-30

## Authorization

- `platform_admin`: all admin capabilities.
- `operations_admin`: catalog, inventory, and operational order actions.
- `support_admin`: account and order support; no role assignment, catalog publish, or stock adjustment.
- `finance_admin`: payment and reconciliation actions. This role is reserved for the next RBAC migration unless the platform uses `platform_admin` temporarily.
- Every command is rechecked by the owning service; GraphQL role checks are not the source of truth.

## Catalog Rules

- Product price must be positive; category IDs must exist and be active.
- Product lifecycle is `draft -> pending_review -> published` or `rejected`; unpublish returns to `draft`.
- Only operations/platform admins can moderate. A rejection requires a non-empty reason.
- Media metadata validates MIME type, size, checksum, and owner; binary storage is outside the product database.
- Delete is soft-delete after the product has order history; hard delete is never exposed publicly.

## Order and Payment Rules

- Admin order reads are projection/read APIs and never mutate Order storage directly.
- Cancellation is allowed only for cancellable order states and must emit an order fact.
- Refund requires payment status eligibility, provider idempotency key, local transaction record, and an audit event.
- Provider webhook remains authoritative for final payment settlement; reconciliation detects local/provider divergence.

## Inventory Rules

- Stock adjustment requires actor, reason, correlation ID, and non-negative resulting available quantity.
- Reservation monitoring distinguishes active, expired, released, and committed reservations.
- Low-stock is calculated from available quantity and reorder level, not total quantity alone.

## Dashboard Definitions

- GMV: sum of successful/settled order gross totals within the selected UTC time window.
- Revenue: settled payment amount after the project’s explicitly configured fee/refund policy.
- AOV: GMV divided by successful order count; zero when there are no successful orders.
- Payment success rate: successful payment attempts divided by all validated payment attempts.
- Top products: settled order quantity and GMV grouped by product ID, with Product metadata resolved from Product-owned data.

## Kafka and Projections

Consume versioned `product_events`, `order_events`, `payment_events`, `inventory_events`, and `admin_events`. Every projection row has `event_id` uniqueness, `occurred_at`, `version`, and source topic. Unknown event versions are quarantined, not silently applied. Redis caches dashboard query results with short TTL and is never the source of truth.

## GraphQL Contract Direction

The public schema will add `adminCatalog`, `adminOrders`, `adminTransactions`, `adminLowStock`, `adminReservations`, `adminDashboard`, and `adminAuditEvents`, plus commands for moderation, stock adjustment, order cancellation, refund request, and reconciliation. Exact fields are added with each vertical slice only after the owning gRPC contract exists.

