# GoshopX load testing with k6

The checked-in [smoke script](../tests/k6/graphql-smoke.js) only probes `/health`; it does not measure commerce. Use [goshopx-load.js](../tests/k6/goshopx-load.js) for an anonymous GraphQL catalog search. It validates HTTP status, GraphQL `errors`, response shape, latency and dropped iterations. This is a starting workload, not evidence of checkout capacity.

## Terms

- **VU (virtual user):** one k6 worker executing a scenario. A VU may wait, think, or make requests; it is not the same as a real person.
- **RPS:** requests started per second. This script uses arrival-rate stages to target iterations/RPS approximately; each iteration currently sends one GraphQL request.
- **Concurrent requests:** HTTP requests in flight at one moment. This depends on RPS and response time (Little's Law: concurrency ≈ throughput × latency).
- **CCU:** concurrently active human sessions under a product definition (e.g. active in the last five minutes). Report it only when session behavior and think time are modeled; VUs alone do not prove CCU.

## Run

```powershell
$env:GOSHOPX_URL = 'https://<verified-host>/graphql'
k6 run -e K6_PROFILE=smoke tests/k6/goshopx-load.js
k6 run -e K6_PROFILE=load tests/k6/goshopx-load.js
k6 run -e K6_PROFILE=stress tests/k6/goshopx-load.js
k6 run -e K6_PROFILE=soak -e K6_SOAK_RPS=10 tests/k6/goshopx-load.js
```

Smoke targets one iteration/s for one minute. Load ramps through 5, 10 and 20 RPS, holding each for three minutes. Stress ramps through 40, 80 and 160 RPS, holding each for two minutes, with at most 200 VUs. Soak holds the chosen rate for 30 minutes. Run against an isolated lab/staging environment with seeded data and known resource limits. Never aim stress tests at an unapproved public production endpoint.

## Commerce workload and guardrails

Add authenticated flows only after provisioned test accounts and unique product inventory exist. A realistic follow-up should mix catalog/detail (roughly 70%), authenticated cart reads/writes (20%), and reservation/checkout through an idempotent payment stub (10%); isolate each VU's account and cart and reset data between runs. Do not call live AI providers in the load workload. Run a separate low-volume provider smoke test under the ₫200,000 AI budget, with provider spend measured from usage telemetry.

Stop immediately if error rate stays above 5%, latency thresholds fail, dropped iterations appear, Kafka lag grows without recovery, DB connections approach the configured cap, pods restart/OOM, or cost/resource limits are reached. Watch p95/p99, GraphQL errors, VUs, RPS, in-flight requests, CPU/memory, throttling, pod restarts, Kafka consumer lag, Redis memory and Cloud SQL connections together. The script can abort on sustained request error rate; an operator must monitor infrastructure and cost signals.

Publish a result with commit/image digests, cluster/node/SQL shape, dataset size, test profile, duration, RPS achieved, VUs used, p95/p99, error rate, dropped iterations and resource peaks. Do not publish capacity from a run where the generator saturated first. No performance number is claimed by this document.
