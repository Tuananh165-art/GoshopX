# Hướng dẫn triển khai GoshopX lên Ubuntu K3s

Tài liệu này là quy trình vận hành cho một VPS Ubuntu 22.04/24.04, 8 GB RAM,
50 GB disk. Nó dùng các script trong `scripts/ubuntu/`; không thay thế GitOps
và không ghi secret vào Git.

## 1. Kiến trúc đích và giới hạn

```text
Internet -> Traefik (K3s Ingress) -> Kong proxy -> Web / GraphQL / payment webhook
                                                -> gRPC services (ClusterIP only)
                                                -> Kafka, Redis, PostgreSQL, Elasticsearch, MinIO, Milvus
GitHub Actions -> GHCR immutable SHA -> Git values -> Argo CD -> K3s
```

Kong là API gateway; Traefik chỉ là ingress controller. Browser chỉ gọi Kong
vào Web/GraphQL. gRPC, PostgreSQL, Kafka, Redis, Elasticsearch, MinIO và Milvus
không được expose public. Kong Admin/Manager và toàn bộ dashboard kỹ thuật là
`ClusterIP` mặc định; dùng `port-forward` cho operator.

Một node 8 GB không có HA. Triển khai theo phase:

| Phase | Bật | Không bật ngay |
| --- | --- | --- |
| 0 | K3s, Kong, Web, GraphQL, core service và backing services bắt buộc | Milvus, recommender, admin, Kibana, dashboard UI |
| 1 | Argo CD, Consul catalog sync | Consul Connect sidecar/mesh |
| 2 | Prometheus, Grafana, Loki/Alloy | retention dài, alertmanager HA |
| 3 | k6 theo job | k6 chạy liên tục |

## 2. Chuẩn bị DNS, firewall và repository

1. Tạo DNS A record `<APP_HOST>` trỏ đến IP VPS.
2. Firewall/security group chỉ cho phép TCP `22`, `80`, `443`. Không mở `6443`,
   NodePort range, Kong `8001/8002`, Grafana, Prometheus hay Argo CD.
3. Đăng nhập VPS bằng user có `sudo`, clone repository rồi vào thư mục checkout.

```bash
git clone https://github.com/OWNER/GoshopX.git
cd GoshopX
chmod +x scripts/ubuntu/*.sh scripts/lab/*.sh
./scripts/ubuntu/00-host-prerequisites.sh
```

Không chạy script bằng `sudo`; script chỉ gọi `sudo` cho thao tác hệ thống cần
thiết. Kiểm tra `free -h` và `df -h` trước mỗi phase.

## 3. Cài K3s và CLI

Chọn một version K3s đã review; script yêu cầu biến này để tránh cài “latest”
không kiểm soát.

```bash
export K3S_VERSION='v1.31.4+k3s1'
./scripts/ubuntu/01-install-k3s.sh
export HELM_VERSION='v3.16.4'
export ARGOCD_VERSION='v2.13.3'
./scripts/ubuntu/02-install-cli-tools.sh
kubectl get nodes
helm version --short
```

K3s được cài single-server, giữ Traefik, tắt `servicelb` và `metrics-server`,
đặt image garbage collection 70/55. `~/.kube/config` là credential quản trị;
không commit hoặc đưa file này vào GitHub Actions.

## 4. Cấu hình GitOps trước deploy

Sửa có review các placeholder trong:

- `deploy/helm/goshopx/values-lab.yaml`: `global.imageRegistry`, `image.tag`,
  `global.ingress.host`, các service enable theo phase.
- `ops/gitops/argocd/applications/goshopx-lab.yaml`: repository URL và GHCR owner.

`image.tag` phải là commit SHA đã tồn tại trong GHCR; không dùng `latest`.
Trong GitHub repo, tạo variable `GHCR_OWNER`, bật GitHub Environment `lab` có
reviewer, bảo vệ branch `main`, và yêu cầu CI quality gates pass.

## 5. Tạo secret runtime ngoài Git

Tạo file ở vị trí an toàn ngoài checkout, chẳng hạn `/opt/goshopx/secrets.env`:

```bash
sudo install -d -m 0700 /opt/goshopx
sudo cp scripts/ubuntu/goshopx-secrets.env.example /opt/goshopx/secrets.env
sudo chown "$USER":"$USER" /opt/goshopx/secrets.env
chmod 600 /opt/goshopx/secrets.env
nano /opt/goshopx/secrets.env
./scripts/ubuntu/03-create-runtime-secret.sh /opt/goshopx/secrets.env
```

