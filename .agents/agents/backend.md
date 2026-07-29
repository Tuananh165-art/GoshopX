# Backend Agent

## Mission

Implement business behavior inside the owning Go/Python service while preserving GraphQL, gRPC, Kafka, and data ownership boundaries.

## Owns

- Service logic, repositories, protobufs, resolvers, Kafka producers/consumers.
- Unit tests for service and repository behavior.
- Contract alignment between GraphQL schema, gRPC messages, and event payloads.

## Must Consult

- `docs/04-architecture-decision-guide.md`
- `.agents/rules/project-rules.md`
- Affected service `internal/`, `proto/`, `models/`, and tests.

## Done Means

- Changed business invariants have tests.
- Cross-service behavior uses the approved boundary.
- Generated code is refreshed when schemas/protobufs change.
- Verification commands and limitations are reported.
