# Phase 1 Data Model: Billing, Quotas, And Usage Accounting

This document captures the entities, storage rules, validation rules, and lifecycle
transitions introduced by Roadmap 38. The quota catalog and enforcement matrix in
[`contracts/quota-catalog.md`](./contracts/quota-catalog.md) and
[`contracts/enforcement-matrix.md`](./contracts/enforcement-matrix.md) are authoritative
planning gates for category and entry-point coverage.

## 1. Entities

### 1.1 Tenant Plan

Tenant-owned entitlement package for hosted, local development, or unlimited use.

- `plan_id` — stable identifier.
- `tenant_id` — owning tenant for tenant-specific assignments.
- `plan_key` — stable plan name such as `development`, `unlimited`, or hosted plan key.
- `status` — `active`, `scheduled`, `disabled`, or `superseded`.
- `effective_at`, `superseded_at`.
- `assigned_by_principal_id`, `assignment_reason`.
- `document_json` — non-secret display metadata and future pricing labels; no payment
  provider state in this roadmap.

Validation rules:

- Every hosted tenant has exactly one active plan before enforcement is enabled.
- Local-first installations have an explicit development or unlimited plan.
- Plan assignment requires the canonical `billing.manage` plan and quota administration
  permission.
- Plan changes emit audit-visible records.

### 1.2 Quota Definition

Durable rule for one quota category.

- `quota_definition_id`.
- `category` — stable identifier from the initial quota catalog.
- `unit` — `count`, `bytes`, `seconds`, `attempts`, or another explicit unit.
- `period_kind` — `none`, `daily`, `monthly`, `rolling_window`, or `custom`.
- `period_anchor` — `UTC` for all period reset boundaries.
- `period_length` — present for rolling or custom periods.
- `default_limit`.
- `carryover_enabled`, `carryover_max`.
- `reservation_rule`, `commit_rule`, `refund_rule`.
- `denial_reason_code`.
- `active`, `created_at`, `updated_at`.

Validation rules:

- `category` is unique among active definitions.
- Period boundaries are UTC regardless of tenant locale, operator locale, or daylight
  saving changes.
- Carryover maximum is required when carryover is enabled.
- Denial reason code is stable and contract-tested.

### 1.3 Quota Override

Tenant-specific override to a plan quota.

- `quota_override_id`.
- `tenant_id`, `category`.
- `limit`, `carryover_enabled`, `carryover_max` where overridden.
- `effective_at`, `expires_at`.
- `reason`, `created_by_principal_id`.

Validation rules:

- Override changes require `billing.manage` and a reason.
- Lowered quota takes effect immediately, existing usage records remain unchanged, and
  new quota-consuming work is denied while effective usage exceeds the new limit.
- Override changes emit audit-visible records.

### 1.4 Effective Quota Projection

Read model returned to tenants, operators, SDK clients, and contract tests.

- `tenant_id`, `plan_key`, `category`.
- `unit`, `period_start`, `period_end`, `period_anchor = UTC`.
- `limit`, `carryover_applied`, `carryover_remaining`.
- `consumed_amount`, `reserved_amount`, `adjusted_amount`.
- `remaining_amount`.
- `enforcement_mode` — `enforced`, `unlimited`, or `not_measurable`.
- `denial_reason_code` when currently denied.

Rules:

- Projection is tenant-scoped and never includes another tenant's quota or usage.
- `remaining_amount` accounts for committed usage, active reservations, carryover, and
  manual adjustments.
- Unlimited/development plans are explicit in projection.

### 1.5 Quota Period

The UTC-bounded accounting window for a category.

- `quota_period_id`.
- `tenant_id`, `category`.
- `period_kind`.
- `period_start`, `period_end` in UTC.
- `carryover_from_period_id`.
- `status` — `open`, `closed`, or `reconciled`.

Validation rules:

- A category with `period_kind = none` has one open no-reset period.
- Daily/monthly/custom/rolling periods calculate boundaries in UTC.
- Period reset and carryover are audit-visible and deterministic.

### 1.6 Usage Counter

Tenant/category/period aggregate used for efficient enforcement.

- `usage_counter_id`.
- `tenant_id`, `category`, `quota_period_id`.
- `committed_amount`.
- `reserved_amount`.
- `adjusted_amount`.
- `carryover_amount`.
- `updated_at`.

Validation rules:

- `(tenant_id, category, quota_period_id)` is unique.
- Counter changes occur in the same durable transaction as lifecycle event records.
- Effective usage cannot become negative after adjustment/refund.

### 1.7 Usage Reservation

Tenant-scoped hold against available quota made before guarded work starts.

- `reservation_id`.
- `tenant_id`, `category`, `quota_period_id`.
- `operation_key`.
- `amount_reserved`, `amount_committed`, `amount_refunded`.
- `status` — `reserved`, `committed`, `released`, `refunded`, `denied`,
  `operator_action_needed`.
- `reservation_point`, `commit_point`, `refund_point`.
- `created_at`, `updated_at`, `expires_at` when category-specific.
- `recovery_reason` when restart recovery cannot prove the outcome.

