# Data Model: Integration Health And Permission Diagnostics

## Overview

Roadmap 42 adds tenant-scoped diagnostic state beside existing integration, credential,
live-validation, ops-readiness, and evaluation product records. All persisted diagnostic
payloads are redacted before storage or fail closed. Tables store complete resource JSON
plus indexed columns for tenant, integration account, provider, domain, reason code,
status, freshness, retention, and pagination.

## Entities

### Integration Diagnostic Result

- **Purpose**: Latest tenant-scoped diagnostic state for an integration account,
  provider capability, or unsupported domain projection.
- **Key fields**: `diagnosticResultId`, `tenantId`, `integrationId`, `integrationAccountId`,
  `domainKind`, `providerKind`, `capability`, `status`, `reasonCode`,
  `remediationOwner`, `remediationHint`, `retrySafety`, `freshnessState`,
  `checkedAt`, `staleAfter`, `runId`, `redactionStatus`, `evidenceSummary`,
  `retentionExpiresAt`, `createdAt`, `updatedAt`.
- **Relationships**: Belongs to an Integration Account. Produced by Diagnostic Runs.
  May reference Provider Error Classifications, Smoke Probe Outcomes, and audit events.
- **State transitions**: `unknown -> healthy|degraded|blocked|unsupported`; any state
  can transition to another state after a current diagnostic run; `fresh -> stale` when
  `checkedAt` is older than 15 minutes.
- **Validation rules**: Must include one tenant. Cached inspection results older than
  15 minutes are marked stale. User-facing action failures must use current diagnostic
  truth before remediation is shown.

### Diagnostic Reason Code

- **Purpose**: Stable system-level classification for diagnostic and remediation
  behavior.
- **Key fields**: `reasonCode`, `category`, `defaultSeverity`, `defaultRetrySafety`,
  `defaultRemediationOwner`, `userMessageKey`, `operatorMessageKey`, `supportedDomains`,
  `createdAt`, `updatedAt`.
- **Relationships**: Referenced by Diagnostic Results, Provider Error Classifications,
  Smoke Probe Outcomes, Remediation Hints, and audit events.
- **Validation rules**: Codes are stable once published. New codes require schema,
  SDK, fixture, and docs updates. Free-form provider text is not a reason code.

### Remediation Hint

- **Purpose**: Redacted user-facing or operator-facing guidance tied to a reason code.
- **Key fields**: `remediationHintId`, `reasonCode`, `owner`, `audience`,
  `shortMessage`, `nextStep`, `retryGuidance`, `localeKey`, `createdAt`, `updatedAt`.
- **Relationships**: Attached to Diagnostic Results, user-facing integration failures,
  and Smoke Probe Outcomes.
- **Validation rules**: Must not include raw provider error text or secret material.
  Owner is one of `product_user`, `tenant_admin`, `operator`, `provider`,
  `none_required`.

### Diagnostic Run

- **Purpose**: Bounded diagnostic execution for one tenant and integration account or
  domain projection.
- **Key fields**: `diagnosticRunId`, `tenantId`, `integrationId`,
  `integrationAccountId`, `domainKind`, `providerKind`, `requestedBy`, `trigger`,
  `status`, `startedAt`, `completedAt`, `checkedCapabilities`, `resultIds`,
  `failureReasonCode`, `redactionStatus`, `retentionExpiresAt`, `idempotencyKey`.
- **Relationships**: Emits Integration Diagnostic Results, Provider Error
  Classifications, audit events, and optional Smoke Probe Outcomes.
- **State transitions**: `queued -> running -> completed`; `queued/running -> failed`;
  `queued/running -> blocked`; terminal states remain readable until retention expiry.
- **Validation rules**: Exactly one tenant. Re-running with the same idempotency key
  returns the same run while it is retained. Runs default to 90-day retention.

### Provider Error Classification

- **Purpose**: Redacted interpretation of provider-specific evidence into stable
  diagnostic meaning.
- **Key fields**: `classificationId`, `tenantId`, `providerKind`, `domainKind`,
  `integrationId`, `operationClass`, `providerErrorClass`, `providerStatusCode`,
  `redactedProviderCode`, `reasonCode`, `retrySafety`, `remediationOwner`,
  `evidenceConfidence`, `ambiguous`, `redactionStatus`, `createdAt`.
- **Relationships**: Produced by classification adapters. Referenced by Diagnostic
  Results, Smoke Probe Outcomes, user-facing failure details, and audit events.
- **Validation rules**: Raw provider payloads are never stored. Ambiguous evidence must
  use an explicit ambiguous or unknown reason instead of inventing a specific cause.
  Redaction uncertainty fails closed.

### Diagnostic Capability Probe

- **Purpose**: A provider or domain check used by diagnostic runs and smoke reports.
- **Key fields**: `probeId`, `tenantId`, `providerKind`, `domainKind`, `capability`,
  `probeKind`, `sideEffectClass`, `requiresTenantAdminApproval`,
  `requiresOperatorApproval`, `readOnlyOrReversible`, `supported`, `createdAt`,
  `updatedAt`.
- **Relationships**: Executed by Diagnostic Runs or Real-Account Smoke Matrices.
  Approval records may be required before risky probes run.
