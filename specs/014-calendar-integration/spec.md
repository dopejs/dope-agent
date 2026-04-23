# Feature Specification: Calendar Integration

**Feature Branch**: `[014-calendar-integration]`  
**Created**: 2026-04-22  
**Status**: Draft  
**Input**: User description: "结合 docs/specs/014-calendar-integration.md 完成 phase 29 的工作"

## Clarifications

### Session 2026-04-22

- Q: Should availability lookup and event mutation be treated as one undifferentiated calendar action? → A: No. Availability lookup, event inspection, and event mutation remain distinct operation classes.
- Q: How should background calendar results reach the user? → A: Reuse the shared delivery targets, preferences, and outcome history established in phase 28.
- Q: When more than one calendar integration could represent the same account, how is the default connection chosen? → A: Reuse the shared integration binding and canonical-default behavior established in phase 27.
- Q: Should recurring events be included in phase 29 mutation scope? → A: No. Phase 29 supports mutation only for single events; recurring events remain inspectable but not mutable.
- Q: Should phase 29 event writes include attendee invitation and RSVP semantics? → A: No. Phase 29 supports only base event-field writes and excludes attendee invitation, RSVP, and external notification semantics.
- Q: Which calendar should phase 29 event writes target by default? → A: Phase 29 supports writes only to the bound account's primary calendar and does not support selecting among multiple writable calendars.
- Q: Should phase 29 support all-day event mutation? → A: No. Phase 29 supports mutation only for timed events; all-day events remain inspectable but not mutable.
- Q: Which timezone should phase 29 timed-event writes use by default? → A: Phase 29 interprets and returns timed-event values using the bound calendar account's primary timezone by default.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Inspect Availability And Calendar State (Priority: P1)

As a user or operator, I need to inspect which calendar account is active, what events
exist, and whether time is free or busy so I can trust the agent before asking it to
schedule or move anything.

**Why this priority**: Phase 29 only closes if the calendar domain can be inspected
truthfully. Without inspectable readiness, event state, and availability, later event
creation or scheduling actions would be unsafe and hard to trust.

**Independent Test**: Connect or inspect a representative calendar account, request
event list or detail information plus a busy/free lookup, and confirm the result shows
the selected account identity, the requested calendar state, and availability truth
without performing any event mutation.

**Acceptance Scenarios**:

1. **Given** a healthy calendar account is available, **When** the user asks what events
   are on the calendar or requests details for a specific event, **Then** the system
   returns truthful event information tied to the inspected account and event identity.
2. **Given** a healthy calendar account is available, **When** the user asks whether a
   time range is free or busy, **Then** the system returns availability truth as a
   lookup result rather than creating, moving, or canceling any event.
3. **Given** more than one calendar integration exists for the same account scope,
   **When** the user performs a calendar inspection without naming a specific
   integration, **Then** the system uses the canonical default account projection and
   makes that choice inspectable to the operator.
4. **Given** more than one calendar integration can satisfy the same request, **When**
   the user provides an explicit `integrationId` on a calendar inspection request,
   **Then** the system uses that integration for the request or returns a truthful
   selection failure instead of silently falling back to another account.

---

### User Story 2 - Create, Move, And Cancel Events Truthfully (Priority: P2)

As a user, I need the agent to create, update, or cancel calendar events and tell me the
truth about what happened so I can rely on it for real scheduling work.

**Why this priority**: Calendar value depends on trustworthy mutation. If the system
cannot preserve event identity and return truthful success or failure, it is not a
production-grade calendar domain.

**Independent Test**: Create a representative event, update its time or details, and
cancel it while confirming each step preserves account identity, event identity, and a
truthful final outcome without requiring attendee invitation or RSVP side effects.

**Acceptance Scenarios**:

1. **Given** a healthy calendar account and a valid event request, **When** the user
   asks the agent to create an event, **Then** the system creates the event and returns
   the resulting event identity, key details, the account used, and the fact that the
   event was written to the bound account's primary calendar.
2. **Given** an existing calendar event, **When** the user asks the agent to move or
   update that event, **Then** the system preserves the event identity and returns the
   changed event state or a truthful explanation of why the change could not be applied.
