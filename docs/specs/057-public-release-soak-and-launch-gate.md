# Public Release Soak And Launch Gate

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 72, the final
non-knowledge public release soak and launch gate before context, knowledge, and memory
work begins.

Primary source documents:
- `docs/specs/024-production-install-upgrade-backup-and-soak.md`
- `docs/specs/028-hosted-operational-profile-and-recovery.md`
- `docs/specs/055-operator-shell-productization.md`
- `docs/specs/056-support-diagnostics-and-evidence-bundle.md`
- `docs/runtime/release-readiness.md`

## Background

After non-knowledge parity specs land, DopeAgent needs one release gate proving the hosted
product can run with real channels, real providers, routines, diagnostics, quota, support
evidence, backup, restore, upgrade, and rollback. This is the entry gate for
context/knowledge/memory work.

## Goal

Run and codify the public beta launch gate for non-knowledge parity, including soak,
real-account smoke matrix, support evidence, rollback, and no-ship criteria.

## Fixed Decisions

- This roadmap is a release gate, not a new feature domain.
- Context, knowledge, and memory remain out of scope.
- Missing required evidence is a no-ship condition.
- Real-account smoke may be skipped only with structured accepted reasons.

## Dependencies On Completed Phases

- Roadmaps 44-70
- Roadmap 39: Production Install, Upgrade, Backup, And Soak
- Roadmap 43: Hosted Operational Profile And Recovery

## In Scope

- public beta release evidence index
- full-duration hosted daemon soak
- real channel smoke matrix
- real calendar and mail smoke matrix
- routine, webhook, delivery, approval, quota, live validation, evaluation, diagnostics,
  and support bundle workload
- backup, restore, upgrade, and rollback rehearsal
- launch readiness no-ship rules

## Out Of Scope

- context engineering
- memory or knowledge plane
- managed self-improvement
- payment-provider launch unless already completed elsewhere

## Operator Or User Problems To Solve

- Release owners need defensible evidence that the product can be offered publicly.
- Engineers need a clear entry condition for starting context/knowledge/memory work.

## User Stories

- As a release owner, I can review one evidence index and make a ship/no-ship decision.
- As an engineer, I can see which non-knowledge parity requirements passed or failed.
- As support, I can verify support-bundle generation during the soak workload.

## Functional Requirements

- The release gate MUST define required workloads, evidence artifacts, freshness,
  retention, redaction, and owner classification.
- The gate MUST include at least three real or skipped-with-reason channel entries.
- The gate MUST include real or skipped-with-reason calendar and mail provider entries.
- The gate MUST exercise activation, setup, channels, sessions, profile binding, routines,
  webhooks, quota denial, diagnostics, evaluation, live validation, support bundle,
  backup, restore, upgrade, and rollback.
- The gate MUST state that context/knowledge/memory may begin only after non-knowledge
  parity release evidence passes or residual exceptions are explicitly accepted.

## Compatibility And Operational Notes

This roadmap should reuse hosted profile and production soak tooling. It may add fixtures
or validators only where the public beta gate needs stronger evidence.

## Verification Expectations

- Validator tests for release evidence index completeness.
- Hosted soak on stable always-on host or VPS.
- Real-account smoke matrix or structured skips.
- Redaction and support bundle validation during soak.
- Final docs update marking non-knowledge parity complete or explicitly blocked.

## Definition Of Done

- DopeAgent has defensible hosted/public non-knowledge parity evidence.
- The team can start context/knowledge/memory design from a stable product and operations
  baseline rather than using memory to fill product gaps.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/057-public-release-soak-and-launch-gate.md 完成 phase 72 的工作`
