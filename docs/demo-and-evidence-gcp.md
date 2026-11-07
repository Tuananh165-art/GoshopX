# Kịch bản demo và thu thập bằng chứng GoshopX trên GCP

Runbook này dành cho Windows PowerShell, project lab `generated-mote-467410-b0` và cluster `goshopx-lab`. Nó ghi lệnh để **quan sát** Terraform/GKE/GCP và kịch bản kiểm tra website/chatbot. Không chạy `terraform apply`, migration, nạp embedding hay đặt đơn thật chỉ để tạo ảnh.

## 1. Bằng chứng nào chứng minh điều gì

Ảnh chỉ chứng minh trạng thái nhìn thấy tại thời điểm chụp. Hãy lưu ngày giờ, commit/revision và kết quả lệnh cùng ảnh. Không dùng ảnh source Terraform để khẳng định tài nguyên đã tồn tại, không dùng HTTP 200 để khẳng định luồng mua hàng hoạt động, và không dùng widget mở để khẳng định AI đã trả lời.

| Mức | Bằng chứng | Kết luận có thể rút ra |
|---|---|---|
| IaC | Terraform version, validate, outputs và state list | Cấu hình hợp lệ và state đang quản lý resource; `validate` không chứng minh apply thành công. |
| GCP | Console hiển thị project/resource | Resource đang hiện diện trong project được chọn. |
| Kubernetes/GitOps | `kubectl` pods/rollout và Argo Application `Synced/Healthy` | Workload đã schedule/ready tại lúc chụp; không chứng minh mọi nghiệp vụ. |
| Website | Browser và HTTP checks | Host truy cập được; chỉ luồng đã thao tác mới được nghiệm thu. |
| AI | Prompt và câu trả lời, session tiếp nối, nguồn/tool trace đã redact | Chỉ xác nhận chức năng được chạy; không suy rộng sang ingestion/vector nếu chưa có generation. |

**Tình trạng nền từ hồ sơ ngày 2026-11-07:** CI/security/build và GitOps promotion được ghi nhận thành công; Argo/rollout và HTTP/GraphQL text smoke đã có trong runbook. Gemini Embedding 2 trước đó trả 429 năm lần; chưa có vector generation/ingestion job trong lần kiểm tra được ghi lại. Vì vậy không gọi rebuild/embedding cho tới khi quota được chủ project xác minh. Ảnh hiện có trong repo là storefront và widget rỗng, không phải E2E chatbot. Tham khảo [runbook provider](./chatbot-provider-gke-runbook.md).

## 2. Chuẩn bị và an toàn thông tin

1. Mở PowerShell tại `F:\GoshopX`; xác nhận branch/commit để gắn evidence với đúng source:

   ```powershell
   Set-Location F:\GoshopX
   git status -sb
   git rev-parse --short HEAD
   Get-Date -Format "yyyy-MM-dd HH:mm:ss zzz"
   ```

2. Đảm bảo công cụ sẵn có, không cài/nâng cấp giữa buổi nghiệm thu:

   ```powershell
   terraform version
   gcloud version
   kubectl version --client
   helm version --short
   ```

3. Tạo thư mục bằng chứng tại `docs/evidence/screenshots/` nếu chưa có. Chụp bằng Snipping Tool (`Win+Shift+S`) và lưu PNG theo tên ở mục 7.
4. Trước khi chụp, ẩn email/tên cá nhân, tab có token, cửa sổ `.env`, Secret Manager value/version payload, kubeconfig, OAuth secret, database URL/password, Cloudflare token, billing CSV và mọi dữ liệu khách hàng. Chụp **tên secret**, không mở nội dung.
5. Không đưa `terraform.tfstate`, plan, terminal history có secret, HAR/network payload hoặc ảnh upload của người dùng vào repo. Có thể giữ bản gốc riêng tư; chỉ commit ảnh đã rà soát.

## 3. Chụp Terraform evidence (chỉ đọc)

```powershell
Set-Location F:\GoshopX\infra\gcp
terraform version
terraform init
terraform validate
terraform output -raw project_id
terraform output -raw cluster_name
terraform output -raw region
terraform output -raw zone
terraform output -raw artifact_registry
terraform output -raw sql_connection_name
terraform output -raw https_ip
terraform state list | Select-String 'google_container_cluster|google_sql_database_instance|google_artifact_registry_repository|google_secret_manager_secret|google_storage_bucket|google_certificate_manager|google_billing_budget'
```

