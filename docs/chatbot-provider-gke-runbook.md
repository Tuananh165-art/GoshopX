# Chạy chatbot với provider thật trên GKE

Runbook này đồng bộ khóa AI từ `.env` cục bộ vào Secret Manager, phát hành image qua CI/GitOps và chạy một lượt smoke có giới hạn. Không chạy k6 với provider thật.

## Điều kiện trước khi bắt đầu

- Xác nhận project là `generated-mote-467410-b0`, cluster `goshopx-lab` và namespace `goshopx`.
- Mở Billing và xác nhận credit còn hiệu lực, hạn lab và chi phí thực tế. Dừng nếu ngân sách AI còn lại không đủ; cảnh báo Budget không tự chặn mọi phí.
- Kiểm tra trần `AI_SPEND_LIMIT_VND=200000` và ledger trong database `recommender`. Rates để trống sẽ giữ lại khoản reserve, không đồng nghĩa provider miễn phí.
- Đảm bảo `.env` gốc có `DEEPSEEK_API_KEY`, `GEMINI_API_KEY`, `TAVILY_API_KEY`, `TYPESAFE_API_KEY` và `LANGSMITH_API_KEY`. Script chỉ báo tên secret thiếu, không in giá trị.
- GKE CLI hoạt động: `gcloud container clusters get-credentials` và `kubectl get nodes` thành công.

## 1. Đồng bộ khóa provider

Mở PowerShell tại repo và tạo version mới chỉ cho năm khóa AI, lấy giá trị từ `.env` hiện có:

```powershell
Set-Location F:\GoshopX
powershell -ExecutionPolicy Bypass -File .\infra\gcp\add-runtime-secret-versions.ps1 `
  -Project generated-mote-467410-b0 `
  -RefreshProviderKeys
```

Không bật `-RefreshProviderKeys` nếu chỉ muốn thêm secret chưa có version. Script không cập nhật mật khẩu DB, JWT hay cấu hình thanh toán khi dùng switch này.

Xác nhận version mới nhất đang `enabled`, chỉ xem metadata:

```powershell
$Project = 'generated-mote-467410-b0'
@('goshopx-deepseek-api-key','goshopx-gemini-api-key','goshopx-tavily-api-key','goshopx-typesafe-api-key','goshopx-langsmith-api-key') |
  ForEach-Object {
    $state = gcloud secrets versions list $_ --project $Project --sort-by='~createTime' --limit=1 --format='value(state)'
    [pscustomobject]@{ Secret = $_; LatestState = $state }
  } | Format-Table -AutoSize
```

Đợi GKE SecretSync cập nhật Kubernetes Secret rồi kiểm tra **tên key, không đọc value**:

```powershell
kubectl describe secretsync goshopx-runtime -n goshopx
$runtime = kubectl get secret goshopx-runtime -n goshopx -o json | ConvertFrom-Json
$runtime.data.PSObject.Properties.Name | Sort-Object
```

Danh sách cần có: `AI_API_KEY`, `DEEPSEEK_API_KEY`, `GEMINI_API_KEY`, `TAVILY_API_KEY`, `TYPESAFE_API_KEY`, `LANGSMITH_API_KEY`.

Container lấy secret qua environment; sau khi SecretSync báo đồng bộ, cập nhật giá trị rollout qua GitOps (ví dụ annotation checksum Secret trong pod template) để tiến trình mới nạp version mới. Không restart Deployment trực tiếp vì Argo CD là owner của workload:

```powershell
kubectl get application goshopx-gcp-lab -n argocd -o wide
kubectl rollout status deployment/recommender-server -n goshopx --timeout=5m
kubectl get pods -n goshopx -l app.kubernetes.io/name=recommender-server
```

Lệnh `rollout status` chỉ quan sát. Thực hiện restart bằng thay đổi GitOps đã review, rồi đợi Argo `Synced/Healthy`.

