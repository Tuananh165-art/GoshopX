# Hybrid RAG Recommendation Chatbot Implementation Plan

> **For Hermes:** Use this plan task-by-task with tests and runtime verification before claiming completion.

**Goal:** Xây dựng chatbot recommendation fullstack cho GoshopX: nhận text và ảnh, hiểu nhu cầu/sở thích người dùng, hybrid-search sản phẩm trong Milvus, dùng LangChain + RAG để tạo câu trả lời có căn cứ, trả cả text và ảnh sản phẩm, đồng bộ với toàn bộ microservices hiện có và chạy bằng các container Python riêng.

**Architecture:** GraphQL là public client boundary. GraphQL gọi recommender qua gRPC nội bộ; recommender gồm API/chat orchestration, Kafka consumer/indexer và worker đồng bộ catalog/interactions. Product metadata được lưu trong PostgreSQL replica hiện tại và index đa trường trong Milvus: dense text, dense image (hoặc multimodal embedding), lexical/BM25 và metadata filters. LLM chỉ được dùng để parse intent, rewrite query, summarize kết quả đã retrieve và phát hiện preference; không được tự tạo product/price/stock/image URL ngoài context.

**Tech Stack:** Python 3.11 + uv/venv, LangChain/LangGraph (chỉ dùng khi có ích), FastAPI hoặc mở rộng gRPC hiện tại cho chat, pymilvus/LangChain Milvus, Kafka, PostgreSQL, MinIO, OpenAI-compatible chat endpoint, local CLIP/OpenCLIP hoặc model embedding tương thích cho ảnh, Go gqlgen, React/Vite, Docker Compose.

---

## 1. Phạm vi và trạng thái hiện tại

Đã khảo sát codebase:

- `recommender/` hiện là Python service dùng implicit ALS, similar-products và popular fallback; chưa có LangChain, Milvus, chatbot, multimodal input hay image fields trong replica.
- API hiện tại trong `recommender/recommender.proto` chỉ có `GetRecommendations` và `GetRecommendationsBasedOnViewed`, response chỉ có id/name/description/price.
- `graphql/schema.graphql` đã có Product với `media`, `thumbnail`, `images`, tags, brand, category, stock/rating; chưa có chat/recommendation conversation contract.
- `graphql/graph/query.go` đang gọi recommender cho user/viewed products nhưng map mất image và metadata.
- Kafka đã có `product_events` và `interaction_events`; `recommender/app/entry/kafka_consumer.py` đồng bộ product cơ bản vào PostgreSQL replica.
- `docker-compose.yaml` đã có các container `recommender-server`, `recommender-sync`, `recommender-train`, nhưng chưa có Milvus/etcd/minio riêng cho vector DB và chưa truyền AI/embedding settings.
- `docker/services/recommender.dockerfile` đã dùng Python 3.11 và uv, là nền tảng phù hợp.
- `.env` bị ignore. Chỉ thêm tên biến vào `.env.example` và truyền runtime secret qua `.env`/secret manager; không commit key.

## 2. Quyết định an toàn bắt buộc về API key

API key đã được đưa trực tiếp trong hội thoại và phải coi là đã lộ. Trước khi triển khai thật, revoke/rotate key tại nhà cung cấp và tạo key mới. Plan chỉ thêm các biến sau vào `.env.example` với giá trị rỗng hoặc placeholder:

```env
AI_ENDPOINT=https://opencode.ai/zen/go/v1/chat/completions
AI_API_KEY=
AI_MODEL=deepseek-v4-flash
AI_TIMEOUT_SECONDS=30
AI_MAX_RETRIES=2

EMBEDDING_PROVIDER=local
TEXT_EMBEDDING_MODEL=sentence-transformers/all-MiniLM-L6-v2
IMAGE_EMBEDDING_MODEL=openai/clip-vit-base-patch32
MILVUS_URI=http://milvus:19530
MILVUS_TOKEN=
MILVUS_COLLECTION=product_catalog_v1
```

Không in key trong log, test output, plan, commit hoặc GraphQL response. Container nhận `AI_API_KEY` từ `.env` runtime; production ưu tiên Docker secrets/KMS.

---

## 3. Nghiệp vụ recommendation cần chốt

### 3.1 Input và intent

Hỗ trợ:

