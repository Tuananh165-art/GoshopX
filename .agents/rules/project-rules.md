# GoshopX Project Rules

## Non-Negotiable Architecture Rules

- Keep GraphQL as the public API for clients.
- Keep gRPC as internal synchronous communication.
- Keep Kafka as asynchronous business-event communication.
- Do not bypass service ownership by reading another service's database directly.
- Preserve account identity and product ownership fields across mappings.
- Treat Compose credentials as local defaults, not production secrets.

## BMAD Rules

- Start with business outcome and acceptance criteria.
- Name domain concepts consistently across docs, schema, protobufs, events, code, and tests.
- Capture architecture decisions when boundaries, data ownership, or external dependencies change.
- Deliver in vertical slices with verification evidence.

## Agile/Scrum Rules

- A story is not ready until business rules, affected contracts, and test scope are known.
- A story is not done until code, tests, docs/process updates, and security checks are complete.
- PM owns priority; BA owns requirement clarity; Architect owns boundary integrity; Tester owns evidence; Security owns abuse-case review.

## Engineering Rules

- Use existing repository patterns before adding abstractions.
- Keep Go code gofmt-formatted and tests named `*_test.go`.
- Keep Python recommender work inside the existing `uv` project layout.
- Prefer focused unit tests before broad e2e tests.
- Do not overclaim runtime verification. State exactly which commands passed.
