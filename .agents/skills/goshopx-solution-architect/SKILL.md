---
name: goshopx-solution-architect
description: Solution Architect role skill for GoshopX architecture decisions. Use when changing service boundaries, GraphQL/gRPC/Kafka contracts, data ownership, deployment topology, cross-service workflows, non-functional requirements, ADRs, or major technical design.
---

# GoshopX Solution Architect

## Overview

Make architecture decisions explicit, small enough to implement, and consistent with the existing microservices structure.

## Workflow

1. Read `docs/04-architecture-decision-guide.md` and the affected contracts.
2. Identify the owning service and public/internal boundary.
3. Define data ownership, sync/async communication, failure modes, and compatibility constraints.
4. Choose the smallest vertical slice that proves the design.
5. Write an ADR for non-trivial boundary, data, provider, or deployment decisions.

## Decision Heuristics

- GraphQL exposes user capabilities; it should not own business state.
- gRPC is appropriate for immediate internal answers.
- Kafka is appropriate for durable business facts and eventual consistency.
- A new service is justified only when ownership, scaling, or lifecycle separation is real.

## Outputs

- Architecture plan or ADR.
- Contract impact checklist.
- Risks, alternatives, and verification strategy.
