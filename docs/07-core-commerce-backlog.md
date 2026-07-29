# Core Commerce Backlog

## Epic

As a platform team,
I want service-owned inventory, cart, and in-app notification workflows,
so that GoshopX can support a safer and more complete commerce loop.

## Priority Order

1. Inventory foundation
2. Cart workflow on top of inventory reservations
3. Notification workflow on top of cart, order, inventory, and payment facts
4. GraphQL integration and end-to-end verification

## Sprint-Ready Stories

### Story 1: Inventory Ownership

As a seller or operator,
I want product stock and reservations owned by a dedicated Inventory service,
so that shoppers cannot reserve more units than are available.

Business rules:

- Stock is owned in PostgreSQL by Inventory.
- Reservations reduce available quantity immediately.
- Reservation expiry restores availability.
- Payment success commits reserved quantity.
- Payment failure or cart clear releases reserved quantity.

Acceptance criteria:

- Given a product with available stock
  When a reservation is created
  Then available quantity decreases by the reserved amount.
- Given an expired reservation
  When availability is queried or cleanup runs
  Then expired quantity no longer counts as reserved.
- Given a commit request for an active reservation
  When payment succeeds
  Then reservation status becomes committed and stock quantity is reduced.

Technical notes:

- Affected services: `inventory`, `graphql`, `cart`
- Contracts: GraphQL availability field and stock mutation; inventory gRPC
- Data/events: `stocks`, `stock_reservations`, `inventory_event_outbox`, `inventory_events`
- Risks: duplicate commit/release, expired reservations, unauthorized stock edits
- Tests: service, repository, gRPC, GraphQL, e2e

### Story 2: Redis Cart

As a shopper,
I want a persistent authenticated cart with reservation-backed quantities,
so that I can prepare checkout without overselling surprises.

Business rules:

- Cart state is keyed by authenticated account ID.
- Each cart line item points to an inventory reservation.
- Updating quantity updates the same reservation when possible.
- Clearing cart releases reservations.
- Checkout preparation returns only active reservation-backed items.

Acceptance criteria:

- Given a logged-in shopper
  When the shopper adds a product with valid quantity
  Then the cart is stored and stock is reserved.
- Given a cart line item
  When the shopper updates quantity beyond available stock
  Then the mutation fails with a deterministic stock error.
- Given a prepared checkout snapshot
  When GraphQL creates an order and payment session
  Then the snapshot uses reservation-backed product quantities only.

Technical notes:

- Affected services: `cart`, `inventory`, `graphql`
- Contracts: GraphQL cart queries and mutations; cart gRPC
- Data/events: Redis cart state; `cart_events`
- Risks: stale cart state, reservation drift, Redis TTL mismatch
- Tests: service, repository, gRPC, GraphQL, e2e

### Story 3: In-App Notifications

As a shopper,
I want to see order, payment, and reservation-related notifications,
so that I understand what happened without leaving the app.

Business rules:

- Notifications are stored per account in PostgreSQL.
- Notification feed is newest-first.
- Read and unread state are explicit.
- Duplicate business events must not create duplicate notifications.

Acceptance criteria:

- Given a payment success event
  When Notification consumes it
  Then a success notification is stored for the shopper.
- Given a payment failure event
  When Notification consumes it
  Then a failure notification is stored for the shopper.
- Given unread notifications
  When the shopper marks one or all as read
  Then unread counts update correctly.

Technical notes:

- Affected services: `notification`, `order`, `payment`, `inventory`, `cart`, `graphql`
- Contracts: GraphQL notification queries and mutations; notification gRPC
- Data/events: `notifications`; consumes `cart_events`, `inventory_events`, `order_events`, `payment_events`
- Risks: malformed events, duplicate events, unread counter regressions
- Tests: consumer, service, repository, GraphQL, e2e

## Dependencies

- Inventory must land before Cart checkout correctness is claimable.
- Cart must land before `checkoutCart` GraphQL orchestration.
- Order and Payment event publication must land before notification coverage is complete.
- GraphQL schema generation depends on final service contract naming.

## Definition of Ready

- Business vocabulary matches `docs/03-business-domain-rules.md`.
- Topic names and event ownership are explicit.
- New GraphQL fields and new gRPC methods are named.
- Data stores per service are identified.
- Test levels per story are identified.

## Definition of Done

- Code follows current Go microservice structure.
- GraphQL remains the public client boundary.
- gRPC and Kafka changes are implemented and exercised by focused tests.
- Docs and ADR are committed with the behavior.
- Exact commands run are captured in the final report.

## Scrum Delivery Plan

- Sprint 1: docs, ADR, inventory service, inventory tests
- Sprint 2: cart service, Redis wiring, cart tests
- Sprint 3: notification service, order/payment event publication, notification tests
- Sprint 4: GraphQL integration, e2e regression, runtime validation
