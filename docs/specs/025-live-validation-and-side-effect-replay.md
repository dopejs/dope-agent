# Live Validation And Side-Effect Replay

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 40, the live
validation executor and full side-effect replay safety boundary.

Primary source documents:
- `docs/product/hosted-productization-roadmap-split.md`
- `docs/specs/018-evaluation-and-replay-harness.md`
- `docs/specs/023-billing-quotas-and-usage-accounting.md`
- `docs/runtime/daemon-roadmaps.md`

## Background

Roadmap 33 intentionally shipped non-live replay by default. A production product still
needs live validation for selected flows, including controlled re-execution of tool calls
and side-effecting integration operations. This is risky and must be permission-gated,
audited, quota-aware, and abortable.

## Goal

Add a tenant-permission-gated live validation executor that can safely re-run selected
side-effecting work with fresh approval, side-effect ledgering, kill-switch behavior, and
clear abort or retry semantics.

## Fixed Decisions

- Live validation is never an implicit replay mode.
- `live_validation.execute` permission is required.
- Fresh operator approval is required before external side effects are performed.
- Side effects are recorded in a dedicated ledger linked to runtime, tool-call, integration,
  and evaluation evidence.
- A tenant or global kill switch can prevent new live validation from starting.
- Quota checks run before live validation starts.
- Every replayable tool class must declare whether it is read-only, idempotent mutation,
  non-idempotent mutation, or unsupported for live replay.
- External side-effect attempts require stable correlation or idempotency keys where the
  downstream system can support them.
- Ambiguous external commits must stop automatic retry and move to an operator-visible
  reconciliation state.

## Dependencies On Completed Phases

- Roadmap 33: Evaluation And Replay Harness
- Roadmap 34: Tenant Identity And Access Foundation
- Roadmap 38: Billing, Quotas, And Usage Accounting
- Roadmap 39: Production Install, Upgrade, Backup, And Soak

## In Scope

- live validation execution mode
- fresh approval flow for side-effecting replay
- side-effect ledger records
- full tool-call-level replay for supported tool-call classes
- tool-class replay support matrix with safety class, approval requirement, idempotency
  expectation, retry policy, and unsupported-state behavior
- abort, retry, and kill-switch semantics
- ambiguous commit detection and reconciliation states
- compensation or manual-confirmation guidance for non-idempotent side effects
- tenant permission and quota gates
- comparison between original and live replay outcomes

## Required Replay Support Matrix

Implementation planning MUST create a replay support matrix before executor work starts.
The matrix MUST include every tool-call class reachable from replay candidates and these
columns:

- `toolClass`: stable class name or resource kind
- `safetyClass`: `read_only`, `idempotent_mutation`, `non_idempotent_mutation`, or
  `unsupported`
- `permission`: required tenant permission
- `approval`: whether fresh approval is required and what approval action is recorded
- `idempotency`: correlation key or downstream idempotency support
- `retryPolicy`: automatic retry, manual retry, or no retry
- `ambiguousCommitBehavior`: state used when submit status is unknown
- `compensation`: automatic compensation, manual confirmation, or unsupported
- `ledgerEvents`: attempted, skipped, completed, failed, aborted, denied, or
  operator-action-needed entries required
- `testCase`: fake-backend test proving the declared behavior

The initial matrix MUST classify at least:

- read-only daemon inspection calls
- runtime local tool calls
- MCP tool calls
- integration probe read operations
- integration mutation probes
- calendar event create/update/cancel
- mail draft create/update
- mail send/reply/forward
- reminder lifecycle mutations
- delivery dispatch attempts
- connector message sends
- unsupported provider or sandbox operations that cannot be safely replayed

No tool class may default to live replay support. Missing matrix rows are treated as
`unsupported`.

## Out Of Scope

- autonomous optimization based on replay results
- broad memory or self-improvement loops
- replay for unsupported tool classes without explicit unsupported-state reporting
- silent live replay from background schedules

## Operator Or User Problems To Solve

- Operators need to validate changes against real systems without uncontrolled side effects.
- Engineers need evidence that tool-call replay preserves approval, audit, and failure
  semantics.
- Tenant owners need a hard way to stop live validation during incidents.

## User Stories

- As an operator, I can select a replay candidate and request live validation with explicit
  side-effect scope.
- As a tenant owner, I can disable live validation for my tenant.
- As an engineer, I can inspect the side-effect ledger and compare original versus replayed
  tool-call outcomes.

## Functional Requirements

- The system MUST require `live_validation.execute` permission for live validation.
- The system MUST require fresh approval for side-effecting replay.
- The system MUST record attempted, skipped, completed, failed, aborted, and denied side
  effects in a ledger.
- The system MUST support abort and bounded retry semantics.
- The system MUST respect tenant and global live-validation kill switches.
- The system MUST classify unsupported tool-call replay instead of silently ignoring it.
- The system MUST maintain a replay support matrix for every tool-call class reachable from
  replay candidates.
- The system MUST attach an idempotency key or correlation ID to side-effect attempts when
  the downstream tool or integration supports it.
- The system MUST not automatically retry non-idempotent side effects after timeout,
  connection loss, unknown provider response, or daemon restart unless the side-effect
  ledger proves the prior attempt did not commit.
- The system MUST expose an `ambiguous_commit` or equivalent operator-action-needed state
  when commit status cannot be proven.
- The system MUST record compensation availability, manual confirmation requirement, or
  unsupported compensation for each side-effecting replay class.

## Compatibility And Operational Notes

- Non-live replay must remain the default and safe path.
- Live validation must never reuse stale approvals.
- Side-effect ledger entries must be durable before or atomically with external mutation
  attempts where feasible.
- For integrations that cannot provide idempotency or reconciliation evidence, live replay
  must either be disabled or require explicit manual confirmation before any retry.

## Verification Expectations

- Permission and quota denial tests.
- Approval-required tests for side-effect replay.
- Ledger tests for completed, failed, skipped, denied, and aborted paths.
- Ledger tests for timeout-after-submit, daemon restart after submit, duplicate retry,
  ambiguous commit, and manual reconciliation paths.
- Replay support matrix tests proving unsupported and non-idempotent classes cannot slip
  into automatic retry.
- Matrix completeness test proving every tool-call class reachable from replay candidates
  has an explicit safety classification.
- Fake integration live-validation tests and opt-in real-account smoke notes.
- Contract tests for live validation and side-effect ledger resources.

## Definition Of Done

- Operators can run controlled live validation for supported work without bypassing tenant
  permissions, approvals, quota, audit, or abort controls.
- Live replay cannot duplicate side effects silently when downstream commit status is
  unknown.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/025-live-validation-and-side-effect-replay.md 完成 phase 40 的工作`
