# Feature Specification: Calendar Attendee And RSVP Workflows

**Feature Branch**: `main`
**Created**: 2026-06-30
**Status**: Draft
**Phase / Roadmap**: Phase 61 — Roadmap 61 (Calendar attendee, invitation, and RSVP workflows)
**Upstream authority**: [docs/specs/046-calendar-attendee-and-rsvp-workflows.md](../../docs/specs/046-calendar-attendee-and-rsvp-workflows.md)
**Provider decision (recorded during clarification)**: **Feishu/Lark Calendar** — continues the real provider closed in Roadmap 60 (spec 045).

## Overview

Roadmap 29 deliberately excluded attendee invitation, RSVP, and external-notification
semantics, and Roadmap 60 (spec 045) rejected attendee-bearing mutations as out of scope.
Roadmap 61 makes attendee-bearing calendar writes and RSVP state first-class, with explicit
provider side-effect truth: an attendee action distinguishes event-field mutation from the
externally-visible notification side effect, records the requested notification behavior and
the provider's result, applies approval/live-validation gates to externally-visible actions,
and links invitation delivery truthfully. Unsupported provider RSVP/notification behavior is
surfaced as an explicit unsupported diagnostic, never silently dropped.

This roadmap does not add recurring-series management, all-day mutation, travel planning, or
memory-driven participant context.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create a meeting with attendees and see invitation status (Priority: P1)

A user creates a timed event with attendees; the system records the attendee request, the
requested notification behavior, and the provider's per-attendee invitation result, and the
externally-visible invitation send is gated by approval.

**Independent Test**: Create an event with two attendees and `notify=true`; verify the
operation records an attendee operation class, each attendee's invitation status, and that
the externally-visible send required approval before any invitation left the system.

**Acceptance Scenarios**:

1. **Given** a connected calendar, **When** a user creates an event with attendees and
   requests notification, **Then** the event is created, attendees are recorded with
   invitation status, and the externally-visible invitation send is gated by approval.
2. **Given** an attendee create with `notify=false`, **When** it runs, **Then** attendees are
   added with no invitation side effect and the recorded notification behavior reflects that.
3. **Given** a provider that cannot send invitations for some attendee, **When** the action
   runs, **Then** that attendee's invitation result is recorded as failed/blocked with a
   stable diagnostic, without corrupting the event-field mutation.

### User Story 2 - Update attendees with explicit notification behavior (Priority: P2)

A user adds or removes attendees on an existing event, choosing whether invitations/updates
are sent, each recorded as a distinct attendee operation with explicit notification behavior.

**Acceptance Scenarios**:

1. **Given** an event with attendees, **When** a user adds an attendee with notification,
   **Then** an attendee-update operation records the added attendee and the invitation result.
2. **Given** an event with attendees, **When** a user removes an attendee with notification,
   **Then** a cancellation/notification side effect is recorded distinctly from event-field edits.
3. **Given** a request to mutate attendees and event fields together, **When** it runs, **Then**
   the field mutation and the attendee notification side effect are recorded as distinguishable
   facts (not merged into one opaque success).

### User Story 3 - Inspect RSVP and reconcile ambiguous invitations (Priority: P3)

An operator inspects RSVP state where the provider exposes it, and reconciles an ambiguous
invitation commit with evidence.

**Acceptance Scenarios**:

1. **Given** a provider that exposes RSVP, **When** RSVP is inspected, **Then** each attendee's
   response state (needs_action/accepted/declined/tentative) is projected onto the resource.
2. **Given** a provider that does not expose RSVP for an action, **When** it is requested,
   **Then** an explicit unsupported diagnostic is returned (not a false-empty result).
3. **Given** an ambiguous invitation commit, **When** the operation completes, **Then** it is
   recorded as ambiguous with reconciliation evidence, not coerced to success or failure.

### Edge Cases

- Partial attendee failure: event field mutation succeeds but one attendee invitation fails —
  both facts are recorded; no silent drop.
- Provider lacks notification control: requested `notify` behavior cannot be honored — recorded
  as unsupported, not silently applied or ignored.
- Approval denied: an externally-visible attendee action without approval performs no invitation
  side effect and is recorded as blocked.
- RSVP unsupported by provider: returns explicit unsupported diagnostic.
- Ambiguous invitation commit: recorded ambiguous with evidence.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST distinguish event-field mutation from attendee notification side
  effects, recording them as distinguishable facts on the single operation ledger.
- **FR-002**: Attendee actions MUST record the operation class, the requested notification
  behavior, the per-attendee provider result, and ambiguity status.
- **FR-003**: RSVP state MUST be inspectable where provider evidence exists, projected onto the
  existing calendar resources (no parallel shape).
- **FR-004**: Approval and live-validation gates MUST apply to externally-visible attendee
  actions (invitation send/update/cancel) before any side effect.
- **FR-005**: Unsupported provider RSVP/notification features MUST return an explicit unsupported
  diagnostic, never a silent drop or false-empty result.
- **FR-006**: Attendee delivery and provider-permission failures MUST map to the existing stable
  integration diagnostics reasons with redaction and retry-safety.
- **FR-007**: Ambiguous invitation commits MUST be recorded as ambiguous with reconciliation
  evidence, consistent with Roadmap 40 live-validation/replay.
- **FR-008**: Existing non-attendee calendar writes MUST remain compatible and unchanged; new
  attendee/RSVP fields MUST be additive and backward compatible.
- **FR-009**: No credential/token material MUST be exposed in any attendee operation, diagnostic,
  artifact, or delivery record.

### Key Entities

- **Attendee Request**: requested attendees with role and the requested notification behavior.
- **Attendee Operation**: a recorded attendee add/update/remove + invitation action on the
  single operation ledger, with per-attendee results and ambiguity status.
- **RSVP Projection**: per-attendee response state where the provider exposes it.
- **Attendee Diagnostic Evidence**: redacted provider permission/delivery failure classified
  into the existing diagnostics reason vocabulary.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility**: Additive. Attendee/RSVP fields are optional additions to the existing
  calendar event/operation resources; existing timed single-event writes are unchanged.
- **Migration / Rollback**: No data migration; attendee fields default empty. Rollback is not
  requesting attendee mutations (or fake backend).
- **Verification**: fake-backend attendee create/update/cancel + RSVP inspection tests;
  live-validation tests for invitation side effects + ambiguous commits; provider-adapter tests
  for attendee payloads; API/schema tests for attendee resources; real-account smoke where safe.
- **Observability**: reuse calendar operation events extended with attendee/notification facts;
  reuse existing diagnostics reason vocabulary; no new event families.

## Success Criteria *(mandatory)*

- **SC-001**: Attendee-bearing create/update/remove record event-field mutation and per-attendee
  notification side effects as distinguishable facts (zero silent drops).
- **SC-002**: 100% of externally-visible attendee actions pass through approval/live-validation
  before any invitation side effect.
- **SC-003**: RSVP state is projected where the provider exposes it; unsupported cases return an
  explicit unsupported diagnostic.
- **SC-004**: Ambiguous invitation commits are recorded ambiguous with reconciliation evidence.
- **SC-005**: Existing non-attendee calendar tests remain green; attendee fields are additive.

## Assumptions

- Provider is Feishu/Lark (continues Roadmap 60). RSVP exposure depends on provider capability.
- Roadmap 40 live-validation/replay and Roadmap 28 delivery are available for gating and linkage.
- Attendee mutations target timed single events on the primary calendar (recurrence/all-day stay
  out of scope for Roadmap 62).
