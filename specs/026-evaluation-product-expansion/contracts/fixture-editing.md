# Contract: Product Fixture Editing

## Goal

Allow authorized users to create, edit, review, and inspect product-managed fixtures while
preserving provenance and keeping repo-managed fixtures immutable from product editing
paths.

## Permissions

| Permission | Purpose | Default Role Guidance |
|------------|---------|-----------------------|
| `evaluation.fixture.read` | View product fixture state, revisions, and provenance. | Operator, engineer, tenant admin. |
| `evaluation.fixture.manage` | Create and edit product-managed fixtures. | Engineer or tenant admin. |
| `evaluation.fixture.review` | Approve or reject fixture revisions for campaign use. | Tenant admin or explicit reviewer. |
| `evaluation.fixture.suppress` | Suppress or archive product fixtures. | Tenant admin or explicit evaluation operator. |

Permission tests must prove users without manage permission cannot create or edit
fixtures, and users without review permission cannot approve revisions.

## Resource Shape

### ProductFixtureResource

Required fields:

- `fixtureId`
- `tenantId`
- `displayName`
- `domainClass`
- `sourceKind`
- `currentRevisionId`
- `reviewState`
- `suppressionState`
- `retentionState`
- `createdBy`
- `createdAt`
- `updatedAt`

Optional fields:

- `sourceRefs`
- `sourceCandidateId`

### FixtureRevisionResource

Required fields:

- `revisionId`
- `fixtureId`
- `tenantId`
- `revisionNumber`
- `redactionStatus`
- `createdAt`

Optional fields:

- `fixturePayload`
- `contentSummary`
- `changeSummary`
- `sourceEvidenceRefs`
- `createdBy`

Revision records are immutable.

## API Routes

### `POST /v1/evaluation/discovered-candidates/{discoveredCandidateId}/product-fixtures`

Purpose: Materialize a discovered candidate into a product-managed fixture.

Request:

- `fixtureId`
- `displayName`
- `domainClass`
- `fixturePayload`
- `changeSummary`
- `idempotencyKey`

Response:

- `ProductFixtureMutationResponse` containing `fixture` and first `revision`

### `GET /v1/evaluation/product-fixtures`

Purpose: List tenant-scoped product-managed fixtures.

Query:

- `domainClass`
- `reviewState`
- `suppressionState`
- `cursor`
- `limit`

Ordering: `updatedAt DESC, fixtureId DESC`.

### `GET /v1/evaluation/product-fixtures/{fixtureId}`

Purpose: Inspect fixture head, provenance, current revision, review state, retention
state, and campaign usage summary.

### `POST /v1/evaluation/product-fixtures/{fixtureId}/revisions`

Purpose: Create a new immutable revision and optionally make it the current draft.

Request:

- `revisionId`
- `fixturePayload`
- `contentSummary`
- `changeSummary`
- `sourceEvidenceRefs`
- `idempotencyKey`

Response:

- `ProductFixtureMutationResponse` containing updated `fixture` and created `revision`

### `POST /v1/evaluation/product-fixtures/{fixtureId}/review`

Purpose: Approve, reject, or return a product fixture revision to draft.

Request:

- `revisionId`
- `decision`: `approved`, `rejected`, or `needs_changes`
- `reason`

Response:

- `ProductFixtureMutationResponse` containing updated `fixture`

### `POST /v1/evaluation/product-fixtures/{fixtureId}/suppress`

Purpose: Suppress or archive a product fixture from future discovery and campaign
selection.

Request:

- `reasonCode`
- `reason`
- optional `expiresAt`

Response:

- `ProductFixtureMutationResponse` containing updated `fixture`

## Repo-Managed Fixture Rules

- Existing repo-managed fixtures remain listed through the current fixture surfaces.
- Product editing routes must reject repo fixture ids unless the action explicitly creates
  a separate product-managed copy.
- Product-managed fixture ids must not collide with repo-managed fixture ids.
- Campaign selection records source type as `product_fixture` or
  `discovered_candidate` for Roadmap 41 product routes. Repo-managed fixtures remain
  available through the existing replay fixture routes and are not silently mutated by
  product fixture editing.
- Deletion or suppression of product fixture state does not delete repo fixture files.

## Review And Campaign Eligibility

- Draft fixtures are not selected for campaigns unless a campaign explicitly allows
  draft sources and records that policy.
- Approved fixtures are selectable unless suppressed, deleted, expired, or denied by
  tenant permissions.
- Rejected revisions remain auditable but are not selectable as current fixture content.

## Retention And Deletion

- Product-managed fixture retention updates the fixture head, revision availability, and
  campaign selection eligibility together.
- Deletion removes selectable product fixture payloads and marks the fixture deleted for
  future discovery and campaign selection.
- Fixture revision metadata, source provenance, audit records, and campaign snapshots
  remain readable where retention policy permits, but deleted payloads are not
  materialized into new campaigns.
- Retention or deletion of product-managed fixtures must not delete or rewrite
  repo-managed fixture files.

## Audit Events

Required audit/events:

- `evaluation.fixture.created`
- `evaluation.fixture.revision_created`
- `evaluation.fixture.reviewed`
- `evaluation.fixture.redaction_failed`
- `evaluation.fixture.suppressed`
- `evaluation.fixture.archived`
- `evaluation.fixture.deleted`
- `evaluation.fixture.denied`

Every event includes tenant id, actor id, fixture id, revision id where applicable,
source evidence refs, outcome, and reason code.

Resource schema: `schemas/api/evaluation-product-fixture-resource.schema.json`.
Event schema: `schemas/events/evaluation-fixture-created.event.schema.json`.

## Required Tests

- Product fixture creation preserves source candidate and redacted evidence provenance.
- Revision numbers are monotonic and immutable.
- Unauthorized create/edit/review actions are denied and audited.
- Product edits do not mutate repo-managed fixtures.
- Retention and deletion remove fixture selectability without removing repo-managed
  fixture files or immutable audit provenance.
- Approved, draft, rejected, suppressed, deleted, and expired states affect campaign
  eligibility exactly as contracted.
- SDK and web clients distinguish repo-managed fixtures from product-managed fixtures.
