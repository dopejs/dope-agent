# Implementation Plan: Calendar Integration

**Branch**: `014-calendar-integration` | **Date**: 2026-04-22 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/014-calendar-integration/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Add a first-class daemon-owned calendar domain on top of the shared integrations,
scheduler, workflow, runtime, and delivery planes. The design closes roadmap 29 by
introducing a dedicated `daemon/internal/calendar` package that can inspect calendar
account projections, list or inspect events, evaluate busy/free windows, and create,
update, or cancel single timed events on the bound account's primary calendar while
preserving integration binding truth, event identity, operator-visible artifacts, and
background-result delivery. Verification stays in `DOPE_ENV=test` by extending the
repo-owned fake integration backend into a deterministic fake calendar path rather than
requiring live third-party calendar accounts.

## Technical Context

**Language/Version**: Go 1.24.0; Markdown docs; JSON Schema contracts  
**Primary Dependencies**: `daemon/internal/api`, new `daemon/internal/calendar`,
`daemon/internal/app`, `daemon/internal/runtime`, `daemon/internal/orchestration`,
`daemon/internal/scheduler`, `daemon/internal/integrations`,
`daemon/internal/delivery`, `daemon/internal/policy`, `daemon/internal/events`,
`daemon/internal/store`, `daemon/internal/contracts`, existing auth wiring, and the
existing repo-owned fake integration backend extended for calendar-domain verification  
**Storage**: SQLite daemon state with additive `calendar_accounts`,
`calendar_operations`, and `calendar_artifacts` persistence plus additive workflow-step
and tool-call projection support for calendar-operation summaries; no binary blob storage
required in phase 29  
**Testing**: `go test ./internal/calendar ./internal/api ./internal/store ./internal/app ./internal/runtime ./internal/orchestration ./internal/scheduler ./internal/integrations ./internal/delivery ./internal/policy ./internal/contracts`,
`make daemon-contract-test`, targeted calendar-route and workflow regressions, and one
manual `DOPE_ENV=test` walkthrough using the fake calendar backend plus `test_sink`
delivery  
**Target Platform**: macOS/Linux local daemon in `DOPE_ENV=test` by default, using the
existing localhost HTTP API, SQLite store, and operator-authenticated `/v1/*` control
plane  
**Project Type**: Go daemon and harness control-plane service with schema-backed HTTP
and event contracts  
**Performance Goals**: inspect account projection, event detail, or busy/free truth from
persisted local state in `<=500 ms`; persist and project one calendar operation result in
`<=1 s` after backend completion on local test hardware; deliver a background calendar
result through the shared delivery plane in `<=2 s` after the operation reaches a
terminal state excluding connector latency. These are local validation targets for phase
29 and require an explicit latency-verification task rather than being treated as an
implicit connector SLA.  
**Constraints**: roadmap 29 MUST reuse phase 27 integration readiness and canonical
default semantics plus phase 28 delivery targets and outcome history; availability
lookup, event inspection, and event mutation remain separate operation classes; mutation
scope is limited to single timed events on the bound account's primary calendar; all-day
events and recurring events are inspectable but not mutable; attendee invitation, RSVP,
and external notification semantics remain out of scope; timed-event writes use the bound
calendar account's primary timezone by default; background calendar work must run through
normal runtime and workflow execution paths; operator-visible artifacts and audit truth
must stay separate from delivery truth; existing non-calendar behavior remains backward
compatible  
**Scale/Scope**: one operator-managed daemon, low tens of calendar integrations and
calendar-account projections, low hundreds of event inspections or mutations per day, one
repo-owned fake calendar backend plus one shared delivery sink sufficient to close
roadmap 29 without live external dependencies

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Roadmap closure: PASS. This plan closes roadmap 29 only: inspectable calendar account
  projection, event inspection, busy/free lookup, single timed-event mutation, normal
  schedule/workflow execution, operator-visible artifacts, and shared delivery reuse.
  Mail, reminders, attendee workflows, recurring-event mutation, and travel or meeting
  summarization remain out of scope.
- Production-grade change control: PASS. The design adds a dedicated
  `daemon/internal/calendar` plane and additive API, schema, event, runtime-projection,
  and store changes rather than refactoring scheduler, delivery, or integrations
  broadly. Rollback is a single change-set revert of calendar-specific routes,
  persistence, projections, and docs while leaving phase 27 and 28 behavior intact.
- Contracts and auditability: PASS. The plan names additive calendar account, event,
  availability, operation, artifact, and background-delivery surfaces together with the
  schema, event, and doc updates required to keep account selection, event identity,
  timezone truth, and delivery linkage inspectable.
