# Data Model: Live Validation And Side-Effect Replay

## Replay Candidate

Evaluation-owned source that can be handed off to live validation.

Fields added or consumed by live validation:
- `candidateId`: source candidate identifier.
- `sourceRefs`: provenance for the original captured work.
- `toolClasses`: complete set of tool-call classes reachable from the candidate
  evidence. Live validation uses this as the support-matrix input; the operator's
  included scope is not treated as the complete reachable set.

Validation rules:
- A candidate with no resolvable `toolClasses` cannot start live validation unless
  the start request supplies `candidateToolClasses`.
- Unsupported reachable classes must be explicitly excluded before supported classes
  may run.

## Live Validation Attempt

Represents an explicit tenant-scoped live validation run for a replay candidate.

Fields:
- `validationId`: stable unique identifier.
- `tenantId`: tenant that owns the validation.
- `candidateId`: replay candidate being validated.
- `sourceAttemptId`: optional source or baseline attempt link.
- `requestedBy`: principal that requested validation.
- `environmentScope`: test, local, hosted, or other existing environment label.
- `requestedScope`: declared side-effect scope and any explicit exclusions.
- `status`: `queued`, `awaiting_approval`, `running`, `completed`, `blocked`,
  `aborted`, `failed`, or `operator_action_needed`.
- `permissionDecision`: result of `live_validation.execute` gate.
- `quotaDecision`: reservation or denial reference for `live_validation_attempts`.
- `killSwitchDecision`: tenant/global kill-switch state observed at start and during run.
- `approvalSummary`: scope-level and per-action approval references.
- `ledgerSummary`: counts by ledger outcome.
- `comparisonId`: latest original-versus-live comparison link.
- `createdAt`, `startedAt`, `completedAt`, `updatedAt`.

Relationships:
- Belongs to one tenant and one replay candidate.
- Has many side-effect ledger entries.
- May have many fresh approvals.
- May have one latest comparison and many historical comparison results.

Validation rules:
- `mode` is always live validation; non-live replay remains in evaluation attempts.
- Cannot enter `running` without permission, quota, kill-switch, support-matrix, and
  required approval gates.
- Hosted attempts fail closed when quota state is unavailable.
- New starts are blocked when tenant or global kill switch is active.

State transitions:
- `queued` -> `blocked` when permission, quota, support, or kill switch denies.
- `queued` -> `awaiting_approval` when approval is required before any side effect.
- `awaiting_approval` -> `running` when required approvals are granted.
- `awaiting_approval` -> `blocked` when approval is denied or expires.
- `running` -> `completed` when all in-scope steps reach terminal ledger states with no
  unresolved operator action.
- `running` -> `aborted` when operator abort or kill switch stops pending/future work.
- `running` -> `operator_action_needed` when any ambiguous commit or manual
  reconciliation state remains.
- Any non-terminal state -> `failed` only for internal validation failure where no more
  truthful outcome can be produced.

## Replay Support Matrix Row

Declares live replay support for a tool-call class or resource kind.

Fields:
- `toolClass`: stable class name or resource kind.
- `safetyClass`: `read_only`, `idempotent_mutation`, `non_idempotent_mutation`, or
  `unsupported`.
- `permission`: required tenant permission.
- `approval`: approval granularity and recorded approval action.
- `idempotency`: correlation key or downstream idempotency support.
- `retryPolicy`: `automatic_retry`, `manual_retry`, or `no_retry`.
- `ambiguousCommitBehavior`: terminal state used when submit status is unknown.
- `compensation`: automatic compensation, manual confirmation, or unsupported.
- `ledgerEvents`: allowed and required ledger event outcomes.
- `testCase`: fake-backend test proving the row.
- `version`: matrix version or declaration revision.

Relationships:
- Used by live validation readiness and execution.
- Referenced by ledger entries and comparison output.

Validation rules:
- Missing row means unsupported.
- Unsupported rows cannot run live side effects.
- Non-idempotent mutation rows cannot allow automatic retry after unknown submit status.
- Every row must have a proving fake-backend or completeness test.

## Side-Effect Scope

Operator-declared scope for a live validation attempt.

Fields:
- `scopeId`: stable identifier for the declared scope.
- `validationId`: owning validation.
- `includedToolClasses`: tool classes allowed to run.
- `excludedToolClasses`: unsupported or intentionally excluded classes.
- `includedActions`: optional per-action allowlist.
- `excludedActions`: optional per-action exclusions.
- `approvalMode`: `scope_level`, `per_action`, or `mixed`.
- `declaredBy`: principal that declared the scope.
- `declaredAt`.

Validation rules:
- Supported steps in mixed candidates may proceed only when unsupported work is
  explicitly excluded.
- Scope-level approval may cover read-only and idempotent classes only.
- Non-idempotent mutation actions require per-action approval.

## Fresh Approval

Current approval evidence for a live-validation scope or action.

Fields:
- `approvalId`: stable approval identifier.
- `validationId`: owning validation attempt.
- `tenantId`: tenant scope.
- `approvalTarget`: `scope` or `action`.
- `toolClass`: support matrix class.
- `safetyClass`: safety class at the time of approval.
- `actionRef`: required for per-action approvals.
- `approvedScope`: explicit scope description for scope-level approvals.
- `status`: `pending`, `approved`, `denied`, or `expired`.
- `requestedBy`, `resolvedBy`, `requestedAt`, `resolvedAt`.

Validation rules:
- Must not be reused from source work, prior replay, or prior live validation.
- Non-idempotent mutation replay cannot proceed from scope-level approval alone.

