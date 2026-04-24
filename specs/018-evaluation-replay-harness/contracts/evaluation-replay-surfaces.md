# Contract Surfaces: Evaluation And Replay Harness

## Goal

Add schema-backed evaluation surfaces that let the web operator shell launch default
non-live replays for curated candidates and inspect plane-level comparison outcomes without
reconstructing truth from raw logs or ad hoc scripts.

## New Evaluation Routes

### `GET /v1/evaluation/replay-candidates`

- Purpose: List curated replay candidates and engineer-managed fixture candidates for the
  active environment.
- Query parameters:
  - `candidateKind`
  - `sourceKind`
  - `readinessStatus`
  - `limit`
- Response requirements:
  - `environmentScope`
  - ordered `items`
  - each candidate includes `candidateId`, `candidateKind`, `displayName`, `sourceKind`,
    `sourceId`, `sourceRefs`, `readinessStatus`, `readinessReasons`, `limitations`,
    `defaultReplayMode`, `latestAttemptId`, `latestComparisonId`, and timestamps
- Truthfulness rules:
  - ordinary completed work is not listed unless curated or fixture-backed
  - candidates with missing evidence are listed as blocked or unreplayable, not hidden

Schema surfaces:

- add `schemas/api/replay-candidate-resource.schema.json`
- add `schemas/api/replay-candidate-list.response.schema.json`

### `GET /v1/evaluation/replay-candidates/{candidateId}`

- Purpose: Inspect one replay candidate and its replay readiness.
- Response requirements:
  - full candidate resource
  - source references and evidence limitations
  - latest attempt and comparison linkage when available

### `POST /v1/evaluation/replay-candidates/{candidateId}/attempts`

- Purpose: Launch a replay attempt for a curated candidate.
- Request requirements:
  - optional `mode`, defaulting to `non_live`
  - optional `changeWindowLabel`
  - optional `baselineAttemptId`
  - optional explicit live-validation safety scope when `mode` is `live_validation`
- Response requirements:
  - replay attempt resource with accepted, blocked, or unreplayable status
- Truthfulness rules:
  - omitted mode means non-live replay
  - non-live replay must not execute real side effects
  - approval-gated steps become blocked or evidence-only unless live validation is
    explicitly selected
  - live validation must persist explicit safety scope

Schema surfaces:

- add `schemas/api/create-replay-attempt.request.schema.json`
- add `schemas/api/replay-attempt-resource.schema.json`

### `GET /v1/evaluation/replay-attempts`

- Purpose: List replay attempts for operator history and shell refresh.
- Query parameters:
  - `candidateId`
  - `status`
  - `limit`
- Response requirements:
  - `environmentScope`
  - ordered replay attempt resources

Schema surfaces:

- add `schemas/api/replay-attempt-list.response.schema.json`

### `GET /v1/evaluation/replay-attempts/{attemptId}`

- Purpose: Inspect one replay attempt.
- Response requirements:
  - source linkage
  - replay mode and safety scope
  - terminal status
  - blocked reasons and evidence refs
  - result run/workflow linkage when produced

### `POST /v1/evaluation/replay-attempts/{attemptId}/compare`

- Purpose: Generate a plane-level comparison for a replay attempt.
- Request requirements:
  - optional `baselineAttemptId` or source baseline reference
  - optional `changeWindowLabel`
- Response requirements:
  - comparison result resource
- Truthfulness rules:
  - comparison includes terminal status plus runtime, policy, integration, delivery, and
    evidence summary differences where available
  - unknown or mixed drift is explicit
  - unavailable or redacted evidence is a limitation, not a matched result

Schema surfaces:

- add `schemas/api/create-replay-comparison.request.schema.json`
- add `schemas/api/replay-comparison-resource.schema.json`
- add `schemas/api/replay-drift-finding.schema.json`

### `GET /v1/evaluation/comparisons`

- Purpose: List comparison history.
- Query parameters:
  - `candidateId`
  - `attemptId`
  - `terminalStatus`
  - `limit`
- Response requirements:
  - `environmentScope`
  - ordered comparison resources

Schema surfaces:

- add `schemas/api/replay-comparison-list.response.schema.json`

### `GET /v1/evaluation/comparisons/{comparisonId}`

- Purpose: Inspect one plane-level comparison result.
- Response requirements:
  - terminal status
  - runtime, policy, integration, delivery, and evidence summaries
  - drift findings grouped by plane
  - limitations and confidence

### `GET /v1/evaluation/fixtures`

- Purpose: List engineer-managed fixtures and their candidate linkage for operator and
  test inspection.
- Query parameters:
  - `domainClass`
- Response requirements:
  - fixture metadata
  - source refs
  - assumptions
  - limitations
  - candidate linkage

Schema surfaces:

- add `schemas/api/replay-fixture-resource.schema.json`
- add `schemas/api/replay-fixture-list.response.schema.json`

## New Event Surfaces

### `evaluation.replay_started`

- Emitted when a replay attempt is accepted for processing.
- Payload includes `candidateId`, `attemptId`, `mode`, `environmentScope`, and source
  summary.

### `evaluation.replay_completed`

- Emitted when a replay attempt completes replay processing and produces evidence ready
  for comparison.
- Payload includes `candidateId`, `attemptId`, `status`, and evidence summary refs.

### `evaluation.replay_blocked`

- Emitted when a replay attempt cannot proceed because readiness, approval, side-effect,
  or evidence limitations block it.
- Payload includes `candidateId`, `attemptId`, `status`, and blocked reasons.

### `evaluation.replay_unreplayable`

- Emitted when a replay attempt reaches an unreplayable terminal state because the source
  scenario cannot produce a trustworthy replay.
- Payload includes `candidateId`, `attemptId`, `status`, and limitations.

### `evaluation.replay_failed`

- Emitted when replay processing itself fails before producing trustworthy replay
  evidence.
- Payload includes `candidateId`, `attemptId`, `status`, and failure reason.

### `evaluation.comparison_completed`

- Emitted when a comparison result is created.
- Payload includes `candidateId`, `attemptId`, `comparisonId`, `terminalStatus`, and drift
  planes present.

Schema surfaces:

- add `schemas/events/evaluation-replay-started.event.schema.json`
- add `schemas/events/evaluation-replay-completed.event.schema.json`
- add `schemas/events/evaluation-replay-blocked.event.schema.json`
- add `schemas/events/evaluation-replay-unreplayable.event.schema.json`
- add `schemas/events/evaluation-replay-failed.event.schema.json`
- add `schemas/events/evaluation-comparison-completed.event.schema.json`

## Reused Existing Routes

Evaluation records link back to authoritative detail routes instead of duplicating every
domain resource:

- `GET /v1/runs`
- `GET /v1/runs/{runId}`
- `GET /v1/runs/{runId}/events`
- `GET /v1/runs/{runId}/workflows`
- `GET /v1/runs/{runId}/workflows/{workflowId}`
- `GET /v1/schedules`
- `GET /v1/schedules/{scheduleId}`
- `GET /v1/integrations`
- `GET /v1/integrations/{integrationId}`
- `GET /v1/deliveries`
- `GET /v1/deliveries/{deliveryId}`
- `GET /v1/policy/approvals`
- `GET /v1/policy/approvals/{approvalId}`
- `GET /v1/runs/{runId}/computer-use/sessions`
- `GET /v1/runs/{runId}/computer-use/sessions/{computerUseSessionId}`
- `GET /v1/runs/{runId}/computer-use/sessions/{computerUseSessionId}/actions`
- `GET /v1/runs/{runId}/computer-use/sessions/{computerUseSessionId}/actions/{computerUseActionId}`
- `GET /v1/events`
- `GET /v1/events/stream`

## SDK Surface

`sdk/ts/src/index.ts` must add typed resources and client methods for:

- `listReplayCandidates(query?)`
- `getReplayCandidate(candidateId)`
- `createReplayAttempt(candidateId, input)`
- `listReplayAttempts(query?)`
- `getReplayAttempt(attemptId)`
- `createReplayComparison(attemptId, input?)`
- `listReplayComparisons(query?)`
- `getReplayComparison(comparisonId)`
- `listReplayFixtures(query?)`

The web shell must consume these SDK methods rather than raw `fetch` calls.

## Web Operator Shell Surface

The primary web shell must expose:

- curated candidate and fixture list
- candidate readiness and limitations
- default non-live replay launch
- explicit live validation scope when offered
- replay attempt status and source linkage
- plane-level comparison summary
- drift finding details with authoritative resource links
- environment scope and change window context

## Persistence And Restart Requirements

- replay candidates, attempts, comparison results, drift findings, fixture metadata,
  source refs, limitations, safety scope, and environment scope persist across daemon
  restart
- events are emitted for freshness and audit but are not the only source of replay state
- redacted or unavailable evidence remains marked as a limitation

## Documentation Surfaces

Docs updated by implementation:

- `docs/harness/harness-architecture.md`
- `docs/runtime/daemon-api-and-event-model.md`
- `docs/runtime/daemon-roadmaps.md`
- `docs/specs/018-evaluation-and-replay-harness.md`

## Truthfulness Constraints

- default replay mode is non-live
- ordinary completed work is not candidate-eligible by default
- fixture authoring is repo-managed and engineer-owned in phase 33
- comparison is plane-level and must not claim full artifact equality
- web operator shell support is required for roadmap closure
- evaluation records remain environment-scoped and secret-redacted
