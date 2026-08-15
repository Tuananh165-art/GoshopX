# GoshopX K3s Lab Operations: Status, Incident Response, and Limits

This is the operational runbook for the current single-node GoshopX K3s lab. It focuses on incident response and decisions for the GCE deployment; it is not a multi-node production deployment procedure.

## 1. Current architecture and traffic boundaries

~~~text
Internet
  -> DNS goshopx.click
  -> GCP firewall
  -> Traefik Ingress
  -> kong-proxy (ClusterIP)
  -> Kong routes
  -> GraphQL public boundary
  -> internal gRPC to data-owning services
  -> Kafka for asynchronous business events
~~~

Kubernetes DNS such as product, account, and order is the primary mechanism for internal calls. Consul currently synchronizes the Kubernetes catalog only; it does not replace Kubernetes DNS and is not yet used as a data-plane service mesh.

See [Application Observability](25-application-observability.md) for the current observability boundaries.

## 2. GitOps workflow and deployment scripts

After the Argo CD application exists, the normal release flow is:

1. Commit and push to main.
2. GitHub Actions builds and pushes images, then updates the GitOps image tag.
3. Argo CD synchronizes the desired state.

Force a repository refresh when needed:

~~~bash
kubectl -n argocd annotate application goshopx-lab \
  argocd.argoproj.io/refresh=hard --overwrite

kubectl -n argocd get application goshopx-lab
~~~

Use the Ubuntu scripts with the following intent:

| Script | When to use it |
| --- | --- |
| scripts/ubuntu/05-deploy-application.sh | Initial application bootstrap or creation; do not run it for every commit. |
| scripts/ubuntu/06-verify-release.sh | Any time you need release status and diagnostic evidence. |
| scripts/ubuntu/07-rollback-application.sh | Only for a known bad revision that must be rolled back. |
| scripts/ubuntu/08-build-and-push-images.sh | A manual build and push only when GitHub Actions is not being used or cannot be awaited. Do not publish competing tags in parallel. |

kubectl rollout restart and kubectl rollout status do not support the --all flag. Restart all deployments with:

~~~bash
kubectl -n goshopx get deployment -o name | \
  xargs -r -n1 kubectl -n goshopx rollout restart
~~~

Prefer a targeted restart while troubleshooting:

~~~bash
kubectl -n goshopx rollout status deployment/product --timeout=5m
kubectl -n goshopx rollout status deployment/graphql --timeout=5m
kubectl -n goshopx get endpoints product graphql
kubectl -n goshopx logs deployment/product --since=10m
kubectl -n goshopx logs deployment/graphql --since=10m

kubectl -n goshopx rollout restart deployment/product deployment/graphql
~~~

## 3. Runtime secret, Google, Gmail, and VNPay

The runtime-secret source is /opt/goshopx/secrets.env. It must not be committed.

After safely updating it, recreate the Kubernetes Secret and restart only the affected workload:

~~~bash
cd ~/GoshopX
bash scripts/ubuntu/03-create-runtime-secret.sh /opt/goshopx/secrets.env

kubectl -n goshopx rollout restart deployment/notification
kubectl -n goshopx rollout status deployment/notification --timeout=5m
~~~

The file is sourced by the shell, so values containing spaces must be quoted. A Gmail App Password can be written as:

~~~dotenv
GMAIL_PASSWORD='app password with spaces'
~~~

Without quotes, the extra words are interpreted as commands and the script reports command not found. Do not print Secret values in logs or commit them. Revoke and rotate any token, SMTP password, OAuth credential, or payment secret that was exposed.

### Gmail delivery

A log similar to 535 5.7.8 Username and Password not accepted is a Gmail SMTP authentication rejection, not a Kafka or payment failure. Use a Google App Password on an account with two-step verification enabled.

Check the presence of the mail variables without printing their values:

~~~bash
kubectl -n goshopx exec deployment/notification -- sh -c '
for key in GMAIL_SMTP_HOST GMAIL_SMTP_PORT GMAIL_USERNAME GMAIL_PASSWORD GMAIL_FROM; do
  value=$(printenv "$key")
  printf "%s: %s bytes\n" "$key" "${#value}"
done'
~~~

### Google Sign-In

Register this exact Authorized JavaScript origin in Google Cloud Console:

~~~text
https://goshopx.click
~~~

Do not add /login, a trailing slash, or the public IP address. Configure redirect URIs separately when the selected OAuth flow requires them. The client ID belongs in the frontend configuration; never expose the client secret to the browser.

### VNPay Sandbox

The relevant runtime settings are:

~~~dotenv
VNPAY_TMN_CODE=...
VNPAY_HASH_SECRET=...
VNPAY_PAYMENT_URL=https://sandbox.vnpayment.vn/paymentv2/vpcpay.html
~~~

