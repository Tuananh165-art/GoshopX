# ADR-20260730-admin-rbac-boundary

Status: Accepted for v1
Date: 2026-07-30

## Context

GoshopX needs operator capabilities for account support and commerce governance. Authentication, JWT issuance, and account persistence already belong to `account`; public traffic terminates at GraphQL; service-to-service calls use gRPC; cross-service facts use Kafka.

## Decision

Add admin roles and account status to the existing `account` service. Implement a deny-by-default RBAC policy, role-bearing JWTs, account-owned audit persistence, and an `admin_events` Kafka topic. GraphQL exposes the public admin contract and performs a fast policy gate, while account gRPC methods revalidate actor and target policy as the authoritative boundary.

No new admin Docker image, PostgreSQL database, Redis namespace, or Elasticsearch index is introduced in v1.

## Ownership Boundaries

- Account: identity, role, status, login, admin audit.
- GraphQL: public schema, authentication context, orchestration.
- Product: catalog and Elasticsearch index; admin must call Product contracts.
- Inventory: stock/reservation state; admin must call Inventory contracts.
- Order/Payment: order and payment facts; no admin database reads.
- Kafka: immutable business/audit facts, not authorization state.
- Redis: active cart state only, not role authority.

## Alternatives Rejected

- Separate admin service now: duplicates identity and introduces cross-service authorization drift before there is enough admin-specific domain state.
- Role checks only in GraphQL: internal callers could bypass the public boundary and account would not be authoritative.
- Shared Redis permission cache: stale permissions could permit abuse and violate ownership clarity.
- Direct database access from GraphQL: breaks service ownership and makes migrations unsafe.

## Consequences

Positive: smallest deployable change, one identity authority, clear audit trail, backward-compatible customer flows, and reuse of current stack. Negative: account initially carries governance responsibilities and audit search is limited to account-owned operational data. A future admin service becomes justified for independent operator tenancy, long-retention audit search, workflow queues, or separate scaling/deployment.

## Compatibility and Security

Old tokens without `role` remain valid for non-admin operations until expiry and are denied admin operations. Role/status changes are audited. Secrets and tokens never enter events or logs. Self-escalation and self-suspension are denied.

## Verification

Run focused account/auth/GraphQL tests, generated-code checks, `docker compose config --quiet`, and the existing e2e suite. Live provider and long-running Kafka failure recovery remain environment-dependent and must be reported separately.

