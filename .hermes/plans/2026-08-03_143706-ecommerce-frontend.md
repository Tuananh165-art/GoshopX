# GoshopX E-commerce Frontend Implementation Plan

> **For Hermes:** Execute this plan as thin, testable vertical slices; do not commit user changes.

**Goal:** Deliver a responsive, accessible web storefront and separately protected administration console that use GoshopX GraphQL as the production API boundary while showing DummyJSON catalog content for demo/development.

**Architecture:** Create an independent TypeScript React/Vite frontend under `web/`. A small API layer routes catalog reads to GraphQL when configured, otherwise reads the public DummyJSON Products API; all mutations and authenticated data use GraphQL only. Client state is local and replaceable: access token is memory/session scoped, cart calls server mutations when authenticated, and graceful visual/demo fallbacks never fabricate payment success or admin authorization.

**Tech Stack:** React, TypeScript, Vite, React Router, Vitest/Testing Library, CSS custom properties, native fetch, GraphQL HTTP POST.

---

## Business outcome and scope

### Actors
- **Guest/shopper:** discover catalog, search/filter products, inspect full product information, manage cart, authenticate, begin checkout, view notifications/orders.
- **Seller:** manage only own catalog where the currently exposed GraphQL contract permits it.
- **Support admin:** account/order support and read-only/order operation surfaces permitted by backend.
- **Operations admin:** catalog moderation, inventory operations, operational order actions and dashboard views.
- **Platform admin:** all administrative capabilities including account role management, except self-target and platform-admin grant restrictions.

### Non-negotiable contract rules
- The browser calls only `POST /graphql` for GoshopX application behavior; gRPC/Kafka/databases are never client endpoints.
- DummyJSON (`https://dummyjson.com`) is a development/demo catalog source only. It powers image-backed read content and must never be treated as a payment, stock, authorization, or order source of truth.
- Prices/totals are server-authoritative at order/checkout; no UI claim of an accepted order or settled payment is made before backend/provider confirmation.
- No JWT, password, provider credential, internal service URL, Kafka topic, or raw internal identifier is visibly rendered.
- Admin navigation and controls are role-gated in the client but every command depends on GraphQL/backend authorization and handles forbidden responses.

## Information architecture and UX specification

### Visual system
- **Surface archetype:** Storefront is **Explore** with a mobile commerce emphasis; admin is **Operate/Monitor**. Avoid a marketing hero/card-grid composition.
- Original warm-orange retail accent, deep ink navigation, clean neutral surfaces, deliberate font pairing and compact mobile-first density; do not clone Shopee branding or proprietary screens.
- Tokens: `--brand`, `--brand-strong`, `--ink`, `--muted`, `--surface`, `--line`, semantic success/warning/danger; 4/8px spacing scale; 10–16px radii; visible keyboard focus; touch controls >=44px.
- Responsive breakpoints: phone <640px, tablet 640–1023px, desktop >=1024px. Header search moves to its own row on phone; catalog filters become a drawer; product grid uses 2/3/4 columns according to available width; admin sidebar collapses behind a labelled menu.

### Storefront routes
1. `/` — search-led catalog, category chips, catalog grid, sort/filter controls, loading/skeleton, empty and retry states.
2. `/products/:id` — gallery, brand/category, price/discount, availability, quantity bounded by availability/minimum order quantity, shipping/return/warranty, review summary/list, recommendations and add-to-cart feedback.
3. `/search?q=&category=` — shareable filter state and paginated catalogue results.
4. `/cart` — reservation-aware items, quantity mutations/removal, cart expiry indication, totals labelled estimate/server-confirmed at checkout, login/empty/error recovery.
5. `/checkout` — authenticated preflight, redirect-transition consent/return state; never collect or store payment credentials in the app.
6. `/login`, `/register` — accessible validation, no sensitive values reflected in errors, return-to-path behavior.
7. `/account/orders`, `/account/notifications` — authenticated user history/feed with unread/read actions, loading/empty/failure states.

### Admin routes
1. `/admin` — time-window dashboard, GMV/revenue/AOV/payment success rate definitions, top product drill-in, empty/error states.
2. `/admin/catalog` — search/filter lifecycle, preview/edit affordance, moderation modal requiring reason on rejection, media validation messaging.
3. `/admin/orders` — filters, detail panel, state-safe cancellation confirmation.
4. `/admin/transactions` — list/filter, reconciliation status, explicit VNPAY unsupported-action state where backend returns it.
5. `/admin/inventory` — low-stock list, reservation monitor, adjustment dialog requiring delta/reorder level/reason and showing non-negative availability constraint.
6. `/admin/accounts` — directory, suspend/reactivate, platform-admin-only role change; self-target controls unavailable and backend errors recoverable.
7. `/admin/audit` — immutable audit search by actor/action/time; never expose credentials/payment payloads.

### State matrix
Every async view has: initial loading, success, empty, recoverable network/backend error with retry, invalid input, auth required, and forbidden where role-scoped. Destructive/admin commands require an explicit confirmation and present server response without optimistic success.

## GraphQL operation inventory

