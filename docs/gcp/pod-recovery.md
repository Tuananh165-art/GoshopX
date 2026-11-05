# Khôi phục pod trên GKE

Runbook này xử lý các lỗi đã thấy trong namespace `goshopx`: Kafka không khởi động vì `lost+found` nằm trực tiếp trong thư mục log, MinIO image không tải được, Milvus không có object storage, GraphQL upload ảnh đang trỏ vào MinIO và Kibana bị OOMKilled.

## 1. Xác nhận kết nối đến cluster

Mở PowerShell mới, lấy endpoint GKE rồi thêm endpoint vào `NO_PROXY`. Không xóa cấu hình proxy toàn máy:

```powershell
Set-Location F:\GoshopX\infra\gcp
$Project = terraform output -raw project_id
$Zone = terraform output -raw zone
$Endpoint = gcloud container clusters describe goshopx-lab --zone $Zone --project $Project --format='value(endpoint)'
$env:NO_PROXY = (@($env:NO_PROXY, $env:no_proxy, $Endpoint) | Where-Object { $_ } | Select-Object -Unique) -join ','
$env:no_proxy = $env:NO_PROXY
gcloud container clusters get-credentials goshopx-lab --zone $Zone --project $Project
kubectl get nodes
```

Nếu endpoint vẫn đi qua `127.0.0.1:9`, kiểm tra `Get-ChildItem Env:*PROXY*` và thêm endpoint/cluster IP vào `NO_PROXY` theo chính sách mạng máy bạn. Đừng đưa credential vào log.

## 2. Tạo bucket riêng tư trước khi phát hành ứng dụng

Thay đổi Terraform tạo hai bucket riêng tư, quyền bucket-level cho runtime identity và output tương ứng. Chúng không chứa dữ liệu legacy; bucket Milvus mới có thể cần tái index catalog.

```powershell
Set-Location F:\GoshopX\infra\gcp
terraform fmt -check -recursive
terraform validate
terraform plan -out goshopx-storage-fix.tfplan
terraform show -no-color goshopx-storage-fix.tfplan
```

Kiểm tra plan chỉ tạo bucket `*-milvus`, `*-media`, ba IAM binding runtime theo bucket và cập nhật output; không được xóa database, cluster, bucket hiện hữu hay project. Xác nhận credit, thời hạn lab và SKU hiện tại rồi mới apply:

```powershell
terraform apply goshopx-storage-fix.tfplan
$MediaBucket = terraform output -raw media_bucket
$MilvusBucket = terraform output -raw milvus_bucket
```

Bucket ảnh bật Public Access Prevention `enforced`. Runtime GSA có quyền đọc/ghi trong bucket để GraphQL phục vụ ảnh qua `/media/...`; không cấp IAM cho `allUsers`. Milvus truy cập GCS bằng Workload Identity, không dùng service-account JSON key.

## 3. Đồng bộ tên bucket vào Helm

