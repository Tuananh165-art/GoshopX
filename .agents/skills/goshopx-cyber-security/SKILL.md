---
name: goshopx-cyber-security
description: Cyber Security role skill for GoshopX secure engineering. Use when reviewing authentication, authorization, JWTs, passwords, payment provider flows, webhook/callback handling, secrets, PII, Docker/CI supply chain, dependency risk, input validation, or threat models.
---

# GoshopX Cyber Security

## Overview

Review changes for concrete abuse paths and enforce controls appropriate for e-commerce, payment, account, and recommendation data.

## Workflow

1. Identify assets: accounts, tokens, passwords, product ownership, orders, payment sessions, provider keys, events, and logs.
2. Identify trust boundaries: GraphQL client input, gRPC calls, Kafka messages, databases, provider redirects/callbacks, Docker images.
3. List threats: spoofing, tampering, replay, information disclosure, denial of service, privilege escalation, and supply-chain compromise.
4. Check controls: authn/authz, validation, secret handling, signature verification, idempotency, logging hygiene, dependency updates.
5. Recommend tests or code changes for material risks.

## Hard Rules

- Never commit secrets or live credentials.
- Never expose JWTs, passwords, provider keys, or internal service URLs in UI/docs/logs.
- Payment state changes need provider authenticity and replay/idempotency review.
- Kafka consumers must tolerate malformed and duplicate messages.
- Authenticated GraphQL operations must derive identity from trusted auth context, not client-provided IDs alone.

## Outputs

- Threat model.
- Findings with severity and affected file/flow.
- Required controls and verification evidence.
