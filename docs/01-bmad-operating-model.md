# BMAD Operating Model

BMAD in this repo means Business, Model, Architecture, and Delivery. Every substantial change should travel through those four lenses before code is considered complete.

## 1. Business

Clarify the customer or operator outcome before implementation. For GoshopX, business impact usually lands in one of these flows:

- Account: register, login, authenticate, and resolve account identity.
- Catalog: create, update, delete, search, and browse products.
- Order: convert selected products into a priced order.
- Payment: create checkout/customer portal sessions and react to provider/payment events.
- Recommendation: consume product and interaction events to produce product discovery suggestions.

Business output should include the problem, users affected, happy path, error paths, data touched, and acceptance criteria.

## 2. Model

Model the domain in repo terms before naming code. Use the same terms in stories, GraphQL schema, protobufs, events, tests, and docs.

- `Account` owns identity and auth token creation.
- `Product` owns catalog data and Elasticsearch indexing.
- `Order` owns order creation, product quantities, and total price.
- `Payment` owns checkout/provider integration and payment transaction state.
- `Recommender` owns recommendation state, training artifacts, and ranking.
- `GraphQL` is the public client contract.
- `Kafka` carries asynchronous business facts between services.
- `gRPC` carries synchronous internal service calls.

## 3. Architecture

Preserve the service boundaries unless an architecture decision record explains why they change. Browser/client traffic should go through GraphQL. Services should not read another service's database directly. Cross-service side effects should be explicit through gRPC or Kafka.

Architecture output should include affected services, contracts, data ownership, failure modes, observability needs, and rollout/backout notes.

## 4. Delivery

Implement in thin vertical slices. A good GoshopX slice normally touches one public GraphQL behavior, one internal service path, one persistence/event concern, and one verification path. Finish with evidence:

- Unit tests for changed service logic.
- Contract or e2e tests when GraphQL/gRPC/Kafka boundaries change.
- `docker compose config --quiet` for Compose changes.
- Security review for auth, payment, secrets, or user data changes.

## Role Handoff Order

1. BA captures business rules and acceptance criteria.
2. PM turns the business need into prioritized stories.
3. Solution Architect defines boundaries, contracts, and risks.
4. Backend and Frontend implement the slice through approved contracts.
5. Tester verifies behavior and regression risk.
6. DevOps verifies runtime, release, observability, and rollback.
7. Cyber Security checks auth, data, secrets, supply chain, and abuse cases.
