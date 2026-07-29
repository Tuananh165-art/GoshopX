# Architecture Decision Guide

## Current Architecture

GoshopX is a Go microservices system with a Python recommender service.

- `graphql/`: public GraphQL gateway.
- `account/`: account registration, login, JWT, account data.
- `product/`: product CRUD, Elasticsearch index, product events.
- `order/`: order creation, product lookup, order persistence, interaction/order events.
- `payment/`: checkout/customer portal integration, transaction persistence, product event consumption.
- `recommender/`: Kafka consumer, recommendation training, gRPC recommendation server.
- `pkg/`: shared middleware, auth, Kafka helpers, and context keys.
- `tests/e2e/`: cross-service behavior tests.

## Boundary Rules

- Browser and frontend traffic must call GraphQL, not internal gRPC services.
- Services own their own persistence; avoid cross-service database reads.
- gRPC is for synchronous internal requests.
- Kafka is for asynchronous facts and eventually consistent projections.
- GraphQL schema changes require resolver, generated model, and client/query alignment.
- Protobuf changes require generated Go/Python code updates and compatibility review.

## ADR Template

```md
# ADR-YYYYMMDD-short-title

Status: Proposed | Accepted | Superseded
Date: YYYY-MM-DD

## Context

## Decision

## Consequences

## Alternatives Considered

## Verification
```

## Change Impact Checklist

- Public API: GraphQL schema, resolver behavior, auth requirement.
- Internal API: protobuf method/message, service URL, timeout/retry behavior.
- Event API: Kafka topic, payload, idempotency key, consumer compatibility.
- Data: table/index/model owner, migration/backfill needs.
- Runtime: Compose env vars, image, ports, volumes, health checks.
- Security: auth, authorization, PII, secrets, provider signatures.
- Observability: logs, metrics, traces, alertable failures.
