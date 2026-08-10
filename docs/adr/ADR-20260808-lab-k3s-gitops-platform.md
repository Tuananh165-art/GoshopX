# ADR-20260808-lab-k3s-gitops-platform

Status: Accepted
Date: 2026-08-08

## Context

GoshopX needs a repeatable DevSecOps laboratory deployment on one VPS limited to
8 GB RAM and 50 GB disk. The public boundary remains GraphQL; internal gRPC and
Kafka contracts must remain private. The platform needs delivery, discovery,
logs, metrics, load tests and GitOps without operating a production-scale
Kubernetes control plane or service mesh.

## Decision

- Run a single-node K3s server with its packaged Traefik ingress. Disable only
  K3s components that duplicate selected platform tools (`servicelb`,
  `metrics-server`).
- Deliver application workloads through the `goshopx` Helm chart. The chart
  defaults to one replica and strict resource requests/limits. GraphQL is the
  only public application route; every gRPC Service is `ClusterIP`.
- Use Kubernetes DNS as the data-plane service discovery mechanism. Install
  Consul in **server-only, one replica** mode and enable its one-way Kubernetes
  sync catalog, so private Services are discoverable in Consul without changing
  application endpoints. Do not enable Consul Connect sidecars or mTLS in this
  lab because their steady memory and operational cost is disproportionate.
- Install Argo CD in a single-replica, notifications-disabled profile. Its
  Application watches this repository's Helm chart and performs automated,
  self-healing syncs. CI never receives cluster credentials.
- Use Prometheus, Grafana and Loki in single-replica retention-limited modes.
  Promtail is used only if node logs are required; the lab baseline relies on
  Grafana Alloy's Kubernetes logs pipeline. Tracing is intentionally deferred.
- GitHub Actions validates, scans, builds immutable GHCR images and updates the
  image tag in Git. Argo CD detects that Git commit and deploys it.

## Consequences

The lab has a clear separation of duties: GitHub Actions has repository and
registry access, while Argo CD has cluster access. It is intentionally not HA:
loss of the VPS interrupts every control-plane and data-plane component. A
separate GitOps repository, HA database, backups, TLS and sealed/external
secrets are prerequisites for a production environment.

## Alternatives Considered

- Full kube-prometheus-stack plus Consul mesh: rejected for the 8 GB budget.
- Direct `kubectl apply` from GitHub Actions: rejected because it grants CI
  cluster credentials and bypasses GitOps reconciliation.
- Consul as a replacement for Kubernetes DNS: rejected; services already use
  DNS names such as `account:8080` and changing contracts would add risk.

## Verification

`helm lint`, `helm template`, `kubectl apply --dry-run=client`, Argo CD sync
status, ready replicas, the GraphQL health endpoint, Prometheus targets, Grafana
datasources, Loki log query, and a bounded k6 smoke test are required before a
lab release is accepted.