- Text tự nhiên tiếng Việt/Anh: nhu cầu, ngân sách, category, brand, màu, kích thước, use case, mức rating.
- Ảnh upload hoặc URL hợp lệ: tìm sản phẩm tương tự theo visual embedding; kết hợp text nếu user gửi cả hai.
- Context user: account ID từ JWT, lịch sử view/click/cart/purchase/review, session ID cho anonymous user, các sản phẩm đã xem để tránh lặp.
- Intent: `search`, `recommend`, `similar`, `compare`, `explain`, `refine`, `out_of_scope`.

LLM parse thành structured `SearchPlan`, nhưng mọi filter quan trọng phải validate bằng Pydantic và allow-list; không cho LLM tự sinh SQL/Milvus expression.

### 3.2 Eligibility và ranking business rules

Trước ranking phải lọc cứng:

1. Chỉ sản phẩm `publishStatus=published` và `moderationStatus=approved`.
2. Không recommend sản phẩm đã bị xóa, không bán hoặc không còn available; có thể cho phép low-stock nhưng phải cảnh báo.
3. Không expose sản phẩm của seller/account bị suspended.
4. Giá, currency, tồn kho lấy từ source of truth/product + inventory service, không lấy giá do LLM suy đoán.
5. Chặn sản phẩm user vừa purchase hoặc đã explicit dislike; giảm lặp sản phẩm đã view.
6. Nếu thiếu dữ liệu hoặc query không rõ, hỏi tối đa 1 câu clarification hoặc trả popular/trending có giải thích rõ là fallback.
7. Không đưa claim y tế/pháp lý/hiệu năng không có trong catalog.

Hybrid score dự kiến: `0.45 dense_text + 0.20 dense_image + 0.15 lexical + 0.10 behavioral + 0.10 business`; trọng số cấu hình được, normalize score trước khi combine. Khi không có ảnh, phân bổ lại image weight vào text/lexical. Khi không có lịch sử user, behavioral weight chuyển sang popularity/rating.

### 3.3 Response contract

Response phải gồm:

- `answer`: text tiếng Việt mặc định, ngắn gọn, grounded.
- `products[]`: `id`, name, description snippet, price, currency, thumbnail, images/media, score, matchedReasons, availability, rating.
- `appliedFilters`, `queryInterpretation`, `nextSuggestions`, `conversationId`, `traceId`.
- `citations`/source product IDs để audit grounding.
- `warnings` khi ảnh không đọc được, hết hàng, fallback hoặc kết quả ít.

Không trả URL ảnh do model tự tạo. Image URL phải lấy từ product service/MinIO và kiểm tra allow-list host.

---

## 4. Kế hoạch triển khai theo task nhỏ

### Task 1: Chốt contracts và ADR

**Files:** Create `docs/adr/XXXX-hybrid-rag-recommender.md`, `recommender/docs/recommendation-contract.md`.

- Ghi rõ data ownership: product service là source of truth catalog; inventory service là source of truth stock; account/auth là source of truth identity; recommender chỉ giữ replica/index.
- Chốt anonymous session, retention, consent, delete/export user data, timeout và fallback.
- Tạo JSON examples cho text-only, image-only, mixed input, no-result, unauthorized.

**Verify:** review schema examples bằng JSON parser/Pydantic test.

### Task 2: Chuẩn hóa Python project và dependency lock

**Files:** Modify `recommender/pyproject.toml`, `recommender/uv.lock`; Create `recommender/app/shared/config/ai.py` và `recommender/app/shared/config/vector.py`.

- Thêm LangChain core/community, LangGraph nếu cần state machine, Pydantic v2, FastAPI/httpx hoặc dependency phù hợp với transport đã chọn, pymilvus/LangChain Milvus.
- Chọn text embedding chạy local để không gửi catalog/user data ra ngoài; image embedding local CLIP ở MVP. Nếu model image quá nặng, tách image-index worker hoặc chọn model configurable.
- Settings fail-fast cho production khi thiếu `AI_API_KEY`, nhưng unit tests dùng fake provider.
- Dùng `uv sync --frozen`; không sửa lock thủ công.

**Tests:** settings validation, redaction, fake LLM/embedding provider.

### Task 3: Mở rộng product replica và Kafka normalization

**Files:** Modify `recommender/app/shared/db/models.py`, `repo.py`, `schema.py`, `kafka_consumer.py`, `shared/product/utils.py`; tests under `recommender/tests/`.