- **Validation rules**: Non-idempotent or externally visible probes cannot run without
  both tenant administrator and authorized operator approval.

### Smoke Matrix Report

- **Purpose**: Structured real-account smoke evidence for one readiness or release run.
- **Key fields**: `smokeReportId`, `tenantId`, `reportKind`, `requestedBy`, `status`,
  `domainSummary`, `startedAt`, `completedAt`, `publishedAt`, `artifactRefs`,
  `retentionExpiresAt`, `createdAt`, `updatedAt`.
- **Relationships**: Has Smoke Probe Outcomes. Links to release-readiness evidence and
  diagnostic audit events.
- **State transitions**: `draft -> running -> completed -> published`;
  `draft/running -> blocked`; `running -> failed`; terminal states expire from normal
  inspection after retention expiry.
- **Validation rules**: Report outcomes distinguish `passed`, `failed`, `blocked`, and
  `skipped`. Missing safe credentials produce structured skipped or blocked outcomes,
  not absent evidence.

### Smoke Probe Outcome

- **Purpose**: Result of a single real-account diagnostic probe.
- **Key fields**: `probeOutcomeId`, `tenantId`, `smokeReportId`, `integrationId`,
  `integrationAccountId`, `domainKind`, `providerKind`, `probeAction`, `result`,
  `reasonCode`, `remediationHintId`, `retrySafety`, `approvalRefs`, `artifactRefs`,
  `checkedAt`, `redactionStatus`, `retentionExpiresAt`.
- **Relationships**: Belongs to a Smoke Matrix Report. May reference Diagnostic Runs,
  Provider Error Classifications, and live-validation ledger entries for retry safety.
- **State transitions**: `pending -> passed|failed|blocked|skipped`; terminal outcome
  is immutable except for retention metadata.
- **Validation rules**: Outcomes must include explicit skip or blocked reason when no
  probe runs. Risky probes require dual approval before transition from `pending`.

### Integration Diagnostic Audit Event

- **Purpose**: Tenant-scoped audit evidence for diagnostic execution and remediation-
  relevant state transitions.
- **Key fields**: `auditEventId`, `tenantId`, `actorId`, `action`, `targetKind`,
  `targetId`, `outcome`, `reasonCode`, `diagnosticRunId`, `smokeReportId`,
  `redactionStatus`, `createdAt`.
- **Relationships**: References Diagnostic Runs, Diagnostic Results, Smoke Reports,
  Smoke Probe Outcomes, permission denials, retention/deletion applications, and
  redaction failures.
- **Validation rules**: Audit records are append-only and redacted. Permission denials
  must not reveal inaccessible tenant, integration account, or credential existence.

### Diagnostic Retention Record

- **Purpose**: Tracks expiry and cleanup of diagnostic runs, result payloads, smoke
  reports, probe outcomes, and redacted evidence.
- **Key fields**: `retentionRecordId`, `tenantId`, `targetKind`, `targetId`,
  `policyRef`, `defaultExpiresAt`, `effectiveExpiresAt`, `retentionState`,
  `appliedAt`, `createdAt`, `updatedAt`.
- **Relationships**: Applies to Diagnostic Runs, Diagnostic Results, Smoke Matrix
  Reports, Smoke Probe Outcomes, and Provider Error Classifications.
- **State transitions**: `active -> expired -> purged`; `active -> extended` when an
  authorized longer retention policy applies.
- **Validation rules**: Default expiry is 90 days. Expired evidence is removed from
  normal inspection while minimal audit evidence remains available to authorized
  operators.

## Reason Code Categories

- `healthy`
- `authorization`: app or bot authorization missing, user authorization missing
- `tenant_approval`: tenant approval pending or denied
- `scope`: provider scope missing
- `token`: token missing, expired, revoked, refresh credentials missing, refresh failed
- `tenant_mismatch`
- `provider`: provider unavailable, transient provider failure
- `network`: local network failed
- `quota`: rate limited
- `retry_safety`: retryable, unsafe to retry, operator action needed, ambiguous commit
- `redaction`: redaction uncertain, redaction failed closed
- `unsupported`: unsupported diagnostic or limited diagnostic
- `unknown`: unknown provider error

## Access And Pagination Rules

- Every diagnostic and smoke resource is tenant-scoped.
- Global operators must resolve an explicit tenant context before reading or running
  diagnostics.
- List ordering is deterministic: primary sort by relevant timestamp descending, then
  stable id descending.
- Cursor pagination is required for diagnostic runs, diagnostic results, smoke reports,
  smoke probe outcomes, and audit event views.
- Cross-tenant references are invalid even when external account labels match.

## Retention And Redaction Rules

- Diagnostic runs and smoke evidence use 90-day default retention.
- Redaction happens before persistence or display. If redaction confidence is uncertain,
  the detail is suppressed and redaction-failure audit evidence is emitted.
- Raw secrets, OAuth tokens, refresh tokens, app secrets, authorization headers, local
  CLI auth material, and credential-bearing request or response payloads are never
  persisted in diagnostic resources, reports, events, fixtures, logs, or SDK/web
  projections.