Kỳ vọng: project là `generated-mote-467410-b0`, cluster `goshopx-lab`, region/zone `asia-southeast1`/`asia-southeast1-a`; state list có các nhóm resource đã áp dụng. `terraform validate` chỉ kiểm tra cú pháp/schema. Chụp terminal với lệnh và output đã che billing/account identifier nếu xuất hiện: `01-terraform-outputs.png`.

Không chạy `terraform plan` hoặc `apply` chỉ để chụp. Nếu đang cần thay đổi hạ tầng, theo [deploy-step-by-step](./gcp/deploy-step-by-step.md), rà soát plan riêng và dùng quy trình review/approval của project.

## 4. Xác minh GKE, workload và Argo CD

```powershell
Set-Location F:\GoshopX\infra\gcp
$Project = terraform output -raw project_id
$Zone = terraform output -raw zone
gcloud config set project $Project
gcloud container clusters get-credentials goshopx-lab --zone $Zone --project $Project
kubectl get nodes -o wide
kubectl get pods -n goshopx -o wide
kubectl get deploy,svc,pvc -n goshopx
kubectl get gateway,httproute -n goshopx
kubectl get application goshopx-gcp-lab -n argocd -o wide
```

Kỳ vọng: node `Ready`; các workload cần cho luồng đang thử `Ready`; Gateway có `PROGRAMMED=True`; Argo `Synced`/`Healthy`. Nếu `get-credentials` lỗi quyền/cache, dừng ở đây và chạy trên máy operator đã đăng nhập GCP. Không in secret hoặc toàn bộ pod environment.

Rollout quan sát (thay deployment nếu tên thực tế khác):

```powershell
kubectl rollout status deployment/graphql -n goshopx --timeout=5m
kubectl rollout status deployment/recommender-server -n goshopx --timeout=5m
kubectl rollout status deployment/web -n goshopx --timeout=5m
```

Chụp `02-gke-cluster.png`, `03-gke-workloads.png`, `04-argocd-application.png`. Có thể dùng Cloud Console → Kubernetes Engine → Clusters/Workloads để ảnh thể hiện rõ project/cluster; chọn đúng project trước khi chụp. Nếu pod không sẵn sàng, lưu ảnh lỗi vào hồ sơ riêng và ghi lỗi; không dùng ảnh cũ làm trạng thái hiện tại.

## 5. Chụp dịch vụ GCP trong Cloud Console

Mở `https://console.cloud.google.com/`, chọn project **generated-mote-467410-b0** (project selector ở thanh trên) rồi chụp từng trang:

1. **GKE** → Kubernetes Engine → Clusters → `goshopx-lab`: tên, location, phiên bản và trạng thái cluster. `05-gcp-gke-cluster.png`.
2. **Artifact Registry** → Repositories → `goshopx`: repository/location và danh sách image/tag/digest. `06-artifact-registry.png`.
3. **Cloud SQL** → Instances → `goshopx-postgres`, rồi databases: instance và database names; không mở credential/connection secret. `07-cloud-sql.png`.
4. **Secret Manager** → Secrets: chỉ tên secret và project, tuyệt đối không bấm “View value”. `08-secret-manager-names.png`.
5. **Cloud Storage** → Buckets: bucket names/location; không mở object chứa upload người dùng. `09-cloud-storage.png`.
6. **Certificate Manager** → Certificates / Maps: certificate `ACTIVE`, hostname và map entry. `10-certificate-manager.png`.
7. **Billing → Budgets & alerts**: có thể chụp budget threshold nhưng che billing account ID/chi tiêu cá nhân. Budget chỉ cảnh báo, không khóa phát sinh phí. `11-billing-budget.png`.

Terraform state list và Console bổ sung cho nhau: Console chứng minh resource đang hiển thị, state list chứng minh Terraform quản lý. Không chụp riêng biểu đồ kiến trúc để thay thế hai bằng chứng này.

Lệnh `gcloud` chỉ đọc bổ sung để đối chiếu các resource trước khi mở Console:

```powershell
gcloud container clusters describe goshopx-lab --location $Zone --project $Project --format="yaml(name,location,status,currentMasterVersion,endpoint)"
gcloud artifacts repositories describe goshopx --location asia-southeast1 --project $Project --format="yaml(name,format,location)"
gcloud sql instances describe goshopx-postgres --project $Project --format="yaml(name,region,databaseVersion,state,settings.tier,ipAddresses)"
gcloud storage buckets list --project $Project --format="table(name,location,storageClass)"
gcloud certificate-manager certificates describe goshopx-apex --location global --project $Project --format="yaml(name,managed.state,managed.domains)"
```

Nếu `format` báo không hỗ trợ field với phiên bản gcloud cục bộ, bỏ `--format` và xem output, nhưng không chụp nội dung có endpoint/địa chỉ cá nhân. Lệnh `gcloud` thành công chứng minh API trả metadata hiện tại; Console ảnh giúp người xem nhận diện service dễ hơn.

