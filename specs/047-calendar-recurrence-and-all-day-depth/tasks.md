# Tasks: Calendar Recurrence And All-Day Depth

**Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md) | **Roadmap**: 62

Stories: US1 all-day, US2 occurrence vs series, US3 recurrence inspection + identities.

## Phase 1: Setup
- [X] T001 [Setup] Baseline green; confirm all-day/recurrence rejection points (manager+api).

## Phase 2: Foundational (model)
- [X] T002 [Foundational] types.go: RecurrenceScope (this_occurrence/this_and_following/
  entire_series); Event.SeriesID/OccurrenceID/OriginalStartsAt/RecurrenceRule/StartDate/EndDate;
  inputs gain RecurrenceRule + RecurrenceScope + StartDate/EndDate. All additive.
- [X] T003 [Foundational] manager: allow all-day+recurring; require scope on recurring
  update/cancel (reject ambiguous); record original+resulting identities; keep alternate-cal reject.
- [X] T004 [Foundational] backend.go: recurrence-scope validation helper + ErrCalendarRecurrenceScopeRequired.

## Phase 3: US1 — all-day
- [X] T005 [US1] fake_backend: all-day events with date boundaries (timezone preserved).
- [X] T006 [US1] feishulark: all-day create/update via start_time.date; read all-day flag.
- [X] T007 [P] [US1] tests: all-day create/inspect preserves date boundaries across DST.

## Phase 4: US2 — occurrence vs series
- [X] T008 [US2] fake_backend + feishulark: occurrence vs series update/cancel by scope.
- [X] T009 [US2] manager records scope + occurrence/series identity per mutation.
- [X] T010 [P] [US2] tests: this_occurrence vs entire_series distinct; ambiguous scope rejected.

## Phase 5: US3 — inspection + unsupported + identities
- [X] T011 [US3] feishulark: read recurrence rule + series/occurrence identity; unsupported
  provider recurrence -> explicit unsupported diagnostic, no partial mutation.
- [X] T012 [US3] operation history preserves original + resulting provider identities.
- [X] T013 [P] [US3] tests: recurrence inspected; unsupported returns explicit diagnostic;
  identities preserved.

## Phase 6: API + polish
- [X] T014 [API] api/calendar.go + types.go: accept/expose recurrence+all-day; relax reject to
  keep only alternate-calendar; map scope.
- [X] T015 [Polish] schemas: additive recurrence/all-day fields + scope enum; contract test.
- [X] T016 [Polish] verify: build/vet/test calendar, feishulark, api, integrations, contracts;
  timed-event suite green.

## Dependencies
T002 blocks all. T005/T006 before T007. T008/T009 before T010. T011 before T013.
</content>
