# Deploying GoshopX to Ubuntu K3s

This document is the operating procedure for an Ubuntu 22.04/24.04 VPS with
8 GB RAM and a 50 GB disk. It uses the scripts in scripts/ubuntu/; it does
not replace GitOps and does not store secrets in Git.

## 1. Target architecture and constraints

~~~text
Internet -> Traefik (K3s Ingress) -> Kong proxy -> Web / GraphQL / payment webhook
                                                -> gRPC services (ClusterIP only)
                                                -> Kafka, Redis, PostgreSQL, Elasticsearch, MinIO, Milvus
GitHub Actions -> GHCR immutable SHA -> Git values -> Argo CD -> K3s
~~~

Kong is the API gateway; Traefik is only the ingress controller. Browsers call
Kong only for Web/GraphQL. gRPC, PostgreSQL, Kafka, Redis, Elasticsearch, MinIO,
and Milvus must not be publicly exposed. Kong Admin/Manager and all technical
dashboards are ClusterIP by default; operators should use port-forward.

An 8 GB single-node environment has no HA. Deploy in phases:

| Phase | Enable | Do not enable immediately |
| --- | --- | --- |
| 0 | K3s, Kong, Web, GraphQL, core services, and required backing services | Milvus, recommender, admin, Kibana, dashboard UIs |
| 1 | Argo CD and Consul catalog sync | Consul Connect sidecar/mesh |
| 2 | Prometheus, Grafana, Loki/Alloy | Long retention and Alertmanager HA |
| 3 | k6 as a job | Continuously running k6 |

## 2. Prepare DNS, firewall, and repository

1. Create a DNS A record mapping <APP_HOST> to the VPS IP address.
2. Allow only TCP 22, 80, and 443 in the firewall/security group. Do not open
   6443, the NodePort range, Kong 8001/8002, Grafana, Prometheus, or Argo CD.
3. Sign in to the VPS with a user that has sudo, clone the repository, and
   enter the checkout directory.

