# Application Observability: Prometheus, Loki, and Jaeger

This document clearly distinguishes installed infrastructure from application
observability evidence. The principal flow is Browser -> Traefik -> Kong ->
GraphQL -> gRPC.

## Current state

| Component | Available | Cannot be inferred automatically |
| --- | --- | --- |
| Prometheus | kube-prometheus-stack, ServiceMonitors for Go services, and Kubernetes dashboards | Every target must be confirmed UP; default dashboards do not imply business KPIs |
| Grafana | Prometheus/Loki data sources and an operator NodePort | Custom business dashboards or automatic correlation |
| Loki + Alloy | Kubernetes log collection and queries through Grafana Explore | A standalone Loki gateway dashboard |
| Jaeger all-in-one | UI, OTLP gRPC collector 4317, HTTP collector 4318, and query UI 16686 | End-to-end tracing for every flow and every language |
| Go services | OTLP configuration, HTTP/gRPC instrumentation, and W3C propagation | Kafka asynchronous context propagation unless consumer headers/spans are verified |
| Python recommender | Workloads run according to application configuration | Python server-side OpenTelemetry instrumentation |

Jaeger **all-in-one** is only a lab packaging of collector, query, and UI; it
does not make services emit traces automatically.

## Verify Prometheus and Loki

1. Open Prometheus at http://VM_PUBLIC_IP:30701/targets and confirm expected
   targets are UP.
2. Open Grafana at http://VM_PUBLIC_IP:30702 and select **Explore**:
   - Loki data source: {namespace="goshopx"}.
   - Prometheus data source: up or a service-emitted metric.
3. If kubectl top reports Metrics API not available, the cluster lacks Metrics
   Server/metrics API; this is not a Prometheus result. Use the
   Prometheus/Grafana UI or install Metrics Server as a separate platform change.

## Verify that Jaeger contains real traces

Confirm the collector and service discovery:

~~~bash
kubectl -n goshopx get svc jaeger graphql product
kubectl -n goshopx run jaeger-api-check --rm -i --restart=Never \
  --image=curlimages/curl:8.10.1 -- \
  curl -fsS http://jaeger:16686/api/services
~~~

Generate catalog-read traffic:

~~~bash
curl -skS https://goshopx.click/graphql \
  -H 'Content-Type: application/json' \
  --data '{"query":"query { product(pagination: { skip: 0, take: 3 }) { id name } }"}'
~~~

Then open Jaeger at http://VM_PUBLIC_IP:30713, select **Last 15 minutes**, and
select service goshopx-graphql. A good trace for this synchronous flow contains
a GraphQL span and matching downstream gRPC spans when the destination service
is called.

The Jaeger service list includes only services that **emitted spans during the
selected period**. Setting OTEL_SERVICE_NAME alone is insufficient for a
service to appear. In particular, Python recommender server-side OpenTelemetry
SDK/instrumentation has not yet been verified, so do not expect server-side spans
from those workloads.

## Consul does not currently provide tracing or a service mesh

The Consul lab only performs one-way catalog synchronization
Kubernetes -> Consul. connectInject is disabled; there are no Envoy sidecars,
mTLS, intentions, or traffic routing through Consul. Therefore an unknown
topology in Consul UI is expected and is not a Jaeger fault. Moving to Consul
service mesh is a separate migration project because it changes traffic,
policies, sidecar resources, and the rollout of every service.

## Conditions for an end-to-end observability conclusion

Call observability end-to-end only when all four forms of evidence exist:

1. The related Pod/service is Ready and its internal endpoint works.
2. The related Prometheus target is UP.
3. Loki contains a log from the request at the verification time.
4. Jaeger has a trace for the same flow with appropriate parent/child spans.

If any part is missing, record the accurate infrastructure or service-level
state rather than calling it complete end-to-end observability.

See [NodePort Operator Access](./24-operator-dashboard-nodeport-runbook.md) and
[Current GoshopX K3s Lab Operations](./26-k3s-lab-current-operations.md).
