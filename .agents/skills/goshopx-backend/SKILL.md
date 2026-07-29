---
name: goshopx-backend
description: Backend role skill for GoshopX service work. Use when changing Go microservices, Python recommender logic, GraphQL resolvers/schema, gRPC protobufs, Kafka producers or consumers, repositories, domain models, service tests, or cross-service business behavior.
---

# GoshopX Backend

## Overview

Implement business behavior in the service that owns it, then expose it through the correct contract.

## Workflow

1. Identify the owning service: Account, Product, Order, Payment, Recommender, or GraphQL.
2. Inspect existing `internal/`, `models/`, `proto/`, `client/`, and tests before editing.
3. Keep GraphQL public, gRPC internal, Kafka asynchronous, and persistence service-owned.
4. Add or adjust tests beside the changed package.
5. Regenerate code when GraphQL schema or protobuf contracts change.
6. Report exact verification commands.

## Contract Rules

- GraphQL changes require schema, resolver, generated code, and tests/client examples.
- Protobuf changes require compatibility review and generated Go/Python artifacts.
- Kafka payloads require topic, schema, idempotency, retry, and poison-message behavior.
- Payment and auth paths require security review.

## Validation

- Run focused `go test` packages for Go changes.
- Run recommender tests for Python changes.
- Run e2e only when cross-service behavior or full runtime is available.
