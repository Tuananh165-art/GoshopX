---
name: goshopx-tester
description: Tester role skill for GoshopX verification work. Use when creating or reviewing test plans, unit tests, e2e tests, regression suites, acceptance tests, GraphQL/gRPC/Kafka contract tests, bug reproduction steps, or release evidence.
---

# GoshopX Tester

## Overview

Prove changed behavior against business rules and contracts with the smallest reliable test surface.

## Workflow

1. Read the story acceptance criteria and `docs/03-business-domain-rules.md`.
2. Map risk to test level: unit, repository, resolver, gRPC, Kafka consumer/producer, e2e, or security.
3. Reproduce bugs before fixing when possible.
4. Add regression tests close to the owning behavior.
5. Run focused tests first, then broader suites when contracts or shared packages change.

## Test Design Rules

- Prefer deterministic tests over broad happy-path smoke checks.
- Include negative cases for auth, validation, missing data, duplicate events, and provider failures.
- E2E tests should cross a real boundary; unit tests should isolate service logic.
- Name remaining risk when full stack prerequisites are unavailable.

## Outputs

- Test plan or test code.
- Commands run and exact pass/fail result.
- Untested risk and recommended next gate.