```powershell
Set-Location F:\GoshopX
$MediaBucket = terraform -chdir=infra/gcp output -raw media_bucket
$MilvusBucket = terraform -chdir=infra/gcp output -raw milvus_bucket
$Path = 'deploy/helm/goshopx/values-gcp-lab.yaml'
$Yaml = [IO.File]::ReadAllText((Join-Path (Get-Location) $Path))
$Yaml = $Yaml.Replace('mediaBucket: "generated-mote-467410-b0-media"', "mediaBucket: `"$MediaBucket`"")
$Yaml = $Yaml.Replace('milvusBucket: "generated-mote-467410-b0-milvus"', "milvusBucket: `"$MilvusBucket`"")
$Yaml = $Yaml.Replace('GCS_MEDIA_BUCKET: "generated-mote-467410-b0-media"', "GCS_MEDIA_BUCKET: `"$MediaBucket`"")
[IO.File]::WriteAllText((Join-Path (Get-Location) $Path), $Yaml, [Text.UTF8Encoding]::new($false))
helm lint deploy/helm/goshopx -f deploy/helm/goshopx/values.yaml -f $Path
helm template goshopx deploy/helm/goshopx -n goshopx -f deploy/helm/goshopx/values.yaml -f $Path > (Join-Path $env:TEMP 'goshopx-gcp-render.yaml')
```

Trong render cần thấy `KAFKA_LOG_DIRS=/tmp/kafka-logs/data`, `MINIO_CLOUD_PROVIDER=gcp`, `MINIO_USE_IAM=true`, bucket Milvus mới, `GCS_MEDIA_BUCKET` đúng bucket, `serviceAccountName: goshopx-runtime` trên GraphQL/Milvus, và giới hạn Kibana `1Gi`. Không được có Deployment/Service tên `minio` hoặc `milvus-minio`; PVC cũ được giữ lại để Helm không xóa persistent disk.

## 4. Phát hành qua GitOps

Rà soát source, đặc biệt GraphQL media route và Terraform plan, rồi commit/push thay đổi lên `main` theo quy trình repo. `release-gcp.yml` phải hoàn tất quality/security/build/scan và tạo PR cập nhật image digest. Review/merge PR promotion; Argo CD là thành phần triển khai chart.

```powershell
git status --short
git diff --check
git diff
```

Sau khi merge promotion, đảm bảo Argo có `Synced` và `Healthy`, rồi theo dõi:

```powershell
kubectl get application goshopx-gcp-lab -n argocd -o wide
kubectl get pods,pvc,svc -n goshopx -o wide
kubectl rollout status deployment/kafka -n goshopx --timeout=5m
kubectl rollout status deployment/milvus -n goshopx --timeout=10m
kubectl rollout status deployment/graphql -n goshopx --timeout=5m
kubectl rollout status deployment/kibana -n goshopx --timeout=5m
```

## 5. Kiểm tra phụ thuộc sau rollout

Kafka PVC có thể chứa `lost+found` ở gốc. Broker mới dùng thư mục con `data`, vì vậy không xóa PVC hoặc chạy `kubectl delete pvc`:

```powershell
kubectl logs deployment/kafka -n goshopx --tail=100
kubectl exec deployment/kafka -n goshopx -- /opt/kafka/bin/kafka-topics.sh --bootstrap-server kafka:9092 --list
kubectl logs deployment/milvus -n goshopx --tail=150
kubectl logs deployment/recommender-sync -n goshopx --tail=100
kubectl logs deployment/notification -n goshopx --tail=100
```

Xác nhận không còn `ImagePullBackOff`, `CrashLoopBackOff`, `OOMKilled`, Kafka `lost+found` error hoặc lỗi Milvus/GCS authorization. `kubectl get pods` phải cho thấy các thành phần đã bật ở trạng thái `Running`/`Ready`; các Job đã hoàn tất có thể hiện `Completed`.

### Kiểm tra GraphQL media

GraphQL upload kiểm tra extension và magic bytes JPEG/PNG/WebP. Sau khi upload ảnh bằng tài khoản admin, URL phải bắt đầu `/media/products/`. Mở URL qua domain GKE; response phải là ảnh với `X-Content-Type-Options: nosniff`. Bucket vẫn private:

```powershell
gcloud storage buckets describe "gs://$MediaBucket" --format="value(iamConfiguration.publicAccessPrevention)"
```

Kết quả mong đợi là `enforced`. Không dùng URL GCS trực tiếp làm image URL; app route proxy đọc từ bucket bằng Workload Identity.

### Quan sát Kibana và thương mại

Kibana limit đã tăng lên `1Gi`. Nếu tiếp tục OOM, kiểm tra `kubectl describe pod` và node memory trước khi tăng thêm tài nguyên; đừng vượt quota/budget một cách mù quáng. Sau khi các phụ thuộc ổn định, chạy nghiệm thu đăng nhập, catalog, cart/reservation, COD, VNPay sandbox, notification, admin, recommender text/image, trace/log/metrics theo [deploy-step-by-step.md](deploy-step-by-step.md) và [load-testing.md](../load-testing.md).

## Trạng thái xác minh

Helm lint/render và unit tests cục bộ là gate trước khi commit; chúng không chứng minh IAM GCP, GCS upload, Milvus readiness hoặc live GKE rollout. Chỉ đánh dấu hoàn tất sau khi plan/apply mới, GitHub release, Argo sync, pod readiness, Kafka/Milvus smoke, GraphQL upload/read, commerce E2E và domain HTTPS đều có bằng chứng chạy thật. Giữ thời gian lab trong giới hạn credit đã xác minh và cleanup trước giờ hết hạn.