Nếu selector không khớp, lấy label từ `kubectl get pods -n goshopx --show-labels` rồi dùng đúng deployment/label. Không in toàn bộ environment hay Secret.

## 2. Bật chế độ shadow và Jev rerank

Trong worktree hiện tại, cấu hình lab đã đặt `JEV_MODE=shadow` và bật `JEV_RERANK_ENABLED`. Shadow chỉ quan sát route; rerank có thể gọi provider nên vẫn tiêu thụ budget. Trần ledger giữ ở 200.000 VND:

```yaml
  JEV_MODE: shadow
  JEV_RERANK_ENABLED: "true"
  AI_SPEND_LIMIT_VND: "200000"
```

Release matrix cũng build/scan riêng image `recommender-ingestion`; GitOps promotion cập nhật digest của worker cùng các service khác. Sau khi thay đổi được commit/push, chờ CI đã cấu hình:

1. `CI quality gates` phải xanh.
2. `GCP lab image build, scan and GitOps promotion` phải xanh.
3. Merge PR promotion do workflow tạo.
4. Đợi Argo CD `Synced` và `Healthy`, deployment rollout hoàn tất. Không sửa digest trực tiếp trong cluster.

Lệnh kiểm tra:

```powershell
kubectl get application goshopx-gcp-lab -n argocd -o wide
kubectl get deploy,pods -n goshopx
```

## 3. Smoke từng provider, một lượt mỗi loại

Mở `https://goshopx.click` và thử theo thứ tự. Không chạy yêu cầu lặp khi response chậm; provider timeout có thể đã tính phí.

1. **DeepSeek text/tool**: hỏi bằng tiếng Việt “Tìm laptop dưới 20 triệu, RAM ít nhất 16 GB”. Xác nhận chỉ trả sản phẩm catalog và giá do catalog cung cấp.
2. **Jev route/rerank**: so sánh hai sản phẩm trong shortlist. Shadow không thay đổi lựa chọn route; rerank có thể đổi thứ tự nhưng không thêm sản phẩm ngoài retrieval.
3. **Tavily**: hỏi một câu kiến thức chung về tiêu chí chọn màn hình/laptop mà catalog không mô tả. Tavily chỉ trả tối đa ba URL HTTPS; không dùng web làm nguồn giá, kho, coupon hoặc tương thích.
4. **Multimodal**: tải một JPEG/PNG/WebP nhỏ, thử tìm mẫu tương tự. Ảnh kết quả phải là ảnh catalog; ảnh đầu vào được xử lý tạm thời.
5. **Gemini Embedding 2**: nếu cần backfill catalog, chỉ chạy `bootstrap-catalog` sau khi xác nhận quota/ledger; không chạy lặp khi thấy 429.
6. **LangSmith**: xác nhận project có trace metadata đã redact. Không bật tự gửi input/output thô.

## 4. Kiểm tra chi phí và dừng

Recommender hiện dùng PostgreSQL `PostgresSaver` của LangGraph cho checkpoint theo `thread_id` HMAC; startup tạo/cập nhật bảng checkpoint bằng `checkpointer.setup()`. Tài khoản database recommender cần quyền `USAGE, CREATE` trên schema `public`. Xóa hội thoại qua API/admin sẽ xóa checkpoint cùng thread. Nếu rollout log báo lỗi quyền bảng checkpoint, dừng rollout và cấp quyền schema cho đúng user `goshopx_recommender` trong database `recommender`, sau đó restart qua GitOps.

Trong Cloud SQL Studio, database `recommender`:

```sql
SELECT provider, status, COUNT(*) AS calls,
       SUM(input_tokens) AS input_tokens, SUM(output_tokens) AS output_tokens,
       SUM(reserved_vnd) AS reserved_vnd,
       SUM(COALESCE(actual_vnd, reserved_vnd)) AS counted_vnd
FROM ai_spend_entries
WHERE created_at >= CURRENT_TIMESTAMP - INTERVAL '7 days'
GROUP BY provider, status
ORDER BY provider, status;
```

