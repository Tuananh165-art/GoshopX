# Business Domain Rules

## Shared Language

- Account: a registered user identity with email, name, password hash, and JWT-based authentication.
- Product: a catalog item with name, description, price, owner account, and searchable index record.
- Order: a purchase intent containing product IDs, quantities, calculated prices, and total price.
- Payment: checkout or customer portal workflow delegated to the payment provider and stored as transaction state.
- Recommendation: ranked product suggestions derived from product catalog and interaction/order events.
- Public contract: GraphQL schema exposed at `/graphql`.
- Internal contract: gRPC protobufs and Kafka topic payloads.

## Business Invariants

- Authentication tokens must not leak in logs, docs, UI copy, or screenshots.
- Product price must be positive before it can be ordered or checked out.
- Orders must calculate total price from product service data, not from client-provided prices.
- Checkout must reference an account, customer identity, order ID, redirect URL, and product quantities.
- Product events and interaction events must remain versionable/idempotent because consumers can replay or receive duplicates.
- Recommender results must degrade gracefully when no trained artifacts or user history exist.
- GraphQL fields must preserve identity fields such as `accountId`; silently returning zero IDs is a business defect.

## Role-Specific Business Questions

- BA: What exact user decision or operational process changes?
- PM: Is the story valuable enough for the sprint compared with risk?
- Architect: Which contract owns the behavior?
- Backend: Which service owns the state and validation?
- Frontend: What must the user see, submit, recover from, or confirm?
- Tester: Which invariant could break without visible compile failure?
- DevOps: What runtime dependency can fail and how do we observe it?
- Cyber Security: What can be abused, leaked, forged, replayed, or escalated?
