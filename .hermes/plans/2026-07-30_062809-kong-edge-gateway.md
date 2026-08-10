# Kong Edge Gateway Implementation Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** Introduce Kong as the only external ingress while preserving GraphQL as GoshopX's public client contract and safely routing the payment-provider callback to its owning service.

**Architecture:** Kong OSS runs DB-less from versioned declarative configuration. It reverse-proxies client GraphQL traffic to a health-checked, round-robin GraphQL upstream and routes the narrow Dodo callback endpoint to Payment. GraphQL keeps schema/orchestration/auth responsibilities; gRPC, Kafka, stores, and internal services remain private.

**Tech Stack:** Docker Compose v2, Kong OSS 3.9 DB-less, declarative YAML, Go/Gin GraphQL, Go payment webhook, gRPC, Kafka.

---

## Business outcome and acceptance criteria

- Clients keep calling `http://localhost:8080/graphql`; only the internal hop changes to Kong -> GraphQL.
- No internal service port is newly exposed to the host.
- `GET /health`, `GET /playground`, and `POST /graphql` work through Kong.
- `POST /webhook/payment` reaches Payment without exposing gRPC and leaves payment HMAC/signature verification authoritative.
- Kong provides a correlation ID and a bounded GraphQL IP rate limit.
- GraphQL can be started with more than one replica because it has no fixed `container_name` and Kong targets its Compose service name.
- Compose and Kong declarative configuration parse successfully.

## Task 1: Record the boundary decision

**Objective:** Make the new deployment and trust boundary auditable before changing runtime wiring.

**Files:**
- Create: `docs/adr/ADR-20260730-kong-edge-gateway.md`
- Create: `docs/18-kong-api-gateway.md`

**Steps:**
1. State that Kong is ingress/reverse proxy/load balancer, not GraphQL replacement.
2. Specify the allowed routes and explicitly prohibit public gRPC/Kafka/database exposure.
3. Assign GraphQL, payment webhook, identity, and business authorization ownership.
4. Document production TLS, shared rate-limit storage, observability, rollback, and the Payment/Dodo callback cutover.
5. Security-review the route list: no Admin API and no secret in source configuration.

**Verification:** Review the ADR against `docs/04-architecture-decision-guide.md` and `docs/03-business-domain-rules.md`.

## Task 2: Create a DB-less declarative Kong contract

**Objective:** Encode stable edge routes and upstream behavior in version-controlled configuration.

**Files:**
- Create: `kong/kong.yaml`

**Steps:**
1. Add a `graphql-upstream` target at `graphql:8080`, `round-robin` algorithm, and active `GET /health` checks.
2. Create GraphQL routes: POST `/graphql`; GET `/playground` and `/health`; do not strip their paths.
3. Apply an IP-scoped local rate limit and `X-Request-ID` correlation only to GraphQL routes. Do not mistake either control for application authorization.
4. Add `payment-webhook-upstream` targeting `payment:8081` and an exact POST `/webhook/payment` route. Preserve its path and do not attach browser/user JWT validation.
5. Parse the file using the exact pinned image:

```bash
docker run --rm -v "$PWD/kong/kong.yaml:/kong/kong.yaml:ro" kong:3.9 kong config parse /kong/kong.yaml
```

Expected: exit status 0 and a normalized declarative configuration; no schema error.

## Task 3: Put Kong on the public Compose boundary

**Objective:** Route the current public port through Kong and allow GraphQL replicas.

**Files:**
- Modify: `docker-compose.yaml:342-380` (`graphql` service)
- Modify: `docker-compose.yaml` (new `kong` service before `volumes`)

**Steps:**
1. Remove `graphql.container_name`, because a fixed name prevents `docker compose --scale graphql=N`.
2. Remove the host mapping `8080:8080` from GraphQL so the public API cannot bypass Kong.
3. Add `kong:3.9`, `KONG_DATABASE=off`, read-only mount of `kong/kong.yaml`, disabled Admin API, and host mapping `8080:8000`.
4. Depend on GraphQL and Payment only for startup ordering; do not claim this proves readiness.
5. Run:

```bash
docker compose config --quiet
docker compose up --build -d
docker compose ps
```

Expected: Compose syntax is valid; Kong is running; GraphQL has no published host port; port 8080 belongs to Kong.

## Task 4: Prove public behavior through the new ingress

**Objective:** Verify routing without changing GraphQL/gRPC business contracts.

**Files:**
- Test only (no new application test required because no resolver/protobuf/domain behavior changes).

**Steps:**
1. Run `curl -fsS http://localhost:8080/health`; expect GraphQL's JSON health response through Kong.
2. Run `curl -i http://localhost:8080/playground`; expect an HTTP 200 browser UI response.
3. Run a public GraphQL introspection-free probe:

```bash
curl -i -X POST http://localhost:8080/graphql \
  -H 'Content-Type: application/json' \
  --data '{"query":"{__typename}"}'
```

Expected: GraphQL HTTP response with `X-Request-ID`; actual auth behavior remains determined by GraphQL middleware.
4. Confirm an unknown endpoint returns Kong 404 and no service port is bound directly on the host.
5. Do not post a fabricated payment webhook. Configure a Dodo test event only with the real secret in the provider dashboard and confirm Payment logs a signature-validated event without leaking the secret.

## Task 5: Verify scale and regression gates

**Objective:** Demonstrate local upstream-resilience wiring and protect existing behavior.

**Files:**
- Test only.

**Steps:**
1. Run `docker compose up -d --scale graphql=2` and re-run `GET /health`; Kong should continue serving the GraphQL upstream.
2. Run focused Go compilation/tests for gateway-related code:

```bash
go test -race -count=1 ./graphql/... ./payment/... ./pkg/...
```

3. Run the repository Compose gate again:

```bash
docker compose config --quiet
```

4. Record actual pass/fail output and environment limitations. Run broader `go test -race -count=1 $(go list ./... | grep -v '/tests/e2e$')` only when local dependency/cache conditions permit.

## Risks, tradeoffs, and follow-ups

- A single local Kong and Docker Compose do not provide production HA; deploy at least two Kong instances behind a cloud load balancer and use orchestrator health/readiness primitives.
- `rate-limiting.policy=local` is correct for one Kong process but must become Redis/shared policy when Kong scales.
- HTTPS, allowed origins, trusted proxy IPs, logs/metrics/tracing, and an externally routable Dodo callback hostname are deployment-specific and need production configuration, not hard-coded local values.
- GraphQL's `token` cookie currently uses domain `localhost`; before moving to a real domain, make cookie domain and Secure/SameSite policy environment-configurable and test login through the production hostname.
- The payment signature handler should be independently security-tested with an authentic provider payload before the callback is published.

## Files expected to change

- `kong/kong.yaml`
- `docker-compose.yaml`
- `docs/adr/ADR-20260730-kong-edge-gateway.md`
- `docs/18-kong-api-gateway.md`
- `design.png` (and optional editable source `design.svg`)

## Rollback

Remove the Kong service and restore GraphQL's direct `8080:8080` mapping, then rebuild/restart only GraphQL. This is routing-only: it does not require schema, gRPC, Kafka, or database rollback.
