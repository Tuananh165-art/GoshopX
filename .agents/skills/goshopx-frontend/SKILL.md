---
name: goshopx-frontend
description: Frontend role skill for GoshopX client experience work. Use when designing, implementing, reviewing, or testing UI flows, GraphQL client behavior, account/catalog/order/checkout/recommendation screens, responsive states, visible copy, accessibility, or user-facing acceptance criteria.
---

# GoshopX Frontend

## Overview

Build user-facing behavior through the GraphQL public contract while hiding internal service details. Keep the experience aligned with e-commerce business language and acceptance criteria.

## Workflow

1. Read `graphql/schema.graphql`, `docs/03-business-domain-rules.md`, and `.agents/rules/project-rules.md`.
2. Identify the user role, task, data shown, data submitted, empty state, error state, and auth state.
3. Use GraphQL queries/mutations only; do not call internal gRPC services, Kafka, or databases from the client.
4. Design states for loading, success, validation failure, auth failure, backend failure, and retry.
5. Verify with UI tests, screenshots, Playwright, or exact GraphQL/manual evidence depending on the repo surface available.

## Checklist

- Do not display JWTs, service URLs, provider tokens, Kafka topics, or raw internal IDs unless required for admin/debug views.
- Show product names, descriptions, prices, quantities, order totals, and recommendation context in readable language.
- Preserve accessibility: labels, keyboard flow, contrast, and responsive text wrapping.
- Treat payment redirect/portal as a sensitive transition with clear confirmation and recovery states.

## Outputs

- UI plan or implementation.
- GraphQL operation list.
- Acceptance-state coverage.
- Verification evidence and remaining UI risk.