- Bổ sung category, brand, SKU, tags, discount, rating, stock/availability, thumbnail, images, media, publish/moderation status, timestamps và version/event ID.
- Normalize cả product-created/updated từ Go event và product API; backward-compatible với payload cũ chỉ có 5 field.
- Idempotency theo event ID/version; delete phải xóa replica và vector document.
- Không dùng PostgreSQL replica làm source cho realtime inventory nếu stale; index availability chỉ là filter/cache và phải revalidate trước trả response.

**Tests:** old/new event payload, idempotent replay, delete, missing optional image fields.

### Task 4: Thiết kế Milvus collection/index lifecycle

**Files:** Create `recommender/app/vector/milvus_client.py`, `schema.py`, `filters.py`, `upsert.py`, `delete.py`, `rebuild.py`.

- Collection versioned `product_catalog_v1` với primary product ID, metadata scalar, text dense vector, image dense vector, lexical/BM25 field hoặc sparse vector.
- Index HNSW/IVF tùy benchmark; metric COSINE cho dense và BM25/sparse metric tương ứng.
- Có collection alias/current version để rebuild blue-green; không drop collection production khi index lỗi.
- Metadata filter cho publish/moderation/category/brand/price/rating/availability; giá trị filter được bind/validate, không ghép query raw từ LLM.
- Batch upsert, retry, dead-letter log, checksum/version để detect stale document.

**Tests:** schema creation idempotent, filter compiler allow-list, upsert/delete/rebuild với fake Milvus hoặc test container.

### Task 5: Xây ingestion pipeline cho text và image

**Files:** Create `recommender/app/indexing/embedding.py`, `document_builder.py`, `index_worker.py`; modify consumer/entrypoint.

- Tạo canonical document chứa title, description, category, tags, brand, normalized price text, rating, shipping/warranty và image references.
- Text embedding từ canonical text; image embedding từ thumbnail/images qua HTTP timeout, content-type/size limit, cache theo URL hash.
- Lưu metadata và image URL, không nhúng binary vào LLM prompt; chỉ gửi ảnh user tới vision-capable provider nếu capability được xác nhận, nếu không dùng local image embedding.
- Product event chỉ enqueue index job; retry có backoff và DLQ, tránh block Kafka consumer.
- Có `python main.py index-rebuild` cho full rebuild từ product API/replica.

**Tests:** deterministic document, malformed image, timeout, duplicate event, partial image failure vẫn index text.

### Task 6: Implement hybrid retriever và reranker

**Files:** Create `recommender/app/retrieval/hybrid.py`, `query_parser.py`, `reranker.py`, `schemas.py`.

- Parse prompt thành `SearchPlan`; keyword extraction fallback nếu LLM timeout.
- Chạy dense text, dense image và lexical/BM25 song song; merge bằng reciprocal-rank fusion hoặc normalized weighted score.
- Apply hard eligibility filter trước combine; behavioral candidates lấy từ ALS/current interaction logic và merge sau retrieval.
- Rerank top N bằng business score, diversity/category cap, dedup variant, stock/rating/price rules.
- Luôn giữ `matchedReasons` từ field match/metadata, không để LLM tạo lý do không có evidence.

**Tests:** hybrid ranking deterministic, image-only, text-only, price filter, out-of-stock exclusion, empty Milvus fallback.

### Task 7: Xây RAG orchestration và guardrails

**Files:** Create `recommender/app/chat/graph.py`, `llm_client.py`, `prompts.py`, `grounding.py`, `guardrails.py`.

- Flow: normalize input -> identify user/session -> parse intent -> retrieve -> filter/rerank -> build compact context -> call OpenAI-compatible chat endpoint -> validate structured output -> grounding check -> response.
- Dùng LangChain `ChatOpenAI`/OpenAI-compatible adapter với `base_url=AI_ENDPOINT` theo API compatibility thực tế; không hardcode key/model.
- Structured output phải chứa product IDs thuộc retrieved set; nếu vi phạm thì replace bằng template deterministic hoặc retry một lần.
- Prompt injection defense: coi catalog/user text là untrusted data, không thực thi instructions trong description/image OCR.
- Timeout budget tổng < 3s cho normal recommendation (configurable), degrade gracefully nếu LLM/Milvus down.
- Cache query-safe results; không cache personalized response sai user.

**Tests:** fake LLM output valid/invalid, hallucinated product rejection, timeout fallback, prompt injection fixture, PII redaction.

