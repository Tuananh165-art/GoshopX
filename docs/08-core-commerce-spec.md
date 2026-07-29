# Core Commerce Detailed Specification

## Domain Terms

- Stock: total owned quantity for a product in Inventory.
- Reserved quantity: portion of stock temporarily locked by active cart or checkout flow.
- Available quantity: `stock.quantity - active reserved quantity`.
- Reservation: a time-bound hold for one account and one product.
- Cart snapshot: the active Redis-backed set of reserved items returned by Cart.
- Notification: an in-app message stored for an account with read state.

## Business Rules

### Inventory

- Inventory owns stock quantities and reservation lifecycle.
- Product must already exist before stock is upserted.
- Stock quantity cannot be negative.
- Reorder level cannot be negative.
- Active reservation status is one of `active`, `released`, `committed`, `expired`.
- Inventory must treat duplicate commit and duplicate release as idempotent.

### Cart

- Cart requires authentication.
- Cart item quantity must be greater than zero.
- Cart never accepts client price as source of truth.
- Cart reservation TTL equals 15 minutes from the latest item update.
- Removing the last item clears the cart key.

### Notification

- Notification feed is scoped to authenticated account.
- Notifications are sorted by `created_at DESC`.
- `markNotificationRead` is idempotent.
- `markAllNotificationsRead` returns the number of notifications transitioned to read.

## Process Flows

### Add to Cart

1. GraphQL authenticates shopper and calls Cart `UpsertCartItem`.
2. Cart calls Inventory `ReserveStock` for the requested quantity.
3. Inventory atomically creates or updates a reservation.
4. Cart persists the item and reservation metadata in Redis.
5. Cart publishes `cart.item_upserted`.

### Checkout

1. GraphQL authenticates shopper and calls Cart `PrepareCheckout`.
2. Cart validates reservation expiry and returns active reserved items.
3. GraphQL creates the order through Order using product IDs and quantities.
4. GraphQL creates a checkout session through Payment.
5. Reservation remains active until payment webhook outcome.

### Payment Success

1. Payment validates webhook authenticity.
2. Payment stores transaction state and publishes `payment.succeeded`.
3. Payment updates Order payment status.
4. Notification consumes `payment.succeeded` and stores a success notification.
5. Cart or orchestration path commits reservations in Inventory using the reservation IDs from the cart snapshot.

### Payment Failure

1. Payment validates webhook authenticity.
2. Payment stores transaction state and publishes `payment.failed`.
3. Payment updates Order payment status.
4. Notification stores a failure notification.
5. Inventory reservations tied to the failed checkout are released.

## Contract Matrix

| Boundary | Owner | Shape |
| --- | --- | --- |
| GraphQL `Product.availability` | GraphQL -> Inventory | availability projection |
| GraphQL `myCart` | GraphQL -> Cart + Product + Inventory | authenticated cart projection |
| GraphQL `notifications` | GraphQL -> Notification | in-app feed |
| GraphQL `checkoutCart` | GraphQL -> Cart -> Order -> Payment | checkout orchestration |
| Inventory gRPC | Inventory | stock and reservation commands |
| Cart gRPC | Cart | cart lifecycle and checkout preparation |
| Notification gRPC | Notification | feed and read-state commands |
| Kafka `inventory_events` | Inventory | reservation and stock facts |
| Kafka `cart_events` | Cart | cart item lifecycle facts |
| Kafka `order_events` | Order | order creation and payment status facts |
| Kafka `payment_events` | Payment | provider transaction outcome facts |

## Event Rules

- Every event includes `version`, `event_id`, `event_type`, `occurred_at`, and `account_id` when user-scoped.
- Notification consumers must safely ignore malformed or unsupported events.
- Replayed events must not produce duplicate notification rows for the same business fact.

## Failure Modes

- If Redis is unavailable, cart mutations fail fast and do not silently skip reservations.
- If Inventory cannot reserve stock, cart mutations fail with stock-related errors.
- If Payment webhook validation fails, order and inventory state must not be changed by the invalid callback.
- If Notification cannot persist a notification, upstream business state still completes; failure is logged for retry or operator review.

## Data Design

### Inventory Tables

- `stocks`
  - `product_id` PK
  - `quantity`
  - `reorder_level`
  - `updated_at`
- `stock_reservations`
  - `reservation_id` unique
  - `account_id`
  - `product_id`
  - `quantity`
  - `status`
  - `expires_at`
  - `created_at`
  - `updated_at`
- `inventory_event_outbox`
  - `id`
  - `event_key`
  - `event_type`
  - `payload`
  - `created_at`

### Cart Redis Shape

- key: `cart:{accountId}`
- value: JSON cart document
- TTL: 15 minutes from latest mutation

### Notification Table

- `notifications`
  - `id`
  - `account_id`
  - `event_id`
  - `event_type`
  - `title`
  - `message`
  - `metadata_json`
  - `is_read`
  - `created_at`
  - `read_at`

## Given / When / Then Acceptance Criteria

- Given a product with quantity `10`
  When shopper A reserves `3`
  Then availability becomes `7`.
- Given an active reservation
  When the cart quantity is changed from `3` to `5`
  Then the same reservation is adjusted if stock is sufficient.
- Given a reservation that expires
  When availability is queried afterward
  Then reserved quantity no longer includes that reservation.
- Given unread notifications
  When the shopper marks all as read
  Then unread count becomes `0`.

## Test Strategy

- Unit: validation, idempotency, stock math, read/unread behavior, cart TTL logic
- Repository: PostgreSQL persistence and Redis serialization
- Contract: gRPC method behavior and GraphQL resolver mapping
- Kafka: event publication and consumer deduplication
- End-to-end: stock seed -> cart -> checkout -> payment outcome -> notification feed

## Rollout and Rollback

### Rollout

- Add services and datastores to Compose.
- Start Inventory before Cart.
- Start Notification after order and payment event publication is available.
- Regenerate GraphQL after schema changes.

### Rollback

- Stop routing GraphQL to new cart and notification fields if contract generation fails.
- Stop new services in Compose while leaving existing account/product/order/payment flows intact.
- Inventory rollback may require clearing orphan reservations before disabling cart integration.
