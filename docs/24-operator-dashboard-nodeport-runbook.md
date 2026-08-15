# Runbook truy cập và vận hành dashboard NodePort

Tài liệu này áp dụng cho K3s lab một node của GoshopX. Dashboard NodePort là
**kênh vận hành**, không phải một phần của website khách hàng. Luồng khách hàng
vẫn là `Internet -> Traefik Ingress -> kong-proxy -> GraphQL -> dịch vụ nội bộ`.

> Không mở các NodePort cho `0.0.0.0/0`. Các giao diện dưới đây không có TLS
> riêng hoặc không có xác thực mặc định. Chỉ cho phép IP Internet của người vận
> hành, hoặc ưu tiên SSH tunnel/VPN khi dùng thường xuyên.

## 1. Kiểm tra trạng thái thật trước khi truy cập

Đừng suy ra một cổng đang mở chỉ vì file `values-lab.yaml` có khai báo cổng đó.
Service đang chạy trên cluster là nguồn sự thật:

```bash
kubectl get svc -A
kubectl -n goshopx get pods
kubectl -n argocd get application goshopx-lab
```

Một Service chỉ truy cập trực tiếp qua NodePort khi cột `TYPE` là `NodePort`
và cột `PORT(S)` có dạng `cổng-service:cổng-nodeport/TCP`.

Ví dụ `consul-ui ... NodePort ... 80:30700/TCP` nghĩa là URL là
`http://VM_PUBLIC_IP:30700`.

### Firewall GCP bắt buộc

Lấy **IP Internet của máy đang mở trình duyệt** trên PowerShell Windows:

```powershell
(Invoke-RestMethod https://api.ipify.org).Trim()
```

Trong Cloud Shell, thay `YOUR_BROWSER_PUBLIC_IP` bằng IP vừa nhận được. Không
điền IP public của VM vào `--source-ranges`.

```bash
gcloud compute firewall-rules update goshopx-operator-nodeports \
  --project=generated-mote-467410-b0 \
  --source-ranges=YOUR_BROWSER_PUBLIC_IP/32
```

Kiểm tra từ máy Windows trước khi mở trình duyệt:

```powershell
Test-NetConnection VM_PUBLIC_IP -Port 30700
```

Khi dùng Cloudflare, truy cập `VM_PUBLIC_IP:NODEPORT` thay vì
`goshopx.click:NODEPORT`: proxy Cloudflare thông thường không chuyển tiếp mọi
cổng tùy ý.

## 2. Bảng URL và trạng thái NodePort

Thay `VM_PUBLIC_IP` bằng IP public hiện tại của VM. Các cổng trong bảng là cấu
hình lab mong muốn; luôn đối chiếu lại bằng `kubectl get svc -A` trước khi dùng.