## 6. Website và kịch bản demo chatbot

### 6.1 Kiểm tra host và mở trình duyệt

```powershell
curl.exe https://goshopx.click/ -sS -o NUL -w "apex HTTP %{http_code}`n"
curl.exe https://www.goshopx.click/ -sS -o NUL -w "www HTTP %{http_code}`n"
curl.exe https://goshopx.click/login -sS -o NUL -w "login HTTP %{http_code}`n"
```

Kỳ vọng HTTP 200 là kiểm tra reachability, không thay thế browser E2E. Mở `https://goshopx.click` bằng browser thường, không dùng ảnh devtools extension error làm bằng chứng ứng dụng. Chụp storefront: `12-storefront-live.png`.

### 6.2 Các luồng web cần thao tác và ghi kết quả

Dùng tài khoản lab riêng, không dùng dữ liệu khách thật. Không tạo đơn hoặc thanh toán thật trong bài demo. Với mỗi luồng, chụp UI trước/sau và ghi kết quả vào bảng nghiệm thu cuối tài liệu.

| Case | Thao tác | Đạt khi |
|---|---|---|
| WEB-01 Catalog | Mở trang chủ, đổi category, tìm tên/SKU, mở chi tiết | Danh sách/chi tiết hiển thị; URL và sản phẩm khớp thao tác. |
| WEB-02 Auth | Đăng nhập tài khoản test thường; Google OAuth nếu đã cấu hình callback cho đúng host | UI phản ánh đúng trạng thái tài khoản; không chụp email/token. |
| WEB-03 Cart | Thêm một sản phẩm còn hàng, đổi số lượng, reload | Cart giữ đúng sản phẩm; lỗi/oversell không được báo thành công. |
| WEB-04 Orders | Xem lịch sử đơn đã có từ dữ liệu lab | Chỉ tài khoản sở hữu thấy đơn; không tạo COD thật để kiểm tra email. |
| WEB-05 Admin | Tài khoản role được cấp vào Admin → Dữ liệu AI; xem danh sách job/sources | Chỉ admin truy cập; không sửa/xuất bản dữ liệu production ngoài ý muốn. |

### 6.3 Chatbot text và memory

Mở trợ lý sản phẩm và lần lượt gửi các tin nhắn **trong cùng một cuộc hội thoại**:

1. `Tìm laptop dưới 20 triệu, ưu tiên ASUS và còn hàng.`
2. `Chỉ ASUS, pin tốt và nhẹ.`
3. `Tăng ngân sách lên 25 triệu; so sánh con thứ hai với mẫu trước.`
4. `Mẫu nào có bảo hành dài hơn? Nếu catalog không có dữ liệu thì nói rõ.`

Ghi lại session/thread tiếp tục sau reload (nếu UI hỗ trợ), tên và ID sản phẩm, giá hiển thị, nguồn/citation, và có hỏi làm rõ khi thiếu dữ kiện hay không. Giá/kho phải khớp backend; không chấp nhận thông số bảo hành tự bịa. Chụp `13-chat-discovery.png` và `14-chat-followup-comparison.png`.

**Tiêu chí pass:** câu sau dùng đúng budget/brand của các lượt trước, “con thứ hai” trỏ đúng shortlist, sản phẩm thuộc catalog và trong budget, dữ liệu thiếu được nêu rõ. Nếu lỗi GraphQL như `value too long for type character varying(64)`, lưu screenshot + request ID (không lưu token) và đánh fail; xem [QA review](./chatbot-qa-review-2026-11-07.md).

### 6.4 Ảnh, Tavily, Jev và tracing

- Ảnh: dùng ảnh test sản phẩm được phép sử dụng, JPEG/PNG/WebP nhỏ hơn giới hạn API. Hỏi “Tìm sản phẩm giống ảnh này”. Đạt khi trả product card/ảnh từ catalog hoặc nêu không tìm thấy; ảnh upload không được hiện nguyên dạng trong lịch sử sau policy lưu trữ. Chụp `15-chat-image-search.png`.
- Negative upload: thử file sai MIME/đổi đuôi, file quá cỡ hoặc ảnh hỏng. API/UI phải từ chối có thông báo, không 500 và không tạo proposal. Ghi riêng lỗi bảo mật nếu server chấp nhận.
- Tavily: hỏi một câu kiến thức tổng quát không có trong catalog (ví dụ tiêu chí chọn màn hình cho văn phòng). Đạt khi có URL nguồn nếu tool được gọi; nội dung web không được dùng làm giá, tồn kho, coupon hoặc khẳng định tương thích sản phẩm. Chụp `16-chat-sources.png`.
- Jev: yêu cầu so sánh shortlist. Chỉ tuyên bố Jev hoạt động nếu trace/operator log cho thấy route/rerank được gọi và candidate IDs vẫn thuộc shortlist. Khi provider lỗi, fallback phải trả kết quả an toàn.
- LangSmith: project trace hiển thị metadata đã redact (route, latency, usage flags), không raw prompt, email, token, ảnh hoặc session ID. Chụp `17-langsmith-redacted-trace.png` sau khi kiểm tra payload.

