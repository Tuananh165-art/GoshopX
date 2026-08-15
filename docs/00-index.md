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
- [Admin Governance Epic](./10-admin-governance-epic.md): role-based governance vision, actors, scope, and release outcome.
- [Admin Governance Backlog](./11-admin-governance-backlog.md): sprint-ready admin stories, priorities, and Scrum controls.
- [Admin Governance Spec](./12-admin-governance-spec.md): role matrix, GraphQL/gRPC/Kafka contracts, rules, tests, and rollout.
- [Admin Implementation Evidence](./13-admin-implementation-evidence.md): delivered slices, verification evidence, and environment limits.
- [Admin Commerce Epic](./14-admin-commerce-epic.md): catalog, order, payment, inventory, dashboard, audit, and reporting scope.
- [Admin Commerce Backlog](./15-admin-commerce-backlog.md): prioritized sprint stories and dependencies.
- [Admin Commerce Spec](./16-admin-commerce-spec.md): business rules, metrics, contracts, and projection rules.
- [K3s DevSecOps Lab Runbook](./22-devsecops-k3s-lab-runbook.md): resource-bounded K3s, Helm, Argo CD, Consul, Grafana, Prometheus, Loki and k6 delivery flow.
- [Ubuntu K3s Step-by-Step Deployment](./23-ubuntu-k3s-step-by-step-deployment.md): full operator sequence from host setup through GitOps deployment and rollback.
- [Operator Dashboard NodePort Runbook](./24-operator-dashboard-nodeport-runbook.md): NodePort URLs, firewall allowlist, login, observability boundaries, and safe use of platform dashboards.
- [Application Observability](./25-application-observability.md): Prometheus ServiceMonitors, Go metrics, Jaeger OTLP tracing, deployment, and evidence checks.
- [K3s Lab Current Operations](./26-k3s-lab-current-operations.md): GitOps, runtime secrets, catalog recovery, database tunnels, performance, DNS/TLS, and handover checks based on the current lab.

## Architecture Decisions

- [ADR-20260729 Core Commerce Services](./adr/ADR-20260729-core-commerce-services.md): service boundaries, sync/async paths, and alternatives rejected.
- [ADR-20260730 Admin RBAC Boundary](./adr/ADR-20260730-admin-rbac-boundary.md): why v1 extends `account` instead of adding an admin service.
- [ADR-20260730 Admin Commerce Read Model](./adr/ADR-20260730-admin-commerce-read-model.md): cross-domain reporting and command ownership.
- [Admin Commerce Implementation Status](./17-admin-commerce-implementation-status.md): delivered reporting slice and remaining integration work.
- [ADR-20260808 K3s GitOps Lab Platform](./adr/ADR-20260808-lab-k3s-gitops-platform.md): delivery boundaries, resource trade-offs and verification strategy.

## Repo-Local Agent Assets

- `.agents/rules/project-rules.md`
- `.agents/memory/goshopx-project-memory.md`
- `.agents/agents/*.md`
- `.agents/skills/*/SKILL.md`

Use these files together: docs describe the project process, rules define constraints, memory preserves stable context, agents define responsibilities, and skills define repeatable execution behavior.