- Verification and observability: PASS. The design requires targeted package, contract,
  schedule/workflow, and fake-backend regressions plus one manual `DOPE_ENV=test`
  walkthrough. Operator-visible account, operation, artifact, and delivery resources
  replace backend guesswork or raw provider logs as the source of truth.
- Environment and secrets: PASS. Local planning and later verification stay in
  `DOPE_ENV=test`; the repo-owned fake calendar backend avoids live calendar credentials;
  any real connector or token use remains optional, operator-owned, redacted, and
  environment-scoped.

Post-design re-check:

- PASS. The design remains roadmap-closed to the first calendar slice and does not drift
  into mail, reminders, attendee orchestration, recurring-event mutation, or travel
  products.
- PASS. Calendar execution remains on the existing runtime, workflow, schedule, and
  delivery planes with additive domain records rather than a second execution ledger.
- PASS. Integration readiness, calendar execution, and delivery outcomes remain distinct
  truths, which preserves operator auditability under partial failure.

## Project Structure

### Documentation (this feature)

```text
specs/014-calendar-integration/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── calendar-domain-surfaces.md
└── tasks.md
```

### Source Code (repository root)

```text
daemon/
├── internal/
│   ├── api/
│   ├── app/
│   ├── calendar/
│   ├── contracts/
│   ├── delivery/
│   ├── events/
│   ├── integrations/
│   ├── orchestration/
│   ├── policy/
│   ├── runtime/
│   ├── scheduler/
│   └── store/
└── go.mod

schemas/
├── api/
└── events/

docs/
├── harness/
└── runtime/

AGENTS.md
```

**Structure Decision**: Keep calendar account projection, backend abstraction, event and
availability reads, timed-event mutation rules, artifact snapshots, and fake-backend
verification support in a new `daemon/internal/calendar` package. `daemon/internal/api`
exposes additive calendar account, event, availability, and operation routes.
`daemon/internal/runtime`, `daemon/internal/orchestration`, and
`daemon/internal/scheduler` remain the owners of run, workflow, and schedule truth while
gaining additive calendar-operation linkage. `daemon/internal/integrations` continues to
own readiness and canonical-default account binding; `daemon/internal/delivery` remains
the owner of background-result routing. `daemon/internal/store` persists calendar-domain
documents and linkage indexes. `schemas/` and `docs/` carry the new contract and
operator-guidance surfaces. `AGENTS.md` should point at this plan for downstream task
generation.

## Complexity Tracking

No constitution violations remain. The design avoids reusing integration probes as the
calendar domain API, avoids collapsing delivery truth into calendar operation results,
avoids live third-party calendar dependencies for roadmap closure, and avoids premature
support for recurring events, all-day events, attendee workflows, or multi-calendar
mutation.

## Implementation Notes

- Add daemon-owned calendar resources rather than deriving calendar truth ad hoc from
  integration probes or live backend reads at response time.
- Reuse an explicit request-scoped `integrationId` when provided on calendar reads or
  writes; otherwise resolve the healthy or degraded canonical default integration for the
  calendar account and surface that choice in calendar-operation truth.
- Persist calendar account projection separately from raw integration readiness so the
  domain can expose primary-calendar and primary-timezone metadata without redefining
  phase 27 resource ownership.
- Treat calendar work as explicit operation classes: `list_events`, `get_event`,
  `busy_free`, `create_event`, `update_event`, and `cancel_event`. Each operation stores
  source linkage, event identity when present, timezone used, and terminal truth.
- Emit structured calendar artifacts for event snapshots on `list_events`, `get_event`,
  `create_event`, `update_event`, and `cancel_event` whenever backend event state is
  observed, and emit availability summary artifacts for `busy_free`. Requests blocked
  before any backend state is observed do not need artifacts.
- Keep fake calendar verification intentionally deterministic: one healthy account
  projection, one primary calendar, timed single events only, background workflow support,
  and reproducible stale-state or conflict scenarios are sufficient to close roadmap 29.

## Automated Verification

- `cd daemon && go test ./internal/calendar ./internal/api ./internal/store ./internal/app ./internal/runtime ./internal/orchestration ./internal/scheduler ./internal/integrations ./internal/delivery ./internal/policy ./internal/contracts`
- `make daemon-contract-test`
- `cd daemon && go test ./internal/api -run 'TestCalendarAccountRoutesProjectPrimaryTimezoneAndCalendar|TestCalendarRoutesSeparateBusyFreeFromMutation|TestCalendarRoutesCreateUpdateCancelTimedPrimaryCalendarEvents|TestScheduledCalendarWorkflowRoutesThroughDeliveryAndPreservesOperationTruth' -count=1`
- `cd daemon && go test ./internal/calendar -run 'TestFakeCalendarBackendReturnsPrimaryCalendarProjection|TestCalendarManagerRejectsRecurringAndAllDayMutation|TestCalendarManagerPreservesEventIdentityAcrossUpdateAndCancel|TestCalendarManagerRecordsConflictAndStaleStateTruth' -count=1`
- manual latency capture in `DOPE_ENV=test` for one explicit-`integrationId` read, one
  canonical-default read, one mutation, and one delivery-linked background run against
  the fake calendar backend to confirm the local latency targets recorded above

