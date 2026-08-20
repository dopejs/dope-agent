# Routine Builder

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 66, the product
routine builder slice for proactive personal-agent workflows.

Primary source documents:
- `docs/specs/010-scheduled-tasks-and-wakeups.md`
- `docs/specs/013-delivery-and-notifications.md`
- `docs/specs/016-tasks-and-reminders.md`
- `docs/specs/018-evaluation-and-replay-harness.md`

## Background

Schedules, workflows, reminders, approvals, and delivery exist as platform primitives.
Users still need a product surface to create explainable recurring routines without raw
API composition or memory-driven planning.

## Goal

Let users create, inspect, pause, resume, and repair explicit routines composed from
triggers, workflow steps, approval expectations, and delivery outcomes.

## Fixed Decisions

- Routine definitions are explicit configuration.
- Natural-language suggestions may prefill fields only if the final routine is structured
  and user-approved.
- Routines execute through existing schedule, workflow, policy, and delivery planes.
- This roadmap does not add autonomous planning or memory.

## Dependencies On Completed Phases

- Roadmap 25: Scheduled Tasks And Wakeups
- Roadmap 28: Delivery And Notifications
- Roadmap 31: Tasks And Reminders
- Roadmap 33: Evaluation And Replay Harness

## In Scope

- routine resource and version history
- trigger selection
- workflow template or step selection
- approval expectation preview
- delivery target and summary preference selection
- pause, resume, cancel, and repair actions
- web shell builder and SDK methods

## Out Of Scope

- memory-generated routines
- self-modifying routine optimization
- marketplace routine packs
- webhook triggers, owned by Roadmap 67

## Operator Or User Problems To Solve

- Users need to create proactive behavior without understanding low-level daemon APIs.
- Operators need to see why a routine ran and what it was allowed to do.

## User Stories

- As a user, I can create a daily routine that runs a workflow and delivers a summary.
- As a user, I can pause or edit a routine safely.
- As an operator, I can inspect routine execution evidence.

## Functional Requirements

- The system MUST persist routine definitions and versions.
- Routines MUST compile to existing schedule and workflow targets.
- Routine edits MUST not rewrite prior execution evidence.
- Approval, delivery, and quota expectations MUST be previewed before activation.
- Routine execution MUST be replay/evaluation eligible.

## Compatibility And Operational Notes

Routine builder should reduce product complexity while preserving underlying runtime
truth. It must not introduce hidden background execution.

## Verification Expectations

- API/store tests for routine lifecycle and versioning.
- Compilation tests proving routine definitions create valid schedule/workflow targets.
- Web tests for builder, pause/resume, and execution inspection.
- Manual `KURA_ENV=test` routine walkthrough.

## Definition Of Done

- Users can create useful proactive routines from product UI while engineers can still
  inspect ordinary schedule/workflow/delivery truth.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/051-routine-builder.md 完成 phase 66 的工作`
