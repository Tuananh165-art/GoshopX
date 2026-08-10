# GoshopX Web

Responsive React/Vite storefront and admin-console prototype for the GraphQL-first GoshopX platform.

## Run locally

```bash
cd web
npm install
npm run dev
```

Open `http://localhost:5173`. During development Vite proxies `/graphql` to `http://localhost:8080`; set `VITE_GRAPHQL_URL` when using another public GraphQL origin. Production must serve the UI and GraphQL from the same approved origin, or configure the gateway/CORS explicitly.

## Data sources and trust boundaries

- Product discovery/detail uses `https://dummyjson.com` as a development/demo image-backed catalogue.
- GoshopX operations are intentionally reserved for GraphQL. The UI must never invoke gRPC, Kafka, databases, or private service URLs.
- DummyJSON is never a source of truth for payment, stock reservation, customer identity, order totals, or admin authorization.
- The current backend has no account-scoped `myOrders` query; the shopper order-history route therefore remains an auth-required contract-gap state rather than fabricating orders.

## Quality commands

```bash
npm test -- --run
npm run build
```

## Docker and public ingress

The production image is a two-stage Node build plus Nginx static server. It exposes only internal port `80`; Compose does not publish it to the host. Kong owns the public `http://localhost:8080` ingress and routes browser `GET` requests to the SPA and `POST /graphql` to GraphQL.

```bash
docker compose build web
docker compose up -d web kong
curl -fsS http://localhost:8080/healthz
```

The Nginx container answers `/healthz` and returns `index.html` for client-side routes. Roll back the UI by stopping/rerolling the `web` image; it has no database, volumes, migrations, or persistent state.

`npm audit` currently reports two high-severity transitive dependency advisories from the freshly resolved toolchain. Do not run `npm audit fix --force` without reviewing the breaking upgrade.

## Source structure

```text
src/
├── app/                 # route composition and app-level state
├── components/          # shared layout and UI primitives
├── domain/              # business-facing frontend types
├── features/
│   ├── admin/            # admin console and operational states
│   ├── auth/             # login/register boundary
│   ├── cart/             # local cart UX and checkout handoff
│   ├── catalog/          # search, category, product detail
│   └── checkout/         # payment-safe handoff
└── shared/
    ├── api/              # DummyJSON adapter and Kong GraphQL client
    └── lib/              # formatting and pure helpers
```

`src/App.tsx` is intentionally only a compatibility entrypoint; feature/page logic belongs in the folders above. The production frontend calls same-origin `/graphql`, which Kong routes to GraphQL. In development, Vite proxies that path to `localhost:8080`.