| Công cụ | Service/namespace | URL hoặc giao thức | Đăng nhập / mục đích |
| --- | --- | --- | --- |
| Consul UI | `consul/consul-ui` | `http://VM_PUBLIC_IP:30700` | Không bật ACL trong lab. Xem catalog K8s đã đồng bộ; không phải mesh data plane. |
| Prometheus | `monitoring/kube-prometheus-stack-prometheus` | `http://VM_PUBLIC_IP:30701` | Không có tài khoản mặc định. Dùng query, Targets, Alerts. |
| Grafana | `monitoring/kube-prometheus-stack-grafana` | `http://VM_PUBLIC_IP:30702` | User `admin`; mật khẩu lấy từ Secret. Explore Loki/Prometheus và dashboard. |
| Loki | `logging/loki-gateway` | `http://VM_PUBLIC_IP:30703` | Đây là HTTP API, không phải dashboard. Xem log trong Grafana Explore. |
| Redis protocol | `goshopx/redis` | `VM_PUBLIC_IP:30704` | TCP Redis, không dùng browser. Dùng RedisInsight. |
| Kafka broker | `goshopx/kafka` | `VM_PUBLIC_IP:30705` | TCP Kafka, không dùng browser. Dùng Kafka UI. |
| Milvus | `goshopx/milvus` | `VM_PUBLIC_IP:30706` | gRPC, không dùng browser. Dùng Attu. |
| Attu | `goshopx/attu` | `http://VM_PUBLIC_IP:30707` | UI Milvus; kiểm tra collections/vector data. |
| Kong Proxy | `goshopx/kong-proxy` | `http://VM_PUBLIC_IP:30708` | API edge; không phải dashboard. Không dùng thay URL website chính. |
| Kong Manager | `goshopx/kong-manager` | `http://VM_PUBLIC_IP:30709` | Quản trị Kong DB-less. Không có auth mặc định, phải giới hạn firewall. |
| Kong Admin API | `goshopx/kong-admin` | `http://VM_PUBLIC_IP:30710` | API quản trị, không phải UI. Không mở rộng rãi. |
| Argo CD | `argocd/argocd-server` | `https://VM_PUBLIC_IP:30711` | User `admin`; TLS trực tiếp có thể hiện cảnh báo chứng chỉ. |
| GraphQL | `goshopx/graphql` | `http://VM_PUBLIC_IP:30712/graphql` | API công khai của ứng dụng, không phải dashboard. Dùng curl/Postman/GraphQL client. |
| Jaeger | `goshopx/jaeger` | `http://VM_PUBLIC_IP:30713` | UI trace. Hiện collector có cài nhưng trace ứng dụng chưa được xác minh là xuất vào đây. |
| Kibana | `goshopx/kibana` | `http://VM_PUBLIC_IP:30601` | UI Elasticsearch catalog; không có auth mặc định. |
| Kafka UI | `goshopx/kafka-ui` | `http://VM_PUBLIC_IP:30602` | UI topics, consumers, messages; chart cấu hình readonly. |
| MinIO Console | `goshopx/minio-console` | `http://VM_PUBLIC_IP:30603` | Đăng nhập bằng `MINIO_ACCESS_KEY` và `MINIO_SECRET_KEY` từ runtime Secret. |
| RedisInsight | `goshopx/redisinsight` | `http://VM_PUBLIC_IP:30604` | Thêm kết nối Redis `redis:6379`; xác nhận có/không có mật khẩu bằng cấu hình Redis thực tế. |

### Lấy mật khẩu không ghi vào tài liệu hoặc Git

Grafana:

```bash
kubectl -n monitoring get secret kube-prometheus-stack-grafana \
  -o jsonpath='{.data.admin-password}' | base64 -d; echo
```

Argo CD (chỉ có khi initial secret vẫn tồn tại):

```bash
kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath='{.data.password}' | base64 -d; echo
```

Không dán mật khẩu vào issue, commit, chat công khai hoặc file `values*.yaml`.

## 3. Vì sao một số URL hiện chưa vào được

Snapshot `kubectl get svc -A` đã cung cấp cho thấy:

- `graphql`, Argo CD, Consul, Attu, Jaeger, Kafka UI, Kibana, RedisInsight,
  Kong và các protocol NodePort đã là `NodePort`.
- Grafana, Prometheus và `loki-gateway` vẫn là `ClusterIP`; vì vậy cổng
  `30701`, `30702`, `30703` chưa tồn tại trên VM dù các file values đã khai báo.
- `traefik` là Service `LoadBalancer` của K3s với các NodePort HTTP/HTTPS cho
  ingress ứng dụng (`30426`, `32727`). Dashboard Traefik chưa được bật/expose,
  nên không có URL dashboard NodePort để đăng nhập.

Áp dụng đúng values cho ba công cụ ClusterIP mà không làm thay đổi mật khẩu
Grafana hiện có:

```bash
cd ~/GoshopX

export GRAFANA_ADMIN_PASSWORD="$(kubectl -n monitoring get secret kube-prometheus-stack-grafana \
  -o jsonpath='{.data.admin-password}' | base64 -d)"

helm upgrade --install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --values ops/observability/kube-prometheus-stack-values-lab.yaml \
  --set grafana.adminPassword="$GRAFANA_ADMIN_PASSWORD" \
  --wait --timeout 10m

helm upgrade --install loki grafana/loki \
  --namespace logging \
  --values ops/observability/loki-values-lab.yaml \
  --wait --timeout 10m

unset GRAFANA_ADMIN_PASSWORD

kubectl -n monitoring get svc kube-prometheus-stack-grafana kube-prometheus-stack-prometheus
kubectl -n logging get svc loki-gateway
```

