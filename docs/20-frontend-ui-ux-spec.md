# Frontend UI/UX Specification — GoshopX

## Product outcome

GoshopX provides a fast, accessible mobile-first discovery and checkout entrypoint for shoppers, and an operational control surface for authorized support/operations/platform administrators. The web client consumes the GraphQL public boundary. Direct DummyJSON calls are restricted to visual product catalogue demo/development data.

## Surfaces and navigation

- **Storefront / Explore:** `/`, `/search`, `/products/:id`, `/cart`, `/checkout`, `/login`, `/register`, `/account/orders`.
- **Admin / Operate + Monitor:** `/admin`, `/admin/catalog`, `/admin/orders`, `/admin/transactions`, `/admin/inventory`, `/admin/accounts`, `/admin/audit`.
- **Responsive behavior:** catalogue grid is 4/3/2 columns across desktop/tablet/phone; filters/chips scroll horizontally on mobile; cart summary stacks; admin navigation collapses to a horizontal menu; controls remain at least 42px high.

## Critical rules traced to contract

| Rule | UX behavior |
| --- | --- |
| Client boundary is GraphQL | Do not route a browser to gRPC, Kafka, Redis or databases. |
| Price/stock server-authoritative | Label cart total as estimated; only backend checkout validates price and reservation. |
| Cart requires authentication and 15-minute reservation | Guest cart is a local pre-cart only; login/checkout explains server validation. Never claim a reservation before GraphQL confirms it. |
| Payment redirect and signed callback | Show a deliberate redirect handoff; do not collect payment credentials or claim settlement on browser return alone. |
| Admin is deny-by-default | Hide unavailable paths/client actions; always display a recoverable forbidden state when server rejects. |
| Product rejection requires reason | Moderation form requires non-empty reason before command submission. |
| Account self-protection | UI must disable self-suspend/self-role-change, but service revalidation is authoritative. |
| Admin metrics | GMV/revenue/AOV/payment-rate definitions follow `docs/16-admin-commerce-spec.md`; no placeholder metrics in an operational dashboard. |

## Sprint backlog

1. **WEB-01 (P0):** Establish Vite/React shell, design tokens, app routing and UI test harness. Done when production build and route smoke test pass.
2. **WEB-02 (P0):** Catalogue/detail/search based on DummyJSON normalized read models, including retry, loading, empty, responsive and keyboard states.
3. **WEB-03 (P0):** Add authenticated GraphQL client/session policy and server cart mutations; test auth, stock rejection and reservation expiry responses.
4. **WEB-04 (P0):** Implement checkout redirect/return state and notifications through GraphQL. Settlement must be callback-derived.
5. **WEB-05 (P0):** Implement real admin data/query and confirmation dialog slices by role, starting dashboard/catalog/orders/inventory/accounts/audit.
6. **WEB-06 (P1):** Add account-scoped `myOrders` GraphQL contract (backend owned) then replace customer orders contract-gap state.
7. **WEB-07 (P1):** Playwright browser regression suite at 1440px and 390px with no horizontal overflow, focus-path, visual and forbidden-state checks.

## Known contract/runtime gaps

- GraphQL query at `localhost:8080` is live, but Product Elasticsearch has no index in this environment; product query returned `index_not_found_exception`. The frontend transparently loads the development catalogue from DummyJSON instead.
- `myOrders` is absent from `graphql/schema.graphql`; it is not safe to use broad `accounts` as a substitute.
- Login, cart reservation, checkout and all admin write actions require backend tokens/role fixtures to test end to end. Do not hard-code test credentials or tokens into UI files.
- Playwright specs are included in `web/e2e/`, but this WSL runtime is missing the host library `libnspr4.so`; Chromium cannot launch until the environment installs its Playwright system dependencies. Unit and production-build checks still pass.

## Recommended follow-up fixes

1. Enable/import DummyJSON seed in the local Product service and initialize Elasticsearch before end-to-end GraphQL catalogue testing.
2. Add an authenticated `myOrders` GraphQL query and an account profile/me query so UI can fetch identity and role without decoding tokens client-side.
3. Add a `web` service/static deployment to Compose/Kong only after selecting the production origin and CORS policy; retain Kong `/graphql` as the sole application API route.
4. Pin frontend dependency versions and review the current `npm audit` advisories before release.
5. Add Playwright plus a supported Chromium runtime to CI for visual/responsive evidence.
