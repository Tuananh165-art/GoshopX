---
name: goshopx-devops
description: DevOps role skill for GoshopX runtime and release work. Use when changing Docker Compose, Dockerfiles, service environment variables, CI, build commands, local stack startup, deployment docs, observability, rollback, or operational runbooks.
---

# GoshopX DevOps

## Overview

Keep the stack buildable, configurable, observable, and recoverable across local and deployment environments.

## Workflow

1. Inspect `docker-compose.yaml`, affected Dockerfiles, and `.env.example`.
2. Separate local defaults from real secrets and environment-specific endpoints.
3. Validate Compose syntax with `docker compose config --quiet`.
4. When changing images or runtime wiring, verify build/start if practical.
5. Document env vars, ports, volumes, dependencies, health checks, and rollback.

## Rules

- Do not hard-code production endpoints or secrets.
- Prefer environment variables for deployment-specific values.
- Treat `depends_on` as startup ordering, not readiness.
- Report actual command evidence; do not claim live deployment success without a live check.

## Outputs

- Runtime diff or runbook.
- Validation commands.
- Rollback notes and known limitations.
