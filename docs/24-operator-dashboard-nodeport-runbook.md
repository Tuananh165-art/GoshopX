# NodePort Dashboard Access Runbook

This document is for operators of the single-node K3s lab. NodePort dashboards
are an administration channel, **not** part of the customer-facing website. The
public application path remains:

Internet -> Traefik -> kong-proxy -> GraphQL -> gRPC services.

Do not expose the ports below to 0.0.0.0/0. Prefer an SSH tunnel or VPN; if
direct NodePort access is required, the GCP firewall must allow only the public
Internet IP address of the workstation running the browser.

## Verify before opening a URL

~~~bash
kubectl get svc -A
kubectl -n goshopx get pods
kubectl -n argocd get application goshopx-lab
~~~

Obtain the public IP address of the **browser workstation**, not the VM:

~~~powershell
(Invoke-RestMethod https://api.ipify.org).Trim()
~~~

Then update the firewall in Cloud Shell. Replace the variables with the correct
project, zone, VM, and the IP just obtained:

~~~bash
gcloud compute firewall-rules update goshopx-operator-nodeports \
  --project="$PROJECT" \
  --source-ranges="$BROWSER_PUBLIC_IP/32"
~~~

The VM must have the network tag goshopx-operator-nodeport and the rule must
allow ranges 30601-30604 and 30700-30713. If goshopx.click uses a Cloudflare
proxy, do not use that domain for arbitrary NodePorts; access
http://VM_PUBLIC_IP:PORT directly.

## Current port catalog

Always treat kubectl get svc -A as the source of truth. This table is the lab
convention.

| Tool | Port | Usage |
| --- | ---: | --- |
| Consul UI | 30700 | http://VM_PUBLIC_IP:30700 |
| Prometheus | 30701 | http://VM_PUBLIC_IP:30701 |
| Grafana | 30702 | http://VM_PUBLIC_IP:30702 |
| Loki gateway | 30703 | HTTP API; it has no standalone dashboard |
| Redis | 30704 | TCP protocol; use RedisInsight rather than a browser |
| Kafka | 30705 | TCP protocol; use Kafka UI rather than a browser |
| Milvus | 30706 | gRPC protocol; use Attu rather than a browser |
| Attu | 30707 | http://VM_PUBLIC_IP:30707 |
| Kong Proxy | 30708 | API edge; not a dashboard |
| Kong Manager | 30709 | Kong administration UI; a strict firewall is mandatory |
| Kong Admin | 30710 | Admin API; not a public UI |
| Argo CD | 30711 | https://VM_PUBLIC_IP:30711; TLS warning is expected when using an IP |
| GraphQL | 30712 | API at /graphql; use curl, Postman, or a client |
| Jaeger | 30713 | http://VM_PUBLIC_IP:30713 |
| Kibana | 30601 | Elasticsearch catalog UI |
| Kafka UI | 30602 | Topic, consumer, and message UI; primarily read-only |
| MinIO Console | 30603 | Object-storage UI |
| RedisInsight | 30604 | Redis UI |

Traefik serves the website ingress through the K3s NodePorts 30426 (HTTP) and
32727 (HTTPS). The Traefik dashboard is not enabled or exposed in this lab, so
there is no supported Traefik dashboard URL.

## Authentication and safe use

- Grafana: username admin; obtain the password from the secret and never put it
  in Git or chat.

  ~~~bash
  kubectl -n monitoring get secret kube-prometheus-stack-grafana \
    -o jsonpath='{.data.admin-password}' | base64 -d; echo
  ~~~

- Argo CD: username admin; the initial password exists only while the bootstrap
  secret has not been deleted.

  ~~~bash
  kubectl -n argocd get secret argocd-initial-admin-secret \
    -o jsonpath='{.data.password}' | base64 -d; echo
  ~~~

- MinIO Console: use MINIO_ACCESS_KEY and MINIO_SECRET_KEY from the runtime
  secret; never copy those values into documentation.
- The Consul lab does not enable ACLs and is intended only for Kubernetes catalog
  viewing. It is not a production administration account.
- Kong Manager/Admin, Argo CD, and every protocol NodePort are sensitive
  operational surfaces: access them only from an allowlisted IP address.

## Quick troubleshooting

If a browser cannot reach a NodePort, check the following in order:

~~~bash
kubectl -n <namespace> get svc <service> -o wide
kubectl -n <namespace> get endpoints <service>
kubectl -n <namespace> get pods
~~~

From the VM, use the node InternalIP to distinguish a Service/Pod fault from a
GCP firewall fault:

~~~bash
NODE_IP=$(kubectl get node -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}')
curl -i --max-time 10 "http://$NODE_IP:NODEPORT/"
~~~

If this succeeds on the VM but not in the browser, the cause is GCP firewall,
IP allowlisting, network routing, or Cloudflare—not Kubernetes. If endpoints
are empty or a Pod is not Ready, wait for or repair the workload before changing
the firewall.

## Grafana, Prometheus, and Loki

- Grafana and Prometheus are web UIs on ports 30702 and 30701.
- The Loki gateway on 30703 is an API. View logs in **Grafana > Explore > Loki**,
  for example with query {namespace="goshopx"}.
- In Prometheus, open **Status > Targets**; only an UP target proves that scraping works.
- The default Grafana dashboards are mainly Kubernetes dashboards. They do not
  automatically prove that every GoshopX API exposes business metrics.

To reapply Prometheus/Grafana while retaining the current Grafana password:

~~~bash
export GRAFANA_ADMIN_PASSWORD="$(kubectl -n monitoring get secret kube-prometheus-stack-grafana \
  -o jsonpath='{.data.admin-password}' | base64 -d)"

helm upgrade --install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --values ops/observability/kube-prometheus-stack-values-lab.yaml \
  --set-string grafana.adminPassword="$GRAFANA_ADMIN_PASSWORD" \
  --wait --timeout 10m

unset GRAFANA_ADMIN_PASSWORD
kubectl -n monitoring rollout status deployment/kube-prometheus-stack-grafana --timeout=10m
~~~

## Tools that are not browser dashboards

- GraphQL and Kong Proxy are APIs; call them with an HTTP client.
- Redis, Kafka, and Milvus are protocol services; use RedisInsight, Kafka UI,
  and Attu for observation.
- Product Elasticsearch is exposed as the Service product-db on port 9200. It is
  not PostgreSQL; use Kibana or inspect it from inside the cluster:

  ~~~bash
  kubectl -n goshopx run elastic-check --rm -i --restart=Never \
    --image=curlimages/curl:8.10.1 -- \
    curl -fsS http://product-db:9200/_cat/indices?v
  ~~~

- In Kafka UI, /tmp/kafka-logs is broker log-directory metadata. The UI is not a
  file browser; inspect Topics, Consumers, consumer lag, and Messages instead.

See [Current GoshopX K3s Lab Operations](./26-k3s-lab-current-operations.md) for
architecture, observability limits, secret updates, GitOps, and the full incident
playbook.
