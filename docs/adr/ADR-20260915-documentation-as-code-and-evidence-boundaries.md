# ADR-20260915-documentation-as-code-and-evidence-boundaries

Status: Accepted  
Date: 2026-09-15

## Context | Bối cảnh

GoshopX spans React, GraphQL, Go/gRPC services, Kafka, Python recommendations, multiple data stores, Compose, Helm/K3s and GitOps. A short README cannot safely preserve ownership, contracts, deployment, verification and known limits. Documentation that treats a rendered manifest, build or Argo health as end-to-end success can cause unsafe operating decisions.

GoshopX bao gồm React, GraphQL, Go/gRPC, Kafka, recommender Python, nhiều data store, Compose, Helm/K3s và GitOps. README ngắn không đủ để lưu giữ ownership, contract, deployment, verification và giới hạn. Tài liệu coi manifest render, build hoặc Argo health là E2E có thể dẫn tới vận hành không an toàn.

## Decision | Quyết định

Maintain versioned documentation beside source. README is the bilingual entry point, includes the supplied UI image and tech icons; `docs/27-technical-architecture-and-devops-guide.md` is the bilingual technical map. Existing focused documents and ADRs remain authoritative in their scope.

Every delivery statement distinguishes: (1) static/config validation, (2) build/type/unit validation, (3) container/endpoint validation, (4) cross-service/E2E validation, and (5) lab/production operational validation. Secrets are never written to documentation; use variable names or `[REDACTED_SECRET]`.

## Consequences | Hệ quả

Requirements can be traced to owner, contract, test and operational check, and reviewers can distinguish confirmed from unverified results. The cost is maintenance: any ownership, contract, topic, provider, data or deployment change updates its documentation/ADR in the same change.

## Alternatives considered | Phương án đã cân nhắc

- README only: rejected; it hides important consistency and operational constraints.
- Docs outside the repository: rejected; they drift and cannot be reviewed with source.
- Declarative success equals E2E: rejected; dependencies, credentials, browser and providers fail independently.

## Verification | Xác minh

README and guide use source paths and relative links, include `asset/images/UI.png`, and avoid secret values. Future delivery changes must execute applicable checks rather than copy historical success claims.
