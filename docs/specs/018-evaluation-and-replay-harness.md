# Evaluation And Replay Harness

Status: implemented

Authority: This document is the authoritative upstream spec for replay, comparison, and regression support needed before knowledge-plane work becomes the main differentiator.

Primary source documents:
- `docs/product/personal-agent-non-knowledge-roadmap-split.md` (removed 2026-08, in git history)
- `docs/harness/harness-architecture.md`
- `docs/product/openclaw-architecture-gaps.md`

## Background

The more ambient the agent becomes, the less acceptable it is to change behavior without replay and comparison support. Evaluation and replay are the reliability bridge between the execution plane and later adaptive knowledge systems.

## Goal

Add replay and comparison primitives that let operators re-run or audit representative work across schedules, workflows, integrations, and computer-use paths.

## Fixed Decisions

- replay is a control-plane and runtime-evidence feature, not a memory feature
- the first slice should focus on deterministic replay and comparison where possible
- evaluation should consume explicit run, workflow, policy, artifact, and delivery truth

## In Scope

- replayable run or workflow envelopes
- before/after comparison support
- regression fixtures for schedules, integrations, and computer use
- operator-visible replay outcomes

## Out Of Scope

- automatic self-improvement loops
- model-training infrastructure
- autonomous metric optimization

## User Stories

- As an operator, I can replay representative personal-agent work after changing policies or integrations.
- As an operator, I can compare results before and after a capability change.
- As an engineer, I can add regression fixtures for real-world personal-agent flows.

## Functional Requirements

- the system MUST support replaying representative run or workflow envelopes with explicit provenance
- comparison output MUST distinguish runtime drift, policy drift, integration drift, and delivery drift where observable
- replay and comparison artifacts MUST remain operator-visible

## Verification Expectations

- replay and comparison regressions in automated test paths
- one manual before/after verification flow against a real or local-fixture scenario

## Definition Of Done

- the daemon has a credible replay and evaluation substrate for non-knowledge-plane personal-agent work

## Implementation Notes

Roadmap 33 is implemented by the feature plan at
`specs/018-evaluation-replay-harness/plan.md`.

The delivered slice includes:

- daemon-owned evaluation records for replay candidates, replay attempts, comparison
  results, drift findings, and regression fixtures
- SQLite persistence and restart-safe restoration through normal daemon state
- schema-backed `/v1/evaluation/*` API routes and additive `evaluation.*` events
- default evidence-backed `non_live` replay behavior with explicit safety, approval,
  side-effect, evidence, and environment scope
- completed `non_live` replay attempts are linked to an `evaluation.replay` runtime run
  and completed replay workflow envelope so replay outcomes are inspectable through the
  existing runtime/workflow truth model
- explicit curated-work candidate registration through the evaluation API
- candidate registration rejects missing source provenance, while fixture candidates
  remain repo-managed
- plane-level comparison summaries for runtime, policy, integration, delivery, and
  evidence differences
- repo-managed schedule, integration, and computer-use fixtures
- TypeScript SDK and web operator-shell Evaluation Replay support

Still out of scope:

- broad automatic replay eligibility for every completed run
- in-product fixture editing
- live validation without explicit operator scope
- live validation execution before a replay executor and approval flow are implemented
- knowledge-plane self-improvement, autonomous optimization, or model training

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/018-evaluation-and-replay-harness.md 完成 phase 33 的工作`
