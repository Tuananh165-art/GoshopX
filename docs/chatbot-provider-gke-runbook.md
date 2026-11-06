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

## 2. Cấu hình Jev route và rerank

Giá trị đang triển khai trong profile GKE lab là `JEV_MODE=active` và `JEV_RERANK_ENABLED=true`: Jev có thể chọn route và sắp thứ tự tối đa 10 sản phẩm truy hồi. Router dự phòng xử lý lỗi hoặc điểm tin cậy thấp; Jev không được quyền thay bộ lọc sản phẩm, giá, kho hay coupon. Mỗi lần gọi đều tính vào ledger; smoke test dùng một lượt hội thoại, không lặp request. Trần ledger giữ ở 200.000 VND:

```yaml
  JEV_MODE: active
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
2. **Jev route/rerank**: yêu cầu so sánh các sản phẩm trong shortlist. Jev có thể chọn agent và đổi thứ tự; backend giữ nguyên shortlist và bộ lọc, không được thêm sản phẩm ngoài retrieval.
3. **Tavily**: hỏi một câu kiến thức chung về tiêu chí chọn màn hình/laptop mà catalog không mô tả. Tavily chỉ trả tối đa ba URL HTTPS; không dùng web làm nguồn giá, kho, coupon hoặc tương thích.
4. **Multimodal**: tải một JPEG/PNG/WebP nhỏ, thử tìm mẫu tương tự. Ảnh kết quả phải là ảnh catalog; ảnh đầu vào được xử lý tạm thời.
5. **Gemini Embedding 2**: chưa chạy khi API trả 429. Khi quota hoạt động, chạy `kubectl exec -n goshopx deployment/recommender-server -- python main.py rebuild-catalog-index` đúng một lần để tạo và activate generation mới; dừng nếu có 429 hoặc ledger chạm trần.
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

## Trạng thái xác minh gần nhất

Ngày 2026-11-07, Argo CD báo `Synced/Progressing` ở revision `84779da1db19b401c2c8321b30747af7b28f4356`. Chat, commerce và recommender deployments đều `1/1 Available`, nhưng Kibana 6.2.4 chưa Ready và đã bị `OOMKilled` 186 lần; Kong Ready với 30 lần restart trước đó. Bản Helm trong workspace đã thêm tùy chọn và tắt Kibana riêng cho GCP vì profile này dùng Grafana/Loki; thay đổi chưa được CI/GitOps triển khai. Read-only Cloud SQL xác nhận hiện có 0 vector generation và 0 ingestion job. Smoke trong pod gọi DeepSeek, Jev và Tavily thật; ledger giữ 3.500 VND dự phòng/ước tính do bảng rate provider chưa cấu hình (chưa phải số tiền hóa đơn). Năm lần gọi Gemini bị HTTP 429 và được ledger giải phóng; chưa có generation catalog active, nên dense retrieval và truy vấn ảnh chưa được nghiệm thu. GCS Workload Identity đã được kiểm tra bằng cách ghi và đọc lại PNG 68 byte từ pod recommender; object kiểm tra nằm dưới prefix lifecycle `chat-upload/`, nhưng chưa chờ xác nhận object tự hết hạn sau một ngày. Không chạy lại embedding cho tới khi quota Gemini được khôi phục.

Một trace LangSmith thử nghiệm đã được truy vấn lại trên project `goshopx-recommender`; payload chỉ có route, số ký tự, số sản phẩm, mã cảnh báo, latency và cờ `redacted`. Không gửi prompt/ảnh/ID phiên trong trace này. Trạng thái `Synced/Healthy` không thay thế nghiệm thu thao tác qua trình duyệt. Browser tool không có browser khả dụng; lần kiểm tra HTTPS từ shell này cũng không bắt tay được do Windows Schannel báo `SEC_E_NO_CREDENTIALS`. Trước đó người dùng đã xác nhận `curl.exe` từ máy của họ trả HTTP 200 cho apex và www, nhưng chatbot, admin ingestion, proposal/coupon quote và mua hàng vẫn cần kiểm tra bằng browser người dùng. GitHub release GCP cho các image gần nhất chưa được xác nhận kết thúc trong lượt này; các image đang chạy đều pin bằng digest và workload hiện Ready.

### Các bước còn lại để nghiệm thu live

1. Khôi phục quota/API access cho Gemini Embedding 2; không gửi key qua chat. Xác minh model/quota/project trong Google AI Studio hoặc Cloud Console, rồi chạy đúng một smoke text theo mục 3.
2. Nếu smoke pass, chạy một lần `kubectl exec -n goshopx deployment/recommender-server -- python main.py rebuild-catalog-index`, theo dõi ledger và chờ registry có generation `active`. Image đặt working directory `/app`; lệnh này rebuild và activate generation mới. Dừng nếu gặp 429, không retry lặp.
3. Từ trang **Admin → Dữ liệu AI**, upload một file lab không nhạy cảm, preview, enqueue, chờ job `completed`, kiểm tra chunk count rồi publish. Sau đó thử retrieval bằng một câu chỉ tài liệu đó trả lời được; hiện database có 0 job nên thao tác này chưa được chứng minh live.
4. Trên browser thật, xác nhận tiếp tục chat sau reload, tải ảnh sản phẩm, xem proposal trước khi xác nhận, thử mã coupon và kiểm tra checkout preview. Không tạo đơn hoặc thanh toán thật để nghiệm thu.
5. Sau khi GitOps triển khai bản Helm tắt Kibana trong GCP, xác nhận Argo `Healthy` và namespace không còn pod Kibana. Theo dõi `kubectl logs deployment/kong` vì pod có lịch sử restart cao; xác nhận object GCS smoke tự hết hạn sau một ngày nếu cần kiểm tra lifecycle end-to-end.