Runbook hiện ghi nhận DeepSeek/Jev/Tavily/LangSmith từng smoke, nhưng Gemini Embedding 2 từng trả HTTP 429 và index generation chưa được xác nhận. **Không** chạy embedding rebuild, upload batch hoặc retry provider nhiều lần cho demo tới khi owner xác nhận quota và ledger còn dư. Chỉ một request smoke cho từng provider; giữ provider key ngoài screenshot.

### 6.5 Cart proposal / coupon quote và admin ingestion

1. Trong chatbot yêu cầu tạo đề xuất giỏ hàng, kiểm tra nội dung và mã `LAB10` nếu coupon đang tồn tại.
2. Chụp proposal preview trước xác nhận: `18-cart-proposal-preview.png`.
3. Xác nhận một lần với tài khoản test; kiểm tra cart thay đổi đúng một lần, proposal hết hạn/đã dùng không thể replay, giá/kho được backend xác minh. Không bấm checkout/place order/Pay cho demo.
4. Coupon quote phải hiển thị subtotal, discount, payable và expiry nhất quán; preview không được tiêu coupon. Chụp `19-cart-quote-preview.png`.
5. Admin ingestion chỉ test nếu worker/UI đang deployed và quota đã được xác nhận: dùng tài liệu synthetic không nhạy cảm, preview chunk/cost, enqueue một job, chờ completed, kiểm tra chunk count rồi publish. Chụp `20-admin-ingestion-job.png`.
6. Runbook hiện ghi nhận trước đây database có 0 ingestion jobs/0 vector generations; nếu trạng thái chưa đổi thì đánh “chưa nghiệm thu”, không tạo dữ liệu sai để ảnh đẹp hơn.

## 7. Danh mục ảnh đề xuất

Lưu tại `docs/evidence/screenshots/`. Đặt ngày/commit trong evidence ledger, không nhúng nhiều ảnh terminal nhỏ khó đọc vào README.

| File | Chủ đề |
|---|---|
| `01-terraform-outputs.png` | Terraform version, validate, outputs/state list đã lọc |
| `02-gke-cluster.png` | GCP Console cluster |
| `03-gke-workloads.png` | kubectl pods/deployments |
| `04-argocd-application.png` | Argo Synced/Healthy |
| `05-gcp-gke-cluster.png` | GKE cluster details (nếu ảnh 02 là overview khác) |
| `06-artifact-registry.png` | Registry và image digests |
| `07-cloud-sql.png` | Cloud SQL instance/database names |
| `08-secret-manager-names.png` | Tên secrets, không có value |
| `09-cloud-storage.png` | Bucket list |
| `10-certificate-manager.png` | Certificate ACTIVE / map |
| `11-billing-budget.png` | Budget alert thresholds đã che dữ liệu riêng |
| `12-storefront-live.png` | Host public đã tải trên browser |
| `13-chat-discovery.png` | Kết quả discovery có product card |
| `14-chat-followup-comparison.png` | Memory + so sánh nhiều lượt |
| `15-chat-image-search.png` | Input ảnh và kết quả catalog |
| `16-chat-sources.png` | Tavily citations nếu tool đã chạy |
| `17-langsmith-redacted-trace.png` | Trace metadata đã kiểm tra redaction |
| `18-cart-proposal-preview.png` | Preview chưa xác nhận |
| `19-cart-quote-preview.png` | Coupon quote/expiry, không đặt đơn |
| `20-admin-ingestion-job.png` | Job trạng thái hoàn tất và chunk count |
| `21-github-actions-release.png` | Workflow build/scan và promotion thành công |

Các ảnh số 02/05 trùng mục tiêu tùy góc chụp; chỉ giữ một ảnh nếu dư thừa. Chỉ commit ảnh có thật, đã che thông tin nhạy cảm và có thể đối chiếu ngày.

## 8. Evidence ledger và trạng thái hiện tại

