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
- abort, retry, and kill-switch semantics
- tenant permission and quota gates
- comparison between original and live replay outcomes

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

## Compatibility And Operational Notes

- Non-live replay must remain the default and safe path.
- Live validation must never reuse stale approvals.
- Side-effect ledger entries must be durable before or atomically with external mutation
  attempts where feasible.

## Verification Expectations

- Permission and quota denial tests.
- Approval-required tests for side-effect replay.
- Ledger tests for completed, failed, skipped, denied, and aborted paths.
- Fake integration live-validation tests and opt-in real-account smoke notes.
- Contract tests for live validation and side-effect ledger resources.

## Definition Of Done

- Operators can run controlled live validation for supported work without bypassing tenant
  permissions, approvals, quota, audit, or abort controls.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/025-live-validation-and-side-effect-replay.md 完成 phase 40 的工作`
