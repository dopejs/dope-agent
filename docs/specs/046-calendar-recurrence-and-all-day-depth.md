# Calendar Recurrence And All-Day Depth

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 61, the calendar
recurrence and all-day event depth slice.

Primary source documents:
- `docs/specs/014-calendar-integration.md`
- `docs/specs/044-real-calendar-provider-closure.md`
- `docs/specs/045-calendar-attendee-and-rsvp-workflows.md`

## Background

Phase 29 supports timed single-event mutation only. Calendar parity requires all-day and
recurring events to be handled as first-class truths rather than explicit unsupported
cases.

## Goal

Add inspectable and mutable all-day and recurring-event support with clear provider
constraints, series identity, occurrence identity, and rollback safety.

## Fixed Decisions

- Recurrence behavior must distinguish series, occurrence, and exception mutations.
- All-day events must preserve timezone semantics.
- Unsupported provider recurrence operations must fail explicitly.
- This roadmap does not add memory-based scheduling preferences.

## Dependencies On Completed Phases

- Roadmap 59: Real Calendar Provider Closure
- Roadmap 60: Calendar Attendee And RSVP Workflows, if attendee recurrence interactions
  are in scope for the selected provider

## In Scope

- recurring event inspection
- single occurrence versus series update/cancel semantics
- all-day event create/update/cancel
- timezone and date boundary rules
- provider artifact evidence for recurrence rules
- diagnostics for unsupported provider recurrence behavior

## Out Of Scope

- AI-generated schedule optimization
- memory-based availability preferences
- complex multi-calendar coordination beyond selected provider capability

## Operator Or User Problems To Solve

- Users need recurring and all-day calendar requests to work or fail with precise reasons.
- Operators need to inspect which occurrence or series was changed.

## User Stories

- As a user, I can create an all-day event.
- As a user, I can update one occurrence or an entire recurring series where supported.
- As an operator, I can inspect recurrence rule and provider evidence.

## Functional Requirements

- The system MUST model event kind, recurrence identity, occurrence identity, and all-day
  date boundaries.
- Mutations MUST state whether they target one occurrence, future occurrences, or the full
  series.
- Provider limitations MUST map to explicit unsupported diagnostics.
- Operation history MUST preserve original and resulting provider identities.

## Compatibility And Operational Notes

Existing timed-event APIs remain compatible. Recurrence fields should be additive where
possible.

## Verification Expectations

- Fake provider tests for all-day create/update/cancel and recurrence mutations.
- Provider adapter tests for selected real provider recurrence payloads.
- API/schema/SDK/web tests for recurrence resources and diagnostics.

## Definition Of Done

- Calendar event depth no longer leaves common all-day and recurrence requests outside the
  public-product capability set.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/046-calendar-recurrence-and-all-day-depth.md 完成 phase 61 的工作`