These commands are expected to cover:

- calendar account projection from integration readiness and canonical-default binding
- event list/detail inspection and busy/free lookup as separate operation classes
- timed single-event create, update, and cancel behavior on the primary calendar only
- rejection of recurring, all-day, attendee, and alternate-calendar mutation requests
- additive linkage from calendar operations to runs, workflow steps, schedules, and
  delivery outcomes
- fake-backend restart safety for persisted calendar account, operation, and artifact
  truth

## Manual Verification

- `make daemon-run-test`
- `make daemon-test-status`
- pair or reuse a local bearer token
- register one fake calendar integration, promote it as canonical default, and mark it
  healthy
- inspect `/v1/calendar/accounts` to confirm primary calendar and primary timezone truth
- create one timed event, inspect it, move it, cancel it, and verify the operation and
  artifact records reflect the same event identity
- run one busy/free lookup that does not create or change an event
- configure one `test_sink` delivery target and preference, then execute a scheduled or
  workflow-driven calendar task and confirm the background result links both the calendar
  operation and delivery outcome truth
- attempt a recurring-event mutation, all-day-event mutation, attendee-bearing write, and
  alternate-calendar write to confirm the system rejects each honestly

## Recorded Verification: 2026-04-23

Automated commands completed successfully:

- `cd daemon && go test ./internal/calendar ./internal/api ./internal/store ./internal/app ./internal/runtime ./internal/orchestration ./internal/scheduler ./internal/integrations ./internal/delivery ./internal/policy ./internal/contracts`
- `cd daemon && go test ./internal/api ./internal/store ./internal/contracts`
- `make daemon-contract-test`

Manual `DOPE_ENV=test` walkthrough completed against the repo-owned fake backend with a
workspace-local data dir and deterministic executable skill:

- explicit account projection, explicit event read, canonical-default event read, and
  busy/free lookup all returned truthful primary-calendar and primary-timezone metadata
- timed single-event create, update, and cancel preserved stable external event identity
  and recorded structured artifacts
- unsupported all-day and attendee-bearing writes returned explicit `400` errors
- one scheduled workflow completed with shared delivery outcome truth on `test_sink`
- one scheduled workflow carrying an inline `calendarAction` created a real calendar
  operation and projected `calendarOperationSummaries` onto workflow steps and schedule
  attempts plus `calendarOperationIds` onto the delivery outcome

Recorded local latency observations from that walkthrough:

- explicit account read: `7.726 ms`
- explicit event list: `8.721 ms`
- canonical-default event list: `7.580 ms`
- availability query: `6.969 ms`
- create event: `5.400 ms`
- update event: `3.941 ms`
- cancel event: `4.471 ms`
- background-linked create event: `7.379 ms`
- projected workflow/schedule/delivery reads: `5.656 ms` to `7.118 ms`

These observations stayed inside the phase-29 local targets of `<=500 ms` for read
surfaces, `<=1 s` for mutation persistence and projection, and `<=2 s` for
delivery-linked background visibility.

## Residual Risks

- A fake backend can close domain truth and workflow linkage, but it will not reveal
  third-party API quirks around organizer permissions, provider-specific conflict rules,
  or eventual consistency on real calendar systems.
- Timezone and primary-calendar defaults are intentionally narrow in phase 29; later
  multi-calendar or cross-timezone features may still require additive schema and route
  work.
- Structured artifact snapshots improve auditability but may still need later compaction
  or retention policy once calendar volume grows beyond the single-operator scope.
- Background workflow delivery depends on accurately identifying terminal calendar
  outcomes. If workflow linkage is too coarse, operators may see duplicate or missing
  delivery results even when the underlying calendar operation succeeded.

## Rollback Notes

- Rollback is a single change-set revert of calendar-specific routes, managers,
  persistence tables, projections, schemas, and docs.
- Phase 27 integrations, phase 28 delivery, and roadmap 25 schedule/workflow behavior
  remain valid if roadmap 29 is reverted because the design is additive.
- If rollback occurs after calendar-domain records exist, preserve calendar account,
  operation, artifact, and delivery-linkage rows as read-only audit truth even if the
  routes are disabled.
