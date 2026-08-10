# Admin Governance Epic

Status: Proposed for implementation
Date: 2026-07-30

## Problem

GoshopX has authenticated accounts and seller-owned products, but it does not yet have a governed operator capability. Any future moderation, account support, stock correction, or operational investigation would otherwise be implemented as scattered special cases. That creates privilege-escalation risk, weak auditability, and inconsistent GraphQL authorization.

## Decision Summary

Version 1 adds role-based access control to the existing `account` service. A separate admin microservice is not justified yet because identity, login, role ownership, account lifecycle, and JWT issuance already belong to `account`. Admin actions are exposed through GraphQL, validated by account-owned policy, executed through existing service gRPC contracts, and recorded as versioned Kafka facts.

## Users and Actors

- Shopper: manages their own account and commerce activity.
- Seller: manages products and stock that they own.
- Support admin: reads accounts/orders and performs explicitly allowed support actions.
- Operations admin: manages catalog and inventory exceptions.
- Security auditor: reads immutable admin audit facts through an approved operational projection.
- System services: consume admin facts without receiving browser credentials.

## Goals

- Establish explicit roles: `customer`, `seller`, `support_admin`, `operations_admin`, and `platform_admin`.
- Enforce authorization at the GraphQL boundary and again in account-owned role operations.
- Include the role in newly issued JWTs while preserving existing tokens until expiry.
- Provide safe admin account listing and role/status management with audit facts.
- Keep Product, Order, Inventory, Payment, Elasticsearch, Redis, and Kafka ownership boundaries intact.
- Make every acceptance criterion testable and every privileged mutation traceable.

## Non-goals

- No separate admin service or admin database in v1.
- No password reset, SSO, MFA, email, push notification, or provider integration in this epic.
- No direct admin reads from another service database or Elasticsearch index.
- No generic shared cache for authorization decisions.
- No silent privilege escalation from seller/customer to an admin role.

## Domain Terms

- Role: coarse-grained capability assigned to an account.
- Privileged operation: a mutation or query that requires an admin role.
- Audit fact: versioned Kafka event describing who attempted or completed a privileged action, without secrets.
- Account status: `active` or `suspended`; suspended accounts cannot obtain new tokens.
- Platform admin: highest role, allowed to manage admin roles under the self-escalation rules in the specification.

## Story Map

1. Authenticate and identify: issue role-bearing JWTs; reject suspended accounts.
2. Govern accounts: list accounts, inspect status, suspend/reactivate accounts, assign non-platform roles.
3. Operate commerce safely: allow support and operations actions only through existing service contracts.
4. Explain actions: publish `admin_events` and retain a local audit record for operational correlation.
5. Verify and release: test authorization, replay/idempotency, migration compatibility, and rollback.

## Business Outcome

Operators can resolve commerce incidents without database access, customers remain isolated from privileged operations, and security reviewers can connect a privileged request to an authenticated actor and target account.

## Release Scope

The release contains account-owned roles/status, role-aware JWT claims, internal account gRPC methods, GraphQL admin queries/mutations, a versioned `admin_events` topic, PostgreSQL audit records, unit/contract/integration tests, and runtime documentation. A future split into an admin service is triggered only when admin workflows require independent scaling, organization-level tenancy, long-retention audit search, or a separate operator deployment boundary.

