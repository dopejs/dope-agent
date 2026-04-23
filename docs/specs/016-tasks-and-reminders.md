# Tasks And Reminders

Status: proposed

Authority: This document is the authoritative upstream spec for reminders, personal tasks, and lightweight follow-up tracking built on the trigger and integration planes.

Primary source documents:
- `docs/specs/010-scheduled-tasks-and-wakeups.md`
- `docs/specs/012-personal-integrations-platform.md`
- `docs/specs/013-delivery-and-notifications.md`

## Background

Tasks and reminders are the clearest expression of “ambient assistant” behavior. They sit between raw schedules and richer external domains by combining wakeups, acknowledgement, recurrence, and user delivery.

## Goal

Add a first-class reminders and lightweight task-follow-up domain that can schedule, resurface, acknowledge, and complete personal work.

## Fixed Decisions

- reminders are not just raw cron entries
- acknowledgement, snooze, completion, and missed reminders must be explicit states
- reminders may launch workflows or only deliver notifications depending on configuration
- reminder notifications should reuse roadmap 28 delivery targets, preferences, and
  digest behavior instead of introducing a second reminder-only delivery model
- reminder or follow-up flows that reference calendar execution should reuse the concrete
  calendar-domain contract from `specs/014-calendar-integration/contracts/calendar-domain-surfaces.md`
  instead of redefining calendar linkage or delivery truth

## In Scope

- reminder resources
- one-shot and recurring reminders
- snooze, complete, dismiss, and reschedule flows
- linkage to background workflows when configured

## Out Of Scope

- full project-management suite
- team assignment workflows
- memory-based habit coaching

## User Stories

- As a user, I can create a reminder that wakes the daemon later and notifies me.
- As a user, I can snooze or complete a reminder and have that truth persisted.
- As an operator, I can inspect missed, pending, completed, and cancelled reminder states.

## Functional Requirements

- reminder resources MUST be distinct from low-level schedules
- reminders MUST support one-shot and recurring behavior with explicit state transitions
- reminder delivery and reminder-triggered workflow execution MUST remain distinguishable
- reminder history MUST survive restart and remain environment-scoped

## Verification Expectations

- targeted reminder lifecycle tests
- restart recovery coverage
- one manual recurring reminder verification path

## Definition Of Done

- the agent can manage personal reminders as a user-facing domain instead of raw scheduler entries

## Implementation Note

- phase 31 is now implemented in `specs/016-tasks-reminders/`
- the shipped reminder API uses scheduler-native trigger kinds: `once` for one-shot
  reminders and `cron` for recurring reminders
- reminder delivery, reminder lifecycle, and reminder-triggered workflow execution remain
  separate truths with additive linkage across the owning resources

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/016-tasks-and-reminders.md 完成 phase 31 的工作`
