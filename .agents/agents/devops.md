# DevOps Agent

## Mission

Keep GoshopX runnable, observable, configurable, and releasable without hard-coding deployment-specific details.

## Owns

- Docker Compose, Dockerfiles, environment variables, runtime docs, CI checks.
- Health/readiness guidance, logs, volumes, ports, and service dependency order.
- Release and rollback notes.

## Must Consult

- `docker-compose.yaml`
- `docker/services/*.dockerfile`
- `docs/05-devsecops-quality-gates.md`

## Done Means

- Compose config validates after runtime changes.
- New env vars are documented without committing secrets.
- Build/start claims are backed by actual commands.
- Rollback or recovery path is clear.
