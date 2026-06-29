# Calendar Attendee And RSVP Workflows

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 61, the calendar
attendee, invitation, and RSVP workflow slice.

Primary source documents:
- `docs/specs/014-calendar-integration.md`
- `docs/specs/045-real-calendar-provider-closure.md`
- `docs/specs/025-live-validation-and-side-effect-replay.md`

## Background

Phase 29 deliberately excluded attendee invitation, RSVP, and external notification
semantics. A public personal agent needs truthful meeting coordination, but only after a
real provider closure proves account and side-effect safety.

## Goal

Support attendee-bearing calendar writes and RSVP state with explicit provider side-effect
truth, approval, diagnostics, and delivery linkage.

## Fixed Decisions

- Attendee and RSVP behavior is side-effecting and must be explicit.
- Provider notification semantics must be represented truthfully, not hidden.
- Ambiguous downstream commits require reconciliation evidence.
- This roadmap does not add memory-driven meeting summarization.

## Dependencies On Completed Phases

- Roadmap 60: Real Calendar Provider Closure
- Roadmap 40: Live Validation And Side-Effect Replay

## In Scope

- attendee request model
- invitation send/update/cancel semantics
- RSVP inspection and response where provider supports it
- approval gates for externally visible attendee actions
- operation and artifact extensions
- diagnostics for attendee delivery and provider permission failures

## Out Of Scope

- recurring-event series management
- all-day event mutation
- travel planning
- memory-driven participant context

## Operator Or User Problems To Solve

- Users need the agent to schedule meetings with people without silently dropping attendee
  side effects.
- Operators need to inspect whether invitations were sent, blocked, or ambiguous.

## User Stories

- As a user, I can create a meeting with attendees and see invitation status.
- As a user, I can update attendees with explicit notification behavior.
- As an operator, I can reconcile ambiguous invitation commits.

## Functional Requirements

- The system MUST distinguish event-field mutation from attendee notification side effects.
- Attendee actions MUST record operation class, requested notification behavior, provider
  result, and ambiguity status.
- RSVP state MUST be inspectable where provider evidence exists.
- Approval and live-validation gates MUST apply to externally visible attendee actions.

## Compatibility And Operational Notes

Existing non-attendee calendar writes remain compatible. Unsupported provider RSVP
features must return explicit unsupported diagnostics.

## Verification Expectations

- Fake provider tests for attendee create/update/cancel and RSVP inspection.
- Live validation tests for invitation side effects and ambiguous commits.
- API/schema/SDK/web tests for attendee operation resources.
- Real-account smoke when safe attendee test accounts are available.

## Definition Of Done

- Meeting attendee and RSVP behavior is truthful, auditable, and safe enough for hosted
  public use.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/046-calendar-attendee-and-rsvp-workflows.md 完成 phase 61 的工作`
