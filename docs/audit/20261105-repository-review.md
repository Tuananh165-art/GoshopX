# Repository review — 2026-11-05

## Scope and result

This review covers the changed recommender/chat path, GraphQL upload and COD checkout, GKE Helm profile, Terraform project files, release workflow and related documentation. It is a source review plus targeted checks; it is not a line-by-line audit of every legacy service, migration, dependency or live deployment.

| Finding | Change / current status | Evidence |
| --- | --- | --- |
| Uploaded image MIME trusted without checking payload bytes; image retained in browser localStorage | GraphQL now checks `http.DetectContentType` and caps at 5 MiB; browser history drops image data | Go upload tests and web typecheck |
| COD checkout changed order state to `paid` immediately | Removed the status update; new COD order remains `pending` | GraphQL test asserts pending status |
| Chat LangGraph had only guard/respond | Added deterministic discovery, compare, cart-advice and multimodal routes | Recommender unit tests |
| Gemini vectors must not share old 512-dimensional hash collection | Added Gemini text and catalog-image embedding adapters and a fresh 768-dimensional GCP collection name | Recommender tests; provider/Milvus integration not run |
| GCP lab manifests exposed operator NodePorts and depended on local-path | Added separate GCP Helm values with NodePorts disabled, Cloud SQL proxy and `premium-rwo`; left chart defaults for local use | Helm binary not present locally; render added to CI workflow |
| GCP project, artifact publishing and teardown instructions missing | Added Terraform project, GKE, Cloud SQL, registry, WIF, storage, secret containers, budget and teardown docs | `terraform fmt` and provider-backed `terraform validate` pass |
| Provider-backed Terraform plan | Blocked because Application Default Credentials are not configured; no cloud changes were applied | Plan error from ADC lookup |
| TLS for apex and `www` | Terraform now requests separate DNS authorizations for both hostnames; Gateway and HTTPRoute accept both | Terraform validation and Helm YAML syntax check; certificate issuance remains live-unverified |
| Billing budget/credit check | Billing account is open and currency is VND. Budget listing API is disabled on the existing project; credit expiry must be checked in Billing UI before apply | Read-only `gcloud billing accounts describe`; user supplied credit screenshot |

## Important unclosed risks

- Secret Manager sync is documented in `docs/gcp/deploy-step-by-step.md` using GKE Secret Sync, but it is still an operator-applied prerequisite rather than Terraform/Helm-managed state. Terraform secret IDs also need the full Helm key inventory and versions before workload deploy.
- Cloud SQL databases exist without SQL users, per-service passwords, DB URL versions or migration run. Provision those outside Terraform state and map each service separately before rollout.
- `values-gcp-lab.yaml` now contains GKE Gateway/HTTPRoute templates with Certificate Manager and a named static IP, but these have not been reconciled against a live cluster.
- Cloudflare records are not automatically managed. Domain cutover remains manual and must retain rollback targets.
- The existing K3s profile file was changed while tightening the lab exposure. Automatic review rejected restoring the old public NodePorts. The existing operator runbook may no longer match that profile; obtain approval before re-enabling any public admin port.
- Cart action proposals/confirmation, coupon ownership and transactional outbox/race-condition work were not implemented. COD status was fixed, but distributed checkout semantics require a broader audit and tests.
- Elasticsearch remains pinned to legacy 6.2.4/olivere v5; no repository-wide migration to Elasticsearch 9.5.6 was attempted.
- KMS image signing and Binary Authorization enforcement, Tavily, Jev System One, LangSmith redaction, provider spend accounting and rate/token limits are not wired.

## Check evidence

- Recommender: 27 `unittest` tests passed on Python 3.11 with `PYTHONPATH=app`; this includes the Gemini REST image payload regression check.
- Go: all unit packages passed with `go test` excluding `tests/e2e`; `go vet` passed. The end-to-end package waits for a running stack and was not run. A Go module-cache stat warning appeared in the sandbox run; checks passed with local cache access.
- Web: 7 Vitest tests passed and `npm run build` passed. One build emitted a warning that the chat page is imported both dynamically and statically, so it cannot be split into a separate chunk. The user's Node 24.11 install triggered a jsdom engine warning; `web/.nvmrc` pins Node 22.23.4 and package engines allow supported Node 22.22.2+ below 23. `npm ci` reported 6 advisories once; a follow-up `npm audit --json` on the same lockfile reported 0, so rerun audit under the pinned Node/registry before release.
- Terraform: provider `hashicorp/google` v7.46.2 initialized; `terraform fmt -check -recursive` and `terraform validate` passed. `terraform plan` stopped before API evaluation because local ADC is missing.
- No GCP resource apply, GKE rollout, public domain change, live provider request, load test, or hosted workflow run occurred.
