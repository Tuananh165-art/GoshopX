# ADR-20260729-core-commerce-services

Status: Accepted
Date: 2026-07-29

## Context

GoshopX already separates public GraphQL, internal gRPC, asynchronous Kafka, Product-owned Elasticsearch search, and PostgreSQL-backed account, order, and payment state. The missing commerce capabilities are stock ownership, active cart state, and in-app notifications.

The new capability must:

- prevent overselling,
- preserve service-owned persistence,
- fit the current GraphQL -> gRPC -> Kafka architecture,
- introduce Redis only where it has direct product value.

## Decision

Create three new Go microservices:

- `inventory`: owns stock, reservations, and inventory events in PostgreSQL.
- `cart`: owns active cart state in Redis and synchronously coordinates with Inventory.
- `notification`: owns in-app notification persistence in PostgreSQL and consumes business events from Kafka.

Keep the following boundaries:

- GraphQL remains the only public client boundary.
- Product keeps catalog and Elasticsearch ownership.
- Order keeps pricing and order persistence ownership.
- Payment keeps provider integration and webhook validation ownership.
- Inventory does not read Product or Order databases directly.
- Notification does not mutate upstream business state.

## Consequences

### Positive

- Stock correctness has a clear owner.
- Redis is used narrowly for active-session value, not broad incidental caching.
- Cart and notification capabilities can evolve without contaminating Product or Order ownership.
- Event-driven notifications stay decoupled from payment and order transaction handling.

### Negative

- The runtime becomes more complex with three more services and two more PostgreSQL databases plus Redis.
- Checkout orchestration spans more synchronous and asynchronous boundaries.
- Reservation expiry and idempotent event handling add implementation complexity.

## Alternatives Considered

### Add cart and inventory into existing Order service

Rejected because Order already owns purchase intent persistence and price calculation. Mixing active cart session state and stock reservations into Order would weaken service ownership and complicate scaling.

### Add notification behavior into GraphQL

Rejected because GraphQL is a public contract boundary, not a durable business-state owner. Notification persistence and event consumption belong in a service.

### Use PostgreSQL instead of Redis for cart

Rejected for v1 because active cart state is short-lived, TTL-oriented, and benefits from Redis semantics. PostgreSQL remains the durable choice for inventory and notifications.

## Verification

- Add focused unit, repository, and service tests for the new services.
- Regenerate GraphQL after schema changes.
- Validate Compose with `docker compose config --quiet`.
- Run focused `go test` packages first, then broader test suites.
