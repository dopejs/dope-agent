# Data Model: Evaluation Product Expansion

## Overview

Roadmap 41 adds product-managed evaluation state beside existing replay harness and
live-validation tables. All new entities are tenant-scoped unless explicitly marked
global, store complete redacted resource JSON, and expose indexed columns for tenant,
status, source reference, timestamps, and pagination.

## Entities

### Discovery Policy

- **Purpose**: Captures tenant-scoped discovery configuration and operator-visible scan
  bounds.
- **Key fields**: `policyId`, `tenantId`, `enabled`, `windowStart`, `windowEnd`,
  `maxInspectedRecords`, `maxEmittedCandidates`, `costBudget`,
  `sensitiveFieldRules`, `retentionPolicyRef`, `createdBy`, `createdAt`, `updatedAt`.
- **Relationships**: Used by Discovery Runs. References tenant permissions and retention
  policy.
- **Validation rules**: Bounds must be positive. Sensitive field rules must be applied
  before evidence persistence. Policy cannot enable unbounded scans.

### Discovery Run

- **Purpose**: A bounded, restart-safe scan for candidate evidence within one tenant.
- **Key fields**: `discoveryRunId`, `tenantId`, `policyId`, `status`, `cursor`,
  `sourceKinds`, `windowStart`, `windowEnd`, `maxInspectedRecords`,
  `maxEmittedCandidates`, `costBudget`, `inspectedRecords`, `emittedCandidates`,
  `partialReason`, `startedBy`, `startedAt`, `completedAt`, `updatedAt`,
  `idempotencyKey`.
- **Relationships**: Emits Discovered Candidates and Candidate Evidence. Reads source
  data from runs, workflows, tool calls, replay attempts, comparisons, repo fixtures, and
  live-validation evidence through tenant-safe accessors.
- **State transitions**: `queued -> running -> completed`; `queued/running -> failed`;
  `running -> partial`; `queued/running -> cancelled`; terminal states are immutable
  except for retention metadata.
- **Validation rules**: Exactly one tenant. Cursor advances monotonically. Re-running
  with the same idempotency key returns the same run.

### Discovered Candidate

- **Purpose**: An automatically suggested replay opportunity before fixture or campaign
  materialization.
- **Key fields**: `discoveredCandidateId`, `tenantId`, `discoveryRunId`, `sourceKind`,
  `sourceId`, `sourceRefs`, `score`, `scoreBand`, `explanationFields`,
  `redactionStatus`, `evidenceRef`, `readinessStatus`, `suppressionState`,
  `retentionState`, `createdAt`, `updatedAt`, `expiresAt`.
- **Relationships**: Belongs to a Discovery Run. May be converted to a Product Fixture
  or selected as a Campaign Item. May be covered by a Suppression Record.
- **State transitions**: `suggested -> selected`; `suggested/selected -> suppressed`;
  `suggested/selected -> expired`; `suggested/selected -> deleted`.
- **Validation rules**: Evidence must be redacted. Source references must resolve within
  the same tenant at creation time. Suppressed or expired candidates are not selectable
  for new campaigns.

### Candidate Evidence

- **Purpose**: Redacted explanatory evidence for a discovered candidate.
- **Key fields**: `evidenceId`, `tenantId`, `discoveredCandidateId`, `sourceRefs`,
  `summary`, `redactedPayload`, `redactionRulesApplied`, `sensitiveFieldsExcluded`,
  `materializationAllowed`, `retentionState`, `createdAt`, `expiresAt`.
- **Relationships**: Belongs to a Discovered Candidate. Can seed Product Fixture
  revisions and Tool-Call Inspection records.
- **Validation rules**: Raw secrets, credentials, tokens, and configured sensitive fields
  are never persisted. Evidence must record which redaction rules were applied.

### Suppression Record

- **Purpose**: Operator decision that excludes a run, workflow, candidate, fixture, or
  source family from future discovery or campaign selection.
- **Key fields**: `suppressionId`, `tenantId`, `targetKind`, `targetId`,
  `targetSourceRef`, `reasonCode`, `reason`, `createdBy`, `createdAt`, `expiresAt`,
  `active`.
- **Relationships**: Evaluated by discovery, fixture selection, and campaign selection.
- **State transitions**: `active -> expired`; `active -> revoked`.
- **Validation rules**: Target must be tenant-owned or tenant-scoped. Suppression does
  not mutate repo-managed fixtures or immutable runtime truth.

### Product-Managed Fixture

- **Purpose**: A fixture created or edited in the product while preserving provenance.
- **Key fields**: `fixtureId`, `tenantId`, `displayName`, `domainClass`, `sourceKind`,
  `sourceRefs`, `sourceCandidateId`, `currentRevisionId`, `reviewState`,
  `suppressionState`, `retentionState`, `createdBy`, `createdAt`, `updatedAt`.
- **Relationships**: Has Fixture Revisions. May originate from Discovered Candidate or
  Candidate Evidence. Can be selected by Campaign Items. Does not overwrite
  Repo-Managed Fixture.
- **State transitions**: `draft -> in_review -> approved`; any non-deleted state may
  transition to `suppressed`, `archived`, or `deleted` according to permissions and
  retention policy.
- **Validation rules**: Editing requires evaluation fixture permission. Current revision
  must point to a valid immutable revision. Repo-managed fixture paths are not writable by
  product fixture edits.

### Fixture Revision

- **Purpose**: Immutable record of product fixture content and edit provenance.
- **Key fields**: `revisionId`, `fixtureId`, `tenantId`, `revisionNumber`,
  `contentSummary`, `fixturePayload`, `changeSummary`, `sourceEvidenceRefs`,
  `redactionStatus`, `createdBy`, `createdAt`.
