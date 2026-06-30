# Implementation Plan: Calendar Recurrence And All-Day Depth

**Branch**: `main` | **Spec**: [spec.md](./spec.md) | **Upstream**: [docs/specs/047-calendar-recurrence-and-all-day-depth.md](../../docs/specs/047-calendar-recurrence-and-all-day-depth.md)
**Phase / Roadmap**: Phase 62 — Roadmap 62

## Summary

Make all-day and recurring calendar events first-class, additively, on the existing calendar
domain and the Feishu/Lark provider. Model event kind, series identity, occurrence identity,
and all-day date boundaries; require an explicit recurrence scope (this_occurrence /
this_and_following / entire_series) for recurring mutations; map provider recurrence/all-day
limitations to explicit unsupported diagnostics with no partial mutation; preserve original +
resulting provider identities in operation history. Timed-event behavior is unchanged.

## Technical Context

- **Language**: Go 1.24 (daemon). Additive API request/response + schema fields.
- **Dependencies**: `internal/calendar` (model/manager/backend/fake/artifacts), `feishulark`
  provider (RRULE + all-day date mapping + occurrence/series ops), `internal/integrations`
  diagnostics (unsupported reason — already wired), `internal/api` calendar handlers + schemas.
- **Storage**: none new — recurrence/all-day ride as optional additive fields.
- **Testing**: fake-backend all-day + recurrence create/update/cancel incl. occurrence vs
  series; provider recurrence payload mapping; unsupported diagnostics; DST date-boundary;
  existing timed-event suite green.
- **Constraints**: additive + backward compatible; recurrence scope required for recurring
  targets; no partial mutation on unsupported; no credential leakage.

## Constitution Check
- **Roadmap closure**: meets upstream DoD — common all-day + recurrence requests are inside the
  public-product capability set or fail with precise reasons.
- **Production-grade**: explicit scope, no-partial-mutation safety, identity preservation,
  unsupported diagnostics.
- **Contracts first**: additive recurrence/all-day fields validated by contract tests.
- **Verification**: fake + provider + manager + diagnostics coverage; timed-event unchanged.
- **Environment**: fake default; real provider only with explicit operator credentials.

## Project Structure
```
specs/047-calendar-recurrence-and-all-day-depth/  spec.md plan.md tasks.md checklists/
daemon/internal/calendar/types.go        # EDIT: RecurrenceScope, Event.SeriesID/OccurrenceID/
                                         #       OriginalStartsAt/RecurrenceRule/StartDate/EndDate;
                                         #       inputs gain RecurrenceRule + RecurrenceScope + date
daemon/internal/calendar/manager.go      # EDIT: allow all-day/recurring; require scope; identities
daemon/internal/calendar/backend.go      # EDIT: validate helper for recurrence scope
daemon/internal/calendar/fake_backend.go # EDIT: all-day + recurring + occurrence/series
daemon/internal/integrations/providers/feishulark/calendar.go  # EDIT: RRULE + all-day + scope
daemon/internal/api/calendar.go, types.go  # EDIT: accept/expose recurrence+all-day; relax reject
schemas/api  # additive recurrence/all-day fields
```

## Complexity Tracking
No violations. All recurrence/all-day additions are optional fields behind the existing seams.
</content>
