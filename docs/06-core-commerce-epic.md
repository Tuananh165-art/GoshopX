# Core Commerce Expansion Epic

## Problem

GoshopX currently supports account, catalog, order, payment, and recommendation flows, but it does not yet have service-owned stock control, an active cart workflow, or in-app notifications. This creates three business gaps:

- shoppers can move directly from product discovery to order creation without a durable cart,
- the platform cannot prevent overselling with reservation-based stock checks,
- users do not receive first-class in-app feedback for checkout, payment, or stock-related events.

## Users

- Shopper: browses products, manages a cart, checks out, and expects clear feedback.
- Seller: needs reliable stock ownership and low-stock visibility.
- Operator: needs event-driven insight into reservations, payment outcomes, and customer-facing alerts.
- Developer: needs a clear GraphQL, gRPC, Kafka, PostgreSQL, Redis, and Elasticsearch contract map that preserves current microservice boundaries.

## Goals

- Add `inventory`, `cart`, and `notification` as service-owned Go microservices.
- Keep GraphQL as the public client boundary.
- Use gRPC for synchronous internal calls and Kafka for asynchronous business facts.
- Introduce Redis where it adds direct product value: active cart state with TTL.
- Prevent overselling with reservation-based stock control.
- Expose an in-app notification feed and unread state through GraphQL.

## Non-Goals

- No email, push, or SMS delivery in v1.
- No promotion, coupon, wishlist, or review workflow in this epic.
- No replacement of Elasticsearch product search ownership.
- No direct client access to Redis, PostgreSQL, Kafka, or internal gRPC services.
- No hard production deployment claims beyond locally verified commands.

## Business Outcome

This epic introduces a checkout-safe core commerce loop:

1. Seller creates a product in the existing Product service.
2. Seller seeds or updates stock in the new Inventory service.
3. Shopper adds items to the new Cart service, which creates or adjusts stock reservations.
4. Shopper checks out through GraphQL, which orchestrates cart snapshot, order creation, and payment session creation.
5. Payment webhook updates order state, commits or releases stock, and emits user-facing notification facts.

## Story Map

### Inventory Capability

- Maintain stock quantity and reorder level per product.
- Reserve stock for active carts and checkout attempts.
- Commit stock after successful payment.
- Release stock after cart clear, payment failure, or reservation expiry.
- List low-stock products for operator or seller review.

### Cart Capability

- Store authenticated shopper cart state in Redis.
- Keep cart TTL aligned with reservation TTL.
- Allow add, update quantity, remove, clear, and prepare checkout actions.
- Never trust client-submitted price or stock state.

### Notification Capability

- Store in-app notifications in PostgreSQL.
- Consume cart, inventory, order, and payment facts from Kafka.
- Provide read and unread management for authenticated users.

## Release Scope

### In Scope for v1

- `inventory`, `cart`, `notification` services and runtime wiring.
- GraphQL cart, inventory availability, and notification API additions.
- Kafka topics: `inventory_events`, `cart_events`, `order_events`, `payment_events`.
- PostgreSQL ownership for inventory and notification.
- Redis ownership for active cart state.
- Unit, repository, contract, and focused end-to-end verification.

### Out of Scope for v1

- Reservation backfill for legacy orders.
- Multi-warehouse stock.
- Partial shipment or return/refund inventory adjustments.
- Email/push providers.
- Admin UI.

## Assumptions

- Reservation TTL defaults to 15 minutes.
- GraphQL adds one inventory mutation to seed stock: `upsertProductStock`.
- Product price remains owned by Product and is recalculated in Order.
- Notification v1 is in-app only.