- **Relationships**: Belongs to Product-Managed Fixture. May reference Candidate Evidence
  and Replay Attempt evidence.
- **Validation rules**: Revision numbers are monotonic per fixture. Revisions are
  immutable after creation. Payload must pass fixture validation before becoming current.

### Replay Campaign

- **Purpose**: Tenant-owned grouping of selected fixtures or candidates, replay attempts,
  comparison summaries, live-validation linkage, and aggregate status.
- **Key fields**: `campaignId`, `tenantId`, `displayName`, `status`, `scopeSummary`,
  `startedBy`, `createdAt`, `startedAt`, `completedAt`, `publishedAt`,
  `retentionState`, `idempotencyKey`.
- **Relationships**: Has Campaign Items, Campaign Attempt Groups, Dashboard Projections,
  and Tool-Call Inspection records.
- **State transitions**: `draft -> queued -> running -> completed -> published`;
  `draft/queued/running -> cancelled`; `running/completed -> failed`;
  terminal result evidence remains readable until retention removes product projections.
- **Validation rules**: Campaign starts require tenant admin or campaign permission.
  Starting with the same idempotency key returns the same campaign start result.

### Campaign Item

- **Purpose**: Immutable selected source within a campaign.
- **Key fields**: `campaignItemId`, `campaignId`, `tenantId`, `sourceType`, `sourceId`,
  `sourceSnapshot`, `selectionReason`, `suppressionCheckedAt`, `createdAt`.
- **Relationships**: References Discovered Candidate, Product-Managed Fixture,
  Repo-Managed Fixture, or existing Replay Candidate by immutable snapshot.
- **Validation rules**: Source snapshot is fixed at campaign start. Suppressed, deleted,
  or expired sources are not selectable for new campaigns.

### Campaign Attempt Group

- **Purpose**: Groups replay attempts, comparisons, live-validation outcomes, and
  operator-action-needed signals for one Campaign Item.
- **Key fields**: `attemptGroupId`, `campaignId`, `campaignItemId`, `tenantId`,
  `replayAttemptIds`, `comparisonIds`, `liveValidationIds`, `status`,
  `driftCount`, `failureCount`, `unsupportedCount`, `operatorActionNeededCount`,
  `summary`, `createdAt`, `updatedAt`.
- **Relationships**: Links existing Replay Attempts, Comparison Results, and Roadmap 40
  Live Validation Attempts/Ledger entries.
- **State transitions**: `queued -> running -> completed`; `queued/running -> failed`;
  `queued/running -> cancelled`; completed groups may later be published as part of the
  campaign result.
- **Validation rules**: Linked attempts and live-validation records must belong to the
  same tenant and selected source snapshot.

### Dashboard Projection

- **Purpose**: Retention-aware aggregate view for evaluation product status and trends.
- **Key fields**: `projectionId`, `tenantId`, `windowStart`, `windowEnd`,
  `campaignStatusCounts`, `driftSummary`, `failureSummary`, `unsupportedSummary`,
  `operatorActionNeededSummary`, `liveValidationSummary`, `candidateSummary`,
  `fixtureSummary`, `generatedAt`, `cursor`, `retentionState`.
- **Relationships**: Derived from campaigns, campaign attempt groups, discovered
  candidates, product fixtures, replay comparisons, and live-validation ledger entries.
- **Validation rules**: Projection must be tenant-scoped and deterministic for the same
  filters and cursor. It must not require raw historical discovery scans at read time.

### Tool-Call Inspection

- **Purpose**: User-facing comparison between original, non-live replay, live-validation,
  unsupported, and missing evidence states.
- **Key fields**: `inspectionId`, `tenantId`, `campaignId`, `campaignItemId`,
  `toolCallRef`, `originalEvidenceRef`, `nonLiveReplayEvidenceRef`,
  `liveValidationLedgerRefs`, `classification`, `diffSummary`, `redactionStatus`,
  `retentionState`, `createdAt`, `updatedAt`.
- **Relationships**: Links runtime tool calls, replay attempts, comparison results, and
  Roadmap 40 ledger entries without replacing runtime truth.
- **Validation rules**: All displayed payloads are redacted. Missing, expired, denied,
  aborted, failed, unsupported, and completed live-validation states are explicit.

### Evaluation Audit Event

- **Purpose**: Tenant-scoped audit record for product evaluation actions.
- **Key fields**: `auditEventId`, `tenantId`, `actorId`, `action`, `targetKind`,
  `targetId`, `outcome`, `reasonCode`, `evidenceRefs`, `createdAt`.
- **Relationships**: References discovery policies, discovery runs, suppressions,
  fixtures, revisions, campaigns, dashboard projections, result publications, inspection
  records, retention/deletion applications, and redaction failures.
- **Validation rules**: Audit records are append-only. Denials, redaction failures,
  retention/deletion applications, and other failed-closed behavior are recorded without
  leaking secrets.

## Source Data And Access Rules

- Candidate discovery may inspect `runs`, `workflows`, `tool_calls`, existing evaluation
  replay candidates/attempts/comparisons/fixtures, and live-validation ledger records
  only through tenant-safe store accessors.
- Product records must include `tenant_id` and indexes for tenant-scoped list queries.
- Cross-tenant references are invalid even if the caller has global operator privileges;
  global operators must select an explicit tenant context.
- Retention may remove product evidence payloads and selectable records, but immutable
  audit records and underlying runtime truth remain distinct.

## Query And Pagination Rules

- List ordering is deterministic: primary sort by relevant timestamp descending, then
  stable id descending.
- Cursor pagination is required for discovery candidates, product fixtures, campaigns,
  campaign items, dashboard detail lists, and tool-call inspections.
- Limit-only reads may remain for legacy replay routes but new Roadmap 41 routes must not
  rely on offset pagination for mutable result sets.
