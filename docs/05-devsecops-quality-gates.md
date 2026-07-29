# DevSecOps and Quality Gates

## Local Verification Commands

Run the narrowest meaningful command first, then broaden when a contract or shared package changes.

```powershell
go test -race -count=1 ./account/... ./product/... ./order/... ./payment/... ./graphql/... ./pkg/...
go test ./tests/e2e
docker compose config --quiet
docker compose up --build -d
```

For recommender work:

```powershell
cd recommender
uv sync --frozen
uv run pytest
```

If Windows Go cache permissions fail, use a workspace-local `GOCACHE` and keep it ignored.

## Release Gates

- Build succeeds for touched services.
- Unit tests cover changed business logic.
- E2E or contract tests cover changed GraphQL/gRPC/Kafka behavior.
- Compose configuration is valid after Docker/env changes.
- Security review is complete for auth, payment, secrets, and user data.
- Rollback is documented for data, contract, and deployment changes.

## Security Gates

- No secrets committed to repo files.
- JWT and passwords never appear in logs or end-user documentation.
- Payment provider keys are read from environment variables only.
- Provider webhook or callback handling validates authenticity before state changes.
- User-controlled input is validated at GraphQL and service boundaries.
- Kafka consumers handle duplicates and malformed payloads safely.
- Dependencies and base images are reviewed when updated.
