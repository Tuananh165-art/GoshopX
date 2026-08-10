# Admin Governance Backlog

Status: Sprint-ready proposal
Date: 2026-07-30

## Prioritized Backlog

### ADM-01: Role and status model (P0)

As the platform, I want every account to have an explicit role and status so that authorization has one owner.

Acceptance criteria:

- Given an existing account, when the migration runs, then it receives `customer` and `active` defaults.
- Given a suspended account, when it logs in, then no token is issued.
- Given a new account, when registration completes, then its role is `customer`.

Technical notes: account PostgreSQL, backward-compatible migration, unit and repository tests.

### ADM-02: Role-bearing JWT (P0)

As the GraphQL gateway, I want validated role claims so that privileged fields can reject unauthorized actors deterministically.

Acceptance criteria:

- Given a valid login, when a token is issued, then it contains `user_id`, `role`, issuer, and expiry.
- Given an old token without a role, when it calls a normal customer operation, then it remains valid until expiry.
- Given an old token without a role, when it calls an admin operation, then it is rejected.

Technical notes: do not log tokens; use a typed context key for role.

### ADM-03: Account administration contract (P0)

As an authorized admin, I want account listing and lifecycle controls through GraphQL so that support does not access databases directly.

Acceptance criteria:

- Given a non-admin, when it queries admin accounts, then GraphQL returns unauthorized.
- Given a support admin, when it suspends an account, then the target becomes suspended and an audit fact is emitted.
- Given an admin attempts to modify itself or grant platform admin, then policy rejects the operation unless the actor is a platform admin and the target is distinct.

Technical notes: account gRPC owns validation; GraphQL maps status codes to stable errors.

### ADM-04: Commerce operations policy (P1)

As an operations admin, I want approved product and inventory controls without direct data access.

Acceptance criteria:

- Given an operations admin, when it invokes an approved stock correction, then Inventory validates and persists it.
- Given a support admin, when it invokes an operations-only mutation, then it is rejected.
- Given an admin request for another service, when the downstream contract has no admin operation, then no database shortcut is introduced.

Technical notes: extend existing gRPC contracts only where the business operation is defined.

### ADM-05: Audit facts and evidence (P0)

As a security auditor, I want privileged attempts and outcomes to be traceable without secrets.

Acceptance criteria:

- Given a privileged action, when it succeeds or fails authorization, then an audit record contains actor, action, target, outcome, request ID, and timestamp.
- Given a duplicate event, when a consumer replays it, then downstream processing is idempotent.
- Given an audit payload, then it contains no JWT, password, payment secret, or full request body.

Technical notes: account-owned `admin_audit_events` plus Kafka `admin_events`.

### ADM-06: Release and recovery (P0)

As the delivery team, I want migration, rollback, and runtime evidence so that the feature is releasable.

Acceptance criteria: Compose config validates; focused tests pass; GraphQL schema regenerates; old customer flows remain green; docs contain exact commands and known limits.

## Dependencies and Sprint Plan

- Sprint 1: ADM-01, ADM-02, ADM-03, schema/protobuf generation.
- Sprint 2: ADM-04, ADM-05, GraphQL integration, contract tests.
- Sprint 3: ADM-06, e2e, security review, rollout rehearsal.

## Definition of Ready

Business owner, role matrix, state transitions, GraphQL names, gRPC methods, Kafka envelope, data owner, failure mapping, and test level are defined. No story may start with an unresolved privilege boundary.

## Definition of Done

Code is formatted, generated artifacts are refreshed, business invariants have tests, authorization has allow/deny coverage, migration is backward-compatible, Compose config validates, README/docs are updated, and the final report separates passed checks from unverified live dependencies.