| UI capability | Contract |
| --- | --- |
| catalog/search/detail | `product(pagination, query, category, id, viewedProductsIds)` |
| category navigation | `categories(activeOnly)` |
| auth | `register`, `login` |
| cart | `myCart`, `addCartItem`, `updateCartItemQuantity`, `removeCartItem`, `clearCart` |
| checkout | `checkoutCart(redirectUrl)` |
| customer orders | account-bound GraphQL account/order data as authorized by current schema |
| notifications | `notifications`, `unreadNotificationCount`, `markNotificationRead`, `markAllNotificationsRead` |
| admin dashboard/audit | `adminDashboard`, `adminAuditEvents` |
| admin commerce | `adminOrders`, `adminOrder`, `adminTransactions`, `adminReconcileTransaction`, `adminLowStock`, `adminReservations`, `adminCancelOrder`, `adminRequestRefund`, `adminAdjustInventory` |
| admin catalog/accounts | `adminAccounts`, `adminCreateCategory`, `adminModerateProduct`, `adminAddProductMedia`, `adminUploadProductMedia`, `suspendAccount`, `reactivateAccount`, `setAccountRole` |

## Delivery slices

### Task 1: Frontend foundation and quality harness
**Files:** Create `web/package.json`, `web/vite.config.ts`, `web/tsconfig*.json`, `web/index.html`, `web/src/main.tsx`, `web/src/styles/*`, `web/src/test/setup.ts`.

1. Add a failing route/render smoke test.
2. Run `npm test` and confirm expected failure.
3. Add minimal app shell, design tokens, router and responsive layout.
4. Run focused test, then `npm run build`.

### Task 2: Typed product source and storefront explore flow
**Files:** Create `web/src/api/dummyjson.ts`, `web/src/api/graphql.ts`, `web/src/domain/*`, `web/src/features/catalog/*`, tests alongside the feature.

1. Write failing tests for DummyJSON normalization and loading/error/retry rendering.
2. Implement only read/catalog mapping and product cards.
3. Verify with Vitest; run app and inspect desktop/mobile screenshot.
4. Add URL-driven search/category/sort/pagination behavior with tests.

### Task 3: Product detail and cart experience
**Files:** Create `web/src/features/product/*`, `web/src/features/cart/*` plus tests.

1. Write failing component tests for product fallback, quantity limits and cart UI states.
2. Implement detail gallery/specification/reviews and client-visible cart workflow.
3. Use GraphQL cart mutation adapter only when authenticated; otherwise direct to login without pretending to reserve stock.
4. Verify focus, keyboard behavior, mobile layout and build.

### Task 4: Authentication, checkout, notifications, and orders
**Files:** Create `web/src/features/auth/*`, `web/src/features/checkout/*`, `web/src/features/account/*` with tests.

1. Write failed tests for validation/auth-required/redirect confirmation.
2. Implement GraphQL operation wrappers and error mapping.
3. Explicitly handle payment provider redirect and return views; do not add client payment fields.
4. Verify using mocked GraphQL tests and manual UI flows.

### Task 5: Admin console and authorization-aware navigation
**Files:** Create `web/src/features/admin/*`, tests for route/role gates and critical dialogs.

1. Write failing tests for non-admin redirect, forbidden response, rejection reason and inventory adjustment validation.
2. Implement dashboard/catalog/orders/transactions/inventory/accounts/audit screens against the exposed GraphQL schema.
3. Implement explicit pending/empty/forbidden/retry states, confirmation dialogs, responsive table-to-card transformations.
4. Verify keyboard access and mobile/desktop screenshots.

### Task 6: Browser verification and documentation
**Files:** Create `docs/20-frontend-ui-ux-spec.md`; modify `README.md` only if frontend startup instructions belong there and do not overwrite existing user work.

1. Run unit tests, production build, and an actual local browser session.
2. Verify primary shopper and admin paths at desktop and 390px mobile viewport with no console errors.
3. Document exact commands, known backend/runtime limitations, recommended next backend/frontend fixes, rollout/rollback notes.

## Acceptance criteria
- Given a guest opens the storefront, when DummyJSON is available, then image-backed product cards show readable title/category/price/rating/discount and navigating a card opens its normalized product detail.
- Given a request fails, when the user presses retry, then the view re-requests data and does not retain a misleading success state.
- Given an unauthenticated guest attempts cart/checkout, when the server-reservation action is required, then the UI asks them to authenticate and does not claim stock is reserved.
- Given a shopper changes quantity, when it exceeds product availability/minimum constraints, then the UI prevents invalid input and explains the correction.
- Given an operations admin rejects a product, when no reason is supplied, then the moderation command cannot be submitted.
- Given an admin is not authorized, when opening/administering a restricted route, then the UI hides ineligible controls and presents a safe forbidden state if the backend denies it.
- Given viewport width is 390px, when navigating catalog/cart/admin pages, then no horizontal viewport overflow occurs; all primary controls are reachable with touch/keyboard.
- Given any release candidate, when production build and focused UI tests run, then they pass; browser verification reports exact commands/screens checked.

## Risks and follow-ups
- The repository currently contains backend/admin code but no verified frontend scaffold. The initial frontend must be isolated under `web/` to avoid changing Go service ownership.
- The schema does not currently expose a dedicated `myOrders` query; customer order history UI must remain unavailable/empty with a documented contract-gap state until an account-scoped query is introduced.
- The GraphQL endpoint is same-origin in local Compose via Kong but Vite development needs an explicit proxy/configurable `VITE_GRAPHQL_URL`.
- DummyJSON data may diverge from local Product ownership/state; direct DummyJSON requests are an intentional demo fallback only, not a production architecture.
- Payments require approved redirect URL and real VNPAY signed callback for final status; browser-only testing cannot verify settlement.

## Verification commands
```bash
cd web
npm install
npm test -- --run
npm run build
npm run dev -- --host 0.0.0.0
```
Browser evidence: inspect `/`, `/products/1`, `/cart`, `/login`, `/admin` at 1440px and 390px, keyboard tab flow, and browser console. Backend contract/runtime evidence remains separate: `docker compose config --quiet` and focused Go tests.
