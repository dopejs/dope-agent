# Roadmap Authority And Release Truth Reconciliation

Status: implementation and local verification complete in `specs/029-roadmap-release-truth/`

Authority: This document is the authoritative upstream spec for Roadmap 44, the planning
and release-evidence reconciliation slice that must close before the remaining
non-knowledge parity program continues.

Primary source documents:
- `docs/runtime/daemon-roadmaps.md`
- `docs/harness/harness-architecture.md`
- `docs/runtime/release-truth-checklist.md`
- `docs/specs/028-hosted-operational-profile-and-recovery.md`
- `specs/028-hosted-operational-profile/`
- `specs/029-roadmap-release-truth/`

## Background

The repository has implementation evidence for late hosted-productization work, but some
roadmap summaries, spec statuses, and release-readiness notes no longer agree. This creates
planning risk: future work can be scheduled against stale "planned" or "pending" labels
instead of the actual implementation and evidence state.

## Goal

Make roadmap status, upstream specs, branch-local speckit artifacts, quickstarts, and
release evidence agree on what is complete, what is locally implemented, and what still
requires hosted or real-account release evidence.

## Fixed Decisions

- This roadmap changes planning truth and evidence indexes, not product behavior.
- Completion labels must distinguish implementation complete, local verification complete,
  stable-host dry-run complete, full hosted soak pending, and real-account smoke pending.
- No roadmap may claim public readiness without linked release evidence.
- Future spec sizing follows the planning boundary in Functional Requirements.

## Dependencies On Completed Phases

- Roadmap 39: Production Install, Upgrade, Backup, And Soak
- Roadmap 41: Evaluation Product Expansion
- Roadmap 42: Integration Health And Permission Diagnostics
- Roadmap 43: Hosted Operational Profile And Recovery

## In Scope

- reconcile `docs/runtime/daemon-roadmaps.md`
- reconcile `docs/harness/harness-architecture.md`
- reconcile `docs/specs/README.md`
- update status wording in upstream specs where implementation evidence exists
- add a release-truth checklist for future roadmap closure
- identify missing full-duration hosted soak or real-account smoke evidence as explicit
  residual work

## Out Of Scope

- new runtime APIs
- connector or provider implementation
- context engineering
- memory or knowledge-plane design

## Operator Or User Problems To Solve

- Release owners need one accurate source of truth before scheduling public-readiness work.
- Engineers need to know whether a roadmap is blocked by code, evidence, hosted soak, or
  real-account credentials.

## User Stories

- As a release owner, I can inspect the roadmap index and see the exact closure state for
  Roadmaps 42 and 43.
- As an engineer, I can start a future spec without reopening completed scope.

## Functional Requirements

- The system documentation MUST use a consistent status vocabulary.
- Every implemented roadmap after Roadmap 39 MUST link its quickstart or release evidence.
- Roadmap 42 and 43 status MUST reflect current implementation evidence and remaining
  hosted/full-duration validation gaps.
- The docs MUST state that future standard specs should remain under 50 tasks.
- The standalone release-truth checklist MUST be linked from roadmap and spec materials.

## Compatibility And Operational Notes

This roadmap is documentation-only. It must not rewrite historical evidence, only link and
classify it consistently.

## Verification Expectations

- `rg` checks for stale Roadmap 42/43 status contradictions.
- Manual review of links from roadmap summary to branch-local quickstart evidence.
- Apply `docs/runtime/release-truth-checklist.md` to Roadmaps 42 and 43.
- No code tests are required unless scripts or validators change.

## Definition Of Done

- Roadmap, harness, upstream spec, and quickstart status agree.
- Remaining release evidence gaps are explicit and not confused with implementation gaps.
- Future planning can proceed from Roadmap 45 without status archaeology.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/029-roadmap-authority-and-release-truth-reconciliation.md 完成 phase 44 的工作`
