# Implementation Plan: Calendar Attendee And RSVP Workflows

**Branch**: `main` | **Spec**: [spec.md](./spec.md) | **Upstream**: [docs/specs/046-calendar-attendee-and-rsvp-workflows.md](../../docs/specs/046-calendar-attendee-and-rsvp-workflows.md)
**Phase / Roadmap**: Phase 61 — Roadmap 61

## Summary

Enable attendee-bearing calendar writes and RSVP, additively, on the existing calendar domain
and the Roadmap 60 Feishu/Lark provider. Distinguish event-field mutation from the
externally-visible attendee notification side effect, record per-attendee invitation results +
notification behavior + ambiguity on the single operation ledger, project RSVP state where the
provider exposes it, gate externally-visible attendee actions through the existing
live-validation/approval matrix, and surface unsupported provider RSVP/notification behavior as
an explicit diagnostic. Existing non-attendee writes are unchanged.

## Technical Context

- **Language**: Go 1.24 (daemon). Additive API request/response + schema fields.
- **Dependencies**: `internal/calendar` (model, manager, backend, fake, artifacts, live_validation),
  `internal/integrations/providers/feishulark` (provider attendee/RSVP mapping),
  `internal/livevalidation` (matrix already gates create per-action / update scope-level),
  `internal/api` (calendar handlers + request/response types), `internal/integrations`
  (diagnostics classifier — add RSVP/notification unsupported reason).
- **Storage**: none new — attendee details ride on the existing event/operation/artifact shapes
  as optional additive fields.
- **Testing**: fake-backend attendee create/update/remove + RSVP inspection; provider attendee
  payload + RSVP mapping over httptest; unsupported-notification diagnostic; ambiguous attendee
  commit; existing non-attendee suite stays green.
- **Constraints**: additive + backward compatible; no recurrence/all-day/alternate-calendar; no
  silent drops of attendee side effects; no credential/token leakage.

## Constitution Check

- **Roadmap closure**: meets upstream DoD — attendee + RSVP behavior is truthful, auditable, and
  gated for hosted use.
- **Production-grade**: explicit notification behavior, per-attendee results, approval gating,
  ambiguous-commit safety, redacted diagnostics.
- **Contracts first**: attendee/RSVP fields are additive on existing calendar resources; contract
  tests validate.
- **Verification**: fake + provider + manager + live-validation + diagnostics coverage; existing
  tests unchanged.
- **Environment**: fake backend default; real provider only with explicit operator credentials.

## Project Structure

```
specs/046-calendar-attendee-and-rsvp-workflows/  spec.md plan.md tasks.md checklists/
daemon/internal/calendar/
  types.go          # EDIT: Attendee, AttendeeRequest, RSVPStatus, InvitationStatus,
                    #       NotificationBehavior, AttendeeOutcome; Event.AttendeeDetails;
                    #       inputs (AttendeeRequests, NotifyAttendees); UpdateAttendeesInput;
                    #       OperationClassUpdateAttendees; Operation.AttendeeOutcome
  backend.go        # EDIT: Backend gains UpdateAttendees; allow attendees in validate
  manager.go        # EDIT: allow attendees; record AttendeeOutcome; UpdateAttendees method
  fake_backend.go   # EDIT: store attendees + simulate invitation/RSVP; UpdateAttendees
  artifacts.go      # EDIT: carry attendee snapshot on event artifact
  live_validation.go# EDIT: add update_attendees gated row
daemon/internal/integrations/providers/feishulark/calendar.go  # EDIT: attendee body + RSVP read + UpdateAttendees
daemon/internal/integrations/diagnostics_classifier.go         # EDIT: rsvp/notification unsupported reason
daemon/internal/api/calendar.go, types.go                      # EDIT: accept attendee detail; expose AttendeeDetails + outcome
schemas/api, schemas/events                                    # additive attendee fields
```

## Complexity Tracking

No violations. All additions are optional fields behind the existing calendar/provider seams.
The attendee notification side effect reuses the existing live-validation matrix gating.
</content>
