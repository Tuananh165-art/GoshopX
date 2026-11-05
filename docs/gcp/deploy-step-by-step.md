# GoshopX GKE lab — triển khai từng bước

Hướng dẫn này dành cho Windows PowerShell và repo `F:\GoshopX`. Nó đưa stack từ kiểm tra trước triển khai tới domain, smoke/load test và teardown. Đây là lab một vùng, hai node tối đa; không xem kết quả là production HA.

> **Trạng thái hiện tại:** Terraform/Helm source và tài liệu đã có, Terraform `fmt`/`validate`, test Go/Python/Web local đã chạy. Chưa có GCP resource nào được tạo, chưa có image lên Artifact Registry, chưa chạy Helm trên cluster và chưa đổi Cloudflare DNS. Terraform plan cần ADC; Secret Manager Secret Sync, đầy đủ secret versions/DB users và một số vận hành catalog còn là gate. Hãy đi đúng thứ tự bên dưới, dừng ở bất kỳ gate nào chưa đạt.

## 0. Điều kiện dừng trước khi bắt đầu

- Kiểm tra lại Billing → Credits ngay trước lab. Ảnh đã gửi trước đây không chứng minh credit còn hạn hiện tại. Ghi lại thời điểm T0, số credit và hạn hết hạn.
- Tổng dự toán 48 giờ cộng thời gian dọn dẹp phải dưới **₫2.500.000**. Billing budget trong Terraform chỉ cảnh báo, không khóa chi tiêu. Dừng nếu hạn credit không còn đủ 48 giờ cộng 12 giờ dự phòng.
- Không tiếp tục nếu chưa xác nhận billing, quyền bật API/tạo GKE/Cloud SQL/IAM/WIF trong project hiện có hoặc quota vùng không đủ.
- Không dán `.env`, mật khẩu, API key, billing export hoặc Terraform state vào Git, issue, chat hay Actions log. Không dùng `.env` làm Terraform variable/plan.
- Terraform target project hiện có `generated-mote-467410-b0`; không tạo, đổi tên, gắn nhãn hoặc xóa project này. Chỉ tạo resource có tên GoshopX riêng và rà soát chúng không trùng tài nguyên hiện hữu.

## 1. Công cụ và baseline

Mở PowerShell tại repo:

```powershell
Set-Location F:\GoshopX
git status --short
git rev-parse HEAD
terraform version
gcloud version
kubectl version --client
gke-gcloud-auth-plugin --version
helm version
k6 version
go version
uv --version
node --version
npm --version
```

