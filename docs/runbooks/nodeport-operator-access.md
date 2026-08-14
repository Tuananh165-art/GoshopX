# Operator NodePort access (lab only)

This lab optionally exposes selected consoles on the VM public address.  DNS
does not map a port: access uses `http://goshopx.click:<nodePort>` or
`https://goshopx.click:<nodePort>` where the upstream service supports TLS.

| Component | NodePort | URL / protocol |
| --- | ---: | --- |
| Consul UI | 30700 | `http://goshopx.click:30700` |
| Prometheus | 30701 | `http://goshopx.click:30701` |
| Grafana | 30702 | `http://goshopx.click:30702` |
| Loki API | 30703 | `http://goshopx.click:30703` |
| Redis protocol | 30704 | Redis client only; use RedisInsight for a UI |
| Kafka broker | 30705 | Kafka client only; use Kafka UI for a browser UI |
| Milvus gRPC | 30706 | Milvus client only; no browser UI is installed |
| Attu (Milvus UI) | 30707 | `http://goshopx.click:30707` |
| Kong proxy | 30708 | `http://goshopx.click:30708` |
| Kong Manager | 30709 | `http://goshopx.click:30709` |
| Kong Admin API | 30710 | API only; do not expose outside a trusted IP allowlist |
| Argo CD | 30711 | `https://goshopx.click:30711` |
| GraphQL | 30712 | `http://goshopx.click:30712/graphql` |
| Jaeger UI | 30713 | `http://goshopx.click:30713` |
| Kafka UI | 30602 | `http://goshopx.click:30602` |
| RedisInsight | 30604 | `http://goshopx.click:30604` |

## Apply

Run the platform bootstrap after pulling this change. It upgrades Argo CD,
Consul, Prometheus/Grafana, Loki and Alloy with their assigned NodePorts.

```bash
cd ~/GoshopX
./scripts/ubuntu/04-bootstrap-platform.sh
```

Then let Argo CD synchronize the application chart:

```bash
kubectl -n argocd annotate application goshopx-lab \
  argocd.argoproj.io/refresh=hard --overwrite
kubectl -n goshopx get svc jaeger graphql kong-proxy kong-manager kong-admin kafka-ui redisinsight
```

## Firewall requirement

Create GCP ingress rules for only the needed TCP ports and restrict source
ranges to the operator public IP. Do not allow `0.0.0.0/0` for Kong Admin,
Argo CD, Grafana, Prometheus, Redis, Kafka, Milvus, or RedisInsight.

NodePort is a lab/operator access mechanism. The customer-facing application
continues to use HTTPS on port 443 through Traefik and Kong.