The callback/return URL must use HTTPS. GoshopX stores and displays totals in VND. VNPay expects vnp_Amount in the smallest unit, therefore it is VND multiplied by 100 only when the VNPay request is created.

For example, a UI total of 31,716,773 VND becomes 3,171,677,300 in the VNPay request. Do not multiply stored order totals or displayed prices a second time.

## 4. Product catalog and Elasticsearch

product-db is Elasticsearch, not PostgreSQL. Kibana is its UI. Check the catalog document count through the in-cluster Service:

~~~bash
kubectl -n goshopx run elastic-check \
  --rm -i --restart=Never \
  --image=curlimages/curl:8.10.1 -- \
  curl -fsS http://product-db:9200/catalog/_count
~~~

If the index contains documents but the web catalog fails, inspect GraphQL, Product, and Service endpoints before reseeding.

A 503, failure to get a peer from the ring-balancer, no route to host, or gRPC deadline error under load is an availability failure. It is not proof that catalog data has been lost.

## 5. Safe database access

PostgreSQL must not be exposed through a public NodePort. Use a local SSH tunnel to the VM and a Kubernetes port-forward running on the VM. The port-forward process must remain running; it is interrupted by closing the terminal or by a restart of the pod or Service.

| Database | Local VM port |
| --- | ---: |
| account-db | 15432 |
| order-db | 15433 |
| payment-db | 15434 |
| inventory-db | 15435 |
| notification-db | 15436 |
| admin-db | 15437 |
| recommender-db | 15438 |

Example on the VM:

~~~bash
kubectl -n goshopx port-forward service/account-db 15432:5432
~~~

Create an SSH tunnel from the workstation to the VM for the same local port, then configure TablePlus to connect to 127.0.0.1 with that port. The current database name is lowercase: goshopx.

nohup can keep a forwarding process alive temporarily, but a systemd unit or a tunnel manager is safer for a long-running operational workflow.

## 6. Performance limits of the single-node lab

The single node hosts Kafka, Elasticsearch, Milvus, PostgreSQL workloads, observability tools, and application services. CPU at 100% while RAM is still available can be caused by high I/O wait (wa): the CPU is waiting for disk operations. The lab has observed I/O wait around 40 to 60 percent on pd-standard storage, followed by gRPC timeouts/cancellations and catalog failures during heavy k6 traffic.

Do not run a long 50-VU test while evaluating the web UI. Start with a bounded smoke test:

~~~bash
kubectl -n goshopx run k6-graphql-smoke \
  --rm -i --restart=Never \
  --image=grafana/k6:0.54.0 \
  --env='GOSHOPX_URL=http://graphql:8080' \
  --env='K6_TARGET_VUS=2' \
  --env='K6_RAMP_UP=30s' \
  --env='K6_HOLD=30s' \
  --env='K6_RAMP_DOWN=15s' \
  -- run - < tests/k6/graphql-smoke.js
~~~

When the node is slow, first confirm that no load-test pod remains and inspect CPU/I/O wait:

~~~bash
kubectl get pods -A | grep -i k6 || true
vmstat 1 10
~~~

The durable solution is balanced/SSD storage, stateful workload separation, or a larger node. Scaling more components on one disk bottleneck is not a remedy.

## 7. Public IP, DNS, TLS, and firewall

Changing an external IP does not delete the VM disk, source code, or Kubernetes configuration. It can break DNS records, GCP firewall allowlists, and ACME certificate validation. Use a reserved static external IP for a persistent deployment.

After an IP change, update the A records for goshopx.click and www.goshopx.click, then verify certificate status:

~~~bash
kubectl -n goshopx get certificate goshopx-tls
kubectl -n goshopx get challenges.acme.cert-manager.io
~~~

The GCP NodePort firewall source range must be the browser workstation's public IP, not the VM public IP. See [Operator Dashboard NodePort Runbook](24-operator-dashboard-nodeport-runbook.md).

## 8. Handover checklist

- Argo CD reports Synced and Healthy.
- Core application and infrastructure pods are Ready.
- The goshopx-tls certificate is Ready.
- The GraphQL catalog query works, the Product Service has endpoints, and the catalog index has documents.
- Gmail delivery is confirmed with a newly generated event.
- Google Sign-In uses the exact authorized origin and current client ID.
- VNPay amount conversion is VND multiplied by 100 only for the VNPay request, and the HTTPS callback is registered.
- Operator NodePorts are limited to the operator IP; Kong Admin, databases, and protocol services are never public.
- The k6 workload is finished before judging UI health or rollout behavior.

Related runbooks:

- [Operator Dashboard NodePort Runbook](24-operator-dashboard-nodeport-runbook.md)
- [Application Observability](25-application-observability.md)