| Evidence | Trạng thái trong repo tại 2026-11-07 | Việc còn lại |
|---|---|---|
| Storefront screenshot | Có `docs/evidence/screenshots/goshopx-storefront-2026-11-07.png` | Chụp lại từ browser live nếu dùng làm bằng chứng deploy hiện tại. |
| Assistant widget screenshot | Có `goshopx-assistant-ready-2026-11-07.png`, widget rỗng | Chỉ chứng minh widget hiển thị; cần ảnh câu trả lời thật để nghiệm thu AI. |
| CI/build/security/GitOps | Thành công được ghi trong provider runbook | Chụp workflow run và PR merge mới nhất nếu cần evidence public. |
| GKE/Argo/HTTP/GraphQL | Thành công được ghi nhận trong runbook ngày 2026-11-07 | Chạy lại lệnh ở mục 4/6.1 trong phiên auth hiện tại. |
| Discovery text | GraphQL text smoke được ghi nhận | Browser multi-turn test vẫn cần thực hiện. |
| Gemini dense/image embedding | Chưa nghiệm thu; lần trước 429, vector generation chưa xác nhận | Khôi phục quota; một smoke có kiểm soát rồi mới index. |
| Ingestion/admin | Chưa nghiệm thu live theo runbook | Xác minh worker và vector quota trước khi chạy job. |
| Cart proposal/quote | Có code/contract và từng lỗi reservation ID trong QA | Browser replay/expiry/coupon quote E2E chưa đủ bằng chứng. |
| Terraform/GCP service screenshot | Chưa có screenshot GCP trong thư mục evidence | Operator chụp Console theo mục 5. |

Ghi thêm dòng gồm `date/time`, `git commit`, `project`, `test ID`, `result`, `screenshot path`, `request ID` (nếu có) vào [screenshot inventory](./evidence/README.md). Không ghi token/account email.

## 9. Deploy/release reference

Quy trình chuẩn hiện có ở [GCP deploy step by step](./gcp/deploy-step-by-step.md), [GCP overview](./gcp/README.md) và [pod recovery](./gcp/pod-recovery.md). Tóm tắt thứ tự:

1. Cập nhật code trên feature branch; chạy quality/security CI.
2. Merge source PR sau khi checks bắt buộc xanh.
3. `release-gcp.yml` build, scan và push image theo digest; workflow mở GitOps promotion PR.
4. Review/merge GitOps PR sau khi checks xanh; không sửa digest trực tiếp trong cluster.
5. Chờ Argo CD `Synced/Healthy`, kiểm tra rollout, Gateway và public smoke.
6. Chạy acceptance browser/provider riêng; giữ chi phí AI trong trần đã duyệt.

Để chụp CI evidence: GitHub repository → **Actions** → workflow **GCP lab image build, scan and GitOps promotion** → mở lần chạy mới nhất → xác nhận quality/security/build-scan/promote xanh; mở artifact/run summary để thấy image matrix nếu có. Chụp `21-github-actions-release.png` sao cho branch/commit, thời điểm và kết quả thấy rõ. Không chụp secret/environment variables. Nếu workflow chỉ mở PR promotion mà chưa merge, đừng gọi đó là deploy hoàn tất; chụp cả trạng thái PR và Argo revision sau khi merge.

Lệnh xác minh sau rollout (chỉ đọc):

```powershell
kubectl get application goshopx-gcp-lab -n argocd -o wide
kubectl get deploy,pods,pvc,svc -n goshopx
kubectl get gateway,httproute -n goshopx
curl.exe https://goshopx.click/ -sS -o NUL -w "HTTP %{http_code}`n"
```

Không chụp ảnh `terraform apply` mới nếu không có hạ tầng cần thay đổi. Nếu cần thay đổi, phải xem plan và tuân theo budget/credit gate trong GCP runbook. Cleanup xem [teardown.md](./gcp/teardown.md).

## 10. Checklist kết thúc demo

- [ ] Ảnh có đúng project/hostname và timestamp; không lộ secret/PII.
- [ ] GKE/Argo/pods đều được kiểm tra lại trong phiên hiện tại.
- [ ] Website catalog, search và auth được thao tác thật; kết quả được ghi theo case.
- [ ] Chat multi-turn dùng đúng context; câu trả lời có nguồn và giá từ backend.
- [ ] Multimodal/Tavily/Jev/LangSmith chỉ đánh dấu pass khi có evidence riêng tương ứng.
- [ ] Proposal/coupon preview không tạo đơn hoặc thanh toán thật.
- [ ] Ingestion không chạy khi quota/chi phí chưa rõ; không gọi Gemini lặp khi 429.
- [ ] Screenshot inventory và README không phóng đại mức nghiệm thu.
