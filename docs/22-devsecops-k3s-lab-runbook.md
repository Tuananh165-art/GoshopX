# DevSecOps K3s lab runbook

## Outcome and acceptance criteria

This lab provides a reproducible, low-resource release path:

1. A pull request runs formatting, unit tests, secret/dependency/container scans
   and Helm rendering.
2. A protected `main` build publishes immutable images to GHCR and commits only
   the image tag to Git.
3. Argo CD reconciles that Git commit into K3s; GitHub Actions has no kubeconfig.
4. Kong is the only application gateway. K3s Traefik publishes Kong only; it
   never routes directly to GraphQL. Databases, Kafka, gRPC, Consul,
   Prometheus, Grafana, Loki and Argo CD remain cluster-private and are accessed
   via authenticated ingress or temporary port-forward.
5. Metrics, logs and a bounded GraphQL k6 smoke test provide release evidence.

## Lab capacity and phased rollout

Do not install every optional component at once. A single 8 GB node is not HA.

| Phase | Components | Approx. requests | Notes |
| --- | --- | ---: | --- |
| 0 | K3s + Traefik + app core | 2.5-3.5 GiB | Start with GraphQL and required backing services only. |
| 1 | Argo CD + Consul catalog sync | 0.7 GiB | No Consul sidecars; Kubernetes Services sync one-way to Consul. |
| 2 | Prometheus + Grafana + Loki | 1.1 GiB | 3-7 day retention; local-path volumes. |
| 3 | k6 job on demand | 0.15 GiB | Never run continuously. |

Keep at least 1 GiB allocatable headroom. Set K3s image garbage collection and
monitor `df -h`; retain five GHCR tags and 3 days of logs for the first test.
Milvus, Kibana and AI training are optional workloads: enable them only after
the core release is stable and memory has been measured.

## Runtime inventory mapped from Compose

| Layer | Images/workloads | Kubernetes exposure |
| --- | --- | --- |
| Edge | `kong:3.9`, GHCR images `goshopx-web`, `goshopx-graphql` | Traefik Ingress -> `kong-proxy` ClusterIP -> Kong routes. Kong Admin/Manager are ClusterIP. |
| Internal domain | account, product, order, payment, inventory, cart, notification, admin, recommender server/sync/train | ClusterIP only; all Go/Python gRPC is port 8080. Product and Payment additionally serve internal HTTP on 8081. |
| Data/messaging | PostgreSQL per owner, Elasticsearch 6.2.4, Redis 7.4, Kafka 4.1.1, MinIO, Milvus 2.5.10 with etcd/MinIO | ClusterIP only. Persistent volumes and per-service credentials are mandatory before enabling a full stack. |
| Operator UIs | Kong Manager, Kibana, Kafka UI, MinIO Console, RedisInsight, Grafana, Argo CD, Consul UI | ClusterIP by default; optional NodePorts are lab-only and require VPS firewall IP allowlists. |

## 1. Prepare the VPS

Use Ubuntu 22.04/24.04, a non-root sudo user and a DNS name before enabling TLS.
Open only SSH, HTTP and HTTPS at the firewall. Do not expose the K3s API,
Grafana, Prometheus, Consul or Argo CD to the Internet.

```bash
sudo apt-get update && sudo apt-get upgrade -y
sudo swapoff -a
curl -sfL https://get.k3s.io | sh -s - server \
  --disable servicelb --disable metrics-server \
  --write-kubeconfig-mode 600 \
  --kubelet-arg=image-gc-high-threshold=70 \
  --kubelet-arg=image-gc-low-threshold=55
sudo kubectl get nodes
```

Install the pinned CLI versions used by this repository (`kubectl`, Helm and
the Argo CD CLI), then copy `/etc/rancher/k3s/k3s.yaml` only to an administrator
workstation. Never add that file to GitHub secrets for this GitOps flow.

## 2. Bootstrap namespaces, secrets and platform charts

Create runtime secrets locally; the Helm chart references the existing secret
and deliberately cannot render credentials from `values*.yaml`.

```bash
kubectl create namespace goshopx
kubectl -n goshopx create secret generic goshopx-runtime \
  --from-literal=SECRET_KEY='replace-with-32-plus-random-bytes' \
  --from-literal=POSTGRES_USER='goshopx' \
  --from-literal=POSTGRES_PASSWORD='replace-with-unique-password' \
  --from-literal=ACCOUNT_DATABASE_URL='postgres://goshopx:replace-with-unique-password@account-db:5432/goshopx?sslmode=disable' \
  --from-literal=ORDER_DATABASE_URL='postgres://goshopx:replace-with-unique-password@order-db:5432/goshopx?sslmode=disable' \
  --from-literal=PAYMENT_DATABASE_URL='postgres://goshopx:replace-with-unique-password@payment-db:5432/goshopx?sslmode=disable' \
  --from-literal=INVENTORY_DATABASE_URL='postgres://goshopx:replace-with-unique-password@inventory-db:5432/goshopx?sslmode=disable' \
  --from-literal=NOTIFICATION_DATABASE_URL='postgres://goshopx:replace-with-unique-password@notification-db:5432/goshopx?sslmode=disable' \
  --from-literal=ADMIN_DATABASE_URL='postgres://goshopx:replace-with-unique-password@admin-db:5432/goshopx?sslmode=disable' \
  --from-literal=RECOMMENDER_DATABASE_URL='postgresql://goshopx:replace-with-unique-password@recommender-db:5432/goshopx' \
  --from-literal=PRODUCT_DATABASE_URL='http://product-db:9200' \
  --from-literal=MINIO_SECRET_KEY='replace-me' \
  --from-literal=VNPAY_HASH_SECRET='only-if-enabled' \
  --dry-run=client -o yaml | kubectl apply -f -

export GRAFANA_ADMIN_PASSWORD='use-a-unique-long-password'
export GITOPS_REPO_URL='https://github.com/OWNER/GoshopX.git'
export GHCR_IMAGE_REGISTRY='ghcr.io/OWNER'
./scripts/lab/bootstrap-platform.sh
```

