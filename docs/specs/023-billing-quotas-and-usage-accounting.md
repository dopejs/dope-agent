# Billing, Quotas, And Usage Accounting

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 38, the hosted
billing, quota, and usage-accounting layer.

Primary source documents:
- `docs/product/hosted-productization-roadmap-split.md`
- `docs/specs/019-tenant-identity-and-access-foundation.md`
- `docs/specs/020-tenant-scoped-data-migration.md`
- `docs/runtime/daemon-roadmaps.md`

## Background

Hosted use requires resource governance. Even if the first version does not collect money
through a payment provider, tenant plans, quotas, usage accounting, and enforcement hooks
must be real product behavior.

## Goal

Introduce tenant plans, quota definitions, usage counters, enforcement hooks, and
operator-visible billing or usage projections.

## Fixed Decisions

- Billing and quota are a full roadmap, not a placeholder field.
- The first version may implement internal plans and quota enforcement without integrating a
  payment processor.
- Usage accounting is tenant-scoped and auditable.
- Quota decisions must be enforced before expensive or side-effecting work starts.

## Dependencies On Completed Phases

- Roadmap 34: Tenant Identity And Access Foundation
- Roadmap 35: Tenant-Scoped Data Migration
- Roadmap 37: Hosted Secrets, Integrations, And Connector Isolation

## In Scope

- tenant plan records
- quota definitions and effective quota projection
- usage counters for runs, workflows, tool calls, live validation, storage/artifacts, and
  integration operations where measurable
- enforcement hooks at run/workflow launch and live validation entry points
- billing or usage inspection APIs
- audit events for quota denial, plan changes, and usage counter adjustment

## Out Of Scope

- external payment-provider checkout by default
- invoices, taxes, and revenue recognition
- metering every token or provider-specific billing unit unless required by a later
  clarification
- cross-tenant pooled quota

## Operator Or User Problems To Solve

- Hosted tenants need predictable limits.
- Operators need to explain why work was denied.
- Later live side-effect validation needs a quota gate before it consumes external systems.

## User Stories

- As a tenant owner, I can inspect my plan and current usage.
- As an operator, I receive a stable error when a tenant exceeds a quota.
- As an admin, I can adjust a tenant plan or quota and see an audit trail.

## Functional Requirements

- The system MUST persist tenant plans and quota definitions.
- The system MUST count usage by tenant for in-scope resource categories.
- The system MUST enforce quotas before starting in-scope expensive or side-effecting work.
- The system MUST expose tenant usage and effective quota through API.
- The system MUST emit auditable quota denial and plan-change events.

## Compatibility And Operational Notes

- Local-first installations can use an unlimited or development plan by default.
- Counter updates must be idempotent across retry and restart boundaries.
- Enforcement must fail closed for hosted tenants when quota state is unavailable.

## Verification Expectations

- Unit tests for quota calculation and idempotent counter updates.
- API tests for plan inspection and quota denial.
- Integration tests proving quota checks prevent run/workflow or live-validation launch.
- Contract tests for usage and quota response shapes.

## Definition Of Done

- Hosted tenants have real plan, usage, and quota behavior that can gate product work before
  resource consumption or external side effects.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/023-billing-quotas-and-usage-accounting.md 完成 phase 38 的工作`
