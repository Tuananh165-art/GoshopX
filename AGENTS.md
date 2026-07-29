# Repository Guidelines

## Project Structure & Module Organization
This repo is a Go microservices stack with a Python recommender service. Core services live in `account/`, `product/`, `order/`, `payment/`, and `graphql/`. Shared Go helpers are under `pkg/`. The recommender service is under `recommender/`, including its Python app, generated code, and `pyproject.toml`. Unit tests sit beside each service in `*/tests/`, while end-to-end coverage lives in `tests/e2e/`. Docker orchestration is defined in `docker-compose.yaml` and `docker/services/*.dockerfile`.

## Build, Test, and Development Commands
- `docker compose up --build -d`: build images and start the full stack locally.
- `go test -race -count=1 $(go list ./... | grep -v '/tests/e2e$')`: run Go unit tests with race detection.
- `go test ./tests/e2e`: run the end-to-end suite.
- `cd recommender && uv sync --frozen`: install Python dependencies from the locked environment.

## Coding Style & Naming Conventions
Use standard Go formatting with `gofmt` before committing. Keep Go package and file names lowercase and descriptive, and follow the existing layout: executable entrypoints in `cmd/<service>/main.go`, business logic in `internal/`, and tests named `*_test.go`. Python code in `recommender/` should follow the existing `uv`/`pyproject.toml` setup and the module layout already in place.

## Testing Guidelines
Prefer focused unit tests for service logic and repository behavior, then add e2e coverage only when a change crosses service boundaries. The CI pipeline runs `go test -race -count=1` across Go packages and excludes `tests/e2e/`, so keep new tests compatible with that pattern. Name new test files after the package they cover and keep assertions close to the behavior under test.

## Commit & Pull Request Guidelines
Recent history uses conventional prefixes such as `fix(recommender): ...`, `chore(docker): ...`, and `refactor(recommender): ...`. Follow the same style: short scope, imperative summary, and a clear reason for the change. Pull requests should explain what changed, why it changed, and how it was verified. Include screenshots or logs only when they help demonstrate a visible change or runtime fix.

## Security & Configuration Tips
Do not commit secrets or local runtime state. Keep environment-specific values in `.env` or your shell, and treat Docker/Kafka/Postgres credentials in Compose as local defaults only. If you touch the recommender or Docker setup, verify the full stack still starts cleanly with `docker compose up --build -d`.