Validation rules:

- `(tenant_id, category, operation_key)` is unique.
- Replaying the same operation identity returns the existing lifecycle outcome.
- Ambiguous restart recovery marks the reservation `operator_action_needed` and denies
  duplicate quota-consuming work for the same operation until resolved.

State transitions:

```text
reserved -> committed
reserved -> released
reserved -> refunded
reserved -> operator_action_needed
reserved -> denied

operator_action_needed -> committed
operator_action_needed -> released
operator_action_needed -> refunded
```

### 1.8 Usage Event

Append-only lifecycle evidence for reservation, commit, refund, release, denial, and
manual adjustment.

- `usage_event_id`.
- `tenant_id`, `category`, `quota_period_id`.
- `operation_key` when applicable.
- `event_kind` — `reservation`, `commit`, `refund`, `release`, `denial`,
  `manual_adjustment`, `period_reset`, or `recovery_decision`.
- `amount`.
- `reason_code`, `reason`.
- `actor_principal_id` where available.
- `created_at`.
- `document_json` — non-secret supporting details.

Validation rules:

- Usage events are tenant-scoped.
- Events include enough structured data to explain denials and adjustments without logs.
- Billing and usage audit records retain indefinitely unless an explicit operator
  retention policy is later applied.

### 1.9 Quota Denial

Stable decision preventing guarded work from starting.

- `denial_id`.
- `tenant_id`, `category`, `quota_period_id`.
- `operation_key`.
- `reason_code`.
- `requested_amount`, `remaining_amount`.
- `guarded_entry_point`.
- `created_at`.

Rules:

- Hosted quota-state-unavailable failures use a stable fail-closed denial reason.
- Denials occur before resource consumption or external side effects.
- Multi-category operations deny the whole operation when any category denies and leave
  accounting consistent for all categories.

### 1.10 Manual Adjustment

Administrator-created correction to usage or entitlement state.

- `adjustment_id`.
- `tenant_id`, `category`, `quota_period_id`.
- `amount_delta`.
- `reason`.
- `created_by_principal_id`.
- `created_at`.

Validation rules:

- Reason is required.
- Adjustment cannot make effective usage negative or inconsistent with the category unit.
- Adjustment emits audit-visible evidence and updates effective projection.

### 1.11 Billing And Usage Audit Retention Policy

Retention rule for billing and usage audit records.

- `tenant_id` or global policy scope.
- `retention_mode` — `indefinite` by default.
- `retention_period` when a future explicit policy exists.
- `created_by_principal_id`, `reason`, `created_at`.

Rules:

- Default retention is indefinite.
- Explicit retention policy changes require operator action and audit evidence.

## 2. Schema Deltas

Expected additive storage changes:

- Add tenant plan and plan assignment records.
- Add quota definitions and tenant quota overrides.
- Add quota periods and usage counters.
- Add usage reservations with `(tenant_id, category, operation_key)` uniqueness.
- Add append-only usage events.
- Add quota denial records.
- Add manual adjustment records.
- Add billing/usage audit retention policy records or explicit default policy projection.
- Add indexes for `(tenant_id, category, period)` and operation-key lookups.

Rollback is backup-restore for storage changes plus assigning tenants an explicit
development/unlimited plan or disabling hosted quota enforcement without deleting
accounting evidence.

## 3. Operation Identity Rules

Operation identity must be stable across client retry and daemon restart. Required shapes
are specified in [`contracts/quota-catalog.md`](./contracts/quota-catalog.md).

General rules:

- Include tenant id, guarded entry point, resource id when known, and client idempotency
  key or daemon-generated stable id.
- Do not use timestamps or attempt counters as the only identity.
- Use the same operation identity for reservation, commit, refund, release, denial, and
  restart recovery.

## 4. State Transitions

### Quota Override Lowered Below Usage

```text
effective quota Q, effective usage U
  -> admin lowers quota to Q2 where Q2 < U
existing usage remains U
new quota-consuming work denied until U <= Q2
```

### Storage Byte Reconciliation

```text
estimate E reserved before write
  -> write succeeds with actual bytes A
commit A and refund/adjust E-A

if A > E and committed actual usage places the tenant over quota
  -> keep A committed with audit-visible over-limit evidence
  -> deny new quota-consuming work until effective usage is within limit

estimate E reserved before write
  -> write fails before consumption
release/refund E
```

### Ambiguous Restart Recovery

```text
reserved
  -> daemon restart
  -> outcome cannot be proven
operator_action_needed + duplicate operation denied
```

### Period Reset

```text
open UTC period
  -> UTC boundary reached
closed period + carryover calculated + new UTC period opened
```

## 5. Redaction And Privacy

- Billing and usage records do not include payment-provider credentials because payment
  providers are out of scope.
- Audit and usage records may include tenant id, category, operation identity, amount,
  period, reason, timestamp, outcome, and actor where available.
- External account tokens and secret material from Roadmap 37 must not be copied into
  billing records.