Điền giá trị thật, không giữ literal `replace`. Mỗi service owner cần URL
database riêng. Với infrastructure nội bộ chart, hostname lần lượt là
`account-db`, `order-db`, `payment-db`, `inventory-db`, `notification-db`,
`admin-db`, `recommender-db`; Product dùng `http://product-db:9200`.

## 6. Bootstrap Argo CD, Consul và observability

```bash
export GRAFANA_ADMIN_PASSWORD='set-a-unique-password-in-your-shell-only'
export GITOPS_REPO_URL='https://github.com/OWNER/GoshopX.git'
export GHCR_IMAGE_REGISTRY='ghcr.io/OWNER'
./scripts/ubuntu/04-bootstrap-platform.sh
unset GRAFANA_ADMIN_PASSWORD
```

Argo CD, Consul single-server/catalog sync, Prometheus, Grafana, Loki và Alloy
được cài vào namespace riêng. Không coi việc Helm `--wait` pass là xác nhận DNS,
TLS hoặc public reachability.

## 7. Build và publish images

Đường chuẩn: push vào `main`; GitHub Actions test, scan, build các image và
promote SHA vào Git. Không cấp kubeconfig cho CI.

Chỉ khi cần build thủ công trên builder tin cậy:

```bash
export GHCR_IMAGE_REGISTRY='ghcr.io/OWNER'
export IMAGE_TAG="$(git rev-parse HEAD)"
export REGISTRY_USERNAME='github-user-or-bot'
read -rsp 'GHCR token: ' REGISTRY_TOKEN; echo
./scripts/ubuntu/08-build-and-push-images.sh
unset REGISTRY_TOKEN
```

Sau đó commit/push image tag vào `values-lab.yaml` để Argo CD nhìn thấy desired
state. Không dùng manual Helm deploy như đường release thường xuyên sau GitOps
bootstrap.

## 8. Deploy ứng dụng lần đầu

Chạy profile nhẹ trước. `ENABLE_INFRA=true` deploy dependency image profile và
7 PostgreSQL PVC 2 GiB; chỉ bật sau khi kiểm tra headroom.

```bash
export GHCR_IMAGE_REGISTRY='ghcr.io/OWNER'
export IMAGE_TAG='COMMIT_SHA_FROM_GHCR'
export INGRESS_HOST='<APP_HOST>'
export TLS_SECRET_NAME='<EXISTING_TLS_SECRET>'
export ENABLE_INFRA=false
./scripts/ubuntu/05-deploy-application.sh
```

Khi backing services đã sẵn sàng và measured capacity đủ, dùng
`ENABLE_INFRA=true`. Trước khi bật Milvus/recommender/admin/Kibana, set các
service tương ứng `enabled: true` trong `values-lab.yaml`, commit để Argo CD
reconcile. Không bật NodePort dashboard trừ lab có firewall allowlist:
`ENABLE_NODEPORT_DASHBOARDS=true`.

## 9. Verify và vận hành

```bash
./scripts/ubuntu/06-verify-release.sh
kubectl -n goshopx get pods,svc
kubectl -n argocd get applications.argoproj.io goshopx-lab
kubectl -n goshopx port-forward svc/kong-manager 8002:8002
```

Kiểm tra từ máy operator: `https://<APP_HOST>/health`, `POST /graphql`, Argo app
status `Synced/Healthy`, Prometheus targets, Grafana Loki datasource và k6
smoke test. Payment webhook chỉ được test bằng event sandbox đã ký hợp lệ.

## 10. Rollback và sự cố thường gặp

Rollback chuẩn là revert commit promotion GitOps và chờ Argo sync. Chỉ khi cần
khôi phục khẩn cấp:

```bash
helm history goshopx -n goshopx
./scripts/ubuntu/07-rollback-application.sh REVISION
```

- `ImagePullBackOff`: kiểm tra image SHA có trong GHCR và quyền pull/imagePullSecret.
- `Pending` PVC: kiểm tra `kubectl get storageclass,pvc -A`; K3s cần `local-path`.
- Elasticsearch crash: kiểm tra `vm.max_map_count` của host và RAM; không bật
  cùng Milvus/recommender trên node khi thiếu headroom.
- Kong 502: kiểm tra `kubectl -n goshopx get endpoints kong-proxy graphql payment`;
  Product/Payment 8081 là internal HTTP, không phải public gRPC.

## Trạng thái verification

Scripts đã được kiểm tra cú pháp Bash; chart đã Helm lint/render. K3s install,
DNS, TLS, GHCR permissions, image pull, Pod readiness và HTTP public phải được
kiểm tra lại trên VPS thực tế.
