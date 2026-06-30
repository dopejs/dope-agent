# Tasks: Calendar Attendee And RSVP Workflows

**Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md) | **Roadmap**: 61

Stories: US1 create-with-attendees, US2 update-attendees, US3 RSVP + reconcile.

## Phase 1: Setup
- [X] T001 [Setup] Baseline green (calendar/provider/api suites); confirm attendee rejection points.

## Phase 2: Foundational (model)
- [X] T002 [Foundational] types.go: add AttendeeRole, RSVPStatus, InvitationStatus,
  NotificationBehavior, AttendeeRequest, Attendee, AttendeeOutcome; Event.AttendeeDetails;
  CreateEventInput/UpdateEventInput AttendeeRequests + NotifyAttendees; UpdateAttendeesInput;
  OperationClassUpdateAttendees; Operation.AttendeeOutcome. All additive.
- [X] T003 [Foundational] manager.go: validateMutationInput allows attendees (keeps recurring/
  all-day); build AttendeeOutcome from backend result; add UpdateAttendees method + gating class.
- [X] T004 [Foundational] backend.go: Backend gains UpdateAttendees(resource, account, input).

## Phase 3: US1 — create with attendees
- [X] T005 [US1] fake_backend: store attendees, simulate per-attendee invitation + RSVP from notify.
- [X] T006 [US1] feishulark: add attendees to create/update body; read attendees+response_status.
- [X] T007 [P] [US1] tests: create with attendees records event-field mutation + per-attendee
  invitation results + notification behavior; notify=false records no-send.

## Phase 4: US2 — update attendees
- [X] T008 [US2] fake_backend + feishulark: UpdateAttendees (add/remove with notification).
- [X] T009 [US2] manager UpdateAttendees records distinct operation (update_attendees), gated.
- [X] T010 [P] [US2] tests: add/remove attendee distinct operations; field+attendee facts distinct.

## Phase 5: US3 — RSVP + diagnostics + ambiguity
- [X] T011 [US3] RSVP projection on Event.AttendeeDetails (read path); explicit unsupported
  diagnostic when provider lacks RSVP/notification control (diagnostics classifier reason).
- [X] T012 [US3] ambiguous attendee invitation commit recorded ambiguous (reuse channel).
- [X] T013 [P] [US3] tests: RSVP projected; unsupported returns explicit diagnostic; ambiguous
  recorded ambiguous.

## Phase 6: API + gating + polish
- [X] T014 [API] api/calendar.go + types.go: accept attendee detail (role), expose AttendeeDetails
  + AttendeeOutcome on responses; stop rejecting attendees; keep recurrence/all-day rejection.
- [X] T015 [API] live_validation.go: add update_attendees gated row; confirm create/update gated.
- [X] T016 [Polish] schemas: additive attendee fields; make daemon-contract-test.
- [X] T017 [Polish] verify: go build/vet/test calendar, feishulark, api, livevalidation,
  integrations, contracts; existing non-attendee tests green. Docs note.

## Dependencies
T002 blocks all. T005/T006 before T007. T008/T009 before T010. T011 before T013.
</content>