3. **Given** an existing calendar event, **When** the user asks the agent to cancel it,
   **Then** the system reports whether the event was successfully canceled, was already
   unavailable for cancellation, or could not be changed for a stated reason.
4. **Given** more than one calendar integration can satisfy the same mutation request,
   **When** the user provides an explicit `integrationId` on a create, update, or cancel
   request, **Then** the system executes against that integration or returns a truthful
   selection failure instead of silently rerouting the mutation.
5. **Given** a recurring calendar event, **When** the user asks the agent to create,
   update, or cancel it, **Then** the system reports that recurring-event mutation is
   not supported in phase 29 while leaving the event inspectable.
6. **Given** an event write request includes attendee invitation, RSVP, or external
   meeting-notification expectations, **When** the user asks the agent to apply that
   change, **Then** the system reports that those attendee semantics are out of scope
   for phase 29 rather than claiming they were executed.
7. **Given** the bound calendar account has more than one writable calendar, **When**
   the user asks the agent to create, update, or cancel an event, **Then** the system
   limits phase 29 mutation to the primary calendar and reports that alternate calendar
   selection is not supported in this phase.
8. **Given** an all-day event, **When** the user asks the agent to create, update, or
   cancel it, **Then** the system reports that all-day-event mutation is not supported
   in phase 29 while leaving the event inspectable.
9. **Given** a timed-event request omits an explicit timezone, **When** the user asks
   the agent to create or update that event, **Then** the system interprets and returns
   the event time using the bound calendar account's primary timezone.

---

### User Story 3 - Run Calendar Work Through Schedules And Shared Delivery (Priority: P3)

As a user, I need scheduled or workflow-driven calendar work to inspect upcoming events
or apply calendar changes and deliver a truthful result even when I am not in an active
chat session.

**Why this priority**: The upstream roadmap explicitly ties calendar behavior to the
shared trigger and delivery planes. The phase remains incomplete if calendar actions only
work in foreground conversations.

**Independent Test**: Run a scheduled or workflow-driven calendar task that inspects
upcoming events or applies a calendar change, then confirm it uses normal calendar
operations and routes its result through the shared delivery behavior.

**Acceptance Scenarios**:

1. **Given** a scheduled workflow that inspects upcoming events, **When** the workflow
   runs in the background, **Then** it can read calendar state and deliver a truthful
   result through the existing background-result delivery path.
2. **Given** a scheduled or workflow-driven calendar mutation is requested, **When** the
   change succeeds or fails, **Then** the delivered outcome preserves the calendar
   account and event identity needed to audit what happened.
3. **Given** background calendar work completes after the underlying event changed
   externally, **When** the result is finalized, **Then** the user and operator can see
   whether the final outcome reflects a successful mutation, a stale request, or an
   inspection-only result.

### Edge Cases

- If no healthy calendar integration is available for the requested account or
  environment, the system reports unavailable calendar readiness explicitly instead of
  silently pretending the calendar is empty.
- If multiple calendar integrations exist for the same account scope, the system uses an
  explicit `integrationId` choice when provided or the canonical default account
  projection otherwise and keeps that selection inspectable.
- If a request names an `integrationId` that is unavailable, non-calendar-capable, or
  outside the current environment, the system fails truthfully instead of silently
  falling back to a different integration.
- If the bound account exposes multiple calendars, phase 29 keeps read behavior
  inspectable but limits write behavior to the primary calendar instead of silently
  selecting another writable calendar.
- If the user requests a busy/free check, the system does not create, update, or cancel
  an event as a side effect of answering the lookup.
- If the user attempts to update or cancel an event that no longer exists or was changed
  externally, the system returns a truthful failure or stale-state result rather than
  claiming success.
- If the requested event time conflicts with existing calendar occupancy or policy, the
  system returns the conflict truthfully rather than silently overwriting the schedule.
- If a request targets an all-day event, the system keeps that event inspectable but
  does not claim a timed-event mutation that phase 29 does not support.
- If a timed-event request is made from an environment whose local timezone differs from
  the bound calendar account timezone, the system still uses the bound account's primary
  timezone by default and reports that timezone truthfully.
- If a request targets a recurring event or one occurrence within a series, the system
  keeps that event inspectable but does not claim a create, update, or cancel mutation
  that phase 29 cannot audit truthfully.
