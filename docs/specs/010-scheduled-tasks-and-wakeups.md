# Scheduled Tasks And Wakeups

Status: proposed

Authority: This document is the authoritative upstream spec for the roadmap slice that turns Kura from a reactive daemon into a triggerable personal agent.

Primary source documents:
- `docs/product/personal-agent-non-knowledge-roadmap-split.md` (removed 2026-08, in git history)
- `docs/product/feature-phasing.md`
- `docs/harness/harness-architecture.md`

## Background

Without durable wakeups, a personal agent is still a chat system that happens to have tools. The first product leap after orchestration is the ability to create future work and wake the daemon back into action safely.

## Goal

Add a first-class trigger plane for one-shot, recurring, and background wakeup execution that launches normal runs or workflows through the existing runtime.

## Fixed Decisions

- schedules launch normal runs or workflows rather than a second execution engine
- trigger truth must be durable and environment-scoped
- retries, backoff, and operator-visible failure history are in scope
- schedule-level approval does not bypass normal step-level approval

## In Scope

- one-shot wakeups
- recurring schedules
- schedule state and history
- retry and backoff policy
- pause, resume, cancel, and inspect flows
- restart-safe schedule persistence

## Out Of Scope

- memory-driven schedule generation
- advanced natural-language planning of routines
- mobile push infrastructure

## Operator Problems To Solve

- create work that starts later without keeping a chat session open
- inspect what is scheduled next and what already fired
- distinguish execution failure from trigger failure

## User Stories

- As an operator, I can schedule a workflow for later and see that it has not executed yet.
- As an operator, I can create a recurring schedule and inspect its next run plus previous attempts.
- As an operator, I can tell whether a failure came from schedule dispatch, workflow start, or downstream execution.

## Functional Requirements

- the daemon MUST expose first-class schedule resources with trigger configuration, state, next-fire time, and execution history
- schedules MUST launch normal runs or workflows through the existing runtime and workflow plane
- daemon restart MUST preserve future schedules and durable trigger history
- trigger failure, launch failure, cancelled schedule, paused schedule, and successful dispatch MUST be distinguishable
- schedule routes, events, and persisted history MUST remain environment-scoped

## Compatibility And Operational Notes

- existing run and workflow routes remain authoritative for execution truth after dispatch
- schedule dispatch should be bounded and should not require connector presence
- a later webhook or external-trigger roadmap may reuse the same trigger resource model

## Verification Expectations

- targeted scheduler and API tests
- restart recovery coverage for persisted schedules
- one manual `DOPE_ENV=test` recurring schedule verification

## Definition Of Done

- a schedule can be created, inspected, paused, resumed, cancelled, and observed through terminal and API surfaces
- a fired schedule creates normal runtime truth instead of hidden background execution
- restart recovery preserves future wakeups and visible trigger history

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/010-scheduled-tasks-and-wakeups.md 完成 phase 25 的工作`
