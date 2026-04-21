# Evaluation And Replay Harness

Status: proposed

Authority: This document is the authoritative upstream spec for replay, comparison, and regression support needed before knowledge-plane work becomes the main differentiator.

Primary source documents:
- `docs/product/personal-agent-non-knowledge-roadmap-split.md`
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

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/018-evaluation-and-replay-harness.md 完成 phase 33 的工作`
