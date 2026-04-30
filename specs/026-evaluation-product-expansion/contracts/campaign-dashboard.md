# Contract: Campaign And Dashboard

## Goal

Group replay attempts, comparisons, discovered candidates, fixtures, and live-validation
outcomes into tenant-owned campaigns and retention-aware dashboard projections.

## Permissions

| Permission | Purpose | Default Role Guidance |
|------------|---------|-----------------------|
| `evaluation.campaign.read` | View campaigns, results, and dashboard projections. | Operator, product viewer, tenant admin. |
| `evaluation.campaign.manage` | Create, start, cancel, and publish campaigns. | Tenant admin or evaluation operator. |
| `evaluation.dashboard.read` | View dashboard aggregates and detail pages. | Operator, product viewer, tenant admin. |

Campaign manage permission does not grant fixture edit permission.

## Campaign Lifecycle

Allowed states:

- `draft`
- `queued`
- `running`
- `completed`
- `published`
- `failed`
- `cancelled`

Allowed transitions:

- `draft -> queued`
- `queued -> running`
- `running -> completed`
- `completed -> published`
- `draft/queued/running -> cancelled`
- `queued/running -> failed`

Terminal states keep immutable result evidence until retention removes product-side
projections.

## Campaign Resource Shape

### CampaignResource

Required fields:

- `campaignId`
- `displayName`
- `status`
- `retentionState`
- `createdAt`

Optional fields:

- `tenantId`
- `scopeSummary`
- `startedBy`
- `startedAt`
- `completedAt`
- `publishedAt`
- `idempotencyKey`

### CampaignItemResource

Required fields:

- `campaignItemId`
- `campaignId`
- `tenantId`
- `sourceType`
- `sourceId`
- `sourceSnapshot`
- `selectionReason`
- `suppressionCheckedAt`
- `createdAt`

Allowed `sourceType` values:

- `discovered_candidate`
- `product_fixture`
- `campaign_item`

### CampaignAttemptGroupResource

Required fields:

- `attemptGroupId`
- `campaignId`
- `campaignItemId`
- `tenantId`
- `status`
- `replayAttemptIds`
- `comparisonIds`
- `liveValidationIds`
- `driftCount`
- `failureCount`
- `unsupportedCount`
- `operatorActionNeededCount`
- `summary`
- `createdAt`
- `updatedAt`

## API Routes

### `POST /v1/evaluation/campaigns`

Purpose: Create a campaign draft or create-and-queue a campaign from selected sources.

Request:

- `campaignId`
- `displayName`
- `sourceSelections`
- `scopeSummary`
- `startImmediately`
- `idempotencyKey`

Response:

- `CampaignResource`

Selected `CampaignItemResource` records are read through
`GET /v1/evaluation/campaigns/{campaignId}/items`.

### `GET /v1/evaluation/campaigns`

Purpose: List tenant-scoped campaigns.

Query:

- `cursor`
- `limit`

Ordering: `createdAt DESC, campaignId DESC`.

### `GET /v1/evaluation/campaigns/{campaignId}`

Purpose: Inspect campaign identity, lifecycle status, selected immutable sources, result
summary, retention state, and publication status.

### `POST /v1/evaluation/campaigns/{campaignId}/start`

Purpose: Start a draft campaign or resume a queued campaign.

Request:

- `idempotencyKey`
- optional `changeWindowLabel`

Response:

- updated `CampaignResource`

### `POST /v1/evaluation/campaigns/{campaignId}/cancel`

Purpose: Stop queued or running campaign work that has not already completed.

Response:

- updated `CampaignResource`
- cancellation summary

### `POST /v1/evaluation/campaigns/{campaignId}/publish-results`

Purpose: Mark completed campaign results as published for dashboard and release-readiness
review.

Response:

- updated `CampaignResource`
- published result summary

### `GET /v1/evaluation/campaigns/{campaignId}/items`

Purpose: List immutable campaign items.

Query:

- `cursor`
- `limit`

Ordering: `createdAt DESC, campaignItemId DESC`.

### `GET /v1/evaluation/campaigns/{campaignId}/attempt-groups`

Purpose: List replay attempt groups and linked comparisons/live validations.

Query:

- `cursor`
- `limit`

Ordering: `updatedAt DESC, attemptGroupId DESC`.

### `GET /v1/evaluation/dashboard`

Purpose: Return tenant-scoped dashboard aggregate projections.

Query:

- `cursor`
- `limit`

Response:

- `EvaluationProductListResponse<DashboardProjectionResource>`
- summary counts for campaigns, candidates, fixtures, drift, failures, unsupported replay,
  live-validation linkage, operator-action-needed states, generated window, and
  retention state

## Live Validation Linkage

Campaign attempt groups may include Roadmap 40 `validationId` and ledger references. The
campaign result must:

- link live-validation evidence instead of copying or replacing runtime truth
- classify missing, denied, aborted, failed, unsupported, completed, and
  operator-action-needed live-validation states
- keep campaign source snapshots immutable even if live-validation retention later
  removes product projections

## Pagination And Ordering

New campaign and dashboard lists use cursor pagination. Ordering is deterministic:

- campaigns: `updatedAt DESC, campaignId DESC`
- items: `createdAt DESC, campaignItemId DESC`
- attempt groups: `updatedAt DESC, attemptGroupId DESC`
- dashboard detail lists: `updatedAt DESC, stable id DESC`

## Retention Behavior

- Draft and cancelled campaigns may expire according to tenant policy.
- Published campaign result summaries remain readable until result retention expires.
- Retention may remove product-side detail payloads while preserving campaign identity,
  aggregate result summary, and audit evidence.
- A retained campaign snapshot must identify when source content has since been deleted,
  expired, or suppressed.
- Dashboard projections must filter expired or deleted product payloads while preserving
  aggregate counts and retention-state indicators needed for release-readiness review.
- Retention application across candidates, evidence, product fixtures, campaign results,
  dashboard projections, and tool-call inspections must be tenant-scoped and auditable.

## Audit Events

Required audit/events:

- `evaluation.campaign.created`
- `evaluation.campaign.started`
- `evaluation.campaign.cancelled`
- `evaluation.campaign.completed`
- `evaluation.campaign.failed`
- `evaluation.campaign.results_published`
- `evaluation.campaign.redaction_failed`
- `evaluation.dashboard.projection_generated`

Every event includes tenant id, actor where available, campaign id, outcome, reason code,
and evidence references.

Campaign schema: `schemas/api/evaluation-campaign-resource.schema.json`.
Campaign event schema: `schemas/events/evaluation-campaign-created.event.schema.json`.
Dashboard schema: `schemas/api/evaluation-dashboard-resource.schema.json`.
Dashboard event schema:
`schemas/events/evaluation-dashboard-projection-generated.event.schema.json`.

## Required Tests

- Campaign selection rejects suppressed, expired, deleted, or cross-tenant sources.
- Campaign start is idempotent by idempotency key.
- Campaign item snapshots remain stable when source fixtures or candidates later change.
- Aggregates count drift, failure, unsupported replay, live-validation linkage, and
  operator-action-needed states correctly.
- Dashboard pagination returns no duplicates, missing records, or cross-tenant records.
- Campaign contract tests prove live-validation outcomes link to Roadmap 40 side-effect
  ledger records without replacing runtime truth.