Terraform phải thỏa `>= 1.10, < 2.0`; Python phải là 3.11; Node phải là 22.22.2+ nhưng vẫn dưới 23 (xem `web/.nvmrc` và `web/package.json`). Nếu dùng nvm-windows, cài/chuyển theo version `web/.nvmrc` (hiện 22.23.4) rồi xác nhận `node --version`. Nếu không dùng nvm, tải Windows x64 MSI tương ứng từ [Node.js official archive](https://nodejs.org/en/download/archive/v22.23.4). Node 24.11 chạy được test/build trong lần kiểm tra này nhưng không nằm trong engine range của jsdom đang khóa. Go phải theo `.github/workflows/tests.yml`. Nếu thiếu Terraform, tải bản Windows AMD64 từ [trang cài Terraform chính thức](https://developer.hashicorp.com/terraform/install), đối chiếu checksum công bố, đặt `terraform.exe` trong thư mục nằm trên `PATH`, rồi mở PowerShell mới và kiểm tra lại `terraform version`.

Kiểm tra local trước khi cấp hạ tầng:

```powershell
Push-Location F:\GoshopX
$env:PYTHONPATH = Join-Path $PWD 'recommender\app'
Push-Location recommender
if (Test-Path '.venv-security\Scripts\python.exe') {
  & .venv-security\Scripts\python.exe -m unittest discover -s tests -v
} else {
  # Isolate from a stale/corrupt .venv and an inaccessible global uv cache.
  $env:UV_PROJECT_ENVIRONMENT = Join-Path $PWD '.venv-gcp-lab'
  $env:UV_CACHE_DIR = Join-Path $env:TEMP 'goshopx-uv-cache'
  uv sync --locked
  if ($LASTEXITCODE -ne 0) { throw 'uv sync failed; do not remove the existing .venv blindly.' }
  uv run --locked python -m unittest discover -s tests -v
}
Pop-Location
go test (go list ./... | Where-Object { $_ -notmatch '/tests/e2e$' })
go vet (go list ./... | Where-Object { $_ -notmatch '/tests/e2e$' })
Push-Location web
$NodeExpected = 'v' + (Get-Content .nvmrc -Raw).Trim()
if ((node --version) -ne $NodeExpected) { Write-Warning "Expected $NodeExpected from web/.nvmrc; switch Node before npm ci." }
npm ci
npm audit
npm test -- --run
npm run build
Pop-Location
Pop-Location
```

E2E `tests/e2e` cần cả stack đang chạy, vì vậy không chạy bằng `go test ./...` khi chưa dựng môi trường. Nếu `uv run` báo không xóa được `recommender\.venv\lib64`, đây là virtualenv cũ có symlink Linux không dùng được trên Windows; dùng `.venv-security` nếu có hoặc nhánh virtualenv/cache riêng phía trên. Không xóa/chown/đổi ACL cho `.venv` hay cache dùng chung mù quáng. `npm ci` từng in 6 vulnerability trong một lần chạy; lần kiểm tra `npm audit --json` kế tiếp trên cùng lockfile báo 0. Chạy `npm audit` sau cài đặt Node 22 để lấy trạng thái hiện tại; không chạy `npm audit fix --force` tự động.

## 2. Đăng nhập và xác nhận billing/quota

```powershell
gcloud auth login
gcloud auth application-default login
gcloud auth list
gcloud auth application-default print-access-token | Out-Null
gcloud billing accounts list
gcloud projects describe generated-mote-467410-b0
gcloud compute regions describe asia-southeast1
```

`gcloud auth application-default login` tạo ADC dùng bởi Terraform. Không in access token ra terminal/log. Trong Console, xác nhận credit, billing currency VND, quyền và quota. Dùng Pricing Calculator để ước tính node `e2-standard-4`, balanced PD, Cloud SQL `db-custom-1-3840`, SSD, NAT, IP/load balancer, GKE, logging/monitoring, Registry, storage và lưu lượng egress; lưu kết quả riêng ngoài Git.

Xác nhận Terraform đang nhắm đúng project hiện hữu và billing liên kết:

```powershell
gcloud projects describe generated-mote-467410-b0
gcloud billing projects describe generated-mote-467410-b0
```

Nếu không xem được billing, trạng thái project hoặc quota thì **dừng** để xử lý quyền trước. Kiểm tra Console xem các resource cùng tên `goshopx-lab`, `goshopx-postgres`, bucket `<project-id>-catalog`, `<project-id>-tfstate`, `<project-id>-loki`, Artifact Registry `goshopx`, service accounts, WIF pool, IP và Certificate Manager entries đã tồn tại chưa. Không import hoặc ghi đè resource hiện hữu một cách mù quáng.

## 3. Hoàn tất secret inventory trước Terraform apply

Helm hiện yêu cầu một Kubernetes Secret tên `goshopx-runtime`. Terraform ban đầu đã tạo 8 secret containers; sau lần apply đầu tiên, cấu hình đã được mở rộng để có đủ secret IDs trong bảng dưới (cùng các provider keys). Chạy plan/apply Terraform mới để tạo các secret IDs còn thiếu trước khi thêm versions. Terraform đặt tên theo `goshopx-` + chữ thường + `_` đổi thành `-`.

| Kubernetes Secret key | Secret Manager ID cần có | Giá trị cần chuẩn bị |
| --- | --- | --- |
| `SECRET_KEY` | `goshopx-jwt-secret` | Khóa ký JWT riêng cho lab |
| `MINIO_ACCESS_KEY` | `goshopx-minio-access-key` | User MinIO, khớp cấu hình MinIO |
| `MINIO_SECRET_KEY` | `goshopx-minio-secret-key` | Mật khẩu MinIO, khớp cấu hình MinIO |
| `ACCOUNT_DATABASE_URL` | `goshopx-account-database-url` | Cloud SQL proxy DSN cho DB/user `account` |
| `PRODUCT_DATABASE_URL` | `goshopx-product-database-url` | `http://product-db:9200` (Product dùng Elasticsearch trong cluster, không phải Cloud SQL) |
| `ORDER_DATABASE_URL` | `goshopx-order-database-url` | Cloud SQL proxy DSN cho `order` |
| `PAYMENT_DATABASE_URL` | `goshopx-payment-database-url` | Cloud SQL proxy DSN cho `payment` |
| `INVENTORY_DATABASE_URL` | `goshopx-inventory-database-url` | Cloud SQL proxy DSN cho `inventory` |
| `NOTIFICATION_DATABASE_URL` | `goshopx-notification-database-url` | Cloud SQL proxy DSN cho `notification` |
| `ADMIN_DATABASE_URL` | `goshopx-admin-database-url` | Cloud SQL proxy DSN cho `admin` |
| `RECOMMENDER_DATABASE_URL` | `goshopx-recommender-database-url` | Cloud SQL proxy DSN cho `recommender` |
| `GOOGLE_CLIENT_ID` | `goshopx-google-client-id` | OAuth client ID nếu bật Google login; không phải client secret |
| `VNPAY_TMN_CODE` | `goshopx-vnpay-tmn-code` | Mã sandbox VNPay |
| `VNPAY_HASH_SECRET` | `goshopx-vnpay-hash-secret` | Secret sandbox VNPay |
| `VNPAY_PAYMENT_URL` | `goshopx-vnpay-payment-url` | Endpoint sandbox VNPay |
| `GMAIL_USERNAME` | `goshopx-gmail-username` | SMTP sender; dùng giá trị lab riêng |
| `GMAIL_PASSWORD` | `goshopx-smtp-password` | SMTP credential; resource Terraform đã có |
| `GMAIL_FROM` | `goshopx-gmail-from` | Địa chỉ gửi mail |
| `AI_API_KEY` và `DEEPSEEK_API_KEY` | `goshopx-deepseek-api-key` | Cùng DeepSeek key; hai Kubernetes keys cần cho compatibility hiện tại |
| `GEMINI_API_KEY` | `goshopx-gemini-api-key` | Gemini key dùng cho embedding/vision |

Các ID trong bảng hiện nằm trong Terraform. Vì apply đầu đã hoàn tất, plan kế tiếp dự kiến thêm 15 Secret Manager containers cùng quyền truy cập tương ứng; xác nhận plan không xóa resource nào trước khi apply. Tạo từng secret version trong Secret Manager sau đó; không đưa version value vào file manifest. Với mỗi domain DB, tạo user riêng và quyền riêng. `PRODUCT_DATABASE_URL` không cần thông tin đăng nhập vì chart hiện chưa bật authentication cho Elasticsearch; không dùng cấu hình này ngoài lab.

## 4. Tạo hạ tầng trong project hiện có bằng Terraform

```powershell
Set-Location F:\GoshopX\infra\gcp
Copy-Item terraform.tfvars.example terraform.tfvars
notepad terraform.tfvars
```

Điền billing account ID đang gắn với project, project ID `generated-mote-467410-b0`, domain và repository `Tuananh165-art/GoshopX`. `terraform.tfvars` bị ignore bởi Git; kiểm tra:

```powershell
git check-ignore terraform.tfvars
terraform init -backend=false
terraform fmt -check -recursive
terraform validate
terraform plan -out lab.tfplan
terraform show lab.tfplan
```

Rà soát project ID, danh sách API/resource, vùng/zone, node count, tên resource và cost estimate. `terraform plan` không thể tính bill chắc chắn; đối chiếu calculator và Billing Console. Đảm bảo plan không có thao tác tạo/sửa/xóa resource `google_project` hoặc resource không thuộc GoshopX. Chỉ khi plan đúng và còn trong time/cost gate mới chạy:

```powershell
terraform apply lab.tfplan
terraform output
```

### Khôi phục khi `terraform apply` tạo được một phần rồi báo lỗi

Apply của Terraform có thể tạo thành công một số resource trước khi resource khác lỗi. Giữ nguyên `terraform.tfstate` và các resource đã tạo; không chạy lại saved plan cũ, không xóa state và không `destroy` project. Kiểm tra state và cluster trước:

```powershell
terraform state list
gcloud container clusters list --project generated-mote-467410-b0
kubectl get nodes
```

Sửa lỗi cấu hình, đặt ADC quota project cho Budget API, rồi tạo plan mới. Với user ADC, Google Terraform provider cũng cần `billing_project` cùng `user_project_override = true`; hai thuộc tính này đã được cấu hình trong provider. Chỉ apply plan mới sau khi xác nhận không có resource ngoài dự kiến bị xóa/thay thế:

```powershell
$Project = "generated-mote-467410-b0"
gcloud auth application-default set-quota-project $Project
terraform fmt -recursive
terraform validate
terraform plan -out lab-recovery.tfplan
terraform show lab-recovery.tfplan
terraform apply lab-recovery.tfplan
```

Lỗi SQL `Invalid Tier ... ENTERPRISE_PLUS` được xử lý bằng cách khai báo rõ edition `ENTERPRISE` cho tier `db-custom-1-3840`. Workload Identity IAM binding phải đợi GKE cluster tạo xong; Certificate Manager resources phải đợi API enable xong. Các phụ thuộc này đã được khai báo trong Terraform. Nếu `set-quota-project` báo thiếu quyền, tài khoản cần `serviceusage.services.use` trong project. Không chia sẻ file state, saved plan hoặc `.tfvars` vì chúng có thể chứa metadata nhạy cảm.

Terraform đang dùng local state ở lượt bootstrap đầu. Sau khi tạo bucket state và kiểm tra output, cấu hình remote backend:

```powershell
$TfBucket = terraform output -raw tfstate_bucket
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($TfBucket)) { throw "Could not read the Terraform state bucket from local state." }
gcloud storage buckets describe "gs://$TfBucket"

@'
terraform {
  backend "gcs" {}
}
'@ | Set-Content -LiteralPath backend.tf -Encoding ascii
terraform init -migrate-state -backend-config="bucket=$TfBucket" -backend-config="prefix=goshopx/terraform/state"
terraform state list
```

Xác nhận Terraform hỏi migrate và state remote chứa resource. Không xóa local state cho đến khi xác minh `terraform state list` và GCS backend hoạt động. State/plan chứa metadata nhạy cảm; không upload Actions artifact hay commit.

## 5. Chuẩn bị Cloud SQL và Secret Manager versions

Lấy outputs và kết nối kubectl:

```powershell
$Project = terraform output -raw project_id
$Zone = terraform output -raw zone
$SqlInstance = terraform output -raw sql_connection_name
gcloud container clusters get-credentials goshopx-lab --zone $Zone --project $Project
kubectl get nodes -o wide
gcloud sql databases list --instance goshopx-postgres --project $Project
gcloud secrets list --project $Project
```

Trong Cloud SQL, tạo PostgreSQL user riêng cho `account`, `order`, `payment`, `inventory`, `notification`, `admin`, `recommender`; cấp quyền tối thiểu để service tạo schema/table migration trong đúng DB. Không dùng user `postgres` cho các service. Tạo DB đã được Terraform khai báo. Truy cập Cloud SQL Studio từ Console hoặc dùng quy trình DBA nội bộ; không ghi password vào câu SQL trong file.

Các database URL được thêm bằng lệnh PowerShell ở bước trước. Với các runtime secret còn lại, `infra/gcp/add-runtime-secret-versions.ps1` đọc biến từ `.env`, chỉ in tên secret, bỏ qua version đã bật và xóa file tạm sau upload. Chạy `powershell -ExecutionPolicy Bypass -File F:\GoshopX\infra\gcp\add-runtime-secret-versions.ps1`; nếu script báo `Cần điền giá trị thật`, hoàn thiện biến tương ứng trong `.env` trước rồi chạy lại. Không dùng placeholder cho OAuth, VNPay sandbox, SMTP hoặc AI provider.

## 6. Bật GKE Secret Sync và tạo `goshopx-runtime`

GKE Secret Sync yêu cầu GKE 1.33+ và gcloud 378.0.0+ theo tài liệu Google; kiểm tra cluster version và cập nhật gcloud trước khi tiếp tục ([hướng dẫn chính thức](https://docs.cloud.google.com/secret-manager/docs/sync-k8-secrets)). Bật controller:

```powershell
gcloud components update
gcloud container clusters describe goshopx-lab --zone $Zone --project $Project --format="value(currentMasterVersion)"
gcloud container clusters update goshopx-lab --location $Zone --project $Project --enable-secret-sync
kubectl api-resources --api-group=secret-sync.gke.io
```

Phải thấy resource `SecretSync`; nếu không thấy, dừng và kiểm tra version/feature status, không tự cài một controller khác chồng lên GKE managed controller. Tạo namespace và KSA gắn với GSA runtime:

```powershell
kubectl create namespace goshopx --dry-run=client -o yaml | kubectl apply -f -
$RuntimeGsa = terraform output -raw runtime_service_account
@"
apiVersion: v1
kind: ServiceAccount
metadata:
  name: goshopx-runtime
  namespace: goshopx
  annotations:
    iam.gke.io/gcp-service-account: `"$RuntimeGsa`"
"@ | kubectl apply -f -
```

Thêm IAM Secret Accessor cho KSA theo principal Workload Identity trên từng secret trong inventory:

```powershell
$ProjectNumber = gcloud projects describe $Project --format="value(projectNumber)"
$KsaPrincipal = "principal://iam.googleapis.com/projects/$ProjectNumber/locations/global/workloadIdentityPools/$Project.svc.id.goog/subject/ns/goshopx/sa/goshopx-runtime"
$SecretNames = @(
  'goshopx-jwt-secret','goshopx-minio-access-key','goshopx-minio-secret-key',
  'goshopx-account-database-url','goshopx-product-database-url','goshopx-order-database-url',
  'goshopx-payment-database-url','goshopx-inventory-database-url','goshopx-notification-database-url',
  'goshopx-admin-database-url','goshopx-recommender-database-url','goshopx-google-client-id',
  'goshopx-vnpay-tmn-code','goshopx-vnpay-hash-secret','goshopx-vnpay-payment-url',
  'goshopx-gmail-username','goshopx-smtp-password','goshopx-gmail-from',
  'goshopx-deepseek-api-key','goshopx-gemini-api-key'
)
foreach ($SecretName in $SecretNames) {
  gcloud secrets add-iam-policy-binding $SecretName --project $Project `
    --role roles/secretmanager.secretAccessor --member $KsaPrincipal
  if ($LASTEXITCODE -ne 0) { throw "Secret IAM binding failed: $SecretName" }
}
```

Manifest `infra/gcp/runtime-secret-sync.yaml` đã được chuẩn bị sẵn; manifest chỉ chứa tên resource/key, không chứa secret value. Trước khi apply, xác nhận mọi secret được tham chiếu đều có version `ENABLED`.

```yaml
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
  name: goshopx-runtime-provider
  namespace: goshopx
spec:
  provider: gke
  parameters:
    secrets: |
      - resourceName: "projects/<PROJECT_ID>/secrets/goshopx-jwt-secret/versions/latest"
        path: jwt-secret
      - resourceName: "projects/<PROJECT_ID>/secrets/goshopx-minio-access-key/versions/latest"
        path: minio-access-key
      - resourceName: "projects/<PROJECT_ID>/secrets/goshopx-minio-secret-key/versions/latest"
        path: minio-secret-key
      - resourceName: "projects/<PROJECT_ID>/secrets/goshopx-account-database-url/versions/latest"
        path: account-database-url
      - resourceName: "projects/<PROJECT_ID>/secrets/goshopx-product-database-url/versions/latest"
        path: product-database-url
      - resourceName: "projects/<PROJECT_ID>/secrets/goshopx-order-database-url/versions/latest"
        path: order-database-url
      - resourceName: "projects/<PROJECT_ID>/secrets/goshopx-payment-database-url/versions/latest"
        path: payment-database-url
      - resourceName: "projects/<PROJECT_ID>/secrets/goshopx-inventory-database-url/versions/latest"
        path: inventory-database-url
      - resourceName: "projects/<PROJECT_ID>/secrets/goshopx-notification-database-url/versions/latest"
        path: notification-database-url
      - resourceName: "projects/<PROJECT_ID>/secrets/goshopx-admin-database-url/versions/latest"
        path: admin-database-url
      - resourceName: "projects/<PROJECT_ID>/secrets/goshopx-recommender-database-url/versions/latest"
        path: recommender-database-url
      - resourceName: "projects/<PROJECT_ID>/secrets/goshopx-google-client-id/versions/latest"
        path: google-client-id
      - resourceName: "projects/<PROJECT_ID>/secrets/goshopx-vnpay-tmn-code/versions/latest"
        path: vnpay-tmn-code
      - resourceName: "projects/<PROJECT_ID>/secrets/goshopx-vnpay-hash-secret/versions/latest"
        path: vnpay-hash-secret
      - resourceName: "projects/<PROJECT_ID>/secrets/goshopx-vnpay-payment-url/versions/latest"
        path: vnpay-payment-url
      - resourceName: "projects/<PROJECT_ID>/secrets/goshopx-gmail-username/versions/latest"
        path: gmail-username
      - resourceName: "projects/<PROJECT_ID>/secrets/goshopx-smtp-password/versions/latest"
        path: gmail-password
      - resourceName: "projects/<PROJECT_ID>/secrets/goshopx-gmail-from/versions/latest"
        path: gmail-from
      - resourceName: "projects/<PROJECT_ID>/secrets/goshopx-deepseek-api-key/versions/latest"
        path: deepseek-api-key
      - resourceName: "projects/<PROJECT_ID>/secrets/goshopx-gemini-api-key/versions/latest"
        path: gemini-api-key
---
apiVersion: secret-sync.gke.io/v1
kind: SecretSync
metadata:
  name: goshopx-runtime
  namespace: goshopx
spec:
  serviceAccountName: goshopx-runtime
  secretProviderClassName: goshopx-runtime-provider
  secretObject:
    type: Opaque
    data:
      - { sourcePath: jwt-secret, targetKey: SECRET_KEY }
      - { sourcePath: minio-access-key, targetKey: MINIO_ACCESS_KEY }
      - { sourcePath: minio-secret-key, targetKey: MINIO_SECRET_KEY }
      - { sourcePath: account-database-url, targetKey: ACCOUNT_DATABASE_URL }
      - { sourcePath: product-database-url, targetKey: PRODUCT_DATABASE_URL }
      - { sourcePath: order-database-url, targetKey: ORDER_DATABASE_URL }
      - { sourcePath: payment-database-url, targetKey: PAYMENT_DATABASE_URL }
      - { sourcePath: inventory-database-url, targetKey: INVENTORY_DATABASE_URL }
      - { sourcePath: notification-database-url, targetKey: NOTIFICATION_DATABASE_URL }
      - { sourcePath: admin-database-url, targetKey: ADMIN_DATABASE_URL }
      - { sourcePath: recommender-database-url, targetKey: RECOMMENDER_DATABASE_URL }
      - { sourcePath: google-client-id, targetKey: GOOGLE_CLIENT_ID }
      - { sourcePath: vnpay-tmn-code, targetKey: VNPAY_TMN_CODE }
      - { sourcePath: vnpay-hash-secret, targetKey: VNPAY_HASH_SECRET }
      - { sourcePath: vnpay-payment-url, targetKey: VNPAY_PAYMENT_URL }
      - { sourcePath: gmail-username, targetKey: GMAIL_USERNAME }
      - { sourcePath: gmail-password, targetKey: GMAIL_PASSWORD }
      - { sourcePath: gmail-from, targetKey: GMAIL_FROM }
      - { sourcePath: deepseek-api-key, targetKey: AI_API_KEY }
      - { sourcePath: deepseek-api-key, targetKey: DEEPSEEK_API_KEY }
      - { sourcePath: gemini-api-key, targetKey: GEMINI_API_KEY }
```

Apply and verify key names only (do not print `.data`):

```powershell
((Get-Content F:\GoshopX\infra\gcp\runtime-secret-sync.yaml -Raw).Replace('<PROJECT_ID>', $Project)) | kubectl apply -f -
kubectl describe secretsync goshopx-runtime -n goshopx
kubectl get secret goshopx-runtime -n goshopx -o json | ConvertFrom-Json | ForEach-Object { $_.data.PSObject.Properties.Name }
```

Expected: SecretSync is ready and all keys from `deploy/helm/goshopx/values.yaml` are listed. Google notes synchronized values become Kubernetes Secrets in etcd; keep RBAC narrow and do not print the secret object. This flow closes the current Secret Manager gap for the lab; before production, split the shared secret object and IAM access by service.

## 7. Set non-secret GCP Helm values and bootstrap Argo CD

Write Terraform outputs into `deploy/helm/goshopx/values-gcp-lab.yaml` locally or commit the non-secret identifiers in the private project repo:

- `global.imageRegistry`: `terraform output -raw artifact_registry`
- `infrastructure.cloudSQL.instanceConnectionName`: `terraform output -raw sql_connection_name`
- `infrastructure.cloudSQL.gcpServiceAccountEmail`: `terraform output -raw runtime_service_account`
- Check `global.gateway.staticIpName=goshopx-https`, `certificateMapName=goshopx-map`, hosts `goshopx.click` and `www.goshopx.click`.

Use Helm `v3.22.1` for local validation; both GitHub workflows pin the same version. Run render/lint before install:

```powershell
helm lint F:\GoshopX\deploy\helm\goshopx `
  -f F:\GoshopX\deploy\helm\goshopx\values.yaml `
  -f F:\GoshopX\deploy\helm\goshopx\values-gcp-lab.yaml
helm template goshopx F:\GoshopX\deploy\helm\goshopx -n goshopx `
  -f F:\GoshopX\deploy\helm\goshopx\values.yaml `
  -f F:\GoshopX\deploy\helm\goshopx\values-gcp-lab.yaml > $env:TEMP\goshopx-gcp-render.yaml
```

Review rendered Service type: no public admin/dashboard NodePorts; review NetworkPolicy disabled (current overlay) and accept only for isolated lab or implement/enable a tested policy first. Scan the rendered file and check image tags/digests. The local machine did not have Helm during the last verification; GitHub workflow is configured to render, but no hosted run has yet proven this chart.

Before Argo sync, configure GitHub repo variables under Settings → Environments → `lab` (create the environment). Both image build and promotion jobs use this environment. When dispatching the workflow, select branch `main` so the WIF provider's branch condition matches:

| Variable | Value |
| --- | --- |
| `GCP_PROJECT_ID` | `terraform output -raw project_id` |
| `GCP_WIF_PROVIDER` | `terraform output -raw github_wif_provider` |
| `GCP_RELEASE_SERVICE_ACCOUNT` | `terraform output -raw github_release_service_account` |
| `GCP_AR_LOCATION` | `asia-southeast1` |
| `GCP_AR_REPOSITORY` | `goshopx` |
| `GOOGLE_CLIENT_ID` | public OAuth client ID if used |

Không tạo service account key. WIF chỉ cho repo/branch được ràng buộc trong Terraform. Đẩy code/workflow lên `main`, vào Actions → **GCP lab image build, scan and GitOps promotion** → Run workflow. Gate phải xanh: Go tests/vet, Python, Web tests/build, Helm render, Terraform validate, CodeQL, TruffleHog, Trivy config/fs, image scan. Workflow tạo PR cập nhật image digest; review rồi merge PR đó.

Bootstrap Argo CD từ repo root:

```powershell
helm repo add argo https://argoproj.github.io/argo-helm
helm repo update
helm upgrade --install argocd argo/argo-cd --namespace argocd --create-namespace --wait
kubectl apply -f F:\GoshopX\ops\gitops\argocd\application-gcp-lab.yaml
kubectl get applications -n argocd
```

Nếu repo private, cấu hình repo credentials trong Argo CD trước khi apply Application. Argo theo dõi `main`, namespace `goshopx`, file `values.yaml` + `values-gcp-lab.yaml`. CI không có kubeconfig và không deploy trực tiếp.

## 8. Chờ rollout, migration, catalog và index

```powershell
kubectl wait --for=condition=Ready nodes --all --timeout=10m
kubectl get applications -n argocd
kubectl get pods,pvc,svc -n goshopx -o wide
kubectl get svc -n goshopx
```

Gate trước khi tiếp tục:

1. Secret `goshopx-runtime` tồn tại và đủ key; không `describe`/in YAML chứa data.
2. PVC cho Kafka, Redis, Elasticsearch/Product, MinIO, Milvus, etcd bind vào `standard-rwo`; không có PVC quản trị public.
3. `cloudsql-proxy` Ready; app logs không có lỗi PostgreSQL/auth/DNS.
4. Account/Order/Payment/Inventory/Notification/Admin migrations hoàn thành trong đúng DB. Deployment names là `account`, `order`, `payment`, `inventory`, `notification`, `admin`; xem `kubectl logs deployment/account -n goshopx --tail=100` (thay service tương ứng). Không rollout toàn bộ nhiều lần khi migration hoặc seed còn lỗi.
5. Product log báo seed count và Product API `products` list endpoint nội bộ trả dữ liệu. `DUMMYJSON_SEED_LIMIT=0` có nghĩa tải hết `total` theo trang, giá VND dùng tỷ lệ demo 25.000, không phải tỷ giá thị trường.
6. Sau khi Product seed xong, recommender-sync phải bootstrap Product API. Nếu consumer khởi động trước Product, kiểm tra log rồi restart đúng một lần: `kubectl rollout restart deployment/recommender-sync -n goshopx`; chờ Milvus index/logs.
7. Chat text/image chỉ kiểm thử sau khi Gemini collection `product_catalog_gemini_v2` được tạo và index có dữ liệu. Không đổi 768-dim collection về collection hash 512-dim.

**Seed là gate cần chú ý:** hiện Product service tự seed khi process khởi động; đây chưa phải Job có version/idempotency hoàn chỉnh. Import có thể ghi đè lại tồn kho DummyJSON khi Product pod restart. Sau lần seed đầu, sửa `DUMMYJSON_SEED_ENABLED` thành `false` trong GCP Helm config, đẩy commit để Argo đồng bộ rồi xác minh ConfigMap/pod. Chưa mở cửa mua hàng trước khi seed đã dừng và stock được kiểm tra. Cần chuyển seed sang Job/versioned importer trước khi gọi quy trình này là production-ready.

Kubernetes readiness gate:

```powershell
kubectl get pods -n goshopx
kubectl get pvc -n goshopx
kubectl get svc -n goshopx
kubectl get gateway,httproute -n goshopx
kubectl get svc -A | Select-String 'NodePort|LoadBalancer'
```

Chỉ public Gateway được nhận Internet. Argo, Grafana, Jaeger, Consul, Kong Admin/Manager, RedisInsight, Kafka UI, MinIO console và database phải ClusterIP; operator truy cập bằng `kubectl port-forward` cùng RBAC.

## 9. Nghiệm thu commerce và chatbot

Thực hiện với tài khoản test, tồn kho test và payment sandbox:

- Login/logout, refresh token, Google OAuth callback nếu bật.
- Catalog list/search/detail, category, ảnh thumbnail; stock/giá hiển thị từ backend.
- Cart add/update/remove; test hai user tranh sản phẩm stock thấp và retry request.
- Reservation/expiry, COD tạo order trạng thái `pending` (không coi là đã thu tiền).
- VNPay sandbox return/IPN chữ ký hợp lệ và sai; không dùng giao dịch tiền thật.
- Notification/email chỉ khi credential và callback đã kiểm tra; admin RBAC qua kênh nội bộ.
- Chat hỏi sản phẩm theo nhu cầu/ngân sách, so sánh, thiếu dữ liệu, ảnh hợp lệ/ảnh sai, ảnh trả về thuộc catalog. Cart agent hiện chỉ gợi ý; nó chưa có server-side proposal/confirm hay coupon.
- AI test tay ít request; **ngân sách AI 200.000đ chưa có request/token/spend limiter trong runtime**, vì vậy chưa bật traffic tự do hoặc đưa AI vào load test.

Theo cùng request ID/log/metric/trace nếu có. Kiểm tra Kafka consumer lag, DB connections, pod restart/OOM, Cloud Monitoring và Grafana trước khi chuyển bước.

## 10. Chứng thư, Cloudflare và domain

Terraform tạo static IP, DNS authorization cho apex và `www`, certificate/map. DNS authorization mỗi domain là riêng; áp dụng hai CNAME từ `terraform output -json certificate_dns_records` theo [tài liệu Google](https://docs.cloud.google.com/certificate-manager/docs/dns-authorizations). Tạo CNAME này trước khi đợi certificate `ACTIVE`:

```powershell
terraform output -json certificate_dns_records | ConvertFrom-Json
gcloud certificate-manager certificates describe goshopx-apex --location=global --project $Project
```

Trong Cloudflare, ghi lại toàn bộ record hiện có trước. Tạo hai CNAME authorization đúng `name`/`data` Terraform output, DNS-only. Chờ certificate Active. Sau đó Argo deploy Gateway/HTTPRoute; chờ `Programmed=True`, `Accepted=True`, IP Gateway khớp `terraform output -raw https_ip`:

```powershell
kubectl describe gateway goshopx-gateway -n goshopx
kubectl describe httproute goshopx-public -n goshopx
$HttpsIp = terraform output -raw https_ip
curl.exe --resolve "goshopx.click:443:$HttpsIp" -I https://goshopx.click/
curl.exe --resolve "www.goshopx.click:443:$HttpsIp" -I https://www.goshopx.click/
```

Chỉ sau khi thử origin thành công mới đổi apex và `www` sang A record trỏ IP GKE (DNS-only ban đầu). Giữ nguyên MX/TXT/mail và lưu target cũ để rollback. Kiểm thử cookie Secure, SPA deep links, GraphQL, ảnh, OAuth redirect URIs, VNPay return/IPN callback trước khi cân nhắc bật proxy Cloudflare. Không đưa Cloudflare API token vào Terraform state hay repo.

## 11. Load/stress test và ghi evidence

Chạy smoke trước, không gửi tải tới AI provider thật:

```powershell
$env:GOSHOPX_URL = 'https://goshopx.click/graphql'
k6 run -e K6_PROFILE=smoke tests/k6/goshopx-load.js
k6 run -e K6_PROFILE=load tests/k6/goshopx-load.js
```

Stress/soak chỉ sau khi xác nhận đủ ngân sách và đang quan sát Cloud Monitoring:

```powershell
k6 run -e K6_PROFILE=stress tests/k6/goshopx-load.js
k6 run -e K6_PROFILE=soak -e K6_SOAK_RPS=10 tests/k6/goshopx-load.js
```

Hiện k6 script chỉ gọi anonymous GraphQL catalog; nó **chưa** đo cart/reservation/checkout hay CCU. Ghi commit và image digest, cấu hình node/SQL, dataset, RPS, VUs, p95/p99, error rate, dropped iterations, pod restarts, Kafka lag và DB connections. Dừng khi error >5%, p95/p99 thresholds fail, dropped iterations xuất hiện, tài nguyên chạm giới hạn hoặc cost tăng ngoài dự kiến. Xem [load testing](../load-testing.md) để phân biệt VU, CCU, RPS và concurrent request.

## 12. Rollback và kết thúc lab

- App release lỗi: revert PR digest trên Git, để Argo sync lại; kiểm tra migration có tương thích chiều ngược không trước rollback DB.
- Cert/DNS lỗi: hoàn lại Cloudflare apex/`www` về record đã lưu; giữ nguyên mail records.
- Chi phí bất thường: dừng workload sau khi chụp/log dữ liệu cần giữ, không mặc định scale-to-zero sẽ dừng Cloud SQL/NAT/IP phí.
- Kết thúc đúng thời điểm sớm hơn `T0 + 48h` hoặc `credit expiry - 12h`; theo [teardown](./teardown.md), xóa Gateway/LB trước, export dữ liệu, dừng Argo, gỡ workload/PVC, Terraform destroy, kiểm tra residual resources/billing. Không xóa billing account hoặc project cũ.

## Hoàn thành khi nào?

Chỉ đánh dấu deploy xong khi tất cả đạt: Terraform state remote; Actions security gates xanh; Argo `Synced/Healthy`; secret/database migrations pass; catalog đã seed và reindex; commerce core + sandbox payment pass; không có admin endpoint public; certificate ACTIVE và DNS origin test pass; k6 smoke pass; teardown đã diễn tập đủ để dừng lab đúng hạn. Hiện repo **chưa có evidence** cho các điều kiện GCP/runtime này.