- If a requested event change depends on attendee invitation delivery, RSVP tracking, or
  other external participation semantics, the system does not claim those side effects
  were performed as part of phase 29 event mutation.
- If a background calendar workflow finishes with no relevant upcoming events, the system
  records and, when configured, delivers a truthful empty or no-change result rather than
  inventing a meeting summary.
- If calendar delivery fails after a successful read or mutation, operators can still
  inspect successful calendar execution separately from failed delivery truth.
- Phase 29 does not include CRM relationship modeling, generalized travel booking, or
  memory-driven meeting summarization.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST expose inspectable calendar account readiness by reusing
  the shared integration readiness, account-binding, and canonical-default contract from
  phase 27.
- **FR-002**: Users or operators MUST be able to inspect calendar event lists and event
  details through the selected calendar account without mutating calendar state.
- **FR-003**: The system MUST support busy/free lookup as a distinct calendar operation
  class that remains separate from event inspection and event mutation.
- **FR-004**: Users MUST be able to request calendar event creation and receive a
  truthful outcome that identifies the account used and the resulting event.
- **FR-005**: Users MUST be able to request calendar event updates, including moving or
  editing an existing event, and receive either the updated event state or a truthful
  explanation of why the change was not applied.
- **FR-006**: Users MUST be able to request calendar event cancellation and receive a
  truthful outcome that distinguishes successful cancellation from no-op or failed
  cancellation cases.
- **FR-006a**: Phase 29 event mutation scope is limited to single events. Recurring
  events and recurring-event occurrences MUST remain inspectable but MUST NOT be created,
  updated, or canceled as part of this phase.
- **FR-006b**: Phase 29 event writes are limited to base event fields. Attendee
  invitation, RSVP state management, and external meeting-notification semantics MUST
  remain out of scope for this phase.
- **FR-006c**: Phase 29 event writes MUST target only the bound account's primary
  calendar. Selecting among multiple writable calendars or moving events between
  calendars MUST remain out of scope for this phase.
- **FR-006d**: Phase 29 event mutation scope is limited to timed events. All-day events
  MUST remain inspectable but MUST NOT be created, updated, or canceled as part of this
  phase.
- **FR-006e**: Phase 29 timed-event writes MUST interpret and return event times using
  the bound calendar account's primary timezone by default unless a later phase adds
  explicit alternate-timezone behavior.
- **FR-007**: Calendar reads and writes MUST preserve calendar account identity, event
  identity, and audit truth across operator-visible history and downstream result
  delivery.
- **FR-008**: Scheduled workflows and other normal runtime workflows MUST be able to
  invoke calendar reads and writes without requiring a special calendar-only execution
  path.
- **FR-009**: Background calendar results MUST reuse the shared delivery targets,
  preferences, and outcome history from phase 28 instead of introducing a calendar-only
  notification plane.
- **FR-010**: Operator-visible history MUST distinguish calendar readiness truth,
  calendar execution truth, and delivery truth so connection problems, mutation failures,
  and delivery failures are not conflated.
- **FR-011**: When multiple calendar integrations can satisfy a request, calendar read
  and write routes MUST honor an explicit request-scoped `integrationId` when provided
  or use the canonical default account projection otherwise, and the chosen integration
  selection MUST remain inspectable in the resulting calendar operation.
- **FR-012**: Calendar behavior MUST remain environment-scoped so test and later live
  environments do not share implicit account bindings, event state, or delivery history.
- **FR-013**: The system MUST produce operator-visible artifacts for `list_events`,
  `get_event`, `create_event`, `update_event`, and `cancel_event` operations whenever
  backend event state is observed, and for `busy_free` operations as an availability
  summary artifact. Requests blocked before any backend state is observed do not require
  an artifact.
- **FR-014**: Phase 29 MUST stay scoped to the first calendar domain slice and MUST NOT
  require CRM modeling, generalized travel booking, or memory-driven meeting
  summarization to claim completion.

### Key Entities *(include if feature involves data)*

- **Calendar Account Projection**: The inspectable calendar-domain view of the active
  integration binding within one environment, including which external calendar account
  is selected, whether selection was explicit or canonical-default, and what primary
  calendar and timezone metadata phase 29 exposes.
