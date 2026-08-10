# ADR-20260730 Kong Edge Gateway

Status: Accepted
Date: 2026-07-30

## Context

GoshopX exposes a GraphQL public contract at `/graphql`. The existing `graphql` service was directly published on host port 8080 and acted as both the public API boundary and protocol/orchestration layer. That is correct for GraphQL business behavior but does not provide a dedicated edge component for reverse proxying, edge rate limits, request IDs, controlled public routes, or upstream load balancing.

The payment provider also needs an HTTP callback endpoint. A provider cannot call the internal gRPC payment service or submit a GraphQL operation. The owning `payment` service has an HTTP `POST /webhook/payment` listener on port 8081 and verifies provider authenticity itself.

## Decision

Add Kong OSS in DB-less mode as the only published application ingress.

- Kong owns edge transport concerns: TLS termination in a deployed environment, reverse proxying, routing, request IDs, edge rate limiting, and upstream health/load-balancing.
- GraphQL remains the only public client API contract and orchestration boundary. Kong does not expose account, product, order, payment gRPC, Kafka, database, Redis, Elasticsearch, MinIO, or recommender ports.
- The client-compatible host URL remains `http://localhost:8080`; Kong maps it to its proxy port 8000. The GraphQL container is private to the Compose network.
- `POST /graphql`, `GET /playground`, and `GET /health` route to GraphQL. `POST /webhook/payment` routes only to Payment's HTTP webhook listener; Dodo must be configured with this public URL after TLS/DNS is deployed.
- Kong uses the `graphql-upstream` for round-robin routing and active `/health` checks. To actually exercise multiple GraphQL instances in Compose, run `docker compose up --scale graphql=N`; GraphQL has no `container_name`, so Compose can create replicas.
- JWT validation, role authorization, business validation, and Dodo webhook signature verification stay in the owning application services. Kong must not become an authorization source of truth or a payment-state owner.
- The Kong Admin API and Kong Manager OSS are enabled only on loopback in Compose (`127.0.0.1:8001` and `127.0.0.1:8002`) so local operators can inspect/configure Kong without publishing management surfaces on the public ingress. Configuration is versioned in `kong/kong.yaml`; production configuration changes still require code review and a rolling deployment.

## Consequences

Positive:

- No direct public bypass to GraphQL or internal services.
- A single entry point supports later TLS, WAF/CDN, observability, and horizontal GraphQL scaling without changing the client URL.
- The external payment callback has an explicit, narrow trust boundary.
- Local developers can inspect gateway entities in Kong Manager OSS without adding a second UI service or exposing management ports beyond the workstation.

Costs and limits:

- Kong adds a network hop and an operational component.
- The `local` rate-limit policy is per Kong instance. A multi-Kong production deployment must move rate-limit counters to Redis or another shared policy store.
- Docker Compose does not provide a production-grade multi-host load balancer. Production replicas need an orchestrator/service discovery layer and at least two Kong instances behind a cloud/edge load balancer.
- Kong can reject unhealthy GraphQL targets, but `depends_on` only orders startup; it does not make application dependencies ready.
- Kong Manager relies on the Admin API. If either surface is published beyond localhost, authentication, HTTPS, and operator network restrictions become mandatory.

## Alternatives Considered

1. Keep GraphQL directly published: rejected because it leaves no dedicated ingress controls or upstream traffic management.
2. Replace GraphQL with Kong: rejected because Kong is an L7 gateway, not a GraphQL schema, resolver, orchestration, or domain authorization layer.
3. Publish every microservice through Kong: rejected because it violates the GraphQL-only public client boundary and exposes internal contracts.
4. Use Kong's JWT plugin as the sole auth boundary: rejected because login/register are public and authorization must be rechecked by GraphQL and the owning services.

## Verification

```bash
docker run --rm -v "$PWD/kong/kong.yaml:/kong/kong.yaml:ro" kong:3.9 kong config parse /kong/kong.yaml
docker compose config --quiet
docker compose up --build -d
docker compose ps
curl -fsS http://localhost:8080/health
curl -i http://localhost:8080/playground
curl -i -X POST http://localhost:8080/graphql -H 'Content-Type: application/json' --data '{"query":"{__typename}"}'
```

For the payment callback, use a real signed Dodo test event only. Never place a webhook secret or JWT in a command, test fixture, diagram, or documentation.
