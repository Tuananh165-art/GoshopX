# Agile/Scrum Delivery Guide

## Artifacts

- Product vision: why the feature matters to shoppers, sellers, operators, or developers.
- Product backlog: ordered stories with business value, risk, and acceptance criteria.
- Sprint backlog: stories selected for the sprint with owners and validation tasks.
- Definition of Ready: the team can implement without guessing business rules.
- Definition of Done: the work is implemented, tested, reviewed, documented, and releasable.

## Story Template

```md
As a <role>,
I want <capability>,
so that <business outcome>.

Business rules:
- ...

Acceptance criteria:
- Given ...
  When ...
  Then ...

Technical notes:
- Affected services:
- Contracts:
- Data/events:
- Risks:
- Tests:
```

## Definition of Ready

- Business owner and user type are known.
- Happy path, validation failures, and authorization rules are clear.
- Affected GraphQL fields, gRPC methods, Kafka topics, and database/index ownership are identified.
- Test level is chosen: unit, integration, e2e, contract, or security.
- External dependencies such as payment provider keys or live services are called out.

## Definition of Done

- Code follows existing Go/Python structure and naming.
- Public client behavior is exposed through GraphQL.
- Internal synchronous calls use gRPC; async facts use Kafka.
- Tests pass for the changed area.
- Docs, agent memory, or rules are updated when domain/process behavior changes.
- Security-sensitive changes include auth, secret, and abuse-case review.
- The final report names exactly what was verified and what was not.

## Ceremonies

- Backlog refinement: BA, PM, Architect, Security, and lead implementers clarify stories.
- Sprint planning: PM confirms priority; Architect confirms implementation path.
- Daily scrum: each role reports progress, blocker, and validation evidence.
- Review: demo working behavior through GraphQL or a visible UI.
- Retrospective: capture process gaps as repo docs, rules, or skill updates.
