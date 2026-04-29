# Contract: Live Validation Surfaces

## Goal

Expose explicit live-validation controls and evidence through daemon-owned contracts,
SDK types, and operator-visible inspection without changing non-live replay defaults.

## Permissions

| Permission | Purpose | Default Role Guidance |
|------------|---------|-----------------------|
| `live_validation.execute` | Start explicit live validation after readiness gates. | Existing operator capability remains valid for start requests. |
| `live_validation.reconcile` or owner/admin authority | Resolve operator-action-needed reconciliation states. | Add explicit permission or equivalent role check; ordinary live-validation executors cannot resolve reconciliation. |

Role and permission contract tests must prove:

- owner/admin or explicit reconciliation permission can resolve reconciliation states,
- users with only `live_validation.execute` cannot resolve reconciliation states,
- viewer/read-only users cannot start live validation or resolve reconciliation states.

## API Routes

### `POST /v1/evaluation/replay-candidates/{candidateId}/live-validations`

- Purpose: start explicit live validation for a replay candidate.
- Required gates before start:
  - tenant context and candidate ownership
  - `live_validation.execute`
  - hosted quota preflight for `live_validation_attempts`
  - tenant/global kill-switch check
  - replay support matrix readiness
  - explicit side-effect scope
  - required fresh approvals or transition to `awaiting_approval`
- Request body:
  - `scope`: included/excluded tool classes and optional action allowlist
  - `changeWindowLabel`
  - `clientKey` or validation request id for idempotent start
  - optional `approvalRefs` for already-resolved fresh approvals
- Response:
  - live validation attempt resource
  - status: `queued`, `awaiting_approval`, `running`, `blocked`, or terminal outcome
  - gate decisions and blocked reasons

### `GET /v1/live-validations`

- Purpose: list tenant-scoped live validation attempts.
- Query parameters:
  - `candidateId`
  - `status`
  - `limit`
- Response:
  - ordered live validation attempt resources

### `GET /v1/live-validations/{validationId}`

- Purpose: inspect one live validation attempt.
- Response:
  - attempt resource
  - gate decisions
  - approval summary
  - ledger summary
  - comparison link
  - retention policy summary

### `POST /v1/live-validations/{validationId}/abort`

- Purpose: abort a live validation attempt.
- Behavior:
  - aborts pending and future side effects
  - preserves already-submitted side effects as completed, failed, or
    operator-action-needed evidence
  - returns updated attempt and ledger summary

### `GET /v1/live-validations/{validationId}/ledger`

- Purpose: list side-effect ledger entries for the validation.
- Query parameters:
  - `outcome`
  - `toolClass`
  - `limit`
- Response:
  - ordered ledger resources

### `GET /v1/live-validations/support-matrix`

- Purpose: expose the effective replay support matrix for operator and contract
  inspection.
- Response:
  - matrix version
  - rows with safety class, approval, idempotency, retry, ambiguous commit,
    compensation, ledger, and test declarations

### `POST /v1/live-validations/kill-switches`

- Purpose: enable or disable tenant or global live-validation kill switch.
- Required authority:
  - tenant owner/admin for tenant scope
  - global operator/admin for global scope
- Response:
  - kill switch resource and audit evidence reference

### `GET /v1/live-validations/kill-switches`

- Purpose: inspect effective tenant and global kill-switch state.

### `POST /v1/live-validations/{validationId}/reconciliations/{ambiguousCommitId}/resolve`

- Purpose: resolve operator-action-needed ambiguous commit state.
- Required authority:
  - tenant owner/admin or explicit reconciliation permission
- Request body:
  - `resolution`: `confirmed_committed`, `confirmed_not_committed`, `compensated`,
    `accepted_manual_state`, or `unsupported_unresolved`
  - `reason`
  - optional evidence references
- Response:
  - reconciliation resolution resource
  - updated ledger entry

### `POST /v1/live-validations/{validationId}/compare`

- Purpose: generate or refresh original-versus-live validation comparison.
- Response:
  - comparison resource including matched outcomes, observed differences, unsupported
    replay, denials, ambiguous commits, and required operator action.

## Event Surfaces

Schema-backed events should be added when implementation exposes event payloads:

- `live_validation.started`
- `live_validation.blocked`
- `live_validation.awaiting_approval`
- `live_validation.aborted`
- `live_validation.completed`
- `live_validation.operator_action_needed`
- `live_validation.kill_switch_changed`
- `live_validation.reconciliation_resolved`
- `live_validation.comparison_completed`
- `live_validation.side_effect_recorded`

Every event must include tenant, validation id, candidate id where available, actor where
available, reason code, timestamp, and evidence references sufficient for audit.

## SDK And Web Contract

The TypeScript SDK must expose typed resources and methods for:

- live validation attempts,
- live validation start/abort,
- support matrix inspection,
- side-effect ledger inspection,
- kill-switch inspection and mutation,
- reconciliation resolution,
- live validation comparison,
- stable denial/error payloads.

The web operator shell must expose enough UI to:

- select explicit live-validation scope,
- see permission/quota/kill-switch/support/approval readiness,
- approve or link approvals according to safety class,
- abort a running attempt,
- inspect ledger and comparison outcomes,
- resolve reconciliation only when authorized,
- distinguish non-live replay from live validation.

## Compatibility Rules

- Existing `POST /v1/evaluation/replay-candidates/{candidateId}/attempts` keeps
  `non_live` as the default.
- Existing `mode: "live_validation"` attempts may be redirected to the new explicit live
  validation route or remain blocked until the new executor is mounted, but must not run
  side effects without Roadmap 40 gates.
- Existing replay candidate, attempt, comparison, and fixture resources remain backward
  compatible unless schema changes are versioned and fixture payloads are updated.

## Final Implemented Names

- Direct start/list route: `POST|GET /v1/live-validations`
- Candidate-scoped start route: `POST /v1/evaluation/replay-candidates/{candidateId}/live-validations`
- Detail/abort: `GET /v1/live-validations/{validationId}`,
  `POST /v1/live-validations/{validationId}/abort`
- Matrix: `GET /v1/live-validations/support-matrix`
- Ledger: `GET /v1/live-validations/{validationId}/ledger`
- Reconciliation: `POST /v1/live-validations/{validationId}/reconciliations/{ambiguousCommitId}/resolve`
- Retention: `GET /v1/live-validations/{validationId}/retention`
- Comparison: `POST /v1/live-validations/{validationId}/compare`
- Kill switches: `GET|POST /v1/live-validations/kill-switches`
- SDK methods: `startLiveValidation`, `listLiveValidations`, `getLiveValidation`,
  `abortLiveValidation`, `listLiveValidationSupportMatrix`,
  `listLiveValidationLedger`, `resolveLiveValidationReconciliation`,
  `getLiveValidationRetention`, `createLiveValidationComparison`,
  `listLiveValidationKillSwitches`, and `updateLiveValidationKillSwitch`.
