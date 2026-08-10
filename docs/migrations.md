# Database migrations

Each PostgreSQL-owning service applies its own versioned migrations at repository startup. Migration state is stored in `schema_migrations` inside that service's database; services never migrate another service's tables.

## Current versions

| Service | Database | Versions |
|---|---|---|
| account | account database | `001_initial`, `002_role_id` |
| order | order database | `001_initial` |
| payment | payment database | `001_initial` |
| inventory | inventory database | `001_initial` |
| notification | notification database | `001_initial` |
| admin | admin database | `001_initial` |
| recommender | recommender database | `001_initial` |

`product` is backed by Elasticsearch and is not part of PostgreSQL migrations.

## Account role migration

`002_role_id` is non-destructive with respect to account rows:

1. The `role_id` integer column is created with default `0`.
2. Existing string roles are backfilled: `customer` becomes `0`; existing non-customer roles become `1`.
3. The old `role` column is removed only after backfill succeeds.
4. The application exposes compatibility role names (`customer`/`admin`) for JWT and GraphQL authorization, but persists only `role_id`.

The migration is idempotent and is recorded only after the schema callback succeeds. Do not delete database volumes to apply it.

## Verification

After starting the services, inspect each service-owned database in TablePlus and confirm:

```sql
SELECT service, version, applied_at
FROM schema_migrations
ORDER BY service, version;
```

For account data:

```sql
SELECT id, email, role_id
FROM accounts
ORDER BY id;
```

Expected role IDs are `0` for customers and `1` for admins. Do not query or print passwords, password-reset hashes, tokens, or other secrets.
