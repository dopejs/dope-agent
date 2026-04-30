# Contract: Candidate Discovery

## Goal

Provide tenant-scoped, explainable, bounded historical candidate discovery without
triggering unbounded scans from product reads or leaking sensitive evidence.

## Source Tables And APIs

Discovery may inspect only tenant-safe projections of:

- `runs`
- `workflows`
- `tool_calls`
- `evaluation_replay_candidates`
- `evaluation_replay_attempts`
- `evaluation_comparisons`
- `evaluation_regression_fixtures`
- `live_validation_attempts`
- `live_validation_ledger_entries`
- domain projection tables already covered by tenant-aware accessors where they explain a
  run or workflow source

Discovery must use daemon-owned store accessors. It must not query raw tables without a
tenant filter, and it must not bypass existing tenant binding checks.

## Permissions

| Permission | Purpose | Default Role Guidance |
|------------|---------|-----------------------|
| `evaluation.discovery.read` | View discovery policies, runs, candidates, and redacted evidence. | Operator and tenant admin roles. |
| `evaluation.discovery.run` | Start or resume bounded discovery. | Tenant admin or explicit evaluation operator. |
| `evaluation.discovery.suppress` | Suppress runs, workflows, candidates, or fixtures. | Tenant admin or explicit evaluation operator. |
| `evaluation.fixture.manage` | Materialize discovered candidate evidence into product fixtures. | Tenant admin or explicit evaluation editor. |

Permission tests must prove viewer/read-only users cannot start discovery, suppress
sources, or materialize fixtures.

## Scan Bounds

Discovery policies are persisted tenant resources. Operators with
`evaluation.discovery.run` can create or update policy bounds; read-only users can inspect
the active policy but cannot loosen bounds, change redaction rules, or enable discovery.

Every Discovery Run records and enforces:

- `tenantId`
- `windowStart` and `windowEnd`
- `sourceKinds`
- `maxInspectedRecords`
- `maxEmittedCandidates`
- `costBudget`
- `cursor`
- `idempotencyKey`

Default bounds should be conservative in the test environment and configurable per
tenant. A run that reaches any bound returns `partial` with `partialReason` rather than
continuing silently.

## Incremental Job Behavior

- Discovery starts with `POST /v1/evaluation/discovery-runs`.
- The request must include a client idempotency key or receive a server-generated one
  returned in the response.
- Repeating the same tenant and idempotency key returns the existing Discovery Run.
- Runs are restart-safe: a persisted cursor controls resume behavior.
- Discovery workers must check suppressions before emitting candidates and before
  materializing evidence.
- Product page loads use `GET` routes over stored discovery results. They do not trigger
  raw historical scans.

## Candidate Scoring And Explanation Fields

Each Discovered Candidate includes:

- `score` and `scoreBand`
- `sourceKind`, `sourceId`, and `sourceRefs`
- `explanationFields`, such as failure recurrence, drift signal, tool-call class,
  live-validation outcome, workflow coverage, recency, and operator relevance
- `discoveryBounds`, showing the policy and run limits used
- `redactionStatus`

The score is advisory. Campaign selection and fixture materialization require explicit
operator action or an approved campaign policy.

## Redaction Rules

Discovery must exclude or redact before persistence:

- secrets
- credentials
- raw access or refresh tokens
- bearer tokens and session tokens
- configured sensitive fields
- tenant-specific sensitive evidence rules
- provider payload fragments marked secret or credential material

Candidate Evidence stores redacted summaries and redaction metadata. Raw evidence must not
be written to discovery, fixture, campaign, dashboard, audit, or inspection records.

## API Routes

### DiscoveryPolicyResource

Required fields:

- `policyId`
- `tenantId`
- `enabled`
- `sourceKinds`
- `windowStart`
- `windowEnd`
- `maxInspectedRecords`
- `maxEmittedCandidates`
- `costBudget`
- `sensitiveFieldRules`
- `retentionPolicyRef`
- `createdBy`
- `createdAt`
- `updatedAt`

### `GET /v1/evaluation/discovery-policies`

Purpose: List tenant-scoped discovery policies, including the currently active policy.

Query:

- `enabled`
- `cursor`
- `limit`

Ordering: `updatedAt DESC, policyId DESC`.

### `GET /v1/evaluation/discovery-policies/{policyId}`

Purpose: Inspect one tenant-scoped discovery policy, including configured bounds,
redaction rules, retention reference, enabled state, and audit metadata.

Response:

- `DiscoveryPolicyResource`
- schema: `schemas/api/evaluation-discovery-policy-resource.schema.json`

### `PUT /v1/evaluation/discovery-policies/{policyId}`

Purpose: Create or update tenant-scoped discovery policy bounds, source rules,
redaction rules, retention reference, and incremental discovery behavior.

Request:

- `enabled`
- `windowStart`
- `windowEnd`
- `sourceKinds`
- `maxInspectedRecords`
- `maxEmittedCandidates`
- `costBudget`
- `sensitiveFieldRules`
- `retentionPolicyRef`
- `idempotencyKey`

Response:

- `DiscoveryPolicyResource`
- audit outcome for policy creation or update
- schema: `schemas/api/evaluation-discovery-policy-resource.schema.json`

### `POST /v1/evaluation/retention/apply`