- **Calendar Event**: A durable calendar item with identity, timing, key participation
  details, lifecycle state, and primary-calendar placement that can be inspected,
  created, updated, or canceled within the phase 29 scope.
- **Timed Event**: A calendar event whose start and end are represented as explicit time
  ranges interpreted in the bound calendar account's primary timezone by default and
  that defines the full phase 29 write scope.
- **Availability Window**: A requested time range whose meaning is limited to busy/free
  inspection and does not itself imply event creation or mutation.
- **Calendar Operation**: The operator-visible record of a calendar read or write,
  including the chosen account projection, the relevant event identity when one exists,
  and the truthful success, no-change, conflict, or failure outcome.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Additive operator-visible contract, schema, event, config,
  and storage surface changes are expected for calendar account projections, event
  inspection and mutation history, and calendar-linked delivery results. Existing
  non-calendar behavior remains backward compatible.
- **Migration / Rollback**: Additive calendar-domain resources and history are required.
  Rollback is a revert of calendar-specific routes, projections, and persistence while
  preserving already-recorded calendar and delivery history as read-only audit truth
  where needed.
- **Verification Strategy**: Required validation includes targeted domain coverage for
  calendar inspection, busy/free lookup, create/update/cancel behavior, preservation of
  event identity and account identity, workflow and schedule-driven calendar execution,
  separation of readiness, execution, and delivery truth, contract coverage for any new
  calendar projections, and at least one repo-owned local or fixture-based verification
  path in `DOPE_ENV=test`.
- **Observability Impact**: Operators must be able to inspect the selected calendar
  account projection, calendar readiness, operation class, affected event identity,
  truthful mutation or lookup outcome, and any downstream delivery outcome without
  reading raw connector logs.
- **Environment & Secrets**: Work defaults to `DOPE_ENV=test`. Live calendar connectors
  are optional for initial validation. Any credentials or tokens used for calendar access
  remain operator-owned, environment-scoped, and redacted from operator-visible history.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In manual validation, an operator can determine the active calendar
  account projection, current readiness, and whether a requested time range is free or
  busy in under 2 minutes using operator-visible surfaces only.
- **SC-002**: In automated and manual validation combined, a representative calendar
  event can be created, updated, and canceled with truthful results and preserved event
  identity in 100% of exercised test cases.
- **SC-003**: In automated verification, 100% of exercised calendar busy/free lookups
  remain distinguishable from event mutations and show no tested case of a lookup being
  recorded as a create, update, or cancel action.
- **SC-004**: At least one scheduled or workflow-driven calendar task can inspect or
  mutate calendar state and deliver its final result through the shared background-result
  delivery path without requiring an active chat session.
- **SC-005**: In manual or fixture-based validation, an operator can determine within 2
  minutes whether a failed calendar outcome was caused by connection readiness, stale or
  conflicting event state, or downstream delivery failure using operator-visible surfaces
  only.

## Assumptions

- Phase 29 builds on the shared integration readiness and account-binding behavior from
  phase 27 instead of redefining connection lifecycle rules inside the calendar domain.
- Phase 29 reuses the shared delivery targets, preferences, and outcome history from
  phase 28 instead of creating a calendar-specific notification plane.
- Existing scheduled-task and workflow capabilities remain the trigger surface for
  background calendar work; this phase does not define a separate scheduling system.
- The first calendar slice focuses on inspectable availability plus truthful event
  create, update, and cancel behavior for personal calendar use.
- Recurring events may appear in inspection results, but recurring-event mutation is
  intentionally deferred beyond phase 29.
- All-day events may appear in inspection results, but all-day-event mutation is
  intentionally deferred beyond phase 29.
- Event writes in phase 29 cover base event fields only and do not require attendee
  invitation delivery, RSVP tracking, or other external participation workflows.
- Event writes in phase 29 target the bound account's primary calendar only; choosing
  among multiple calendars is deferred beyond this phase.
- Timed-event writes in phase 29 use the bound calendar account's primary timezone by
  default rather than the caller's local environment timezone.
- Single-operator environment behavior remains the default; multi-user tenancy remains
  out of scope for this phase.
