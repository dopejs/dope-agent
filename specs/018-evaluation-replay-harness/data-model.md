# Data Model: Evaluation And Replay Harness

## Entities

### Replay Candidate

- Purpose: Curated representative work or engineer-managed fixture that can be replayed.
- Fields:
  - `candidateId`
  - `candidateKind`: `curated_work` or `fixture`
  - `displayName`
  - `description`
  - `sourceKind`: `run`, `workflow`, `schedule`, `integration`, `computer_use`, or
    `fixture`
  - `sourceId`
  - `sourceRefs`
  - `environmentScope`
  - `readinessStatus`: `fully_replayable`, `partially_replayable`, `blocked`, or
    `unreplayable`
  - `readinessReasons`
  - `limitations`
  - `defaultReplayMode`: `non_live`
  - `fixtureId`
  - `createdAt`
  - `updatedAt`
- Validation rules:
  - ordinary completed work is not candidate-eligible unless curated or represented by an
    engineer-managed fixture
  - every candidate must preserve source provenance and environment scope
  - blocked or unreplayable candidates must expose at least one readiness reason or
    limitation
  - default replay mode is non-live

### Replay Attempt

- Purpose: One replay execution request and outcome for a replay candidate.
- Fields:
  - `attemptId`
  - `candidateId`
  - `sourceRefs`
  - `environmentScope`
  - `mode`: `non_live` or `live_validation`
  - `status`: `queued`, `running`, `completed`, `blocked`, `unreplayable`, `failed`, or
    `cancelled`
  - `safetyScope`
  - `approvalHandling`: `blocked`, `evidence_only`, or `fresh_approval_required`
  - `sideEffectHandling`: `blocked`, `evidence_only`, or `live`
  - `launchedBy`
  - `changeWindowLabel`
  - `baselineAttemptId`
  - `resultRunId`
  - `resultWorkflowId`
  - `evidenceRefs`
  - `blockedReasons`
  - `startedAt`
  - `completedAt`
  - `createdAt`
  - `updatedAt`
- Validation rules:
  - non-live attempts must not execute real side effects
  - live validation attempts must persist explicit safety scope
  - every attempt must remain linked to one replay candidate
  - terminal attempts must expose one terminal status and supporting evidence or blocked
    reason

### Comparison Result

- Purpose: Operator-visible before/after comparison between a baseline and replay attempt.
- Fields:
  - `comparisonId`
  - `candidateId`
  - `baselineRef`
  - `attemptId`
  - `environmentScope`
  - `terminalStatus`: `matched`, `drifted`, `blocked`, or `unreplayable`
  - `runtimeSummary`
  - `policySummary`
  - `integrationSummary`
  - `deliverySummary`
  - `evidenceSummary`
  - `confidence`: `high`, `medium`, or `low`
  - `limitations`
  - `driftFindings`
  - `generatedAt`
- Validation rules:
  - comparison must include terminal status plus plane-level summaries where evidence is
    available
  - unknown or mixed drift must be reported explicitly when precise classification is not
    defensible
  - comparison results must not claim equality for evidence that was unavailable,
    redacted beyond reuse, or intentionally excluded

### Drift Finding

- Purpose: One material difference observed during comparison.
- Fields:
  - `findingId`
  - `comparisonId`
  - `plane`: `runtime`, `policy`, `integration`, `delivery`, `evidence`, `unknown`, or
    `mixed`
  - `severity`: `info`, `warning`, or `critical`
  - `summary`
  - `baselineValue`
  - `replayValue`
  - `evidenceRefs`
  - `recommendedAction`
  - `createdAt`
- Validation rules:
  - every material difference must map to one plane or explicitly state `unknown` or
    `mixed`
  - findings must link to supporting evidence references when available
  - redacted or unavailable evidence must be identified as a limitation rather than
    fabricated

### Regression Fixture

- Purpose: Engineer-curated, repo-managed scenario used for automated and manual replay
  validation.
- Fields:
  - `fixtureId`
  - `displayName`
  - `domainClass`: `schedule`, `integration`, or `computer_use`
  - `manifestPath`
  - `sourceRefs`
  - `capturedEvidenceRefs`
  - `assumptions`
  - `limitations`
  - `expectedReplayMode`
  - `expectedComparisonSummary`
  - `createdAt`
  - `updatedAt`
- Validation rules:
  - phase 33 must include at least one fixture for each required domain class
  - fixtures are authored through repo-managed definitions and captured evidence
  - operators may consume fixture replay/comparison outcomes but do not create or edit
    fixtures in-product

### Evaluation Event

- Purpose: Additive event emitted when replay or comparison state changes.
- Fields:
  - `eventId`
  - `name`: `evaluation.replay_started`, `evaluation.replay_completed`,
    `evaluation.replay_blocked`, `evaluation.replay_unreplayable`,
    `evaluation.replay_failed`, or `evaluation.comparison_completed`
  - `environmentScope`
  - `candidateId`
  - `attemptId`
  - `comparisonId`
  - `occurredAt`
  - `payload`
- Validation rules:
  - events are freshness and audit signals, not the sole source of replay state
  - event payloads must not expose unredacted secret material

## State Transitions

### Replay Candidate

- `fully_replayable` -> `partially_replayable` when optional evidence is missing but a
  bounded replay remains possible
- any replayable state -> `blocked` when required source evidence, policy context, or
  fixture data is missing
- any state -> `unreplayable` when the source scenario cannot be evaluated safely or
  truthfully
- `blocked` or `unreplayable` -> replayable only after the missing evidence or fixture
  limitation is resolved by engineer curation

### Replay Attempt

- `queued` -> `running` when replay processing starts
- `queued` or `running` -> `blocked` when required evidence, approvals, or side-effect
  boundaries prevent default non-live replay
- `queued` or `running` -> `unreplayable` when the candidate cannot produce a trustworthy
  replay
- `running` -> `completed` when replay finishes and evidence is ready for comparison
- `running` -> `failed` when replay processing itself fails
- any non-terminal state -> `cancelled` if cancellation is supported during
  implementation

### Comparison Result

- created as `matched`, `drifted`, `blocked`, or `unreplayable`
- regenerated only by creating or replacing a comparison for a specific attempt and
  baseline; prior comparison history remains auditable unless explicitly superseded

## Relationships

- one replay candidate may have many replay attempts
- one replay attempt belongs to exactly one replay candidate
- one replay attempt may produce zero or more comparison results
- one comparison result has zero or more drift findings
- one regression fixture may produce one replay candidate
- one replay candidate or attempt may reference many source resources through
  `sourceRefs`
- one evaluation event references one candidate and optionally one attempt or comparison

## Derived Views

- web operator shell candidate list filters by `readinessStatus`, `candidateKind`,
  `sourceKind`, and environment scope
- replay attempt history filters by `candidateId`, terminal status, change window, and
  environment scope
- comparison view groups drift findings by plane
- operator activity and diagnostics may summarize replay and comparison outcomes but do
  not replace the canonical evaluation records