### Task 8: Mở rộng gRPC API và generated code

**Files:** Modify `recommender/recommender.proto`; regenerate `recommender/generated/pb/*` and Go generated client; modify `recommender/client/client.go`, `app/entry/grpc_server.py`.

- Add `ChatRecommendation` request: text, image bytes or approved image URL, user ID, session ID, viewed IDs, locale, pagination, filters, conversation ID, request ID.
- Add response product media/availability/score/reasons and answer/warnings/trace ID.
- Giới hạn payload ảnh, upload size, accepted mime, deadline; không log bytes/token.
- Giữ hai RPC cũ backward-compatible trong transition.

**Verify:** protobuf regeneration command, Go compile, Python gRPC smoke test.

### Task 9: Expose GraphQL public boundary

**Files:** Modify `graphql/schema.graphql`, `graphql/graph/query.go`, create/update GraphQL generated files via gqlgen, modify `graphql/graph/graph.go`, `graphql/config/*` nếu cần.

- Add `recommendChat(input: RecommendationChatInput!): RecommendationChatResponse!` mutation/query theo decision ở ADR.
- Derive authenticated account ID from JWT; client không được override user ID. Anonymous client chỉ được session ID opaque.
- GraphQL resolver stream hay non-stream trước tiên; MVP non-stream để ổn định, chuẩn bị field/cursor cho streaming sau.
- Map full product media/image fields bằng product service hoặc enrich từ source service; không làm mất data như path hiện tại.
- Add auth/rate limit/size validation and request ID propagation.

**Tests:** gqlgen resolver, auth ownership, anonymous flow, invalid image, response contract.

### Task 10: UI chatbot fullstack

**Files:** Create `web/src/features/recommendation/ChatbotPage.tsx`, components/hooks/types/tests; modify `web/src/app/App.tsx`, `web/src/shared/api/graphql.ts`, navigation/layout.

- Chat input text + image picker/drag-drop, preview/remove, loading/stream state, retry, empty/error/fallback state.
- Render answer, product cards/carousel, thumbnail/gallery, price/stock/rating/reasons, CTA xem chi tiết/thêm giỏ.
- Persist conversation ID/session ID appropriately; không lưu raw sensitive image lâu hơn policy.
- Mobile accessibility: keyboard, alt text, size validation, no broken image fallback.
- Track view/click/add-to-cart/purchase through existing Kafka interaction contract; do not count recommendation impression as purchase.

**Tests:** Vitest component tests, GraphQL mocked tests, Playwright text/image/no-result/mobile flows.

### Task 11: Milvus + service container orchestration

**Files:** Modify `docker-compose.yaml`, `docker/services/recommender.dockerfile`, create `docker/services/milvus` config if needed, `.env.example`, README/docs.

- Add Milvus standalone plus etcd and Milvus object storage dependency theo official compose-compatible topology; không reuse recommender PostgreSQL as vector DB.
- Add healthchecks and `depends_on` readiness, persistent volume `milvus_data`; preserve existing volumes.
- Split image names/commands rõ ràng: `recommender-server`, `recommender-sync`, `recommender-index`, `recommender-train` nếu worker indexing khác lifecycle.
- Pass `AI_*`, `MILVUS_*`, embedding settings, MinIO settings only to containers cần dùng.
- Build riêng image Python recommender, ensure non-root where feasible, pinned base/dependency versions, no secret baked in layer.
- Verify Docker network: GraphQL -> recommender gRPC; sync -> Kafka/Milvus; recommender -> Milvus/Postgres/MinIO/AI endpoint.

**Tests:** `docker compose config`, image build, healthchecks, restart persistence, no secret in image history.

### Task 12: Observability, evaluation và data governance

**Files:** Create `recommender/app/observability/*`, `recommender/evaluation/*`, docs/runbooks.

- Metrics: retrieval latency by branch, Milvus errors, LLM latency/tokens/errors, fallback rate, no-result rate, CTR, add-to-cart rate, conversion, diversity, stale index count.
- Trace IDs xuyên GraphQL/gRPC/Kafka/LLM; redact prompt/image/PII. Không log API key.
- Offline evaluation dataset anonymized: Recall@K, NDCG@K, MRR, grounded-answer rate, latency p95, image/text query slices, cold start.
- Online A/B chỉ sau guardrail; define rollback to existing ALS/popular RPC.
- GDPR-like deletion: delete user interaction data/index personalization cache by account; retention policy and admin runbook.

