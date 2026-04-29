# Contract: Side-Effect Ledger

## Goal

Record truthful, durable, tenant-scoped evidence for every live validation side-effect
decision so operators can prove what was attempted, skipped, completed, failed, aborted,
denied, or left for reconciliation.

## Ledger Outcomes

| Outcome | Meaning | Automatic Retry Allowed |
|---------|---------|-------------------------|
| `attempted` | The system durably recorded intent before or atomically with the side-effect attempt. | Depends on matrix retry policy. |
| `skipped` | The step was not attempted because it was excluded, unsupported, or not in scope. | No. |
| `completed` | The side effect completed or was proven committed. | No further retry. |
| `failed` | The side effect failed before commit or the matrix proves no commit occurred. | Only if matrix retry policy allows. |
| `aborted` | Pending or future work was stopped by operator abort or kill switch. | No unless explicitly restarted by a new validation. |
| `denied` | Permission, quota, approval, support, or kill-switch gate denied before attempt. | No until the denial condition changes. |
| `operator_action_needed` | Commit status is ambiguous or manual reconciliation is required. | No automatic retry. |

## Durability Rules

- A ledger entry for `attempted` must be durable before or atomically with external
  mutation attempts where feasible.
- Ledger entries link to validation attempt, replay source, tool class, safety class,
  approval evidence, tenant, actor, timestamp, correlation/idempotency evidence, and
  downstream references where available.
- Daemon restart after submit must reload ledger state before deciding whether retry is
  allowed.
- Ledger records remain inspectable after restart and after live validation is disabled.

## Correlation And Idempotency

- External side-effect attempts must include a stable correlation or idempotency key when
  the downstream system supports it.
- The operation key must derive from tenant, validation id, source action, tool class, and
  downstream idempotency support.
- If downstream idempotency cannot be proven, non-idempotent mutation rows must use
  `no_retry` after submit-unknown and move to `operator_action_needed`.

## Abort And Kill Switch Rules

- Operator abort and kill switch activation abort pending and future side effects.
- Already-submitted side effects cannot be hidden by an abort. They must resolve to
  `completed`, `failed`, or `operator_action_needed`.
- Comparisons must distinguish unattempted, skipped, completed, failed, aborted, denied,
  and reconciliation-needed work.

## Ambiguous Commit Rules

Ambiguous commit is required when any of these occur after submit or when submit status
cannot be proven:

- timeout after submit,
- connection loss,
- unknown provider response,
- daemon restart after submit,
- duplicate retry with conflicting evidence,
- downstream response that cannot prove commit or non-commit.

Ambiguous commit behavior:

- set ledger outcome to `operator_action_needed`,
- stop automatic retry,
- expose reconciliation guidance,
- require tenant owner/admin or explicit reconciliation permission to resolve.

## Reconciliation Resolution

Resolution values:

- `confirmed_committed`
- `confirmed_not_committed`
- `compensated`
- `accepted_manual_state`
- `unsupported_unresolved`

Resolution records must include resolver, authority, reason, timestamp, evidence refs,
and affected ledger entry. Resolution does not delete or rewrite original ledger facts.

## Retention

Live-validation attempts, side-effect ledger entries, reconciliation decisions, and
comparison evidence are retained indefinitely by default. A later explicit operator
retention policy may change retention, but active operator-action-needed states cannot be
deleted before resolution.

## Contract Tests

Contract tests must prove:

- all ledger outcomes serialize through API and event schemas,
- attempted ledger records are present before fake backend mutation,
- timeout-after-submit creates operator-action-needed and stops automatic retry,
- restart-after-submit preserves ambiguous commit state,
- duplicate retry cannot duplicate non-idempotent side effects,
- abort and kill switch distinguish pending/future work from already-submitted work,
- unauthorized users cannot resolve reconciliation,
- retention defaults to indefinite.

## Implemented Event And API Names

- Ledger list: `GET /v1/live-validations/{validationId}/ledger`
- Reconciliation resolve: `POST /v1/live-validations/{validationId}/reconciliations/{ambiguousCommitId}/resolve`
- Retention inspect: `GET /v1/live-validations/{validationId}/retention`
- Comparison create: `POST /v1/live-validations/{validationId}/compare`
- Side-effect event: `live_validation.side_effect_recorded`
- Operator-action event: `live_validation.operator_action_needed`
- Reconciliation event: `live_validation.reconciliation_resolved`
- Completion event: `live_validation.completed`
- Comparison event: `live_validation.comparison_completed`

Fake-backend proving tests live under `daemon/internal/integrations`,
`calendar`, `mail`, `delivery`, `connectors`, and `reminders` with
`live_validation_fake_test.go` file names.
