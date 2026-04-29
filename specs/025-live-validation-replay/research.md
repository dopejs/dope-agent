# Research: Live Validation And Side-Effect Replay

## Decision: Centralize Live Validation In `daemon/internal/livevalidation`

**Decision**: Add a focused `daemon/internal/livevalidation` package for executor
coordination, replay support classification, kill switches, side-effect ledger lifecycle,
abort/retry decisions, ambiguous-commit handling, reconciliation, and retention policy.

**Rationale**: Roadmap 40 spans evaluation, billing, identity, approvals, runtime,
integrations, delivery, connectors, MCP, events, and API. Centralizing the safety policy
prevents each domain from inventing its own live-replay behavior while allowing domain
packages to provide explicit adapters and fake-backend proof cases.

**Alternatives considered**:

- Extend `daemon/internal/evaluation` directly. Rejected because evaluation already owns
  replay candidates, non-live attempts, and comparison; adding side-effect execution
  policy there would blur the boundary between evidence replay and live mutation.
- Implement per-domain live replay handlers. Rejected because unsupported defaults,
  approval granularity, ambiguous commits, and retry safety must be uniform.

## Decision: Keep Non-Live Replay As The Default Evaluation Mode

**Decision**: `non_live` remains the default replay mode. Live validation requires
explicit mode selection, `live_validation.execute`, quota preflight, kill-switch checks,
readiness support, and fresh approvals before side effects.

**Rationale**: Roadmap 33 intentionally made replay evidence-preserving by default.
Changing defaults would create hidden external mutation risk and violate compatibility.

**Alternatives considered**:

- Auto-upgrade replay to live validation for supported candidates. Rejected because live
  side effects must never be implicit.
- Add a separate product unrelated to evaluation replay. Rejected because Roadmap 40 is
  specifically the live executor and safety boundary for selected replay candidates.

## Decision: Reuse Roadmap 38 Live-Validation Quota Gate

**Decision**: Concrete live-validation start paths use the existing
`live_validation_attempts` quota category and preflight semantics from Roadmap 38, then
reserve integration operation quota when side-effecting integration work is in scope.

**Rationale**: Billing already defines hosted fail-closed behavior, operation identity,
retry idempotency, and denial shape. Reusing it prevents a second quota system and keeps
hosted side effects denied before external work begins.

**Alternatives considered**:

- Add a separate live-validation quota store. Rejected as duplicate accounting.
- Charge quota after validation completes. Rejected because quotas must gate expensive or
  side-effecting work before it starts.

## Decision: Version And Test A Replay Support Matrix Before Executor Work

**Decision**: Treat the replay support matrix as a planning and runtime contract. Every
reachable tool-call class must declare safety class, permission, approval, idempotency,
retry policy, ambiguous-commit behavior, compensation, ledger events, and a fake-backend
test case. Missing rows are unsupported.

**Rationale**: The biggest Roadmap 40 failure mode is implicit support for unsafe work.
A matrix with completeness tests turns support into a deliberate declaration.

**Alternatives considered**:

- Infer safety from domain names or method names. Rejected because naming cannot prove
  side-effect semantics.
- Allow an unknown class to run read-only by default. Rejected because missing evidence
  must fail closed for live replay.

## Decision: Approval Granularity Follows Safety Class

**Decision**: Scope-level approval can cover read-only and idempotent classes when the
approved scope is explicit. Non-idempotent mutation replay requires per-action approval.

**Rationale**: This keeps low-risk validation usable while forcing the strongest human
control at the duplication-prone boundary.

**Alternatives considered**:

- Per-action approval for everything. Rejected because it would make read-only and
  idempotent validation unnecessarily hard to operate.
- One approval for the full scope. Rejected because non-idempotent external mutations
  need narrower consent and evidence.

## Decision: Kill Switches Affect Running Attempts Conservatively

**Decision**: Tenant and global kill switches block new starts and abort pending or
future side effects in running attempts. Already-submitted side effects resolve to
completed, failed, or operator-action-needed ledger evidence.

**Rationale**: Operators need immediate containment without pretending submitted
external mutations can always be cancelled. The ledger remains truthful about what is
already in flight.

**Alternatives considered**:

- Only block new starts. Rejected because operators need containment during incidents.
- Mark the whole attempt aborted immediately. Rejected because it can hide submitted
  side effects that still need reconciliation.

## Decision: Ambiguous Commits Stop Automatic Retry

**Decision**: Timeout-after-submit, connection loss, unknown provider response, daemon
restart after submit, or conflicting reconciliation evidence moves the ledger entry to an
operator-action-needed ambiguous-commit state and stops automatic retry unless durable
evidence proves the prior attempt did not commit.

**Rationale**: Duplicate external side effects are the primary safety hazard. Automatic
retry is acceptable only when idempotency or durable evidence makes it safe.

**Alternatives considered**:

- Retry all failures through the existing retry system. Rejected because non-idempotent
  external mutations cannot be safely repeated after unknown submit status.
- Treat unknown responses as failed. Rejected because failure is not proof that the
  downstream system did not commit.

## Decision: Reconciliation Resolution Requires Elevated Authority

**Decision**: Resolving operator-action-needed reconciliation states requires tenant
owner/admin authority or an explicit reconciliation permission. Users with only
`live_validation.execute` may inspect permitted evidence but cannot close the state.

**Rationale**: Reconciliation can decide whether a real tenant-side external mutation
committed. That is broader than launching validation and needs stronger authorization.

**Alternatives considered**:

- Allow any live-validation executor to resolve. Rejected because it weakens separation
  between execution and audit correction.
- Restrict all reconciliation to global operators. Rejected because tenant owners/admins
  should be able to manage their own tenant-scoped external state.

## Decision: Retain Live-Validation Audit Evidence Indefinitely By Default

**Decision**: Live-validation attempts, side-effect ledger entries, reconciliation
decisions, and comparison evidence are retained indefinitely unless an explicit operator
retention policy is later applied.

**Rationale**: Live validation can mutate real systems. Operators need durable evidence
for rollback, support, and audit even if live validation is disabled later.

**Alternatives considered**:

- Fixed automatic expiry. Rejected because it could delete evidence needed for external
  reconciliation.
- Retain summaries only. Rejected because ambiguous commits require detailed evidence.

## Decision: Fake Backends Are Mandatory; Real-Account Smoke Is Opt-In

**Decision**: Automated acceptance uses fake backend live-validation tests for all
supported side-effect paths. Real-account smoke is optional, explicit, documented, and
records operator-selected scope without logging secrets.

**Rationale**: The feature must be testable without touching live connectors or
production accounts, while still giving operators a controlled path for confidence
checks.

**Alternatives considered**:

- Require real external accounts for acceptance. Rejected because it is unsafe and
  non-repeatable for default development.
- Only unit-test the ledger. Rejected because side-effect boundaries need fake backend
  behavior for timeout, restart, duplicate retry, and ambiguous commit cases.
