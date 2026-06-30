# Feature Specification: Calendar Recurrence And All-Day Depth

**Feature Branch**: `main`
**Created**: 2026-06-30
**Status**: Draft
**Phase / Roadmap**: Phase 62 — Roadmap 62
**Upstream authority**: [docs/specs/047-calendar-recurrence-and-all-day-depth.md](../../docs/specs/047-calendar-recurrence-and-all-day-depth.md)
**Provider decision**: **Feishu/Lark Calendar** (continues Roadmap 60/61).

## Overview

Roadmaps 29/60/61 supported timed single-event mutation (plus attendees). All-day and
recurring events were rejected as explicit unsupported cases. Roadmap 62 makes all-day and
recurring events first-class: inspectable and mutable with clear series identity, occurrence
identity, all-day date boundaries (timezone preserved), and explicit unsupported diagnostics
where the provider cannot honor a recurrence operation. Mutations state whether they target a
single occurrence, this-and-following, or the entire series. Operation history preserves both
the original and the resulting provider identities. Existing timed-event APIs stay compatible.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create and inspect all-day events (Priority: P1)

**Acceptance Scenarios**:
1. **Given** a connected calendar, **When** a user creates an all-day event, **Then** it is
   created with date boundaries (timezone semantics preserved) and surfaced as an all-day event.
2. **Given** an all-day event spanning a DST boundary, **When** it is inspected, **Then** its
   date boundaries remain correct (no off-by-one from timezone math).

### User Story 2 - Update or cancel one occurrence vs the whole series (Priority: P2)

**Acceptance Scenarios**:
1. **Given** a recurring event, **When** a user updates a single occurrence, **Then** only that
   occurrence changes and the operation records occurrence identity + scope=this_occurrence.
2. **Given** a recurring event, **When** a user cancels the entire series, **Then** the series
   is cancelled and the operation records series identity + scope=entire_series.
3. **Given** a recurring event, **When** a user updates this-and-following, **Then** the
   operation records scope=this_and_following.
4. **Given** a provider that cannot honor a requested recurrence scope, **When** it is
   attempted, **Then** an explicit unsupported diagnostic is returned (no partial mutation).

### User Story 3 - Inspect recurrence rule and provider evidence (Priority: P3)

**Acceptance Scenarios**:
1. **Given** a recurring event, **When** inspected, **Then** the recurrence rule and series/
   occurrence identity are projected onto the existing resources.
2. **Given** any recurrence mutation, **When** it completes, **Then** operation history records
   the original provider identity and the resulting provider identity.

### Edge Cases
- All-day across DST: date boundaries preserved.
- Provider lacks single-occurrence edit: explicit unsupported diagnostic, no partial mutation.
- Series vs occurrence ambiguity: scope must be stated; default rejected if ambiguous.
- Mixed all-day + recurring event: both modeled.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST model event kind (timed/all-day), recurrence identity (series id),
  occurrence identity (occurrence id + original start), and all-day date boundaries.
- **FR-002**: Recurrence mutations MUST state scope (this_occurrence / this_and_following /
  entire_series); an ambiguous or missing scope on a recurring target MUST be rejected.
- **FR-003**: Provider recurrence/all-day limitations MUST map to explicit unsupported
  diagnostics (existing reason vocabulary), performing no partial mutation.
- **FR-004**: Operation history MUST preserve the original and resulting provider identities for
  recurrence mutations.
- **FR-005**: All-day events MUST preserve timezone/date-boundary semantics (no off-by-one).
- **FR-006**: Existing timed-event APIs MUST remain compatible; recurrence/all-day fields MUST
  be additive and backward compatible.
- **FR-007**: No credential/token material MUST be exposed in any operation/diagnostic/artifact.

### Key Entities
- **Recurring Event Series**: series identity + recurrence rule.
- **Event Occurrence**: occurrence identity + original start instant.
- **All-Day Event**: date-boundary representation with preserved timezone semantics.
- **Recurrence Mutation Scope**: this_occurrence / this_and_following / entire_series.

## Compatibility & Operational Impact *(mandatory)*
- **Compatibility**: Additive recurrence/all-day fields; timed-event behavior unchanged.
- **Migration / Rollback**: No migration; fields default empty/false. Rollback = not requesting
  recurrence/all-day.
- **Verification**: fake-backend all-day + recurrence create/update/cancel + occurrence/series;
  provider recurrence payload mapping; unsupported diagnostics; existing timed suite green.
- **Observability**: reuse calendar operation events extended with recurrence/all-day facts.

## Success Criteria *(mandatory)*
- **SC-001**: All-day create/inspect preserves date boundaries across DST (no off-by-one).
- **SC-002**: Occurrence vs series mutations record the stated scope and correct identities.
- **SC-003**: Unsupported provider recurrence operations return explicit unsupported diagnostics
  with no partial mutation.
- **SC-004**: Operation history preserves original + resulting provider identities.
- **SC-005**: Existing timed-event tests remain green; new fields additive.

## Assumptions
- Provider is Feishu/Lark; recurrence support depends on provider capability (RRULE).
- Recurrence scope semantics follow common calendar conventions.
