# ADR: DummyJSON seed source for the Product catalog

- Status: Accepted
- Date: 2026-08-03

## Context

The Product service needs realistic catalog data for Product Detail, search, category filtering, pagination, cart flows, and recommender development without requiring manually authored seed data.

## Decision

Use `https://dummyjson.com` as an optional development/demo source. The Product service owns the local catalog after import and writes normalized records to Elasticsearch. DummyJSON product IDs are preserved so carts and repeatable seed runs remain deterministic.

The mapping keeps the complete product detail payload: title/description, category, price, discount, rating, stock, tags, brand, SKU, weight, dimensions, warranty, shipping, availability, reviews, return policy, minimum order quantity, metadata, thumbnail, and image URLs. Imported records are marked `published` and `approved` for demo use.

GraphQL remains the public boundary. gRPC carries the expanded Product contract internally. Search stays in Elasticsearch and searches name, description, brand, SKU, category, and tags. Category filtering and pagination are exposed through the existing `product` query.

## Configuration

- `DUMMYJSON_BASE_URL`: source URL; defaults to `https://dummyjson.com`.
- `DUMMYJSON_SEED_ENABLED`: opt-in startup seed; defaults to `false`.
- `DUMMYJSON_SEED_LIMIT`: maximum products; `0` imports all available products.

## Consequences

- Local development gets realistic, image-backed catalog data quickly.
- Startup with seed enabled depends on external network availability and can take up to two minutes.
- DummyJSON is placeholder data and must not be treated as a production source of truth.
- Product ownership and authorization fields remain local concerns; imported products use account ID zero for demo catalog records.
