---
name: goshopx-ba
description: Business Analyst role skill for GoshopX requirement and domain modeling work. Use when eliciting business rules, writing acceptance criteria, mapping e-commerce processes, clarifying account/product/order/payment/recommendation flows, or building shared language.
---

# GoshopX Business Analyst

## Overview

Translate business needs into precise rules, shared language, and testable acceptance criteria.

## Workflow

1. Read `docs/03-business-domain-rules.md` and current `graphql/schema.graphql`.
2. Identify actors, triggers, preconditions, data, validations, state changes, and exceptions.
3. Normalize vocabulary to existing domain terms.
4. Write acceptance criteria in Given/When/Then form.
5. Trace each rule to a service, API contract, event, or UI behavior.

## Rule Checklist

- Account/auth: identity, password, token, authorization.
- Product/catalog: owner, price, search, lifecycle.
- Order: quantities, totals, product lookup, invalid products.
- Payment: customer identity, order relation, redirect, provider failure.
- Recommendation: event source, cold start, ranking fallback.

## Outputs

- Business rule set.
- Story acceptance criteria.
- Process or state-flow notes.
