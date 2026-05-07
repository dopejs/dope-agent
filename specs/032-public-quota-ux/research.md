# Research: Public Quota UX

## Decision: Extend Roadmap 38 Billing Projections Additively

Existing billing state, quota definitions, usage counters, denials, manual adjustments,
and recovery decisions remain authoritative. Roadmap 47 adds product projections and UX
metadata on top of those records rather than introducing a new accounting ledger.

**Rationale**: Roadmap 38 already owns reservation, commit, refund, fail-closed denial,
tenant scoping, stable reason codes, and audit evidence. Reusing it avoids split-brain
usage semantics and preserves backward compatibility.

**Alternatives considered**:

- New product billing ledger: rejected because it would duplicate accounting state and
  create reconciliation risk.
- Web-only aggregation: rejected because SDK consumers and support tools need stable
  daemon contracts.

## Decision: Public Dashboard Shows All Enforced Catalog Categories

The quota dashboard includes all enforced categories from the existing quota catalog,
grouped into user-readable sections.

**Rationale**: Every enforced category can deny work. Hiding categories would leave users
and support without context for some denials.

**Alternatives considered**:

- Show only categories with recent usage: rejected because a zero-usage category can still
  have a limit, override, or restriction.
- Show only launch and live-validation categories: rejected because tool, integration,
  storage, and evaluation quotas are also public denial sources.

## Decision: Usage Window Is Current Period Plus Previous Completed Period

Quota summaries expose the current active quota period and the immediately previous
completed period for each category.

**Rationale**: This explains reset behavior and recent spikes without turning the roadmap
into analytics. It is also bounded enough for schema, storage, and UI tests.

**Alternatives considered**:

- Current period only: rejected because reset behavior and support disputes need minimal
  history.
- Last 30 days for every category: rejected because quota periods can be daily, monthly,
  or none, and a calendar window could mislead users.

## Decision: Near-Limit Warnings Are Deterministic

Near-limit status appears when a finite quota category reaches 80% consumed or has less
than one category-defined typical operation remaining, whichever occurs first. Count and
attempt quotas use `1`; byte quotas use the catalog's configured artifact-write
reservation estimate.

**Rationale**: A pure percentage warning fails for low absolute limits, while a
category-defined operation-remaining check catches categories where the next action would
be denied without making byte quotas ambiguous.

**Alternatives considered**:

- Warn only at exhaustion: rejected because users need preventive action.
- Category-specific thresholds during planning: rejected because it would leave acceptance
  tests vague for this roadmap.

## Decision: Abuse Restrictions Hide Detection Signals

Tenant users see abuse restriction status, affected category, duration when available, and
recovery action. Detection signals, enforcement thresholds, and trigger details are not
exposed.

Abuse restrictions are represented by explicit additive billing abuse restriction records
plus audit evidence. They are not inferred from ordinary quota exhaustion and are not
modeled as tenant quota overrides.

**Rationale**: Users need actionable recovery information, but public threshold disclosure
would weaken abuse controls.

**Alternatives considered**:

- Generic restriction only: rejected because it creates unnecessary support load.
- Full trigger detail: rejected because it exposes evasion guidance.
- Support-only visibility: rejected because tenant users still need recovery guidance.

## Decision: Support Evidence Export Is Structured Redacted JSON

Support operators export a structured redacted JSON package for quota and abuse disputes.

**Rationale**: JSON gives support, tests, and SDK consumers a stable contract. Redaction is
explicit and testable.

**Alternatives considered**:

- Text report only: rejected because it is hard to validate and parse.
- Both JSON and text: deferred to a future presentation layer; this roadmap needs one
  stable export contract first.

## Decision: Contract Changes Are Additive

New dashboard, denial-detail, and evidence-export resources are added without repurposing
existing Roadmap 38 response fields. Existing `GET /v1/billing/plan`,
`GET /v1/billing/usage`, `GET /v1/billing/quotas`, and `GET /v1/billing/denials` remain
compatible.

**Rationale**: Existing SDK and client consumers already branch on stable billing fields
and quota denial payloads. Additive resources reduce migration cost and avoid semantic
breakage.

**Alternatives considered**:

- Replace current usage response with a dashboard response: rejected as a likely breaking
  semantic change.
- Web-only DTOs outside schemas: rejected because it would make SDK and contract testing
  weaker.

## Decision: Permission Boundaries Use Billing Visibility And `billing.evidence_export`

Quota dashboard and denial detail require billing visibility. Structured evidence export
requires the canonical additive `billing.evidence_export` permission.

**Rationale**: Billing visibility is already a product permission, but exporting support
evidence has higher privacy risk than viewing the dashboard.

**Alternatives considered**:

- Reuse `billing.view` for export: rejected because export packages may contain broader
  dispute metadata.
- Reuse `billing.manage`: rejected because plan mutation and support export are different
  duties.
- Role-only export gate: rejected because it would not be visible as a stable schema/SDK
  permission contract.
