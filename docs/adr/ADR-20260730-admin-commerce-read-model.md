# ADR-20260730-admin-commerce-read-model

Status: Proposed
Date: 2026-07-30

## Context

Account RBAC is sufficient for identity governance but not for cross-domain ecommerce operations. Dashboards and audit search need data from Product, Order, Payment, and Inventory without violating their PostgreSQL ownership.

## Decision

Introduce an Admin Commerce boundary for orchestration and reporting projections. Product, Order, Payment, and Inventory remain command owners. Kafka carries immutable versioned facts. Admin reporting stores idempotent PostgreSQL aggregate/read-model tables and uses Redis only as a cache. GraphQL remains the only public client API.

## Alternatives Rejected

- GraphQL joins directly into service databases: violates ownership and creates fragile coupling.
- Calculate monthly revenue from live transactional tables on every request: slow, inconsistent during replay, and hard to reconcile.
- Elasticsearch as the universal analytics store: Product owns Elasticsearch; payment/order financial facts need relational consistency.
- Redis as the dashboard source: cache loss would lose reporting state.

## Consequences

The system gains reliable cross-domain reporting and audit search, but must handle eventual consistency, event replay, schema evolution, consumer lag, and reconciliation. Each command still requires a synchronous owning-service contract.

## Verification

Test event idempotency, aggregate correctness, cache invalidation, authorization, provider reconciliation, and replay from a clean projection database.