Kết quả kỳ vọng là Grafana `80:30702/TCP`, Prometheus `9090:30701/TCP` và Loki
gateway `80:30703/TCP`.

Traefik dashboard cần được cấu hình riêng trong Helm release K3s, không nên mở
tự động ra Internet. Kiểm tra trước xem dashboard/API có được bật hay không:

```bash
kubectl -n kube-system get deploy traefik \
  -o jsonpath='{.spec.template.spec.containers[0].args}'
kubectl -n kube-system get svc traefik -o wide
```

Nếu cần dashboard Traefik, hãy dùng port-forward tạm thời sau khi xác nhận
entrypoint dashboard trong Helm values; không tái sử dụng `30426`/`32727` cho
dashboard vì đó là ingress khách hàng trên cổng 80/443.

## 4. Elasticsearch có tồn tại không?

Có. Trong chart, Elasticsearch OSS `6.2.4` được triển khai dưới tên
`goshopx/product-db`, lắng nghe cổng `9200`; tên này gây dễ nhầm với PostgreSQL
nhưng đây là kho catalog/search của Product. Kibana dùng biến
`ELASTICSEARCH_URL=http://product-db:9200` và là UI để xem Elasticsearch.

Kiểm tra an toàn từ trong cluster:

```bash
kubectl -n goshopx get deploy product-db kibana
kubectl -n goshopx get svc product-db kibana

kubectl -n goshopx run elastic-check --rm -i --restart=Never \
  --image=curlimages/curl:8.10.1 -- \
  curl -fsS http://product-db:9200/_cat/indices?v
```

Elasticsearch không có NodePort trong lab hiện tại. Không expose `9200` công
khai; dùng Kibana `30601`, hoặc port-forward tạm thời khi cần debug:

```bash
kubectl -n goshopx port-forward service/product-db 19200:9200
```

## 5. Mức độ kết nối và quan sát hiện tại

| Liên kết | Trạng thái từ cấu hình | Cách kiểm chứng |
| --- | --- | --- |
| Web -> Traefik -> Kong -> GraphQL | Là luồng public được thiết kế. | Mở `https://goshopx.click`, sau đó kiểm tra `kubectl -n goshopx logs deploy/kong` và `deploy/graphql`. |
| GraphQL -> dịch vụ nghiệp vụ | GraphQL là boundary public; các Service gRPC dùng DNS nội bộ. | `kubectl -n goshopx get endpoints graphql product account order payment cart inventory`. |
| Dịch vụ -> Kafka | Các topic/bootstrap server được cấp qua ConfigMap/runtime env. | Kafka UI: kiểm tra topics và consumer groups; kiểm tra log service. |
| Product -> Elasticsearch | `PRODUCT_DATABASE_URL` trỏ `product-db:9200`; Product seed/index catalog. | Chạy `elastic-check` ở mục 4 và gọi catalog GraphQL. |
| Alloy -> Loki | Đã cấu hình discovery pod Kubernetes và push vào `loki-gateway`. | Grafana -> Explore -> chọn datasource Loki, query `{namespace="goshopx"}`. |
| Grafana -> Prometheus/Loki | Values thêm datasource Loki; Prometheus stack cung cấp metrics cluster. | Grafana -> Connections/Data sources; Prometheus query `up`. |
| Consul | Chỉ đồng bộ catalog một chiều K8s -> Consul. `connectInject: false`. | Consul UI -> Services; không dùng Consul cho traffic ứng dụng. |
| Jaeger/OpenTelemetry | Jaeger collector/UI có triển khai tại 4317/4318/16686. Chart hiện không cấp `OTEL_EXPORTER_OTLP_ENDPOINT` cho application services và không có exporter toàn dịch vụ được xác nhận. | Jaeger UI trống là bình thường. Chỉ kết luận trace hoạt động sau khi tạo request và thấy trace trong UI. |
| Prometheus metrics của ứng dụng | kube-prometheus-stack đang có cho metrics cluster. Source hiện không có `ServiceMonitor`/`PodMonitor` cho các service GoshopX. | Prometheus -> Status -> Targets; chỉ các target `UP` mới được scrape. Không coi app metrics đã tích hợp nếu chưa có target tương ứng. |

