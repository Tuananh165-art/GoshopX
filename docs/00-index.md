# GoshopX Documentation Index

This folder is the source of truth for planning and operating GoshopX with BMAD and Agile/Scrum discipline.

## Core Documents

- [BMAD Operating Model](./01-bmad-operating-model.md): role workflow from business idea to verified release.
- [Agile/Scrum Delivery Guide](./02-agile-scrum-delivery.md): ceremonies, artifacts, definition of ready, and definition of done.
- [Business Domain Rules](./03-business-domain-rules.md): e-commerce business vocabulary, invariants, and acceptance rules.
- [Architecture Decision Guide](./04-architecture-decision-guide.md): current service boundaries, data ownership, and decision templates.
- [DevSecOps and Quality Gates](./05-devsecops-quality-gates.md): validation commands, release gates, and security checks.
- [Core Commerce Epic](./06-core-commerce-epic.md): product vision, scope, actors, and outcome for inventory, cart, and notification.
- [Core Commerce Backlog](./07-core-commerce-backlog.md): epic breakdown, sprint-ready stories, priorities, and Scrum planning.
- [Core Commerce Spec](./08-core-commerce-spec.md): business rules, acceptance criteria, contracts, and failure modes.
- [Runtime Configuration and Local Stack](./09-runtime-configuration.md): `.env`, Compose wiring, Redis/Kafka/Postgres notes, and local recovery steps.

## Architecture Decisions

- [ADR-20260729 Core Commerce Services](./adr/ADR-20260729-core-commerce-services.md): service boundaries, sync/async paths, and alternatives rejected.

## Repo-Local Agent Assets

- `.agents/rules/project-rules.md`
- `.agents/memory/goshopx-project-memory.md`
- `.agents/agents/*.md`
- `.agents/skills/*/SKILL.md`

Use these files together: docs describe the project process, rules define constraints, memory preserves stable context, agents define responsibilities, and skills define repeatable execution behavior.
