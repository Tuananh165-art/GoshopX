# Kong Edge Gateway Runbook

## Purpose and ownership

`kong/kong.yaml` is the declarative, DB-less configuration for the GoshopX edge gateway. Kong is a reverse proxy and load balancer; it is not a replacement for GraphQL.

| Concern | Owner |
| --- | --- |
| Public schema, resolver orchestration, JWT parsing | `graphql` |
| Account and role authorization | GraphQL plus owning service recheck |
| Internal synchronous calls | gRPC on private Compose network |
| Business facts | Kafka on private Compose network |
| Payment webhook authenticity and state transitions | `payment` |
| Edge routing, request correlation, basic rate limit, upstream health | Kong |

## Public routes

| Caller | Method/path | Upstream | Notes |
| --- | --- | --- | --- |
| Browser/mobile client | `POST /graphql` | `graphql:8080` | The only public client API. Kong rate-limits by source IP. |
| Developer browser | `GET /playground` | `graphql:8080` | Disable or protect in production if it is not needed. |
| Health probe | `GET /health` | `graphql:8080` | Also used by Kong active upstream checks. |
| Dodo Payments | `POST /webhook/payment` | `payment:8081` | Narrow callback route; Payment verifies the signed event. |

No proxy route may expose gRPC, Kafka, a database, Redis, Elasticsearch, MinIO, internal service health endpoints, or Kong Admin API. In local Compose, Kong's management ports are bound only to `127.0.0.1`.

## Local management UI

Kong OSS 3.9 includes Kong Manager OSS in the same container, so the local stack does not need a separate Konga/Konnect UI service.

| Surface | Local URL | Exposure |
| --- | --- | --- |
| Kong Admin API | `http://localhost:8001` | Loopback only |
| Kong Manager OSS | `http://localhost:8002` | Loopback only |

The browser UI uses the Admin API under the hood. Keep both bindings private to the developer machine unless you also add authentication, HTTPS, and IP restrictions.

## Local startup

```bash
docker run --rm -v "$PWD/kong/kong.yaml:/kong/kong.yaml:ro" kong:3.9 kong config parse /kong/kong.yaml
docker compose config --quiet
docker compose up --build -d
docker compose ps
curl -fsS http://localhost:8080/health
curl -fsS http://localhost:8001/
```

The externally stable local application endpoint is still `http://localhost:8080`. GraphQL is no longer directly host-published, so it cannot bypass Kong. Kong Manager is available at `http://localhost:8002` for local gateway inspection and configuration review.

## GraphQL scale test

GraphQL must be stateless for horizontal use. It already reads authentication from the request token/cookie and does not persist HTTP session state. To test Kong's upstream balancing locally:

```bash
docker compose up -d --scale graphql=2
curl -fsS http://localhost:8080/health
```

Keep `container_name` absent from `graphql`; Compose cannot scale a service that declares it. This Compose test demonstrates service-level balancing only, not multi-host or zone failure tolerance.

## Production rollout checklist

1. Build/publish an image or deploy Compose with `kong/kong.yaml` reviewed in the same change.
2. Terminate HTTPS before or at Kong and redirect HTTP to HTTPS. Set a real public DNS name.
3. Configure the payment provider callback to `https://<public-host>/webhook/payment` only after its signature verification is tested.
4. Keep the Kong Admin API and Kong Manager private or disabled outside trusted operator networks. Do not mount a mutable configuration database for this DB-less deployment.
5. Replace the `local` rate-limit policy with shared Redis policy before running more than one Kong instance.
6. Send Kong and application logs to centralized, redacted logging; propagate `X-Request-ID` and alert on 4xx/5xx spikes, unhealthy upstreams, and webhook signature failures.
7. Disable `GET /playground` in production unless it has a documented operational requirement.
8. Verify GraphQL authorization and Payment webhook signature behavior after the gateway is live. Kong routing success is never evidence that a business action is authorized.

## Rollback

To restore the previous direct local GraphQL exposure, remove the `kong` service and restore the GraphQL port mapping `8080:8080` in `docker-compose.yaml`, then run `docker compose up -d --build graphql`. Do not revert Payment's internal signature verification or alter business data as part of gateway rollback.

## Security notes

- Do not add JWTs, Dodo keys, HMAC secrets, or database passwords to `kong/kong.yaml`.
- Kong must preserve the webhook path and raw request body for Payment signature verification.
- Treat rate limits as DoS control, not identity/authorization. Account and GraphQL service checks remain authoritative.
- The current payment webhook handler is the security boundary for callback authenticity; test it with a real Dodo signed event before publishing DNS.
- If you publish Kong Manager or the Admin API beyond localhost, add authentication, HTTPS, and network restrictions in the same change.