Purpose: Apply tenant-scoped retention/deletion policy across Roadmap 41 product
resources without mutating repo-managed fixtures or runtime truth.

Request:

- `resourceKinds`: `discovered_candidate`, `candidate_evidence`, `product_fixture`,
  `campaign`, `dashboard_projection`, or `tool_call_inspection`
- optional `windowEnd`
- optional `dryRun`
- `idempotencyKey`

Response:

- retention/deletion application summary by resource kind
- counts for expired, deleted, tombstoned, skipped, and failed-closed records
- audit event references

### `POST /v1/evaluation/discovery-runs`

Purpose: Start or resume bounded tenant-scoped discovery.

Request:

- `windowStart`
- `windowEnd`
- `sourceKinds`
- `maxInspectedRecords`
- `maxEmittedCandidates`
- `costBudget`
- `idempotencyKey`

Response:

- `DiscoveryRunResource`
- status: `queued`, `running`, `completed`, `partial`, `failed`, or `cancelled`
- current bounds, cursor summary, and emitted count
- schema: `schemas/api/evaluation-discovery-run-resource.schema.json`

### `GET /v1/evaluation/discovery-runs`

Purpose: List tenant-scoped discovery runs.

Query:

- `status`
- `sourceKind`
- `cursor`
- `limit`

Ordering: `updatedAt DESC, discoveryRunId DESC`.

### `GET /v1/evaluation/discovery-runs/{discoveryRunId}`

Purpose: Inspect a run, its bounds, status, cursor summary, counts, and partial/failure
reason.

### `GET /v1/evaluation/discovered-candidates`

Purpose: List redacted tenant-scoped discovered candidates.

Query:

- `discoveryRunId`
- `sourceKind`
- `readinessStatus`
- `suppressionState`
- `scoreBand`
- `cursor`
- `limit`

Ordering: `score DESC, updatedAt DESC, discoveredCandidateId DESC`.

### `GET /v1/evaluation/discovered-candidates/{discoveredCandidateId}`

Purpose: Inspect one candidate, explanation, source provenance, redacted evidence
summary, suppression state, and retention state.

Schema: `schemas/api/evaluation-discovered-candidate-resource.schema.json`.

### `POST /v1/evaluation/suppressions`

Purpose: Create a suppression for a run, workflow, discovered candidate, product fixture,
repo fixture, or source family.

Request:

- `targetKind`
- `targetId`
- `targetSourceRef`
- `reasonCode`
- `reason`
- optional `expiresAt`
- `idempotencyKey`

Response:

- `SuppressionRecordResource`
- shared schema: `schemas/api/evaluation-product-pagination.schema.json`

## Retention And Deletion

- Discovered candidates and Candidate Evidence expire according to tenant retention
  policy.
- Deletion removes selectable product candidate/evidence payloads and marks tombstones
  where needed for campaign history.
- Retention application is an explicit product operation that updates discovered
  candidates, Candidate Evidence, product-managed fixtures, campaign result detail
  payloads, dashboard projections, and tool-call inspection payloads consistently within
  the tenant.
- Discovery policy retention references must be resolved before a discovery run starts;
  a missing or unauthorized retention policy fails closed.
- Suppression prevents future discovery emission and campaign selection but does not
  mutate immutable repo-managed fixtures or runtime truth.
- Campaign snapshots keep immutable references to deleted or expired sources and indicate
  the current retention state.

## Audit Events

Schema-backed events and audit records are required for:

- `evaluation.discovery.started`
- `evaluation.discovery.completed`
- `evaluation.discovery.partial`
- `evaluation.discovery.failed`
- `evaluation.discovery.policy_changed`
- `evaluation.discovery.candidate_suggested`
- `evaluation.discovery.redaction_failed`
- `evaluation.discovery.suppressed`
- `evaluation.discovery.retention_applied`

Every audit/event payload includes tenant id, actor where available, target id, outcome,
reason code, bounds or policy reference where applicable, and redacted evidence
references.

Discovery lifecycle, suppression, redaction-failed, and retention-applied events are
validated by `schemas/events/evaluation-discovery-started.event.schema.json`. Shared
audit/retention envelopes are validated by
`schemas/events/evaluation-product-audit-recorded.event.schema.json`.

## Compatibility Rules

- Existing `GET /v1/evaluation/replay-candidates` remains unchanged for legacy replay
  candidates and repo fixtures.
- Discovered candidates are separate resources until explicit materialization or campaign
  selection.
- Adding discovered candidates must not add new enum values to existing replay candidate
  schemas unless client compatibility work and contract tests are updated in the same
  change.

## Required Tests

- Discovery never reads or emits cross-tenant records.
- Discovery stops on time, inspected-record, emitted-candidate, and cost bounds.
- Discovery policy create/update/list routes enforce tenant permissions and persisted
  policy bounds.
- Page loads and dashboard refreshes do not start historical scans.
- Redaction occurs before persistence and display.
- Suppressed targets are excluded from future discovery and campaign selection.
- Retention deletes or expires discovered candidates, Candidate Evidence, product-managed
  fixtures, campaign result detail payloads, dashboard projections, and tool-call
  inspection payloads without mutating runtime truth or repo-managed fixtures.
- Idempotent discovery start returns the same Discovery Run.
