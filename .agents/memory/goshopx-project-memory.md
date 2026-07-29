# GoshopX Project Memory

Use this as stable context for future agents working in this repository.

## Product

GoshopX is an e-commerce microservices platform. Core business flows are account registration/login, product catalog management, product search, order creation, payment checkout/customer portal sessions, and recommendations.

## Architecture

- Go services: `account`, `product`, `order`, `payment`, `graphql`.
- Python service: `recommender`.
- Public API: GraphQL at `/graphql`.
- Internal synchronous contracts: gRPC protobufs under each service.
- Async contracts: Kafka topics such as `product_events` and `interaction_events`.
- Data stores: PostgreSQL for account/order/payment/recommender, Elasticsearch for product catalog search.
- Runtime: Docker Compose in `docker-compose.yaml`.

## Stable Decisions

- Clients should not call internal services directly.
- Services should own their state.
- Product search belongs to Product through Elasticsearch.
- Recommendations consume product/interaction facts and serve through recommender gRPC.
- Payment provider credentials must stay in environment variables.
- Business logic correctness matters more than adding broad scaffolding.

## Validation Memory

- Use `go test -race -count=1` for Go unit packages.
- Use `go test ./tests/e2e` only when the full stack or test prerequisites are available.
- Use `docker compose config --quiet` after Compose changes.
- Use `cd recommender && uv sync --frozen` and recommender tests for Python changes.

## Known Caution

The root README may display mojibake in some emoji/heading text. Prefer UTF-8-aware editing and validate Vietnamese docs with an editor or UTF-8-aware commands.
