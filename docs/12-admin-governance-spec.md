# Admin Governance Specification

Status: Proposed
Date: 2026-07-30

## Role Matrix

| Capability | customer | seller | support_admin | operations_admin | platform_admin |
|---|---:|---:|---:|---:|---:|
| Own customer operations | yes | yes | yes | yes | yes |
| Own product operations | no | yes | no | yes | yes |
| Read account directory | no | no | yes | yes | yes |
| Suspend/reactivate customer or seller | no | no | yes | yes | yes |
| Assign customer/seller/support/operations role | no | no | no | no | yes |
| Grant platform admin | no | no | no | no, never | no, self-target prohibited |

## State and Rules

- Registration always creates `role=customer`, `status=active`.
- Login checks password, status, issuer, and expiry before issuing a token.
- Role checks use deny-by-default policy; absent role means customer for ordinary operations but never admin.
- An actor cannot suspend itself, change its own role, or grant platform admin.
- Platform admin is not granted by public registration or a normal admin mutation. Bootstrap is an operator-controlled database/config procedure documented outside GraphQL.
- Suspension prevents new login. Existing JWTs are stateless and remain valid until expiry in v1; immediate revocation is a future token-version/denylist story.
- Audit writes are best-effort only after the account state transaction commits; a failed Kafka publish must not roll back a successful account mutation, and must be observable.
- Audit data contains identifiers and outcome metadata only; credentials, cookies, JWTs, and payment payloads are prohibited.

## GraphQL Contract

Additions:

- `Account.role: AccountRole!`
- `Account.status: AccountStatus!`
- `adminAccounts(pagination: PaginationInput): [Account!]!`
- `suspendAccount(id: Int!): Boolean!`
- `reactivateAccount(id: Int!): Boolean!`
- `setAccountRole(id: Int!, role: AccountRole!): Account!`

All three mutations and `adminAccounts` require `support_admin`, `operations_admin`, or `platform_admin` according to the matrix. Existing `accounts` remains a compatibility field during v1 and is not treated as an admin boundary until explicitly migrated.

## gRPC Contract

Extend `AccountService` with version-compatible methods:

- `GetAccountAccess(id)`: returns account identity, role, and status.
- `ListAccounts(request)`: returns paginated non-password account views.
- `SetAccountStatus(request)`: validates target and emits audit fact.
- `SetAccountRole(request)`: validates role transition and emits audit fact.

The GraphQL gateway passes the authenticated actor identity and role context to these internal methods. The account service remains authoritative and rechecks policy; clients cannot self-assert admin privileges.

## Kafka Event Contract

Topic: `admin_events`

Envelope:

```json
{
  "version": 1,
  "event_id": "uuid",
  "event_type": "account_suspended",
  "occurred_at": "RFC3339",
  "actor_account_id": 1,
  "target_account_id": 2,
  "actor_role": "support_admin",
  "outcome": "success",
  "request_id": "opaque-correlation-id"
}
```

The event key is `target_account_id`; consumers must deduplicate by `event_id`. Failed authorization can be recorded locally and published with `outcome=denied` without exposing the denial reason beyond a stable code.

## PostgreSQL Ownership

Account owns:

- `accounts.role` with a constrained role value.
- `accounts.status` with a constrained status value.
- `admin_audit_events(event_id unique, actor_account_id, target_account_id, action, outcome, request_id, occurred_at)`.

Passwords are never returned by GraphQL or gRPC admin responses. Product moderation remains Product-owned and Elasticsearch remains Product-owned. Inventory corrections remain Inventory-owned. Redis remains Cart-owned; it is not used as an authorization source.

## Process Flows

### Admin account suspension

1. GraphQL validates JWT and extracts actor ID/role.
2. Policy gate checks the role and target restrictions.
3. Account gRPC transaction updates status and writes an audit row.
4. Account publishes `admin_events` asynchronously.
5. GraphQL returns success; publish failure is logged/metriced for retry tooling.

### Login after suspension

1. Account loads the user by email.
2. Password is verified.
3. Status is checked.
4. Suspended status returns an authentication failure and no JWT.

## Failure Modes and Mapping

| Failure | Public result | Recovery |
|---|---|---|
| Missing/invalid JWT | unauthorized | authenticate again |
| Insufficient role | forbidden | use an approved operator role |
| Target missing | not found | refresh account directory |
| Self-suspension or self-role change | invalid operation | select a distinct target |
| Kafka unavailable | mutation may succeed; operational alert | retry event publication/reconcile audit |
| Migration incompatible | service fails readiness | rollback app, restore migration snapshot |

## Test Matrix

- Unit: role matrix, self-target rules, default role/status, suspended login, JWT compatibility.
- Repository integration: migration, constraints, audit uniqueness, status/role updates.
- gRPC contract: request/response mapping, error mapping, actor revalidation.
- Kafka: versioned payload, missing fields, duplicate event ID, publish failure.
- GraphQL: allow/deny matrix, no password leakage, old token behavior, stable errors.
- E2E: register customer, bootstrap admin, list accounts, suspend/reactivate, login rejection, audit event evidence.

## Rollout and Rollback

Deploy the additive schema first, then account code that understands defaults, then GraphQL fields. Existing tokens continue to support ordinary flows. Roll back the GraphQL image first if public behavior regresses; retain additive columns and audit rows because they are backward-compatible. Do not drop columns during an application rollback. Reconcile failed Kafka publishes from the account audit table.