The script installs Argo CD, Consul, Prometheus, Loki/Grafana and applies the
Argo Application. Review its chart versions before each upgrade. It is
idempotent but requires a connected cluster and Helm; it does not validate an
Internet-facing hostname or TLS certificate. The application chart can render
the Compose-matched dependency image set through
`--set infrastructure.enabled=true`, including a 2 GiB persistent PostgreSQL
StatefulSet for each service owner. For a first lab, use managed
PostgreSQL/Redis/Kafka/OpenSearch-compatible catalog endpoints or enable the
single-replica stateful profile only after measuring headroom. Keep a
 different database and least-privilege login for each owning service in any
shared PostgreSQL server. The chart injects only the secret keys each workload
requires, rather than exposing every runtime secret to every Pod.

## 3. Deploy application and observe

For the initial manual bootstrap (Argo CD takes over after this), run:

```bash
helm upgrade --install goshopx ./deploy/helm/goshopx \
  --namespace goshopx --create-namespace \
  --values ./deploy/helm/goshopx/values-lab.yaml \
  --set global.imageRegistry=ghcr.io/OWNER \
  --set global.runtimeSecretName=goshopx-runtime
kubectl -n goshopx rollout status deployment/graphql --timeout=5m
kubectl -n goshopx get pods
```

Replace `OWNER` with the lowercase GitHub organization/user. Set the actual
host in `global.ingress.host`; Traefik then forwards to `kong-proxy`, whose
DB-less configuration owns `/`, `/graphql`, `/health`, `/playground`, and the
payment webhook route. Leave ingress disabled until DNS/TLS is ready.

Kong Admin API and Kong Manager, Kibana, Kafka UI, MinIO Console and
RedisInsight are management surfaces, not application APIs. Their Kubernetes
Services are `ClusterIP` by default. The chart contains optional NodePorts only
for a firewall-restricted lab; do not enable them on a public VPS. Prefer:

```bash
kubectl -n goshopx port-forward svc/kong-manager 8002:8002
kubectl -n goshopx port-forward svc/kong-admin 8001:8001
```
For lab-only access use `kubectl -n goshopx port-forward svc/graphql
8080:8080` and call `http://127.0.0.1:8080/health`.

## 4. CI/CD and GitOps promotion

Configure repository variables `GHCR_OWNER` and `IMAGE_REPOSITORY`; protect
`main` and require the CI workflow. The workflow's `contents: write` permission
is used only on pushes to `main` to change the immutable `image.tag` in
`values-lab.yaml`. Prefer a GitHub environment with reviewers for this job.

Argo CD polls/reconciles the `main` branch. Verify with:

```bash
kubectl -n argocd get applications.argoproj.io
argocd app get goshopx-lab
```

To roll back, revert the promotion commit (preferred) and wait for Argo sync,
or run `helm rollback goshopx <revision> -n goshopx` only as emergency recovery;
then reconcile Git immediately so Git remains authoritative.

## 5. Operational checks and recovery

```bash
kubectl -n monitoring get pods
kubectl -n monitoring port-forward svc/kube-prometheus-stack-grafana 3000:80
kubectl -n monitoring port-forward svc/prometheus-operated 9090:9090
kubectl -n logging port-forward svc/loki-gateway 3100:80
kubectl -n goshopx run k6 --rm -i --restart=Never \
  --image=grafana/k6:0.54.0 -- run - < tests/k6/graphql-smoke.js
```

Check Prometheus `up`, Grafana datasource health, and query Loki for
`{namespace="goshopx"}`. Back up manifests and database volumes separately;
K3s local-path PVs are tied to this VPS. Restore testing must be performed on a
separate lab node before trusting a backup.

## Security review

Threats and controls: CI supply-chain tampering is constrained with versioned
actions, least privileges, Dependabot review and immutable SHA tags; secret leakage is constrained
by pre-commit/CI scans and existing Kubernetes Secrets; public abuse is limited
to GraphQL ingress; cluster lateral movement is reduced with NetworkPolicies;
log disclosure is reduced by avoiding request bodies, credentials and JWTs in
logs. NetworkPolicies need a CNI that enforces them (K3s default Flannel does
not); install Cilium/Calico before treating them as enforced controls.