~~~bash
git clone https://github.com/OWNER/GoshopX.git
cd GoshopX
chmod +x scripts/ubuntu/*.sh scripts/lab/*.sh
./scripts/ubuntu/00-host-prerequisites.sh
~~~

Do not run the scripts with sudo; scripts invoke sudo only for required system
operations. Check free -h and df -h before every phase.

## 3. Install K3s and CLI tools

Choose a reviewed K3s version. The script requires this variable to avoid
installing an uncontrolled latest version.

~~~bash
export K3S_VERSION='v1.31.4+k3s1'
./scripts/ubuntu/01-install-k3s.sh
export HELM_VERSION='v3.16.4'
export ARGOCD_VERSION='v2.13.3'
./scripts/ubuntu/02-install-cli-tools.sh
kubectl get nodes
helm version --short
~~~

K3s is installed as a single server, keeps Traefik, disables servicelb and
metrics-server, and configures image garbage collection at 70/55.
~/.kube/config is an administrative credential; do not commit it or put it
in GitHub Actions.

## 4. Configure GitOps before deployment

Review and replace placeholders in:

- deploy/helm/goshopx/values-lab.yaml: global.imageRegistry, image.tag,
  global.ingress.host, and service enablement for the current phase.
- ops/gitops/argocd/applications/goshopx-lab.yaml: repository URL and GHCR owner.

image.tag must be a commit SHA that exists in GHCR; never use latest.
In the GitHub repository, create the GHCR_OWNER variable, enable a GitHub
Environment named lab with reviewers, protect the main branch, and require
passing CI quality gates.

## 5. Create the runtime secret outside Git

Create a file in a protected location outside the checkout, for example
/opt/goshopx/secrets.env:

~~~bash
sudo install -d -m 0700 /opt/goshopx
sudo cp scripts/ubuntu/goshopx-secrets.env.example /opt/goshopx/secrets.env
sudo chown "$USER":"$USER" /opt/goshopx/secrets.env
chmod 600 /opt/goshopx/secrets.env
nano /opt/goshopx/secrets.env
./scripts/ubuntu/03-create-runtime-secret.sh /opt/goshopx/secrets.env
~~~

Provide real values and do not retain literal replace placeholders. Each
service owner requires a separate database URL. For chart-managed internal
infrastructure, hostnames are account-db, order-db, payment-db, inventory-db,
notification-db, admin-db, and recommender-db; Product uses
http://product-db:9200.

## 6. Bootstrap Argo CD, Consul, and observability

~~~bash
export GRAFANA_ADMIN_PASSWORD='set-a-unique-password-in-your-shell-only'
export GITOPS_REPO_URL='https://github.com/OWNER/GoshopX.git'
export GHCR_IMAGE_REGISTRY='ghcr.io/OWNER'
./scripts/ubuntu/04-bootstrap-platform.sh
unset GRAFANA_ADMIN_PASSWORD
~~~

Argo CD, single-server Consul/catalog sync, Prometheus, Grafana, Loki, and Alloy
are installed into separate namespaces. A successful Helm --wait does not
confirm DNS, TLS, or public reachability.

## 7. Build and publish images

The standard path is to push to main: GitHub Actions tests, scans, builds the
images, and promotes the SHA to Git. Do not grant CI a kubeconfig.

Build manually only when necessary on a trusted builder:

~~~bash
export GHCR_IMAGE_REGISTRY='ghcr.io/OWNER'
export IMAGE_TAG="$(git rev-parse HEAD)"
export REGISTRY_USERNAME='github-user-or-bot'
read -rsp 'GHCR token: ' REGISTRY_TOKEN; echo
./scripts/ubuntu/08-build-and-push-images.sh
unset REGISTRY_TOKEN
~~~

Then commit and push the image tag to values-lab.yaml so Argo CD can observe
the desired state. Do not use manual Helm deployment as the routine release
path after GitOps bootstrap.

## 8. Deploy the application for the first time

Start with the lightweight profile. ENABLE_INFRA=true deploys the dependency
image profile and seven 2 GiB PostgreSQL PVCs; enable it only after confirming
resource headroom.

~~~bash
export GHCR_IMAGE_REGISTRY='ghcr.io/OWNER'
export IMAGE_TAG='COMMIT_SHA_FROM_GHCR'
export INGRESS_HOST='<APP_HOST>'
export TLS_SECRET_NAME='<EXISTING_TLS_SECRET>'
export ENABLE_INFRA=false
./scripts/ubuntu/05-deploy-application.sh
~~~

When backing services are ready and measured capacity is sufficient, use
ENABLE_INFRA=true. Before enabling Milvus, recommender, admin, or Kibana, set
the relevant services to enabled: true in values-lab.yaml and commit so Argo CD
can reconcile. Do not enable NodePort dashboards unless the lab has a firewall
allowlist: ENABLE_NODEPORT_DASHBOARDS=true.

## 9. Verify and operate

~~~bash
./scripts/ubuntu/06-verify-release.sh
kubectl -n goshopx get pods,svc
kubectl -n argocd get applications.argoproj.io goshopx-lab
kubectl -n goshopx port-forward svc/kong-manager 8002:8002
~~~

From an operator workstation, verify https://<APP_HOST>/health, POST /graphql,
Argo application status Synced/Healthy, Prometheus targets, the Grafana Loki
data source, and a k6 smoke test. Test payment webhooks only with valid signed
sandbox events.

## 10. Rollback and common incidents

The standard rollback is to revert the GitOps promotion commit and wait for
Argo synchronization. Use an emergency recovery only when required:

~~~bash
helm history goshopx -n goshopx
./scripts/ubuntu/07-rollback-application.sh REVISION
~~~

- ImagePullBackOff: verify the image SHA exists in GHCR and check pull permission/imagePullSecret.
- Pending PVC: inspect kubectl get storageclass,pvc -A; K3s needs local-path.
- Elasticsearch crash: check host vm.max_map_count and RAM; do not enable it
  with Milvus/recommender on a node without sufficient headroom.
- Kong 502: inspect kubectl -n goshopx get endpoints kong-proxy graphql payment;
  Product/Payment port 8081 is internal HTTP, not public gRPC.

## Verification status

The scripts have been checked for Bash syntax and the chart has passed Helm
lint/render. K3s installation, DNS, TLS, GHCR permissions, image pulls, pod
readiness, and public HTTP must be verified again on the real VPS.
