# Data Model: Public Quota UX

## TenantQuotaDashboard

Tenant-scoped product projection used by the web shell and SDK to display the active
tenant's quota posture.

Fields:

- `tenantId`: active tenant identifier.
- `plan`: `PlanSummary`.
- `sections[]`: ordered `QuotaSection` groups containing every enforced quota category
  from the Roadmap 38 catalog.
- `restrictions[]`: active or recently relevant `AbuseRestrictionSummary` records.
- `denialHighlights[]`: recent `QuotaDenialDetail` summaries for user-visible recovery.
- `generatedAt`: time the projection was generated.
- `permission`: visibility state for the caller.

Validation rules:

- Must contain only the resolved active tenant.
- Must include every enforced quota category exactly once.
- Must not include payment checkout, invoice, tax, marketplace, secret, or connector
  payload data.

## PlanSummary

User-readable summary of the active tenant plan.

Fields:

- `planKey`: stable plan key.
- `enforcementMode`: `enforced`, `unlimited`, or `not_measurable`.
- `status`: active plan status.
- `effectiveAt`: plan assignment effective time.
- `basePlanLabel`: display label derived from plan metadata or key.
- `checkoutAvailable`: always false for this roadmap.

Validation rules:

- Existing Roadmap 38 plan assignment remains authoritative.
- Unlimited and not-measurable plans must not be presented as missing billing data.

## QuotaSection

User-readable grouping of quota categories.

Fields:

- `sectionKey`: stable key such as `launches`, `runtime`, `integrations`, `storage`, or
  `evaluation`.
- `label`: user-facing section label.
- `items[]`: `QuotaStatusItem` records.

Validation rules:

- Grouping must not change category identifiers.
- A category cannot appear in multiple sections.

## QuotaStatusItem

Projection for one quota category in one active tenant.

Fields:

- `category`: stable Roadmap 38 quota category.
- `unit`: quota unit.
- `status`: `available`, `near_limit`, `exhausted`, `unlimited`, `restricted`, or
  `unavailable`.
- `currentPeriod`: `UsagePeriodSummary`.
- `previousPeriod`: optional `UsagePeriodSummary` for the immediately previous completed
  period.
- `limit`: effective finite limit when measurable.
- `remainingAmount`: effective remaining amount.
- `nearLimit`: true when 80% consumed or below one typical operation remaining.
- `nearLimitReason`: `percent_threshold` or `below_one_typical_operation`.
- `typicalOperationAmount`: category-defined amount used for the below-one-operation
  warning rule. Count and attempt quotas use `1`; byte quotas use the catalog's
  configured artifact-write reservation estimate.
- `baseLimit`: base plan limit when visible.
- `effectiveLimit`: effective limit after override or restriction.
- `override`: optional `QuotaOverrideSummary`.
- `restriction`: optional `AbuseRestrictionSummary`.
- `denialReasonCode`: stable denial reason for exhausted categories.
- `recoveryActions[]`: ordered recovery actions: `wait`, `reduce_scope`,
  `request_override`, `contact_support`, or `operator_resolution_required`.

Validation rules:

- Near-limit status applies only to finite enforced quotas.
- `typicalOperationAmount` must be present for every finite quota category.
- Abuse restriction fields must hide detection signals and enforcement thresholds.
- Previous period must be the immediately previous completed period for the category's
  period semantics.

State transitions:

```text
available -> near_limit -> exhausted
available -> restricted
near_limit -> restricted
restricted -> available
unavailable -> available
```

## UsagePeriodSummary

Bounded usage summary for one quota category and period.

Fields:

- `periodStart`
- `periodEnd`
- `periodAnchor`: UTC.
- `consumedAmount`
- `reservedAmount`
- `adjustedAmount`
- `carryoverApplied`
- `remainingAmount`
- `overLimit`

Validation rules:

- Current period is open at projection time.
- Previous period is closed and immediately precedes the current period where the quota
  has daily or monthly periods.

## QuotaDenialDetail

Product-visible detail for one denied guarded operation.

Fields:

- `denialId`
- `tenantId`
- `operationRef`: stable operation reference safe for product display.
- `operationKey`: stable operation key for support correlation.
- `guardedEntryPoint`
- `category`
- `reasonCode`
- `classification`: `quota_exhaustion`, `abuse_restriction`,
  `quota_state_unavailable`, `unauthorized`, or `operator_action_needed`.
- `requestedAmount`
- `remainingAmount`
- `periodStart`
- `periodEnd`
- `recoveryActions[]`
- `restriction`: optional `AbuseRestrictionSummary`.
- `createdAt`

Validation rules:

- Must be tenant-scoped and permission-gated.
- Must not imply work started when quota denial happened before side effects.
- Must keep stable reason codes for SDK and support tests.

## AbuseRestrictionSummary

User-actionable restriction projection that avoids exposing abuse-control internals.

Source of truth:

- Abuse restrictions are explicit additive billing abuse restriction records plus audit
  evidence.
- They are separate from normal plan quotas and tenant quota overrides.
- Projection may summarize an active, expired, or operator-action-needed restriction, but
  it must not infer abuse restrictions solely from exhausted quota counters.

Fields:

- `restrictionId`
- `status`: `active`, `expired`, or `operator_action_needed`.
- `affectedCategory`
- `duration`: optional human-readable duration or structured start/end time.
- `recoveryAction`
- `supportContactAllowed`
- `visibleReasonCode`
- `sourceAuditRef`

Validation rules:

- Must not expose detection signals, thresholds, raw trigger events, or evasion-relevant
  data.
- Must distinguish abuse restriction from normal quota exhaustion.
- Lifecycle is `active -> expired` for time-bound restrictions and `active ->
  operator_action_needed -> expired` when support/operator resolution is required.

## QuotaOverrideSummary

Visible explanation of an effective limit that differs from the base plan.

Fields:

- `category`
- `baseLimit`
- `effectiveLimit`
- `reasonVisible`
- `effectiveAt`
- `expiresAt`
- `createdByPrincipalId`: visible only where allowed.

Validation rules:

- Lowered overrides apply to effective projections without rewriting usage history.
- Reason text must not include secrets or connector payload data.

## SupportEvidenceExport

Structured redacted JSON package generated from a denial record for ordinary quota
denial and abuse-restriction denial disputes. Standalone abuse-restriction export
without an associated denial is out of scope for this phase.

Fields:

- `exportId`
- `tenantId`
- `generatedAt`
- `generatedByPrincipalId`
- `denial`: `QuotaDenialDetail`
- `usageSnapshot`: relevant `QuotaStatusItem` and period summaries.
- `effectiveLimitState`: base plan, override, restriction, and enforcement mode.
- `auditRefs[]`: redacted audit or usage event references.
- `redactions[]`: redaction records with field path, reason, and replacement marker.
- `schemaVersion`

Validation rules:

- Export requires the canonical `billing.evidence_export` permission.
- Export must not include secrets, connector payloads, unrelated run content, or
  cross-tenant data.
- Export must be deterministic enough for contract tests.

## PermissionDeniedState

Stable authorization state for billing UX surfaces.

Fields:

- `code`
- `requiredPermission`
- `tenantId`: omitted when revealing it would leak data.
- `message`

Validation rules:

- Unauthorized users must not receive partial quota, denial, override, restriction, or
  evidence data.