## Side-Effect Ledger Entry

Durable audit evidence for one live replay action.

Fields:
- `ledgerEntryId`: stable identifier.
- `validationId`: owning live validation attempt.
- `tenantId`: tenant scope.
- `candidateId`: replay candidate link.
- `sourceRef`: source replay evidence or original tool call.
- `toolClass`: matrix row used for the action.
- `safetyClass`: row safety class at execution time.
- `actionRef`: stable action identity.
- `approvalId`: approval evidence when required.
- `correlationKey`: idempotency or correlation identity where supported.
- `downstreamRef`: optional provider or fake-backend reference.
- `outcome`: `attempted`, `skipped`, `completed`, `failed`, `aborted`, `denied`, or
  `operator_action_needed`.
- `reasonCode`: stable outcome reason.
- `attemptedAt`, `completedAt`, `updatedAt`.
- `evidenceRefs`: related runtime, integration, tool-call, approval, or event evidence.
- `retryCount`: number of retry attempts.
- `ambiguousCommit`: boolean.
- `reconciliationId`: optional reconciliation resolution link.

Validation rules:
- Must be durable before or atomically with external mutation attempts where feasible.
- Unknown submit status produces `operator_action_needed`.
- Non-idempotent unknown submit status stops automatic retry.
- Kill switches and aborts distinguish unattempted, aborted, already-submitted, and
  reconciliation-needed work.

## Live Validation Kill Switch

Tenant or global control that prevents new live validation and contains running work.

Fields:
- `killSwitchId`: stable identifier.
- `scope`: `tenant` or `global`.
- `tenantId`: present for tenant scope.
- `enabled`: boolean.
- `reason`: operator-provided reason.
- `changedBy`, `changedAt`.
- `expiresAt`: optional future expiry.

Validation rules:
- Enabled switches block new starts.
- Enabled switches abort pending/future side effects in running attempts.
- Historical evidence and non-live replay inspection remain available.

## Ambiguous Commit

A side-effect state where commit status cannot be proven.

Fields:
- `ambiguousCommitId`: stable identifier.
- `ledgerEntryId`: affected side-effect ledger entry.
- `validationId`: owning validation.
- `tenantId`: tenant scope.
- `cause`: timeout, connection loss, unknown provider response, daemon restart,
  conflicting evidence, or other stable cause.
- `lastKnownRequestRef`: correlation or downstream request evidence.
- `automaticRetryStopped`: boolean.
- `createdAt`, `updatedAt`.

Validation rules:
- Always requires operator-action-needed evidence.
- Cannot be resolved by a user lacking tenant owner/admin authority or explicit
  reconciliation permission.

## Reconciliation Resolution

Authorized decision that closes an operator-action-needed state.

Fields:
- `reconciliationId`: stable identifier.
- `ambiguousCommitId`: ambiguous commit being resolved.
- `tenantId`: tenant scope.
- `resolvedBy`: tenant owner/admin or explicit reconciliation permission holder.
- `resolution`: `confirmed_committed`, `confirmed_not_committed`,
  `compensated`, `accepted_manual_state`, or `unsupported_unresolved`.
- `reason`: required operator explanation.
- `evidenceRefs`: supporting external or internal evidence.
- `resolvedAt`.

Validation rules:
- Requires tenant owner/admin authority or explicit reconciliation permission.
- Must be audit-visible.
- Must not erase original ledger evidence.

## Live Validation Comparison

Operator-visible comparison between original replay evidence and live validation results.

Fields:
- `comparisonId`: stable identifier.
- `validationId`: live validation attempt.
- `candidateId`: replay candidate.
- `baselineRef`: original replay/source evidence.
- `terminalStatus`: `matched`, `drifted`, `blocked`, `unsupported`, or
  `operator_action_needed`.
- `ledgerSummary`: side-effect outcome counts.
- `unsupportedClasses`: unsupported classes encountered or excluded.
- `denials`: permission, quota, kill-switch, approval, or support denials.
- `ambiguousCommits`: unresolved ambiguous commit references.
- `driftFindings`: runtime, policy, integration, delivery, evidence, or mixed findings.
- `generatedAt`.

Validation rules:
- Must not imply success for unsupported or excluded work.
- Must remain inspectable after restart.

## Live Validation Retention Policy

Retention rule for live-validation evidence.

Fields:
- `policyId`: stable identifier.
- `tenantId`: optional tenant scope.
- `appliesTo`: attempts, ledger entries, reconciliation decisions, comparisons, or all.
- `mode`: `indefinite` or explicit retention rule.
- `reason`: operator explanation for non-default policy.
- `changedBy`, `changedAt`.

Validation rules:
- Default is indefinite retention.
- Applying a retention policy must be operator-visible and must not delete evidence needed
  for active reconciliation.

## Final State Transition Notes

- Start gates are evaluated in order: tenant permission, Roadmap 38 quota preflight,
  kill switch, support matrix, then fresh approvals.
- Quota reservations are released when a later preflight gate blocks before live start,
  and committed only after a running attempt is durably recorded.
- Kill-switch activation aborts queued, awaiting-approval, and running attempts for the
  affected scope. Terminal ledger entries remain unchanged; non-terminal entries move
  to `aborted`.
- Ambiguous commits write `operator_action_needed` ledger evidence and stop automatic
  retry until an authorized reconciliation resolution is recorded.
- Default retention remains indefinite for attempts, ledger entries, reconciliation
  decisions, and comparisons.
