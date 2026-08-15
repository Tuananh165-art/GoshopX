# Quan sát ứng dụng GoshopX: Prometheus và Jaeger

Tài liệu này mô tả phần quan sát ở mức ứng dụng. Nó bổ sung cho
[runbook NodePort](./24-operator-dashboard-nodeport-runbook.md), không thay đổi
luồng public đã có:

`Browser -> Traefik -> Kong -> GraphQL -> gRPC service`.

## Phạm vi đã triển khai

- Các service Go `graphql`, `account`, `product`, `order`, `payment`,
  `inventory`, `cart`, `notification` và `admin` mở endpoint `/metrics` nội bộ
  trên cổng `9090`.
- Helm tạo một `ServiceMonitor` cho từng service Go đang bật. Các monitor mang
  nhãn `release: kube-prometheus-stack` để Prometheus của lab phát hiện chúng.
- GraphQL có HTTP request metric; các gRPC server có metric request/duration;
  runtime Go/process metrics cũng xuất hiện tại endpoint này.
- Khi `OTEL_EXPORTER_OTLP_ENDPOINT` có giá trị, các service Go export span
  OTLP/gRPC tới Jaeger. Trace context W3C được truyền qua các gRPC client nội bộ.
- Overlay lab đặt endpoint là `jaeger:4317`; để tắt trace không xóa code, đặt
  endpoint rỗng trong values của môi trường đó.

Không expose `/metrics`, OTLP hay database trực tiếp ra Internet. Chỉ dashboard
operator mới có NodePort và firewall phải giới hạn theo IP của người vận hành.

## Phần chưa thể tuyên bố hoàn tất

`recommender-server`, `recommender-sync` và `recommender-train` là Python.
Chúng chưa có OpenTelemetry SDK/instrumentation trong `recommender/uv.lock`, nên
không được gắn ServiceMonitor và chưa phát span server-side trong thay đổi này.
Các span của Go client gọi recommender vẫn có thể xuất hiện trong Jaeger. Cần một
thay đổi riêng có `uv lock` hợp lệ và test runtime Python trước khi bật trace đầy
đủ cho ba workload này.

Consul vẫn chỉ sync K8s catalog; `connectInject` đang tắt nên Consul không phải
service mesh và không tạo trace lưu lượng. Loki là log API; xem log qua Grafana,
không phải UI độc lập.

## Triển khai trên VPS sau khi CI xanh

Push commit này để CI build image mới, chờ job `Build, scan and promote GitOps
image` thành công, sau đó trên VPS:

```bash
cd ~/GoshopX
git pull --ff-only origin main

# Đồng bộ manifest ứng dụng từ GitOps và kiểm tra rollout.
kubectl -n argocd annotate application goshopx-lab \
  argocd.argoproj.io/refresh=hard --overwrite
kubectl -n goshopx rollout status deployment/graphql --timeout=10m
kubectl -n goshopx rollout status deployment/product --timeout=10m
kubectl -n goshopx get servicemonitor
```

Để áp dụng ba dashboard hiện còn `ClusterIP` thành NodePort, dùng Helm với
values trong repo. Giữ nguyên password Grafana đang tồn tại:

```bash
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
kubectl -n monitoring get svc kube-prometheus-stack-prometheus kube-prometheus-stack-grafana
kubectl -n logging get svc loki-gateway
```

Kết quả mong đợi là Prometheus `30701`, Grafana `30702`, Loki gateway `30703`.
Xác nhận cả GCP firewall chỉ allow IP public của máy đang mở browser trước khi
truy cập các NodePort này.

## Xác minh có bằng chứng

### Prometheus

1. Mở `http://VM_PUBLIC_IP:30701/targets`.
2. Tìm các target tên `graphql`, `product`, `account`, … với state `UP`.
3. Chạy PromQL:

```promql
sum by (service, code) (rate(goshopx_grpc_requests_total[5m]))
```

Sau một request GraphQL, GraphQL có thêm HTTP metric:

```promql
sum by (route, status) (rate(goshopx_http_requests_total{service="graphql"}[5m]))
```

### Jaeger

1. Mở `http://VM_PUBLIC_IP:30713`.
2. Chọn `goshopx-graphql`, khoảng thời gian `Last 15 minutes`.
3. Gọi một query đọc catalog từ website hoặc lệnh sau:

```bash
curl -skS https://goshopx.click/graphql \
  -H 'Content-Type: application/json' \
  --data '{"query":"query { product(pagination: { skip: 0, take: 3 }) { id name } }"}'
```

4. Trace phải có span GraphQL và các gRPC span downstream nếu service đích đang
   sẵn sàng. Nếu Jaeger trống, kiểm tra trước pod/Service và endpoint thay vì
   sửa dashboard:

```bash
kubectl -n goshopx get svc jaeger graphql product
kubectl -n goshopx exec deployment/graphql -- printenv OTEL_EXPORTER_OTLP_ENDPOINT
kubectl -n goshopx logs deployment/graphql --since=10m
```

### Loki/Grafana

Grafana là UI duy nhất để correlate metrics/logs. Vào **Explore**, chọn Loki và
chạy `{namespace="goshopx"}`; sau đó chọn Prometheus để xem metric cùng thời
điểm. Loki gateway `30703` chỉ dành cho API/debug, không phải màn hình dashboard.