Vì vậy, không thể khẳng định tất cả công cụ đang “sync/call toàn bộ API”:

1. Công cụ hạ tầng đã được triển khai và phần lớn đã được nối qua Kubernetes
   Service/DNS.
2. Loki log collection có cấu hình rõ ràng qua Alloy.
3. Consul không phải service mesh trong lab này.
4. Jaeger và Prometheus application-level observability cần thêm instrumentation
   và ServiceMonitor/PodMonitor trước khi có trace/metric đầy đủ theo từng API.

## 6. Cách dùng nhanh từng giao diện

### Grafana, Prometheus và Loki

1. Đăng nhập Grafana bằng `admin` và mật khẩu Secret.
2. Vào **Explore**; chọn **Loki**, chạy `{namespace="goshopx"}` để tìm log.
3. Chọn **Prometheus**, chạy `up` để thấy target đang scrape.
4. Trong Prometheus UI, vào **Status -> Targets** để chẩn đoán target down.
5. Loki NodePort chỉ phù hợp cho API/debug; không có màn hình dashboard riêng.

### Argo CD

1. Mở URL Argo CD HTTPS, chấp nhận cảnh báo chứng chỉ chỉ khi đang dùng direct
   IP trong lab và đã giới hạn firewall.
2. Đăng nhập `admin`.
3. Mở application `goshopx-lab`; kiểm tra **Sync Status**, **Health**, Events
   và pod bị Degraded trước khi restart workload.

### Consul, Kafka UI và RedisInsight

1. Consul: chỉ xem Service catalog và tình trạng đăng ký. Cảnh báo topology
   unknown là bình thường khi không bật ACL intentions/Connect mesh.
2. Kafka UI: kiểm tra Topics, Consumers, consumer lag và messages; lab đang
   readonly nên không chỉnh topic/xóa message qua UI.
3. RedisInsight: tạo database connection tới host `redis`, port `6379` từ trong
   cluster, hoặc dùng SSH tunnel/port-forward; không expose Redis public chỉ để
   dùng UI.

### Kibana, Attu, Jaeger, Kong và GraphQL

1. Kibana: vào **Management -> Index Patterns**, tạo pattern `catalog*` hoặc
   index thực tế từ `_cat/indices`; đây là dữ liệu catalog Elasticsearch.
2. Attu: xem Milvus collection, schema và vector count. Không xóa collection
   trên môi trường đang dùng.
3. Jaeger: chọn service và time range; nếu trống, xác minh OTEL exporter trước,
   không coi đó là lỗi của UI.
4. Kong Manager/Admin: chỉ dùng từ IP allowlist; thay đổi route/plugin có thể
   làm website lỗi. Với DB-less, cấu hình mong muốn phải được lưu trong Git/Helm
   rồi đồng bộ, không chỉnh tay để tránh drift.
5. GraphQL: kiểm tra endpoint bằng query read-only:

```bash
curl -sS http://VM_PUBLIC_IP:30712/graphql \
  -H 'Content-Type: application/json' \
  --data '{"query":"query { product(pagination: { skip: 0, take: 3 }) { id name price } }"}'
```

## 7. Checklist sau mỗi thay đổi

```bash
kubectl get svc -A
kubectl -n goshopx get pods
kubectl -n argocd get application goshopx-lab
kubectl -n monitoring get pods
kubectl -n logging get pods
```

Chỉ đánh dấu hoàn tất khi:

- Service có đúng `NodePort` mong muốn;
- firewall GCP chỉ cho phép IP người vận hành;
- URL/port thực tế trả phản hồi;
- application `goshopx-lab` là `Synced` và `Healthy`;
- dashboard cho thấy dữ liệu/target/trace tương ứng, thay vì chỉ đăng nhập được.