**Tests:** metric emission, redaction, golden grounded responses, load test with mocked AI/Milvus.

### Task 13: End-to-end acceptance và rollout

**Files:** Create/modify `tests/e2e/*`, `recommender/tests/integration/*`, deployment docs.

- Seed catalog có full images/media/stock and deterministic interactions.
- Run text query, image query, mixed query, logged-in personalization, anonymous cold start, price/category filters, unavailable product, AI outage, Milvus outage, Kafka replay.
- Verify frontend -> Kong -> GraphQL -> gRPC -> Milvus/LLM -> response images; verify click/cart/purchase events return to recommender.
- Canary behind feature flag; rollback switch to existing `product` search/ALS path.

**Required commands:**

```bash
cd /mnt/f/GoshopX/recommender
uv sync --frozen
uv run pytest -q

cd /mnt/f/GoshopX
go test -race -count=1 $(go list ./... | grep -v '/tests/e2e$')
go test ./tests/e2e

docker compose config
docker compose build recommender-server recommender-sync recommender-train graphql web
# After implementing Milvus services:
docker compose up --build -d
# Verify real runtime:
docker compose ps
docker compose logs --no-color recommender recommender-sync graphql milvus
```

Do not claim success from build output alone: execute a real GraphQL chatbot request with text, a multipart/image request through the selected boundary, and inspect returned product IDs plus image URLs. Confirm Kafka event consumption and Milvus document count with authenticated admin tooling.

---

## 5. Files dự kiến thay đổi

### New

- `.hermes/plans/...hybrid-rag-recommendation-chatbot.md`
- `docs/adr/....md`
- `recommender/app/vector/*`
- `recommender/app/indexing/*`
- `recommender/app/retrieval/*`
- `recommender/app/chat/*`
- `recommender/app/observability/*`
- `recommender/evaluation/*`
- `web/src/features/recommendation/*`
- integration/e2e tests và runbook.

### Modify

- `recommender/pyproject.toml`, `recommender/uv.lock`, `recommender/recommender.proto`, generated protobuf files.
- `recommender/app/shared/config/*`, DB models/repo/schema, Kafka consumer, gRPC server, existing recommendation service.
- `graphql/schema.graphql`, gqlgen generated output, GraphQL server/resolvers/client.
- `docker-compose.yaml`, `docker/services/recommender.dockerfile`, `.env.example`, docs.

Không sửa destructive các database volume hiện tại và không commit `.env`.

## 6. Open questions cần quyết định trước implementation

1. AI endpoint/model có hỗ trợ vision input và OpenAI-compatible `base_url`/structured output không? Nếu chưa xác nhận, image retrieval dùng local CLIP và LLM chỉ nhận text/structured product context.
2. Chọn Milvus standalone hay Zilliz Cloud cho môi trường production; local Compose dùng standalone.
3. Ảnh user nhận bằng GraphQL Upload, MinIO presigned URL hay URL đã tồn tại? MVP ưu tiên bounded GraphQL upload rồi xử lý memory/temp file an toàn.
4. Có cần streaming token không? MVP non-stream sẽ giảm thay đổi Kong/GraphQL.
5. Policy retention cho conversation và ảnh user là bao lâu? Mặc định không lưu binary, lưu metadata/trace ngắn hạn.
6. Product event payload hiện có đầy đủ image/category/availability hay cần product API fetch/enrichment? Cần xác minh bằng sample event/runtime trước Task 3.
7. Embedding model nào được phép chạy trong image production và CPU/RAM budget của WSL/Docker là bao nhiêu?

## 7. Definition of Done

- Catalog và interaction sync idempotent; product update/delete phản ánh trong Milvus.
- Text/image/mixed input chạy được qua GraphQL và trả grounded text + product images/media.
- Hard business rules không bị LLM bypass; stock/price được enrich từ source of truth.
- Existing recommendation RPC/frontend không bị regression.
- Python unit/integration, Go unit, GraphQL, web component/e2e và Docker smoke tests pass.
- `docker compose up --build -d` healthy với Milvus persistent volume; restart không mất index.
- Secrets không xuất hiện trong git diff, image layer, logs, test fixtures hoặc response; API key đã rotate nếu từng dùng key đã gửi trong hội thoại.
- Có metrics, trace ID, fallback/rollback, runbook và offline evaluation baseline.
