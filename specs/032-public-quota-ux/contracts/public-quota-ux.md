# Contract: Public Quota UX

This contract defines additive daemon, schema, SDK, and web-shell behavior for Roadmap 47.
Roadmap 38 billing records remain authoritative.

## Compatibility Rules

- Existing routes remain compatible:
  - `GET /v1/billing/plan`
  - `GET /v1/billing/usage`
  - `GET /v1/billing/quotas`
  - `GET /v1/billing/denials`
- Existing fields keep their current meaning. New fields are additive and nullable where
  older data cannot provide them.
- Stable quota denials continue to use `code = quota_denied` and category-specific
  `reasonCode` values.
- Clients must branch on stable codes and classifications, not display messages.
- Tenant scoping remains resolved through the protected tenant context and SDK tenant
  override behavior.

## API Surfaces

### Quota Dashboard

```text
GET /v1/billing/quota-dashboard
```

Permission: billing visibility for the active tenant.

Response: `BillingQuotaDashboardResponse`

Required top-level fields:

- `tenantId`
- `plan`
- `sections[]`
- `generatedAt`

Each response must include every enforced quota category from the Roadmap 38 catalog,
grouped into readable sections. Finite quota items expose current active period and
immediately previous completed period usage where available.

Quota item required fields:

- `category`
- `unit`
- `status`
- `currentPeriod`
- `limit`
- `remainingAmount`
- `nearLimit`
- `typicalOperationAmount`
- `recoveryActions[]`

Near-limit status is true when a finite category is at least 80% consumed or has less than
one typical operation remaining. Typical operation amount is category-defined: count and
attempt quotas use `1`, while byte quotas use the catalog's configured artifact-write
reservation estimate.

### Denial Detail

```text
GET /v1/billing/denials/{denialId}
```

Permission: billing visibility for the active tenant.

Response: `BillingDenialDetailResponse`

Required fields:

- `denialId`
- `tenantId`
- `operationRef`
- `operationKey`
- `guardedEntryPoint`
- `category`
- `reasonCode`
- `classification`
- `requestedAmount`
- `remainingAmount`
- `recoveryActions[]`
- `createdAt`

Allowed `classification` values:

- `quota_exhaustion`
- `abuse_restriction`
- `quota_state_unavailable`
- `unauthorized`
- `operator_action_needed`

The response must not imply that guarded work started when the denial occurred before
expensive or side-effecting work.

### Evidence Export

```text
POST /v1/billing/denials/{denialId}/evidence-export
```

Permission: `billing.evidence_export`.

Response: `BillingEvidenceExportResponse`

Scope: evidence export is generated from a denial record. It covers ordinary quota
denials and abuse-restriction denials; standalone abuse-restriction export without an
associated denial is out of scope for this phase.

Required fields:

- `schemaVersion`
- `exportId`
- `tenantId`
- `generatedAt`
- `generatedByPrincipalId`
- `denial`
- `usageSnapshot`
- `effectiveLimitState`
- `auditRefs[]`
- `redactions[]`

The package is structured redacted JSON. It must exclude secrets, connector payloads,
unrelated run content, and data from other tenants. Each removed or masked field must be
represented in `redactions[]` with a path and reason.

## Abuse Restriction Visibility

Abuse restriction projections are backed by explicit additive billing abuse restriction
records plus audit evidence. They are not inferred from ordinary quota exhaustion and are
not modeled as tenant quota overrides.

Abuse restriction projections expose:

- status
- affected quota category
- duration when available
- recovery action
- support contact state
- visible reason code
- source audit reference

They must not expose:

- detection signals
- enforcement thresholds
- raw trigger events
- evasion-relevant event patterns

## SDK Contract

The TypeScript SDK adds typed resources and methods:

```ts
getBillingQuotaDashboard(tenantOptions?): Promise<BillingQuotaDashboardResponse>
getBillingDenialDetail(denialId: string, tenantOptions?): Promise<BillingDenialDetailResponse>
exportBillingDenialEvidence(denialId: string, tenantOptions?): Promise<BillingEvidenceExportResponse>
```

Required SDK types:

- `BillingQuotaDashboardResponse`
- `BillingQuotaSection`
- `BillingQuotaStatusItem`
- `BillingUsagePeriodSummary`
- `BillingDenialDetailResponse`
- `BillingAbuseRestrictionSummary`
- `BillingQuotaOverrideSummary`
- `BillingEvidenceExportResponse`
- `BillingEvidenceRedaction`

SDK tests must prove:

- tenant override headers propagate to new methods
- quota denial classifications surface without string parsing
- evidence export is typed as JSON data, not text
- unauthorized and tenant-denied states map to stable client errors

## Web Shell Contract

The web shell must provide:

- active-tenant quota dashboard with all enforced categories grouped for scanning
- current and previous-period usage
- finite/unlimited/not-measurable plan state
- near-limit, exhausted, restricted, and unavailable statuses
- denial detail view with recovery actions
- abuse restriction messaging that hides detection signals and thresholds
- tenant-owner/admin visibility for base plan versus effective override
- support evidence export action for authorized support operators on ordinary quota
  denials and abuse-restriction denials
- stable unauthorized states for users without visibility or export permission

Tenant switching must clear or hide quota dashboard and denial details from the previous
tenant before rendering new tenant data.

## Schema Artifacts

Implementation must add or update:

- `schemas/api/billing-quota-dashboard.response.schema.json`
- `schemas/api/billing-denial-detail.response.schema.json`
- `schemas/api/billing-evidence-export.response.schema.json`
- `schemas/api/billing-usage.response.schema.json` for additive summary fields if reused
- `schemas/api/billing-quota-resource.schema.json` for additive status/period fields if reused
- `schemas/api/billing-denial-resource.schema.json` for additive classification/detail fields if reused
- `schemas/api/tenant-permission-resource.schema.json` for `billing.evidence_export`

## Required Contract Tests

- Dashboard schema includes all enforced quota categories and status fields.
- Current and immediately previous completed periods are projected correctly.
- Near-limit status triggers at 80% consumed and below one typical operation remaining for count, attempt, and byte quota categories.
- Denial detail schema preserves stable reason code and operation reference.
- Abuse restriction responses come from explicit abuse restriction records and hide detection signals and thresholds.
- Evidence export requires `billing.evidence_export`; `billing.view` alone is denied.
- Evidence export schema is structured JSON with redaction records for ordinary quota
  denials and abuse-restriction denials.
- Unauthorized callers receive stable denials without partial quota, denial, override,
  restriction, or evidence data.
- Tenant switch and SDK tenant override tests prove no cross-tenant leakage.
- Fail-closed quota-state-unavailable responses remain side-effect-free.
