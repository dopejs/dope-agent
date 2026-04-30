# Billing, Quotas, And Usage Accounting

Status: implemented

Implementation evidence: Roadmap 38 tasks and verification results are recorded in
`specs/023-billing-quotas-usage/quickstart.md`.

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
- Quotas have explicit periods and reset semantics.
- Usage enforcement uses reservation, commit, and refund semantics so retries, failures, and
  concurrent launches do not double-count or bypass limits.

## Dependencies On Completed Phases

- Roadmap 34: Tenant Identity And Access Foundation
- Roadmap 35: Tenant-Scoped Data Migration
- Roadmap 37: Hosted Secrets, Integrations, And Connector Isolation

## In Scope

- tenant plan records
- quota definitions and effective quota projection
- quota period, reset, and carryover rules
- usage counters for runs, workflows, tool calls, live validation, storage/artifacts, and
  integration operations where measurable
- usage reservation, commit, refund, and manual adjustment records
- enforcement hooks at run/workflow launch and live validation entry points
- idempotency keys for usage updates across retry and restart
- billing or usage inspection APIs
- audit events for quota denial, plan changes, and usage counter adjustment

## Required Quota Catalog

Implementation planning MUST define the first quota catalog before coding. Each quota
category MUST include:

- `category`: stable quota identifier
- `unit`: count, bytes, seconds, attempts, or another explicit unit
- `period`: none, daily, monthly, rolling window, or custom period
- `carryover`: whether unused quota carries over and the maximum carryover amount
- `reservationPoint`: route or service method that reserves usage before work starts
- `commitPoint`: event or state transition that commits actual usage
- `refundPoint`: failure, cancellation, denial, or retry transition that releases reserved
  usage
- `operationKey`: idempotency key shape used across retry and restart
- `concurrencyGuard`: transaction, row lock, compare-and-swap, or queue serialization used
  to prevent double-spend
- `denialShape`: stable API error and audit reason code

The initial catalog MUST cover at least these quota dimensions:

- run launches
- workflow launches
- runtime tool calls
- live validation attempts
- integration operations
- persisted artifact or storage bytes where measurable
- replay or evaluation campaign attempts where measurable

## Required Enforcement Matrix

The implementation plan MUST include an enforcement matrix with one row per guarded entry
point. Required columns:

- API route or internal service entry point
- tenant context source
- quota categories touched
- reservation amount calculation
- commit/refund transition
- idempotency key source
- behavior when quota storage is unavailable
- tests for allowed, denied, retry, restart, and concurrent launch scenarios

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
- The system MUST define quota periods, reset behavior, and whether unused quota carries
  over for every quota category.
- The system MUST count usage by tenant for in-scope resource categories.
- The system MUST enforce quotas before starting in-scope expensive or side-effecting work.
- The system MUST reserve usage before work starts, commit usage after successful
  consumption, and refund or release reservations when work is denied, cancelled, or fails
  before consuming the resource.
- The system MUST make usage updates idempotent by stable operation key so daemon restart or
  client retry cannot double-count usage.
- The system MUST handle concurrent quota checks without allowing two launches to consume
  the same remaining quota.
- The system MUST expose tenant usage and effective quota through API.
- The system MUST emit auditable quota denial, plan-change, reservation, commit, refund, and
  manual-adjustment events.

## Compatibility And Operational Notes

- Local-first installations can use an unlimited or development plan by default.
- Counter updates must be idempotent across retry and restart boundaries.
- Enforcement must fail closed for hosted tenants when quota state is unavailable.

## Verification Expectations

- Unit tests for quota calculation and idempotent counter updates.
- Unit tests for quota period reset, reservation, commit, refund, and concurrent launch
  enforcement.
- API tests for plan inspection and quota denial.
- Integration tests proving quota checks prevent run/workflow or live-validation launch.
- Restart tests proving pending reservations recover into a safe committed, released, or
  operator-action-needed state.
- Contract tests for usage and quota response shapes.
- Matrix completeness test proving each in-scope expensive or side-effecting entry point
  has an enforcement row.

## Definition Of Done

- Hosted tenants have real plan, usage, and quota behavior that can gate product work before
  resource consumption or external side effects.
- Usage accounting is idempotent, period-aware, and safe under retry, restart, and
  concurrent launch pressure.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/023-billing-quotas-and-usage-accounting.md 完成 phase 38 的工作`