Đối chiếu với billing từng nhà cung cấp; ledger là ước tính theo rate do vận hành nhập và không thay hóa đơn. Dừng smoke nếu ledger chạm trần, billing tăng bất thường, có 429 liên tiếp hoặc không xác định được chi phí. Không chạy tải/stress bằng khóa provider thật.

## 5. Kiểm tra worker ingestion và dữ liệu quản trị

Worker ingestion chỉ được bật sau khi bản Helm/GitOps mới đã được promote. Kiểm tra rollout, cấu hình GCS và lỗi gần nhất mà không in environment hoặc Secret:

```powershell
kubectl get application goshopx-gcp-lab -n argocd -o wide
kubectl get deploy,pods -n goshopx -l app.kubernetes.io/name=recommender-ingestion
kubectl rollout status deployment/recommender-ingestion -n goshopx --timeout=5m
kubectl logs deployment/recommender-ingestion -n goshopx --since=10m --tail=100
```

Nếu label không khớp, dùng `kubectl get deploy -n goshopx` để lấy tên deployment hiện tại. Upload tài liệu PDF/DOCX/TXT/MD/JSON/CSV qua **Admin → Dữ liệu AI**; PDF tối đa 200 trang và cần lớp văn bản, file scan phải OCR trước. Job phải về `completed`, số chunk lớn hơn 0; chỉ sau đó mới bấm **Xuất bản**. Kiểm tra lại bằng một câu hỏi có đáp án trong tài liệu. Job thất bại chỉ retry sau khi xử lý `errorCode`; không tải lại cùng tài liệu bằng idempotency key mới nếu job cũ còn chạy.

## 6. Xác minh ứng dụng trên GKE và domain

```powershell
kubectl get application goshopx-gcp-lab -n argocd -o wide
kubectl get deploy,pods,pvc,svc -n goshopx
kubectl get gateway,httproute -n goshopx
curl.exe https://goshopx.click/ -sS -o NUL -w "apex HTTP %{http_code}`n"
curl.exe https://www.goshopx.click/ -sS -o NUL -w "www HTTP %{http_code}`n"
```

Chỉ tiếp tục live smoke khi Argo `Synced/Healthy`, deployment cần thiết `Available`, HTTPS trả `200` và provider secret-sync đã cập nhật. Kết quả `curl /` không chứng minh đăng nhập, checkout hay chatbot hoạt động; các luồng này cần kiểm tra bằng trình duyệt và dữ liệu lab.

## 7. Tắt sau smoke và giới hạn hiện tại

- Đặt `JEV_MODE: off` và `JEV_RERANK_ENABLED: "false"` qua GitOps nếu không tiếp tục dùng; giữ trần chi phí.
- Chỉ xóa lịch sử chat từ UI/GraphQL có ownership. Secret rotation thực hiện theo policy riêng, không xóa Secret Manager bừa bãi.
- Admin document upload/job lifecycle/publish, no-provider chunk/cost preview, actor-attributed ingestion audit, and source-to-product linking are implemented for PDF/DOCX/TXT/MD/JSON/CSV; scan-only PDFs require OCR. Product-linked material is retrieved only when its product is shortlisted. Cart proposals require user confirmation; Order persists a non-consuming coupon quote with expiry and cart version, then rechecks coupon and price at order creation. Admin coupon lifecycle remains open. VNPAY retries reuse the pending order transaction if URL creation failed after transaction persistence.
- Native LangGraph checkpoint, provider invoice reconciliation, account/guest isolation under production auth, and public-browser acceptance remain separate follow-ups. The application currently persists owner-bound, compacted history in PostgreSQL; it does not resume an interrupted graph mid-node. WebP pixels are decoded and normalized in the current source. Offline tests do not prove live provider or GKE operation.
